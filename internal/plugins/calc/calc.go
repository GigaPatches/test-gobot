// Package calc provides a scientific calculator plugin for gobot.
//
// One command, `calc`, evaluates infix expressions through a hand-written
// tokenizer and recursive-descent parser: operator precedence, right
// associative exponentiation, postfix factorials, implicit multiplication
// (2pi), named constants, a library of math functions, user variables, a
// deg/rad angle mode, and a result history.
//
// Values also carry units of measurement, taken from the internal/units engine
// the convert plugin shares: 100 km / 2 h is 5 km/h, 5 m + 3 ft is 5.9144 m, and
// `to` converts a result, so `calc 10 km to mi` works too. See `calc help`.
package calc

import (
	"fmt"
	"strings"
	"sync"

	"gobot/internal/plugin"
)

// HistoryLen is how many past results `calc hist` remembers.
const HistoryLen = 20

// Plugin is the calculator. Its one command owns mutable state: the variable
// table, the angle mode, and the result history.
type Plugin struct {
	mu      sync.Mutex
	vars    map[string]quantity
	mode    angleMode
	history []entry
	count   int // total results produced, used to number history entries
}

type entry struct {
	n      int // 1-based position, kept stable as older entries are dropped
	expr   string
	result string
}

// New returns a calculator with an empty variable table, radian mode, and no
// history. Each call is fully independent of any other.
func New() *Plugin {
	return &Plugin{vars: make(map[string]quantity), mode: rad}
}

// Name implements plugin.Plugin.
func (p *Plugin) Name() string { return "calc" }

// Description implements plugin.Plugin.
func (p *Plugin) Description() string {
	return "scientific calculator: expressions, functions, variables, history"
}

// Commands implements plugin.Plugin.
func (p *Plugin) Commands() []*plugin.Command {
	return []*plugin.Command{{
		Name:        "calc",
		Description: "evaluate a math expression, e.g. `calc sqrt(2) * pi`",
		Handler:     p.handle,
	}}
}

var _ plugin.Plugin = (*Plugin)(nil)

// handle dispatches on the first word: subcommands manage state, anything else
// is treated as an expression.
func (p *Plugin) handle(args []string) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(args) == 0 {
		return usage
	}
	word := strings.ToLower(args[0])
	switch word {
	case "help", "?":
		return helpText
	case "mode":
		return p.setMode(args[1:])
	case "vars":
		return p.listVars()
	case "hist", "history":
		return p.listHistory()
	case "clear":
		p.vars = make(map[string]quantity)
		p.history = nil
		p.count = 0
		return "cleared variables and history"
	case "deg", "rad":
		// Only a bare `calc deg` switches modes; `calc deg(1)` is an
		// expression, and so is `calc deg * 2`.
		if len(args) == 1 {
			if word == "deg" {
				p.mode = deg
			} else {
				p.mode = rad
			}
			return "mode: " + p.mode.String()
		}
	}
	return p.evaluate(strings.Join(args, " "))
}

// evaluate evaluates src, remembers it, and makes the result available as ans.
func (p *Plugin) evaluate(src string) string {
	v, err := p.eval(src)
	if err != nil {
		return "error: " + err.Error()
	}
	out := v.String()
	p.vars["ans"] = v
	p.count++
	p.history = append(p.history, entry{n: p.count, expr: src, result: out})
	if len(p.history) > HistoryLen {
		keep := len(p.history) - HistoryLen
		p.history = append(p.history[:0], p.history[keep:]...)
	}
	return out
}

func (p *Plugin) setMode(args []string) string {
	if len(args) == 0 {
		return "mode: " + p.mode.String()
	}
	switch strings.ToLower(args[0]) {
	case "deg", "degree", "degrees":
		p.mode = deg
	case "rad", "radian", "radians":
		p.mode = rad
	default:
		return fmt.Sprintf("error: unknown angle mode %q, use deg or rad", args[0])
	}
	return "mode: " + p.mode.String()
}

func (p *Plugin) listVars() string {
	var b strings.Builder
	fmt.Fprintf(&b, "mode: %s", p.mode)
	if len(p.vars) == 0 {
		return b.String() + "\nno variables set, try `calc x = 3`"
	}
	b.WriteString("\n")
	for _, name := range sortedKeys(p.vars) {
		fmt.Fprintf(&b, "%s = %s\n", name, p.vars[name])
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (p *Plugin) listHistory() string {
	if len(p.history) == 0 {
		return "no history yet"
	}
	var b strings.Builder
	// Newest first: the interesting results are the recent ones.
	for i := len(p.history) - 1; i >= 0; i-- {
		e := p.history[i]
		fmt.Fprintf(&b, "%d) %s = %s\n", e.n, e.expr, e.result)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

const usage = "usage: calc <expression> | calc help" + "\n" +
	"example: calc 2^10 + sqrt(2) * pi"

// helpText documents the language, generated from the function and constant
// tables so it cannot drift out of sync with them.
var helpText = buildHelp()

func buildHelp() string {
	var b strings.Builder
	b.WriteString("scientific calculator — `calc <expression>`\n\n")
	b.WriteString("operators: + - * / ^ (or **) % (remainder) ! (factorial), parentheses,\n")
	b.WriteString("           implicit multiplication (2pi, 3(4), 10 km), right-associative ^\n")
	b.WriteString("           juxtaposition binds tighter than * and /, so 100 km / 2 h is 50 km/h\n")
	b.WriteString("constants: " + strings.Join(sortedKeys(constants), ", ") + "\n")
	b.WriteString("variables: `calc x = 3` stores x, `ans` is the previous result\n")
	b.WriteString("state:     `calc mode [deg|rad]`, `calc vars`, `calc hist`, `calc clear`\n\n")
	b.WriteString("units:   numbers can carry units through arithmetic, and convert with\n")
	b.WriteString("         to or ->:\n")
	b.WriteString("           calc 10 km to mi          calc 100 km/h -> mph\n")
	b.WriteString("           calc 5 m + 3 ft           calc 100 km / 2 h\n")
	b.WriteString("           calc 2 kg * 3 m/s^2       calc sin(45 deg)\n")
	b.WriteString("         a result keeps the units it was computed with, additions keep\n")
	b.WriteString("         the left side's, and powers and roots work on units as well:\n")
	b.WriteString("           calc (2 m)^2              calc sqrt(16 m^2)\n")
	b.WriteString("         spellings keep their case (kB is a kilobyte, kb a kilobit).\n")
	b.WriteString("         `convert units` lists every unit name. Adding two different\n")
	b.WriteString("         temperature scales goes through kelvin, while one scale adds as\n")
	b.WriteString("         it stands, so 5 C + 3 C is 8 C. deg, rad and min are functions\n")
	b.WriteString("         when followed by ( and units otherwise.\n\n")
	b.WriteString("functions:\n")
	for _, name := range sortedKeys(funcs) {
		fmt.Fprintf(&b, "  %s\n", funcs[name].doc)
	}
	return strings.TrimSuffix(b.String(), "\n")
}
