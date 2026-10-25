package calc

import (
	"strings"
	"testing"
)

func TestUnitArithmetic(t *testing.T) {
	tests := []struct{ expr, want string }{
		// A unit survives arithmetic and is printed as it was typed.
		{"10 km", "10 km"},
		{"3 * 4 m", "12 m"},
		{"(1 + 2) km", "3 km"},
		{"1 m + 1 m", "2 m"},
		{"1 in + 1 in", "2 in"},
		{"5 m + 3 ft", "5.9144 m"},
		{"10 km + 5 mi", "18.04672 km"},
		{"10 km - 500 m", "9.5 km"},

		// Products and quotients combine units the way the numbers combine.
		{"100 km / 2 h", "50 km/h"},
		{"100 km/h", "100 km/h"},
		{"5 m/s * 3 s", "15 m"},
		{"2 kg * 3 m/s^2", "6 kg*m/s^2"},
		{"100 W * 3 h", "300 W*h"},
		{"1 lb/ft^3", "1 lb/ft^3"},
		{"1/3 m", "0.333333333333 1/m"},

		// Powers and roots work on units too.
		{"(2 m)^2", "4 m^2"},
		{"2 m^2", "2 m^2"},
		{"2 ft^2 to in^2", "288 in^2"},
		{"sqrt(16 m^2)", "4 m"},
		{"sqrt(1 ha)", "100 m"},
		{"(1 m)^3", "1 m^3"},

		// Functions that accept or keep units.
		{"sin(45 deg)", "0.707106781187"},
		{"cos(60 deg)", "0.5"},
		{"abs(-5 m)", "5 m"},
		{"floor(2.9 m)", "2 m"},
		{"min(1 m, 3 ft)", "0.9144 m"},
		{"max(1 m, 3 ft)", "1 m"},
		{"hypot(3 m, 4 m)", "5 m"},
		{"7 m mod 3 m", "1 m"},
		{"90 deg to rad", "1.57079632679 rad"},
		{"5 min to s", "300 s"},
		{"cos(0 rad)", "1"},
	}
	for _, tt := range tests {
		if got := evalString(t, tt.expr); got != tt.want {
			t.Errorf("calc %s = %s, want %s", tt.expr, got, tt.want)
		}
	}
}

func TestUnitConversions(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"10 km to mi", "6.21371192237 mi"},
		{"10 km -> mi", "6.21371192237 mi"},
		{"100 km/h to mph", "62.1371192237 mph"},
		{"100 km/h to m/s", "27.7777777778 m/s"},
		{"10 km to mi to yd", "10936.1329834 yd"},
		{"1 stone to kg", "6.35029318 kg"},
		{"1 GiB to MB", "1073.741824 MB"},
		{"1 kB to bit", "8000 bit"},
		{"1 kb to bit", "1000 bit"},
		{"1 gallon to litres", "3.785411784 litres"},
		{"2^10 m to km", "1.024 km"},
		{"sqrt(2) mi to m", "2275.95611132 m"},

		// Temperature scales are affine, so they need the engine's offsets.
		{"32 F to C", "0 C"},
		{"100 C to F", "212 F"},
		{"-40 C to F", "-40 F"},
		{"0 C to K", "273.15 K"},
		{"5 C + 3 C", "8 C"},
		{"2 * 5 C", "10 C"},
	}
	for _, tt := range tests {
		if got := evalString(t, tt.expr); got != tt.want {
			t.Errorf("calc %s = %s, want %s", tt.expr, got, tt.want)
		}
	}
}

func TestUnitErrors(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"5 m + 3 s", "error: cannot add 3 s (time) to 5 m (length)"},
		{"5 m - 3 s", "error: cannot subtract 3 s (time) from 5 m (length)"},
		{"10 km to kg", "error: cannot convert km (length) to kg (mass)"},
		{"10 km to mi to kg", "error: cannot convert mi (length) to kg (mass)"},
		{"10 km to 5", `error: expected a unit after "to", got 5`},
		{"5 to m", "error: cannot convert 5 to m, it has no unit"},
		{"ln(5 m)", "error: ln needs a plain number, not 5 m"},
		{"sqrt(2 m)", "error: sqrt of 2 m is not a unit"},
		{"root(2, 2 m)", "error: root of 2 m is not a unit"},
		{"(2 m)^0.5", "error: cannot raise 2 m to 0.5, the power of a unit has to be a whole number"},
		{"pow(2 m, 0.5)", "error: cannot raise 2 m to 0.5, the power of a unit has to be a whole number"},
		{"sin(5 m)", "error: sin needs an angle, not 5 m"},
		{"deg(1 m)", "error: deg needs an angle, not 1 m"},
		{"min(1 m, 2)", "error: min needs the same units, got 1 m (length) and 2 (dimensionless)"},
		{"5 m!", "error: factorial needs a plain number, not 1 m"},
		{"5 km to fht", `error: unknown name "fht"`},
		{"1/0 m", "error: division by zero"},
	}
	for _, tt := range tests {
		if got := evalString(t, tt.expr); got != tt.want {
			t.Errorf("calc %s = %q, want %q", tt.expr, got, tt.want)
		}
	}
}

// TestUnitNamesAreCaseSensitive covers the data units, where a capital B is the
// difference between bytes and bits.
func TestUnitNamesAreCaseSensitive(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"1 kB to bit", "8000 bit"},
		{"1 kb to bit", "1000 bit"},
		{"1 GiB to MiB", "1024 MiB"},
		{"1 MB to kB", "1000 kB"},
	}
	for _, tt := range tests {
		if got := evalString(t, tt.expr); got != tt.want {
			t.Errorf("calc %s = %s, want %s", tt.expr, got, tt.want)
		}
	}
}

// TestFunctionNamesDoubleAsUnits covers the three names that are in both tables:
// a call needs parentheses, anything else is the unit.
func TestFunctionNamesDoubleAsUnits(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"deg(pi)", "180"},
		{"rad(180)", "3.14159265359"},
		{"min(3, 1, 2)", "1"},
		{"90 deg to rad", "1.57079632679 rad"},
		{"1 rad to deg", "57.2957795131 deg"},
		{"5 min to s", "300 s"},
	}
	for _, tt := range tests {
		if got := evalString(t, tt.expr); got != tt.want {
			t.Errorf("calc %s = %s, want %s", tt.expr, got, tt.want)
		}
	}
}

// TestAngleQuantitiesIgnoreTheMode is the point of accepting an angle: 45 deg
// means the same thing in either mode, while a bare number follows the mode.
func TestAngleQuantitiesIgnoreTheMode(t *testing.T) {
	p := New()
	p.handle(strings.Fields("deg"))
	for _, tt := range []struct{ expr, want string }{
		{"sin(90)", "1"},
		{"sin(45 deg)", "0.707106781187"},
		{"sin(1 rad)", "0.841470984808"},
	} {
		if got := p.handle(strings.Fields(tt.expr)); got != tt.want {
			t.Errorf("calc %s in deg mode = %s, want %s", tt.expr, got, tt.want)
		}
	}
}

func TestVariablesAndHistoryKeepUnits(t *testing.T) {
	p := New()
	steps := []struct{ expr, want string }{
		{"x = 5 m", "5 m"},
		{"x * 3", "15 m"},
		{"x + 1 m", "6 m"},
		{"10 km to mi", "6.21371192237 mi"},
		{"ans + 1 mi", "7.21371192237 mi"},
	}
	for _, tt := range steps {
		if got := p.handle(strings.Fields(tt.expr)); got != tt.want {
			t.Errorf("calc %s = %s, want %s", tt.expr, got, tt.want)
		}
	}
	want := "mode: rad\nans = 7.21371192237 mi\nx = 5 m"
	if got := p.handle(strings.Fields("vars")); got != want {
		t.Errorf("vars =\n%s\nwant\n%s", got, want)
	}
	want = "5) ans + 1 mi = 7.21371192237 mi\n4) 10 km to mi = 6.21371192237 mi\n3) x + 1 m = 6 m\n2) x * 3 = 15 m\n1) x = 5 m = 5 m"
	if got := p.handle(strings.Fields("hist")); got != want {
		t.Errorf("hist =\n%s\nwant\n%s", got, want)
	}
}

// TestHelpMentionsUnits guards the documentation of the new syntax.
func TestHelpMentionsUnits(t *testing.T) {
	got := New().handle(strings.Fields("help"))
	for _, want := range []string{"units:", "calc 10 km to mi", "100 km / 2 h", "sqrt(16 m^2)"} {
		if !strings.Contains(got, want) {
			t.Errorf("calc help is missing %q:\n%s", want, got)
		}
	}
}
