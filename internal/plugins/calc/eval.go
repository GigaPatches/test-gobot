package calc

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
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
func (p *Plugin) eval(src string) (float64, error) {
	pr, err := newParser(p, src)
	if err != nil {
		return 0, err
	}
	v, err := pr.parseStmt()
	if err != nil {
		return 0, err
	}
	if pr.peek().kind != tokEOF {
		return 0, fmt.Errorf("unexpected %s", describe(pr.peek()))
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
	text  string
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
// as a synonym for ^ while tokenizing.
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
			toks = append(toks, token{kind: tokIdent, text: strings.ToLower(s[i:end])})
			i = end
		case c == '*' && i+1 < len(s) && s[i+1] == '*':
			toks = append(toks, token{kind: tokOp, text: "^"})
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
//	stmt    := ident "=" expr | expr
//	expr    := term (("+" | "-") term)*
//	term    := unary (("*" | "/" | "%" | "mod" | implicit) unary)*
//	unary   := ("+" | "-") unary | power
//	power   := postfix ("^" unary)?          # right associative, -2^2 == -4
//	postfix := primary ("!")*
//	primary := number | constant | variable | func "(" args ")" | "(" expr ")"
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

func (pr *parser) parseStmt() (float64, error) {
	t := pr.peek()
	if t.kind == tokIdent && pr.toks[pr.pos+1].kind == tokOp && pr.toks[pr.pos+1].text == "=" {
		pr.next() // name
		pr.next() // "="
		if _, ok := constants[t.text]; ok {
			return 0, fmt.Errorf("cannot assign to constant %q", t.text)
		}
		if _, ok := funcs[t.text]; ok {
			return 0, fmt.Errorf("cannot assign to function %q", t.text)
		}
		v, err := pr.parseExpr()
		if err != nil {
			return 0, err
		}
		pr.p.vars[t.text] = v
		return v, nil
	}
	return pr.parseExpr()
}

func (pr *parser) parseExpr() (float64, error) {
	v, err := pr.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		switch {
		case pr.isOp("+"):
			pr.next()
			var rhs float64
			if rhs, err = pr.parseTerm(); err != nil {
				return 0, err
			}
			v += rhs
		case pr.isOp("-"):
			pr.next()
			var rhs float64
			if rhs, err = pr.parseTerm(); err != nil {
				return 0, err
			}
			v -= rhs
		default:
			return v, nil
		}
	}
}

func (pr *parser) parseTerm() (float64, error) {
	v, err := pr.parseUnary()
	if err != nil {
		return 0, err
	}
	for {
		var op string
		switch {
		case pr.isOp("*"), pr.isOp("/"), pr.isOp("%"):
			op = pr.next().text
		case pr.peek().kind == tokIdent && pr.peek().text == "mod":
			pr.next()
			op = "mod"
		case pr.startsPrimary():
			op = "*" // implicit multiplication: 2pi, 3(4), (1+2)(3+4)
		default:
			return v, nil
		}
		rhs, err := pr.parseUnary()
		if err != nil {
			return 0, err
		}
		switch op {
		case "*":
			v *= rhs
		case "/":
			if rhs == 0 {
				return 0, errors.New("division by zero")
			}
			v /= rhs
		case "%", "mod":
			if rhs == 0 {
				return 0, errors.New("modulo by zero")
			}
			v = math.Mod(v, rhs)
		}
	}
}

// startsPrimary reports whether the next token can begin a value, which is how
// the term rule spots implicit multiplication.
func (pr *parser) startsPrimary() bool {
	switch t := pr.peek(); t.kind {
	case tokNum, tokIdent:
		return true
	case tokOp:
		return t.text == "("
	default:
		return false
	}
}

func (pr *parser) parseUnary() (float64, error) {
	switch {
	case pr.isOp("-"):
		pr.next()
		v, err := pr.parseUnary()
		if err != nil {
			return 0, err
		}
		return -v, nil
	case pr.isOp("+"):
		pr.next()
		return pr.parseUnary()
	}
	return pr.parsePower()
}

func (pr *parser) parsePower() (float64, error) {
	base, err := pr.parsePostfix()
	if err != nil {
		return 0, err
	}
	if !pr.isOp("^") {
		return base, nil
	}
	pr.next()
	// The exponent is a unary so that 2^-3 parses and 2^3^2 stays right
	// associative.
	exp, err := pr.parseUnary()
	if err != nil {
		return 0, err
	}
	return math.Pow(base, exp), nil
}

func (pr *parser) parsePostfix() (float64, error) {
	v, err := pr.parsePrimary()
	if err != nil {
		return 0, err
	}
	for pr.isOp("!") {
		pr.next()
		if v, err = factorial(v); err != nil {
			return 0, err
		}
	}
	return v, nil
}

func (pr *parser) parsePrimary() (float64, error) {
	t := pr.next()
	switch t.kind {
	case tokNum:
		return t.value, nil
	case tokIdent:
		if fn, ok := funcs[t.text]; ok {
			return pr.parseCall(t.text, fn)
		}
		if v, ok := pr.p.vars[t.text]; ok {
			return v, nil
		}
		if v, ok := constants[t.text]; ok {
			return v, nil
		}
		return 0, fmt.Errorf("unknown name %q", t.text)
	case tokOp:
		if t.text == "(" {
			v, err := pr.parseExpr()
			if err != nil {
				return 0, err
			}
			if !pr.isOp(")") {
				return 0, errors.New(`missing ")"`)
			}
			pr.next()
			return v, nil
		}
	}
	return 0, fmt.Errorf("unexpected %s", describe(t))
}

func (pr *parser) parseCall(name string, fn function) (float64, error) {
	if !pr.isOp("(") {
		return 0, fmt.Errorf("expected \"(\" after %s", name)
	}
	pr.next()
	var args []float64
	if !pr.isOp(")") {
		for {
			v, err := pr.parseExpr()
			if err != nil {
				return 0, err
			}
			args = append(args, v)
			if !pr.isOp(",") {
				break
			}
			pr.next()
		}
	}
	if !pr.isOp(")") {
		return 0, fmt.Errorf(`missing ")" in %s(...)`, name)
	}
	pr.next()
	if len(args) < fn.min || (fn.max >= 0 && len(args) > fn.max) {
		return 0, fmt.Errorf("%s takes %s, got %d", name, fn.arity(), len(args))
	}
	return fn.fn(pr.p.mode, args)
}
