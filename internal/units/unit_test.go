package units

import (
	"math"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in     string
		dims   Dims
		factor float64
	}{
		{"km", dimLength, 1000},
		{"km/h", dimSpeed, 1000.0 / 3600.0},
		{"km per hour", dimSpeed, 1000.0 / 3600.0},
		{"m/s^2", dimAccel, 1},
		{"m s^-1", dimSpeed, 1},
		{"ft2", dimArea, 0.3048 * 0.3048},
		{"kg*m/s^2", dimForce, 1},
		{"kg m/s^2", dimForce, 1},
		{"cm^3", dimVolume, 1e-6},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.in, err)
			continue
		}
		if got.Dims != tt.dims {
			t.Errorf("Parse(%q) dims = %+v, want %+v", tt.in, got.Dims, tt.dims)
		}
		if math.Abs(got.scale()-tt.factor) > 1e-12 {
			t.Errorf("Parse(%q) factor = %v, want %v", tt.in, got.scale(), tt.factor)
		}
	}
}

func TestParseRejectsBadInput(t *testing.T) {
	for _, in := range []string{"", "zzz", "km//h", "/km", "km^", "km^0", "km/2", "2"} {
		if got, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = %+v, want an error", in, got)
		}
	}
}

// TestParseKeepsTheSpellingAsTyped is what lets a result be printed the way it
// was asked for: km stays km, miles stays miles.
func TestParseKeepsTheSpellingAsTyped(t *testing.T) {
	tests := []struct{ in, want string }{
		{"km", "km"},
		{"kilometres", "kilometres"},
		{"km/h", "km/h"},
		{"m/s^2", "m/s^2"},
		{"ft2", "ft^2"},
		{"kg*m/s^2", "kg*m/s^2"},
		{"s^-1", "1/s"},
		{"ft^-3", "1/ft^3"},
		{"°C", "C"},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.in, err)
			continue
		}
		if got.String() != tt.want {
			t.Errorf("Parse(%q).String() = %q, want %q", tt.in, got.String(), tt.want)
		}
	}
}

func TestUnitArithmetic(t *testing.T) {
	km := mustLiteral(t, "km")
	m := mustLiteral(t, "m")
	s := mustLiteral(t, "s")
	kg := mustLiteral(t, "kg")

	speed := km.DividedBy(mustLiteral(t, "h"))
	if got := speed.String(); got != "km/h" {
		t.Errorf("km/h rendered %q", got)
	}
	if speed.Dims != dimSpeed {
		t.Errorf("km/h dims = %+v, want %+v", speed.Dims, dimSpeed)
	}
	if math.Abs(speed.scale()-1000.0/3600.0) > 1e-12 {
		t.Errorf("km/h scale = %v, want %v", speed.scale(), 1000.0/3600.0)
	}

	force := kg.Times(m).DividedBy(mustPower(t, s, 2))
	if got := force.String(); got != "kg*m/s^2" {
		t.Errorf("newton rendered %q", got)
	}
	if force.Dims != dimForce {
		t.Errorf("newton dims = %+v, want %+v", force.Dims, dimForce)
	}

	// Multiplying by a bare number, which arrives as the zero unit, must not
	// scale anything away.
	if got := (Unit{}).Times(km); got.String() != "km" || math.Abs(got.scale()-1000) > 1e-12 {
		t.Errorf("plain × km = %s x%v, want km x1000", got, got.scale())
	}
	if got := km.DividedBy(km); !got.IsEmpty() || got.String() != "" {
		t.Errorf("km/km = %q, want dimensionless", got)
	}

	area := mustPower(t, km, 2)
	if got := area.String(); got != "km^2" {
		t.Errorf("(km^2) rendered %q", got)
	}
	if math.Abs(area.scale()-1e6) > 1e-6 {
		t.Errorf("km^2 scale = %v, want 1e6", area.scale())
	}
	root, err := area.Root(2)
	if err != nil {
		t.Fatalf("sqrt(km^2): %v", err)
	}
	if got := root.String(); got != "km" {
		t.Errorf("sqrt(km^2) rendered %q", got)
	}
	if math.Abs(root.scale()-1000) > 1e-9 {
		t.Errorf("sqrt(km^2) scale = %v, want 1000", root.scale())
	}
	if _, err := km.Root(2); err == nil {
		t.Error("sqrt(km) should not be a unit")
	}
}

func TestPlainUnitChangesNothing(t *testing.T) {
	if !Plain.IsEmpty() || Plain.String() != "" {
		t.Errorf("Plain renders %q", Plain.String())
	}
	if got := Plain.ToBase(2.5); got != 2.5 {
		t.Errorf("Plain.ToBase(2.5) = %v", got)
	}
	km := mustLiteral(t, "km")
	if got := Plain.Times(km); got.String() != "km" {
		t.Errorf("Plain × km = %q", got)
	}
}

func TestConvertToRefusesMismatchedDimensions(t *testing.T) {
	km := mustLiteral(t, "km")
	kg := mustLiteral(t, "kg")
	got, err := km.ConvertTo(kg, 1)
	if err == nil {
		t.Fatalf("km to kg = %v, want an error", got)
	}
	if err.Error() != "cannot convert km (length) to kg (mass)" {
		t.Errorf("error = %q", err)
	}
}

func TestBaseUnit(t *testing.T) {
	tests := []struct {
		d    Dims
		want string
	}{
		{dimNone, ""},
		{dimLength, "m"},
		{dimSpeed, "m/s"},
		{dimForce, "kg*m/s^2"},
		{dimTemp, "K"},
	}
	for _, tt := range tests {
		if got := BaseUnit(tt.d).String(); got != tt.want {
			t.Errorf("BaseUnit(%s) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestLiteralRejectsNonUnits(t *testing.T) {
	for _, name := range []string{"", "foo", "kC", "squared"} {
		if u, ok := Literal(name); ok {
			t.Errorf("Literal(%q) = %v, want not a unit", name, u)
		}
	}
	if u, ok := Literal("°F"); !ok || !u.Affine() {
		t.Errorf("Literal(°F) = %v, %v, want an affine unit", u, ok)
	}
}

func TestSameDimsAndScale(t *testing.T) {
	km, _ := Literal("km")
	kilometres, _ := Literal("kilometres")
	miles, _ := Literal("miles")
	kg, _ := Literal("kg")
	c, _ := Literal("C")
	f, _ := Literal("F")
	if !km.SameDims(miles) || km.SameDims(kg) {
		t.Error("km and miles should measure the same thing, kg should not")
	}
	if !km.SameScale(kilometres) {
		t.Error("km and kilometres are one scale spelled two ways")
	}
	if km.SameScale(miles) {
		t.Error("a mile is not the same scale as a kilometre")
	}
	if c.SameScale(f) {
		t.Error("C and F convert differently, so they are not the same scale")
	}
}

func mustLiteral(t *testing.T, name string) Unit {
	t.Helper()
	u, ok := Literal(name)
	if !ok {
		t.Fatalf("Literal(%q) failed", name)
	}
	return u
}

func mustPower(t *testing.T, u Unit, n int) Unit {
	t.Helper()
	got, err := u.Power(n)
	if err != nil {
		t.Fatalf("Power(%d): %v", n, err)
	}
	return got
}
