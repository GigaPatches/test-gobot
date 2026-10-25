package calc

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

// evalString evaluates src with a fresh calculator and returns the exact text
// the user would see.
func evalString(t *testing.T, src string) string {
	t.Helper()
	out := New().handle(strings.Fields(src))
	if out == "" {
		t.Fatalf("calc %s: empty reply", src)
	}
	return out
}

func TestArithmetic(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"2+2", "4"},
		{"2 + 3 * 4", "14"},
		{"(2+3)*4", "20"},
		{"10/4", "2.5"},
		{"1/8", "0.125"},
		{"2^10", "1024"},
		{"2**10", "1024"},
		{"2^3^2", "512"},
		{"-2^2", "-4"},
		{"2^-1", "0.5"},
		{"--2", "2"},
		{"7%3", "1"},
		{"mod(7, 4)", "3"},
		{"5!", "120"},
		{"0.1+0.2", "0.3"},
		{"1e3", "1000"},
		{"1.5e-4", "0.00015"},
		{"2e", "5.43656365692"},
		{"2e3/2", "1000"},
		{"2pi", "6.28318530718"},
		{"pi/2", "1.57079632679"},
		{"tau/2", "3.14159265359"},
		{"3(4)", "12"},
		{"(1+2)(3+4)", "21"},
		{"2pi^2", "19.7392088022"},
	}
	for _, tt := range tests {
		if got := evalString(t, tt.expr); got != tt.want {
			t.Errorf("calc %s = %s, want %s", tt.expr, got, tt.want)
		}
	}
}

func TestFunctions(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"sqrt(16)", "4"},
		{"cbrt(-8)", "-2"},
		{"root(3, 27)", "3"},
		{"root(3, -8)", "-2"},
		{"pow(2, 8)", "256"},
		{"fact(6)", "720"},
		{"fact(0)", "1"},
		{"0.5!", "0.886226925453"},
		{"abs(-5)", "5"},
		{"sign(-9)", "-1"},
		{"sign(0)", "0"},
		{"floor(-1.5)", "-2"},
		{"ceil(1.2)", "2"},
		{"trunc(-1.7)", "-1"},
		{"round(2.5)", "3"},
		{"round(-2.5)", "-3"},
		{"round(1.234, 2)", "1.23"},
		{"min(3, 1, 2)", "1"},
		{"max(3, 1, 2)", "3"},
		{"hypot(3, 4)", "5"},
		{"hypot(3, 4, 12)", "13"},
		{"log(1000)", "3"},
		{"log(8, 2)", "3"},
		{"log2(1024)", "10"},
		{"logb(81, 3)", "4"},
		{"ln(e)", "1"},
		{"exp(0)", "1"},
		{"deg(pi)", "180"},
		{"rad(180)", "3.14159265359"},
		{"tanh(0)", "0"},
		{"atan2(1, 1)*4", "3.14159265359"},
	}
	for _, tt := range tests {
		if got := evalString(t, tt.expr); got != tt.want {
			t.Errorf("calc %s = %s, want %s", tt.expr, got, tt.want)
		}
	}
}

func TestTrigonometryUsesRadiansByDefault(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"sin(0)", "0"},
		{"cos(0)", "1"},
		{"sin(pi/2)", "1"},
		{"asin(1)*2", "3.14159265359"},
		{"acos(1)", "0"},
	}
	for _, tt := range tests {
		if got := evalString(t, tt.expr); got != tt.want {
			t.Errorf("calc %s = %s, want %s", tt.expr, got, tt.want)
		}
	}
}

func TestAngleMode(t *testing.T) {
	p := New()
	if got := p.handle(nil); got != usage {
		t.Fatalf("bare calc = %q, want usage", got)
	}
	if got := p.handle(strings.Fields("sin(90)")); got != "0.893996663601" {
		t.Errorf("sin(90) in rad mode = %s, want 0.893996663601", got)
	}
	if got := p.handle(strings.Fields("mode")); got != "mode: rad" {
		t.Errorf("mode = %q, want mode: rad", got)
	}
	if got := p.handle(strings.Fields("deg")); got != "mode: deg" {
		t.Errorf("deg = %q, want mode: deg", got)
	}
	if got := p.handle(strings.Fields("sin(90)")); got != "1" {
		t.Errorf("sin(90) in deg mode = %s, want 1", got)
	}
	if got := p.handle(strings.Fields("atan(1)")); got != "45" {
		t.Errorf("atan(1) in deg mode = %s, want 45", got)
	}
	// Conversion helpers ignore the mode, and `deg(1)` is an expression.
	if got := p.handle(strings.Fields("rad(180)")); got != "3.14159265359" {
		t.Errorf("rad(180) = %s, want 3.14159265359", got)
	}
	if got := p.handle(strings.Fields("mode rad")); got != "mode: rad" {
		t.Errorf("mode rad = %q, want mode: rad", got)
	}
	if got := p.handle(strings.Fields("mode bogus")); got != `error: unknown angle mode "bogus", use deg or rad` {
		t.Errorf("mode bogus = %q", got)
	}
}

func TestVariablesAndAns(t *testing.T) {
	p := New()
	tests := []struct{ expr, want string }{
		{"x = 3", "3"},
		{"x^2", "9"},
		{"ans + 1", "10"},
		{"x = x + 1", "4"},
		{"x", "4"},
		{"y = 2x", "8"},
		{"x + y", "12"},
	}
	for _, tt := range tests {
		if got := p.handle(strings.Fields(tt.expr)); got != tt.want {
			t.Errorf("calc %s = %s, want %s", tt.expr, got, tt.want)
		}
	}
}

func TestVariablesListing(t *testing.T) {
	p := New()
	p.handle(strings.Fields("a = 1"))
	p.handle(strings.Fields("b = 2.5"))
	want := "mode: rad\na = 1\nans = 2.5\nb = 2.5"
	if got := p.handle(strings.Fields("vars")); got != want {
		t.Errorf("vars =\n%s\nwant\n%s", got, want)
	}
	if got := New().handle(strings.Fields("vars")); got != "mode: rad\nno variables set, try `calc x = 3`" {
		t.Errorf("empty vars = %q", got)
	}
}

func TestHistory(t *testing.T) {
	p := New()
	p.handle(strings.Fields("1+1"))
	p.handle(strings.Fields("2+2"))
	want := "2) 2+2 = 4\n1) 1+1 = 2"
	if got := p.handle(strings.Fields("hist")); got != want {
		t.Errorf("hist =\n%s\nwant\n%s", got, want)
	}
	if got := New().handle(strings.Fields("hist")); got != "no history yet" {
		t.Errorf("empty hist = %q", got)
	}
	if got := p.handle(strings.Fields("history")); got != want {
		t.Errorf("history = %q, want the same as hist", got)
	}
}

func TestPluginMetadata(t *testing.T) {
	p := New()
	if got := p.Name(); got != "calc" {
		t.Errorf("Name() = %q, want calc", got)
	}
	if p.Description() == "" {
		t.Error("Description() is empty")
	}
	cmds := p.Commands()
	if len(cmds) != 1 {
		t.Fatalf("Commands() returned %d commands, want 1", len(cmds))
	}
	if cmds[0].Name != "calc" || cmds[0].Description == "" || cmds[0].Handler == nil {
		t.Errorf("Commands() = %+v, want one usable calc command", cmds[0])
	}
}

func TestHistoryIsCapped(t *testing.T) {
	p := New()
	for i := range HistoryLen + 5 {
		p.handle(strings.Fields(strconv.Itoa(i)))
	}
	got := strings.Split(p.handle(strings.Fields("hist")), "\n")
	if len(got) != HistoryLen {
		t.Fatalf("history has %d entries, want %d", len(got), HistoryLen)
	}
	if got[0] != "25) 24 = 24" {
		t.Errorf("newest entry = %q, want %q", got[0], "25) 24 = 24")
	}
	if got[len(got)-1] != "6) 5 = 5" {
		t.Errorf("oldest entry = %q, want %q", got[len(got)-1], "6) 5 = 5")
	}
}

func TestClear(t *testing.T) {
	p := New()
	p.handle(strings.Fields("x = 3"))
	p.handle(strings.Fields("1+1"))
	if got := p.handle(strings.Fields("clear")); got != "cleared variables and history" {
		t.Fatalf("clear = %q", got)
	}
	if got := p.handle(strings.Fields("x")); got != `error: unknown name "x"` {
		t.Errorf("x after clear = %q, want an unknown-name error", got)
	}
	if got := p.handle(strings.Fields("hist")); got != "no history yet" {
		t.Errorf("hist after clear = %q", got)
	}
}

func TestStateIsPerInstance(t *testing.T) {
	first := New()
	first.handle(strings.Fields("x = 5"))
	first.handle(strings.Fields("deg"))
	second := New()
	if got := second.handle(strings.Fields("x")); got != `error: unknown name "x"` {
		t.Errorf("second instance leaked a variable: %q", got)
	}
	if got := second.handle(strings.Fields("mode")); got != "mode: rad" {
		t.Errorf("second instance leaked the angle mode: %q", got)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"1/0", "error: division by zero"},
		{"5 % 0", "error: modulo by zero"},
		{"mod(5, 0)", "error: modulo by zero"},
		{"sqrt(-4)", "error: sqrt of -4 is not a real number"},
		{"root(2, -4)", "error: 2-th root of -4 is not a real number"},
		{"root(0, 4)", "error: root needs a non-zero degree"},
		{"ln(0)", "error: ln is undefined for 0, want a positive value"},
		{"log2(-1)", "error: log2 is undefined for -1, want a positive value"},
		{"log(8, 1)", "error: log base must be positive and not 1, got 1"},
		{"asin(2)", "error: asin is undefined for 2, want -1 to 1"},
		{"acos(2)", "error: acos is undefined for 2, want -1 to 1"},
		{"(-1)!", "error: factorial is undefined for -1"},
		{"foo(1)", `error: unknown name "foo"`},
		{"sqrt", `error: expected "(" after sqrt`},
		{"sqrt(1, 2)", "error: sqrt takes one value, got 2"},
		{"atan2(1)", "error: atan2 takes 2 values, got 1"},
		{"hypot(1)", "error: hypot takes 2 or more values, got 1"},
		{"min()", "error: min takes one or more values, got 0"},
		{"logb(-1, 3)", "error: logb is undefined for -1, want a positive value"},
		{"logb(8, 0)", "error: logb base must be positive and not 1, got 0"},
		{"(1+2", `error: missing ")"`},
		{"sqrt(1", `error: missing ")" in sqrt(...)`},
		{"2 +", "error: unexpected end of expression"},
		{"1 2 )", `error: unexpected ")"`},
		{"3 = 4", "error: unexpected \"=\""},
		{"pi = 3", `error: cannot assign to constant "pi"`},
		{"sqrt = 4", `error: cannot assign to function "sqrt"`},
		{"2 @ 3", `error: unexpected character "@"`},
		{"2 × 3", `error: unexpected character "×"`},
		{".", `error: invalid number "."`},
	}
	for _, tt := range tests {
		if got := evalString(t, tt.expr); got != tt.want {
			t.Errorf("calc %s = %q, want %q", tt.expr, got, tt.want)
		}
	}
}

func TestHelpAndUsage(t *testing.T) {
	p := New()
	if got := p.handle(nil); got != usage {
		t.Errorf("bare calc = %q, want usage", got)
	}
	for _, word := range []string{"help", "?"} {
		got := p.handle(strings.Fields(word))
		for _, want := range []string{"operators:", "constants:", "functions:", "sin(x)", "hypot(a, b, ...)"} {
			if !strings.Contains(got, want) {
				t.Errorf("calc %s is missing %q:\n%s", word, want, got)
			}
		}
	}
}

// TestNoSilentNaN guards against a function returning NaN where the user should
// have been given an error message.
func TestNoSilentNaN(t *testing.T) {
	for _, expr := range []string{"sqrt(2)", "ln(2)", "0.1+0.2", "1/3", "sin(1)", "2^0.5"} {
		got := evalString(t, expr)
		v, err := strconv.ParseFloat(got, 64)
		if err != nil {
			t.Errorf("calc %s = %q, want a number", expr, got)
			continue
		}
		if math.IsNaN(v) {
			t.Errorf("calc %s = NaN, want a number or an error", expr)
		}
	}
}
