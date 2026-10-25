package units

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// separators are the words that split the two sides of a request; the arrows are
// handled separately because they are often typed without spaces.
var separators = map[string]bool{"to": true, "into": true, "as": true}

// splitConversion finds the two sides of a request. Arrows split the line even
// when typed without spaces; word separators split it on whole words. Without
// any separator the final word is taken as the target, so `convert 10 km mi`
// works too.
func splitConversion(line string) (string, string, error) {
	for _, arrow := range []string{"->", "=>", "→"} {
		i := strings.Index(line, arrow)
		if i < 0 {
			continue
		}
		left, right := strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+len(arrow):])
		if left == "" {
			return "", "", fmt.Errorf("nothing to convert")
		}
		if right == "" {
			return "", "", fmt.Errorf("nothing to convert to")
		}
		return left, right, nil
	}
	fields := strings.Fields(line)
	for i, field := range fields {
		if !separators[strings.ToLower(field)] {
			continue
		}
		if i == 0 {
			return "", "", fmt.Errorf("nothing to convert")
		}
		if i == len(fields)-1 {
			return "", "", fmt.Errorf("nothing to convert to")
		}
		return strings.Join(fields[:i], " "), strings.Join(fields[i+1:], " "), nil
	}
	if len(fields) < 3 {
		return "", "", errors.New(usage)
	}
	return strings.Join(fields[:len(fields)-1], " "), fields[len(fields)-1], nil
}

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

func isDigitByte(c byte) bool { return c >= '0' && c <= '9' }

// formatValue renders a result the way a calculator would: no trailing zeros,
// and enough rounding to hide float64 noise.
func formatValue(v float64) string {
	return strconv.FormatFloat(v, 'g', 12, 64)
}
