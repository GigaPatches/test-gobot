// Package units provides a unit conversion plugin for gobot.
//
// The `convert` command turns "<amount> <from> to <to>" into a result. The work
// is done by the internal/units engine, which every unit carries as a scale
// factor over a base unit plus a dimension vector. Because dimensions are
// exponent vectors, compound units fall out of the same arithmetic: km/h, m/s^2
// and ft^2 all convert without special cases. Temperature units are affine (they
// carry offsets, not just a scale), so they are only allowed as whole units.
package units

import (
	"fmt"
	"strings"

	"gobot/internal/plugin"
	"gobot/internal/units"
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
		return units.Listing()
	}
	return p.convert(strings.Join(args, " "))
}

// convert answers one conversion request, formatted as "from = to". The units
// are echoed as they were typed, so `convert 1 mile to metres` repeats both
// spellings back rather than canonicalising them.
func (p *Plugin) convert(line string) string {
	left, right, err := splitConversion(line)
	if err != nil {
		return "error: " + err.Error()
	}
	amount, fromText, err := splitAmount(left)
	if err != nil {
		return "error: " + err.Error()
	}
	from, err := units.Parse(fromText)
	if err != nil {
		return "error: " + err.Error()
	}
	to, err := units.Parse(right)
	if err != nil {
		return "error: " + err.Error()
	}
	out, err := from.ConvertTo(to, amount)
	if err != nil {
		return "error: " + err.Error()
	}
	return fmt.Sprintf("%s %s = %s %s", formatValue(amount), fromText, formatValue(out), right)
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
` + units.Listing()
