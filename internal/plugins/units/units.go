// Package units provides a unit conversion plugin for gobot.
//
// The `convert` command turns "<amount> <from> to <to>" into a result, where a
// unit is a scale factor over a base unit plus a dimension vector. Because
// dimensions are tracked as exponent vectors, compound units fall out of the
// same arithmetic: km/h, m/s^2 and ft^2 all convert without special cases.
// Temperature units are affine (they carry offsets, not just a scale), so they
// are only allowed as whole units.
package units

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gobot/internal/plugin"
)

// Plugin converts between units of measurement. It keeps no state: every
// conversion is answered from the unit table alone.
type Plugin struct{}

// New returns the converter plugin.
func New() *Plugin { return &Plugin{} }

// Name implements plugin.Plugin.
func (p *Plugin) Name() string { return "units" }

// Description implements plugin.Plugin.
func (p *Plugin) Description() string {
	return "convert between units, including compound ones like km/h and ft^2"
}

// Commands implements plugin.Plugin. The command is registered twice so that
// both `convert` and the shorter `units` reach it.
func (p *Plugin) Commands() []*plugin.Command {
	primary := &plugin.Command{
		Name:        "convert",
		Description: "convert an amount between units, e.g. `convert 10 km to mi`",
		Handler:     p.handle,
	}
	shortcut := &plugin.Command{
		Name:        "units",
		Description: "same as `convert`",
		Handler:     p.handle,
	}
	return []*plugin.Command{primary, shortcut}
}

var _ plugin.Plugin = (*Plugin)(nil)

func (p *Plugin) handle(args []string) string {
	if len(args) == 0 {
		return usage
	}
	switch strings.ToLower(args[0]) {
	case "help", "?":
		return helpText
	case "units", "list":
		return listing()
	}
	return p.convert(strings.Join(args, " "))
}

// convert answers one conversion request, formatted as "from = to".
func (p *Plugin) convert(line string) string {
	left, right, err := splitConversion(line)
	if err != nil {
		return "error: " + err.Error()
	}
	amount, fromText, err := splitAmount(left)
	if err != nil {
		return "error: " + err.Error()
	}
	from, err := parseUnitExpr(fromText)
	if err != nil {
		return "error: " + err.Error()
	}
	to, err := parseUnitExpr(right)
	if err != nil {
		return "error: " + err.Error()
	}
	if from.dims != to.dims {
		return fmt.Sprintf("error: cannot convert %s (%s) to %s (%s)",
			fromText, from.dims, right, to.dims)
	}
	base := (amount+from.pre)*from.factor + from.post
	out := (base-to.post)/to.factor - to.pre
	return fmt.Sprintf("%s %s = %s %s", formatValue(amount), fromText, formatValue(out), right)
}

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

// formatValue renders a result the way a calculator would: no trailing zeros,
// and enough rounding to hide float64 noise.
func formatValue(v float64) string {
	return strconv.FormatFloat(v, 'g', 12, 64)
}

const usage = "usage: convert <amount> <unit> to <unit> — try `convert help`"

// helpText documents the syntax and lists every known unit.
var helpText = `unit converter — convert <amount> <from> to <to>

examples:
  convert 10 km to mi
  convert 100 km/h to mph
  convert 32 F to C
  convert 5 L to cm^3
  convert 1 GiB to MB
  convert 2 ft^2 to in^2
  convert 1 stone to kg

syntax:
  separate the units with to, into, as, -> or →, or just name both:
    convert 10 km mi
  compound units divide with / or the word per, and multiply with *, a dot
    or plain juxtaposition: km/h, m/s^2, kg*m/s^2, ft^2 (also ft2), s^-1
  SI prefixes (k M G T m u n ...) and binary ones (Ki Mi Gi ...) are understood,
    abbreviated or spelled out, so MHz, mm, GiB, dam, kilometres and megabytes
    all work; names are case-sensitive (kB is a kilobyte, kb a kilobit), and
    plurals and spelled-out unit names are accepted: miles, feet, hectares
  temperature scales convert only as whole units, not inside a compound one

units by family:
` + listing()
