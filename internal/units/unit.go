package units

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// term is one named unit raised to an integer power inside a unit expression.
type term struct {
	name string
	exp  int
}

// Unit is a parsed unit expression: a product of named units with integer
// exponents, together with the dimension vector and scale they imply. The zero
// Unit is a plain, dimensionless number.
type Unit struct {
	// Dims is the dimension vector the terms add up to.
	Dims   Dims
	terms  []term // sorted by name, non-zero exponents
	factor float64
	pre    float64
	post   float64
	affine bool
}

// Plain is the dimensionless unit. Multiplying a quantity by it changes nothing,
// which is why the zero Unit has to scale by one rather than by nothing.
var Plain = Unit{factor: 1}

// IsEmpty reports a dimensionless unit.
func (u Unit) IsEmpty() bool { return len(u.terms) == 0 }

// scale is the factor that carries a value in this unit to base units. A Unit
// built as a zero value — the shape a caller reaches for when a number simply
// has no unit — scales by one.
func (u Unit) scale() float64 {
	if u.factor == 0 {
		return 1
	}
	return u.factor
}

// Affine reports a unit that carries an offset, such as a temperature scale.
// Offsets only make sense for a whole unit, never a compound one.
func (u Unit) Affine() bool { return u.affine }

// String renders the unit the way it would be written: km/h, kg*m/s^2, ft^2.
func (u Unit) String() string {
	var pos, neg []string
	for _, t := range u.terms {
		name := t.name
		exp := t.exp
		if exp < 0 {
			exp = -exp
		}
		if exp != 1 {
			name += "^" + strconv.Itoa(exp)
		}
		if t.exp < 0 {
			neg = append(neg, name)
		} else {
			pos = append(pos, name)
		}
	}
	switch {
	case len(pos) == 0 && len(neg) == 0:
		return ""
	case len(neg) == 0:
		return strings.Join(pos, "*")
	case len(pos) == 0 && len(neg) == 1:
		return "1/" + neg[0]
	case len(pos) == 0:
		return "1/(" + strings.Join(neg, "*") + ")"
	case len(neg) == 1:
		return strings.Join(pos, "*") + "/" + neg[0]
	default:
		return strings.Join(pos, "*") + "/(" + strings.Join(neg, "*") + ")"
	}
}

// ToBase converts a value measured in this unit into base units.
func (u Unit) ToBase(v float64) float64 { return (v+u.pre)*u.scale() + u.post }

// FromBase is the inverse of ToBase.
func (u Unit) FromBase(v float64) float64 { return (v-u.post)/u.scale() - u.pre }

// Times combines two units, as when quantities are multiplied. Multiplying by a
// dimensionless unit changes nothing, which is what lets a bare number carry a
// temperature scale through: 32 F keeps the offsets that make Fahrenheit F.
func (u Unit) Times(o Unit) Unit {
	if o.IsEmpty() {
		return u
	}
	if u.IsEmpty() {
		return o
	}
	return Unit{
		Dims:   u.Dims.add(o.Dims, 1),
		terms:  mergeTerms(u.terms, o.terms, 1),
		factor: u.scale() * o.scale(),
	}
}

// DividedBy combines two units, as when quantities are divided.
func (u Unit) DividedBy(o Unit) Unit {
	if o.IsEmpty() {
		return u
	}
	return Unit{
		Dims:   u.Dims.add(o.Dims, -1),
		terms:  mergeTerms(u.terms, o.terms, -1),
		factor: u.scale() / o.scale(),
	}
}

// Power scales a unit by an integer, as when a quantity is exponentiated.
func (u Unit) Power(n int) (Unit, error) {
	if u.affine {
		return Unit{}, fmt.Errorf("cannot raise %s to a power", u)
	}
	terms := make([]term, 0, len(u.terms))
	for _, t := range u.terms {
		terms = append(terms, term{t.name, t.exp * n})
	}
	return Unit{
		Dims:   u.Dims.scaleBy(n),
		terms:  terms,
		factor: math.Pow(u.scale(), float64(n)),
	}, nil
}

// Root takes an n-th root of a unit, which needs every exponent to divide
// evenly: sqrt(m^2) is m, sqrt(m) is not a unit.
func (u Unit) Root(n int) (Unit, error) {
	if n < 1 {
		return Unit{}, fmt.Errorf("%s has no %d-th root", u, n)
	}
	if u.affine {
		return Unit{}, fmt.Errorf("cannot take a root of %s", u)
	}
	terms := make([]term, 0, len(u.terms))
	for _, t := range u.terms {
		if t.exp%n != 0 {
			return Unit{}, fmt.Errorf("%s has no exact %d-th root", u, n)
		}
		terms = append(terms, term{t.name, t.exp / n})
	}
	return Unit{
		Dims:   u.Dims.dividedBy(n),
		terms:  terms,
		factor: math.Pow(u.scale(), 1/float64(n)),
	}, nil
}

// SameDims reports whether two units measure the same thing.
func (u Unit) SameDims(o Unit) bool { return u.Dims == o.Dims }

// SameScale reports two units that convert identically, so a value can move from
// one to the other untouched: 5 C and 3 C, 1 km and 3 kilometres.
func (u Unit) SameScale(o Unit) bool {
	return u.factor == o.factor && u.pre == o.pre && u.post == o.post
}

// ConvertTo re-expresses v from this unit in o, refusing a dimension mismatch.
func (u Unit) ConvertTo(o Unit, v float64) (float64, error) {
	if u.Dims != o.Dims {
		return 0, fmt.Errorf("cannot convert %s (%s) to %s (%s)", u, u.Dims, o, o.Dims)
	}
	return o.FromBase(u.ToBase(v)), nil
}

// BaseUnit builds the unit a dimension is expressed in, so a computed result can
// be printed even when no single table entry matches it.
func BaseUnit(d Dims) Unit {
	u := Unit{Dims: d, factor: 1}
	for _, b := range baseDims {
		if e := b.get(d); e != 0 {
			u.terms = append(u.terms, term{b.sym, e})
		}
	}
	sort.Slice(u.terms, func(i, j int) bool { return u.terms[i].name < u.terms[j].name })
	return u
}

// mergeTerms adds or subtracts two term lists, dropping terms that cancel out.
func mergeTerms(a, b []term, sign int) []term {
	sums := make(map[string]int, len(a)+len(b))
	for _, t := range a {
		sums[t.name] += t.exp
	}
	for _, t := range b {
		sums[t.name] += sign * t.exp
	}
	out := make([]term, 0, len(sums))
	for name, exp := range sums {
		if exp != 0 {
			out = append(out, term{name, exp})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// scaleBy multiplies every exponent by n.
func (d Dims) scaleBy(n int) Dims {
	return Dims{
		Length: d.Length * n, Mass: d.Mass * n, Time: d.Time * n,
		Temp: d.Temp * n, Data: d.Data * n, Angle: d.Angle * n,
	}
}

// dividedBy divides every exponent by n. Root only calls it once it knows each
// exponent divides evenly.
func (d Dims) dividedBy(n int) Dims {
	return Dims{
		Length: d.Length / n, Mass: d.Mass / n, Time: d.Time / n,
		Temp: d.Temp / n, Data: d.Data / n, Angle: d.Angle / n,
	}
}

// Literal builds the unit for a single spelling, as typed inside an expression:
// km, miles, GiB. The name is kept as written so a result can be printed the way
// it was asked for. It reports false when the name is not a unit, and for a
// prefixed temperature scale, whose offset would no longer mean anything.
func Literal(name string) (Unit, bool) {
	name = strings.ReplaceAll(name, "°", "")
	u, prefix, ok := lookup(name)
	if !ok || (u.affine() && prefix != 1) {
		return Unit{}, false
	}
	return literal(u, prefix, name), true
}

// literal is the shared shape of a single resolved unit, keeping the spelling
// that resolved to it.
func literal(u *unit, prefix float64, name string) Unit {
	return Unit{
		Dims:   u.dims,
		terms:  []term{{name, 1}},
		factor: u.scale * prefix,
		pre:    u.pre,
		post:   u.post,
		affine: u.affine(),
	}
}

// Parse reads a unit expression such as km/h, m/s^2, ft^2, ft2, "miles per
// hour" or kg*m/s^2. Juxtaposition, "*", "." and "·" multiply; "/" and the word
// "per" divide; "^" or trailing digits raise to a power.
func Parse(text string) (Unit, error) {
	toks, err := tokenizeUnits(text)
	if err != nil {
		return Unit{}, err
	}
	if len(toks) == 0 {
		return Unit{}, fmt.Errorf("missing a unit")
	}
	expr := Unit{factor: 1}
	sign := 1
	terms := 0
	for i := 0; i < len(toks); {
		switch toks[i].kind {
		case tokSlash:
			if i == 0 {
				return Unit{}, fmt.Errorf("%q does not start with a unit", text)
			}
			if sign == -1 {
				return Unit{}, fmt.Errorf("repeated \"/\" in %q", text)
			}
			sign = -1
			i++
			continue
		case tokStar:
			if i == 0 {
				return Unit{}, fmt.Errorf("%q does not start with a unit", text)
			}
			sign = 1
			i++
			continue
		case tokNum:
			return Unit{}, fmt.Errorf("unexpected number %q in %q", toks[i].text, text)
		}

		name := strings.ReplaceAll(toks[i].text, "°", "")
		i++
		exp := 1
		switch {
		case i < len(toks) && toks[i].kind == tokCaret:
			i++
			if i >= len(toks) || toks[i].kind != tokNum {
				return Unit{}, fmt.Errorf("expected an exponent after \"^\" in %q", text)
			}
			exp, err = strconv.Atoi(toks[i].text)
			if err != nil {
				return Unit{}, fmt.Errorf("bad exponent %q", toks[i].text)
			}
			i++
		case i < len(toks) && toks[i].kind == tokNum:
			// ft2 means ft^2, the way it is typed on a keyboard.
			exp, err = strconv.Atoi(toks[i].text)
			if err != nil {
				return Unit{}, fmt.Errorf("bad exponent %q", toks[i].text)
			}
			i++
		}
		if exp == 0 {
			return Unit{}, fmt.Errorf("%q raised to the power of zero is not a unit", name)
		}

		u, prefix, ok := lookup(name)
		if !ok {
			return Unit{}, unknownUnit(name)
		}
		power := exp * sign
		if u.affine() && (terms > 0 || power != 1 || prefix != 1) {
			return Unit{}, fmt.Errorf("%s can only be converted on its own, not inside %q", name, text)
		}
		expr = expr.Times(literal(u, prefix, name).scaled(power))
		if u.affine() {
			expr.pre, expr.post, expr.affine = u.pre, u.post, true
		}
		terms++
	}
	return expr, nil
}

// scaled raises a literal to a signed power while keeping its name.
func (u Unit) scaled(power int) Unit {
	if power == 1 {
		return u
	}
	terms := make([]term, 0, len(u.terms))
	for _, t := range u.terms {
		terms = append(terms, term{t.name, t.exp * power})
	}
	return Unit{
		Dims:   u.Dims.scaleBy(power),
		terms:  terms,
		factor: math.Pow(u.scale(), float64(power)),
	}
}

func unknownUnit(name string) error {
	if hint := Suggest(name); hint != "" {
		return fmt.Errorf("unknown unit %q, did you mean %q?", name, hint)
	}
	return fmt.Errorf("unknown unit %q", name)
}

type unitTokKind int

const (
	tokName unitTokKind = iota
	tokNum
	tokSlash
	tokStar
	tokCaret
)

type unitTok struct {
	kind unitTokKind
	text string
}

// tokenizeUnits splits a unit expression into names, exponents and operators.
// Unit names may contain letters from any script plus the degree sign, so "°C"
// and "kilometre" both scan cleanly.
func tokenizeUnits(s string) ([]unitTok, error) {
	var toks []unitTok
	for i := 0; i < len(s); {
		r, width := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && width <= 1:
			return nil, fmt.Errorf("invalid character in %q", s)
		case unicode.IsSpace(r):
			i += width
		case r == '/':
			toks = append(toks, unitTok{kind: tokSlash})
			i += width
		case r == '*' || r == '·' || r == '.':
			toks = append(toks, unitTok{kind: tokStar})
			i += width
		case r == '^':
			toks = append(toks, unitTok{kind: tokCaret})
			i += width
		case r == '-' && len(toks) > 0 && toks[len(toks)-1].kind == tokCaret:
			// A minus right after ^ negates the exponent: s^-1.
			j := i + width
			start := j
			for j < len(s) && isDigitByte(s[j]) {
				j++
			}
			if start == j {
				return nil, fmt.Errorf("expected digits after \"^\"")
			}
			toks = append(toks, unitTok{kind: tokNum, text: "-" + s[start:j]})
			i = j
		case unicode.IsLetter(r) || r == '°':
			j := i
			for j < len(s) {
				r2, w2 := utf8.DecodeRuneInString(s[j:])
				if !unicode.IsLetter(r2) && r2 != '°' {
					break
				}
				j += w2
			}
			text := s[i:j]
			if strings.EqualFold(text, "per") {
				toks = append(toks, unitTok{kind: tokSlash})
			} else {
				toks = append(toks, unitTok{kind: tokName, text: text})
			}
			i = j
		case r >= '0' && r <= '9':
			j := i
			for j < len(s) && isDigitByte(s[j]) {
				j++
			}
			toks = append(toks, unitTok{kind: tokNum, text: s[i:j]})
			i = j
		default:
			return nil, fmt.Errorf("unexpected character %q in a unit", string(r))
		}
	}
	return toks, nil
}

func isDigitByte(c byte) bool { return c >= '0' && c <= '9' }
