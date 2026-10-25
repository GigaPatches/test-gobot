package units

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// unitExpr is a parsed unit: the combined dimension vector and scale factor of
// every term, plus the offset when a single affine unit stands alone.
type unitExpr struct {
	dims   dims
	factor float64
	pre    float64
	post   float64
}

// parseUnitExpr reads a unit expression such as km/h, m/s^2, ft^2, ft2,
// "miles per hour" or kg*m/s^2. Juxtaposition, "*", "." and "·" multiply; "/"
// and the word "per" divide; "^" or trailing digits raise to a power.
func parseUnitExpr(text string) (unitExpr, error) {
	toks, err := tokenizeUnits(text)
	if err != nil {
		return unitExpr{}, err
	}
	if len(toks) == 0 {
		return unitExpr{}, fmt.Errorf("missing a unit")
	}
	expr := unitExpr{factor: 1}
	sign := 1
	terms := 0
	for i := 0; i < len(toks); {
		switch toks[i].kind {
		case tokSlash:
			if i == 0 {
				return unitExpr{}, fmt.Errorf("%q does not start with a unit", text)
			}
			if sign == -1 {
				return unitExpr{}, fmt.Errorf("repeated \"/\" in %q", text)
			}
			sign = -1
			i++
			continue
		case tokStar:
			if i == 0 {
				return unitExpr{}, fmt.Errorf("%q does not start with a unit", text)
			}
			sign = 1
			i++
			continue
		case tokNum:
			return unitExpr{}, fmt.Errorf("unexpected number %q in %q", toks[i].text, text)
		}

		name := strings.ReplaceAll(toks[i].text, "°", "")
		i++
		exp := 1
		switch {
		case i < len(toks) && toks[i].kind == tokCaret:
			i++
			if i >= len(toks) || toks[i].kind != tokNum {
				return unitExpr{}, fmt.Errorf("expected an exponent after \"^\" in %q", text)
			}
			exp, err = strconv.Atoi(toks[i].text)
			if err != nil {
				return unitExpr{}, fmt.Errorf("bad exponent %q", toks[i].text)
			}
			i++
		case i < len(toks) && toks[i].kind == tokNum:
			// ft2 means ft^2, the way it is typed on a keyboard.
			exp, err = strconv.Atoi(toks[i].text)
			if err != nil {
				return unitExpr{}, fmt.Errorf("bad exponent %q", toks[i].text)
			}
			i++
		}
		if exp == 0 {
			return unitExpr{}, fmt.Errorf("%q raised to the power of zero is not a unit", name)
		}

		u, prefix, ok := lookup(name)
		if !ok {
			return unitExpr{}, unknownUnit(name)
		}
		power := exp * sign
		if u.affine() && (terms > 0 || power != 1 || prefix != 1) {
			return unitExpr{}, fmt.Errorf("%s can only be converted on its own, not inside %q", name, text)
		}
		expr.dims = expr.dims.add(u.dims, power)
		expr.factor *= math.Pow(u.scale, float64(power)) * math.Pow(prefix, float64(power))
		if u.affine() {
			expr.pre, expr.post = u.pre, u.post
		}
		terms++
	}
	return expr, nil
}

func unknownUnit(name string) error {
	if hint := suggest(name); hint != "" {
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

// splitAmount peels a leading number off the left side of a conversion, so both
// "10 km" and "10km" work. It understands decimals and scientific notation, and
// stops before a lone "e" so "10eV" is ten electronvolts.
func splitAmount(s string) (float64, string, error) {
	s = strings.TrimSpace(s)
	i := 0
	if i < len(s) && (s[i] == '-' || s[i] == '+') {
		i++ // a leading sign, as in -40 F
	}
	for i < len(s) && (isDigitByte(s[i]) || s[i] == '.') {
		i++
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j < len(s) && isDigitByte(s[j]) {
			for j < len(s) && isDigitByte(s[j]) {
				j++
			}
			i = j
		}
	}
	if i == 0 {
		return 0, "", fmt.Errorf("expected an amount before %q", strings.TrimSpace(s))
	}
	text := s[:i]
	v, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, "", fmt.Errorf("%q is not a number", text)
	}
	return v, strings.TrimSpace(s[i:]), nil
}
