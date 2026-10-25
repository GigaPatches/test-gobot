package calc

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"gobot/internal/units"
)

// angleMode decides how trigonometric functions read their input and report
// their output.
type angleMode int

const (
	rad angleMode = iota
	deg
)

func (m angleMode) String() string {
	if m == deg {
		return "deg"
	}
	return "rad"
}

func (m angleMode) toRadians(x float64) float64 {
	if m == deg {
		return x * math.Pi / 180
	}
	return x
}

func (m angleMode) fromRadians(x float64) float64 {
	if m == deg {
		return x * 180 / math.Pi
	}
	return x
}

// eval evaluates src against the plugin's variables and angle mode. It neither
// locks the plugin nor records history: callers own both.
func (p *Plugin) eval(src string) (quantity, error) {
	pr, err := newParser(p, src)
	if err != nil {
		return quantity{}, err
	}
	v, err := pr.parseStmt()
	if err != nil {
		return quantity{}, err
	}
	if pr.peek().kind != tokEOF {
		return quantity{}, fmt.Errorf("unexpected %s", describe(pr.peek()))
	}
	return v, nil
}

// formatFloat renders v the way a calculator would: no trailing zeros, plain
// notation at ordinary magnitudes, exponent notation at the extremes, and just
// enough rounding to hide float64 noise, so 0.1+0.2 prints as 0.3.
func formatFloat(v float64) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "Inf"
	case math.IsInf(v, -1):
		return "-Inf"
	}
	return strconv.FormatFloat(v, 'g', 12, 64)
}

// --- tokenizer ---

type tokKind int

const (
	tokEOF tokKind = iota
	tokNum
	tokIdent
	tokOp
)

type token struct {
	kind  tokKind
	text  string // identifiers are folded to lowercase
	raw   string // an identifier as typed, which is how a unit is spelled
	value float64
}

// describe names a token for error messages.
func describe(t token) string {
	if t.kind == tokEOF {
		return "end of expression"
	}
	return strconv.Quote(t.text)
}

// operators are the single characters the grammar understands. ** is accepted
// as a synonym for ^ and -> as a synonym for the word to, while tokenizing.
const operators = "+-*/^%!(),="

func tokenize(s string) ([]token, error) {
	var toks []token
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '.' || (c >= '0' && c <= '9'):
			tok, width, err := scanNumber(s[i:])
			if err != nil {
				return nil, err
			}
			toks = append(toks, tok)
			i += width
		case isNameByte(c):
			end := i
			for end < len(s) && isNameByte(s[end]) {
				end++
			}
			toks = append(toks, token{kind: tokIdent, text: strings.ToLower(s[i:end]), raw: s[i:end]})
			i = end
		case c == '*' && i+1 < len(s) && s[i+1] == '*':
			toks = append(toks, token{kind: tokOp, text: "^"})
			i += 2
		case c == '-' && i+1 < len(s) && s[i+1] == '>':
			toks = append(toks, token{kind: tokOp, text: "->"})
			i += 2
		case strings.IndexByte(operators, c) >= 0:
			toks = append(toks, token{kind: tokOp, text: string(c)})
			i++
		default:
			r, _ := utf8.DecodeRuneInString(s[i:])
			return nil, fmt.Errorf("unexpected character %q", string(r))
		}
	}
	return append(toks, token{kind: tokEOF}), nil
}

// scanNumber reads a decimal literal, including an optional exponent, and
// reports how many bytes it consumed.
func scanNumber(s string) (token, int, error) {
	i := 0
	for i < len(s) && (s[i] == '.' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	// Only treat e as an exponent marker when a real exponent follows, so that
	// 2e3 is a number while 2e stays "2 * e".
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j < len(s) && s[j] >= '0' && s[j] <= '9' {
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			i = j
		}
	}
	text := s[:i]
	v, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return token{}, 0, fmt.Errorf("invalid number %q", text)
	}
	return token{kind: tokNum, text: text, value: v}, i, nil
}

func isNameByte(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}

// --- parser ---

// parser is a recursive-descent parser for this grammar, lowest precedence
// first:
//
//	stmt     := ident "=" conv | conv
//	conv     := expr [("to" | "->") expr]  # the right side supplies a target unit
//	expr     := term (("+" | "-") term)*
//	term     := product (("*" | "/" | "%" | "mod") product)*
//	product  := unary (unary)*             # implicit multiplication binds tighter
//	unary    := ("+" | "-") unary | power
//	power    := postfix ("^" unary)?       # right associative, -2^2 == -4
//	postfix  := primary ("!")*
//	primary  := number | unit | constant | variable | func "(" args ")" | "(" conv ")"
//
// A name that is none of the known functions, variables or constants is tried as
// a unit, which is what makes 10 km a quantity rather than an unknown name.
type parser struct {
	toks []token
	pos  int
	p    *Plugin
}

func newParser(p *Plugin, src string) (*parser, error) {
	toks, err := tokenize(src)
	if err != nil {
		return nil, err
	}
	return &parser{toks: toks, p: p}, nil
}

func (pr *parser) peek() token { return pr.toks[pr.pos] }

func (pr *parser) next() token {
	t := pr.toks[pr.pos]
	if t.kind != tokEOF {
		pr.pos++
	}
	return t
}

func (pr *parser) isOp(op string) bool {
	t := pr.peek()
	return t.kind == tokOp && t.text == op
}

func (pr *parser) parseStmt() (quantity, error) {
	t := pr.peek()
	if t.kind == tokIdent && pr.toks[pr.pos+1].kind == tokOp && pr.toks[pr.pos+1].text == "=" {
		pr.next() // name
		pr.next() // "="
		if _, ok := constants[t.text]; ok {
			return quantity{}, fmt.Errorf("cannot assign to constant %q", t.text)
		}
		if _, ok := funcs[t.text]; ok {
			return quantity{}, fmt.Errorf("cannot assign to function %q", t.text)
		}
		v, err := pr.parseConversion()
		if err != nil {
			return quantity{}, err
		}
		pr.p.vars[t.text] = v
		return v, nil
	}
	return pr.parseConversion()
}

// parseConversion is a whole expression plus the optional `to <unit>` that
// converts its result. The right-hand side is evaluated like any other
// expression and only its unit is used, so `to mph`, `to mi/h` and `to km/h` all
// work, and conversions chain: 10 km to mi to yd.
func (pr *parser) parseConversion() (quantity, error) {
	v, err := pr.parseExpr()
	if err != nil {
		return quantity{}, err
	}
	for pr.isConversion() {
		pr.next()
		target, err := pr.parseExpr()
		if err != nil {
			return quantity{}, err
		}
		if target.isPlain() {
			return quantity{}, fmt.Errorf("expected a unit after \"to\", got %s", target)
		}
		if v, err = v.in(target.unit); err != nil {
			return quantity{}, err
		}
	}
	return v, nil
}

// isConversion reports the start of a `to <unit>` tail.
func (pr *parser) isConversion() bool {
	t := pr.peek()
	return (t.kind == tokIdent && t.text == "to") || (t.kind == tokOp && t.text == "->")
}

func (pr *parser) parseExpr() (quantity, error) {
	v, err := pr.parseTerm()
	if err != nil {
		return quantity{}, err
	}
	for {
		switch {
		case pr.isOp("+"):
			pr.next()
			var rhs quantity
			if rhs, err = pr.parseTerm(); err != nil {
				return quantity{}, err
			}
			if v, err = v.add(rhs); err != nil {
				return quantity{}, err
			}
		case pr.isOp("-"):
			pr.next()
			var rhs quantity
			if rhs, err = pr.parseTerm(); err != nil {
				return quantity{}, err
			}
			if v, err = v.sub(rhs); err != nil {
				return quantity{}, err
			}
		default:
			return v, nil
		}
	}
}

func (pr *parser) parseTerm() (quantity, error) {
	v, err := pr.parseProduct()
	if err != nil {
		return quantity{}, err
	}
	for {
		var op string
		switch {
		case pr.isOp("*"), pr.isOp("/"), pr.isOp("%"):
			op = pr.next().text
		case pr.peek().kind == tokIdent && pr.peek().text == "mod":
			pr.next()
			op = "mod"
		default:
			return v, nil
		}
		rhs, err := pr.parseProduct()
		if err != nil {
			return quantity{}, err
		}
		switch op {
		case "*":
			v = v.mul(rhs)
		case "/":
			if v, err = v.div(rhs); err != nil {
				return quantity{}, err
			}
		case "%", "mod":
			if v, err = v.mod(rhs); err != nil {
				return quantity{}, err
			}
		}
	}
}

// parseProduct collects implicit multiplication, which binds tighter than the
// explicit operators: 100 km / 2 h divides 100 km by (2 h), the way it would be
// written on paper, and comes out as 50 km/h.
func (pr *parser) parseProduct() (quantity, error) {
	v, err := pr.parseUnary()
	if err != nil {
		return quantity{}, err
	}
	for pr.startsPrimary() {
		rhs, err := pr.parseUnary()
		if err != nil {
			return quantity{}, err
		}
		v = v.mul(rhs)
	}
	return v, nil
}

// startsPrimary reports whether the next token can begin a value, which is how
// the product rule spots implicit multiplication. The words "to" and "mod" are
// left out: they are operators, not factors.
func (pr *parser) startsPrimary() bool {
	switch t := pr.peek(); t.kind {
	case tokNum:
		return true
	case tokIdent:
		return t.text != "to" && t.text != "mod"
	case tokOp:
		return t.text == "("
	default:
		return false
	}
}

func (pr *parser) parseUnary() (quantity, error) {
	switch {
	case pr.isOp("-"):
		pr.next()
		v, err := pr.parseUnary()
		if err != nil {
			return quantity{}, err
		}
		v.value = -v.value
		return v, nil
	case pr.isOp("+"):
		pr.next()
		return pr.parseUnary()
	}
	return pr.parsePower()
}

func (pr *parser) parsePower() (quantity, error) {
	base, err := pr.parsePostfix()
	if err != nil {
		return quantity{}, err
	}
	if !pr.isOp("^") {
		return base, nil
	}
	pr.next()
	// The exponent is a unary so that 2^-3 parses and 2^3^2 stays right
	// associative.
	exp, err := pr.parseUnary()
	if err != nil {
		return quantity{}, err
	}
	return base.pow(exp)
}

func (pr *parser) parsePostfix() (quantity, error) {
	v, err := pr.parsePrimary()
	if err != nil {
		return quantity{}, err
	}
	for pr.isOp("!") {
		pr.next()
		if !v.isPlain() {
			return quantity{}, fmt.Errorf("factorial needs a plain number, not %s", v)
		}
		if v.value, err = factorial(v.value); err != nil {
			return quantity{}, err
		}
	}
	return v, nil
}

func (pr *parser) parsePrimary() (quantity, error) {
	t := pr.next()
	switch t.kind {
	case tokNum:
		return plain(t.value), nil
	case tokIdent:
		return pr.parseName(t)
	case tokOp:
		if t.text == "(" {
			v, err := pr.parseConversion()
			if err != nil {
				return quantity{}, err
			}
			if !pr.isOp(")") {
				return quantity{}, errors.New(`missing ")"`)
			}
			pr.next()
			return v, nil
		}
	}
	return quantity{}, fmt.Errorf("unexpected %s", describe(t))
}

// parseName resolves an identifier as a function call, a variable, a constant or
// a unit, in that order. A function name only counts as one when "(" follows,
// which is what lets deg, rad and min be units as well: deg(pi) is a call while
// 45 deg is an angle and 5 min is five minutes.
func (pr *parser) parseName(t token) (quantity, error) {
	name := t.text
	if fn, ok := funcs[name]; ok && pr.isOp("(") {
		return pr.parseCall(name, fn)
	}
	if v, ok := pr.p.vars[name]; ok {
		return v, nil
	}
	if v, ok := constants[name]; ok {
		return plain(v), nil
	}
	// Unit spellings are case-sensitive — kB is a kilobyte, kb a kilobit — so the
	// name as typed gets the first try, and the folded one is the fallback.
	for _, spelling := range []string{t.raw, name} {
		if unit, ok := units.Literal(spelling); ok {
			return quantity{value: 1, unit: unit}, nil
		}
	}
	if _, ok := funcs[name]; ok {
		return quantity{}, fmt.Errorf("expected \"(\" after %s", name)
	}
	return quantity{}, fmt.Errorf("unknown name %q", name)
}

func (pr *parser) parseCall(name string, fn function) (quantity, error) {
	if !pr.isOp("(") {
		return quantity{}, fmt.Errorf("expected \"(\" after %s", name)
	}
	pr.next()
	var args []quantity
	if !pr.isOp(")") {
		for {
			v, err := pr.parseConversion()
			if err != nil {
				return quantity{}, err
			}
			args = append(args, v)
			if !pr.isOp(",") {
				break
			}
			pr.next()
		}
	}
	if !pr.isOp(")") {
		return quantity{}, fmt.Errorf(`missing ")" in %s(...)`, name)
	}
	pr.next()
	if len(args) < fn.min || (fn.max >= 0 && len(args) > fn.max) {
		return quantity{}, fmt.Errorf("%s takes %s, got %d", name, fn.arity(), len(args))
	}
	vals, unit, err := fn.apply(pr.p, name, args)
	if err != nil {
		return quantity{}, err
	}
	v, err := fn.fn(pr.p.mode, vals)
	if err != nil {
		return quantity{}, err
	}
	return quantity{value: v, unit: unit}, nil
}
