package units

import (
	"math"
	"testing"
)

func TestLookupAppliesPrefixesBeforePlurals(t *testing.T) {
	tests := []struct {
		name       string
		wantSymbol string
		wantScale  float64
	}{
		{"km", "m", 1000},
		{"MHz", "Hz", 1e6},
		{"GiB", "B", 1 << 30},
		{"dam", "m", 10},
		{"ms", "s", 1e-3},   // millisecond, not "m" plus a plural
		{"knot", "knot", 1}, // exact name beats k + tonne
		{"kt", "knot", 1},
		{"miles", "mi", 1},
		{"inches", "in", 1},
		{"hectares", "ha", 1},
		{"kilometres", "m", 1000},
		{"kms", "m", 1000},
		{"centimetre", "m", 1e-2},
		{"milligrams", "g", 1e-3},
		{"megabytes", "B", 1e6},
		{"kibibyte", "B", 1 << 10},
	}
	for _, tt := range tests {
		u, scale, ok := lookup(tt.name)
		if !ok {
			t.Errorf("lookup(%q) failed", tt.name)
			continue
		}
		if u.symbol != tt.wantSymbol || scale != tt.wantScale {
			t.Errorf("lookup(%q) = %s x%v, want %s x%v", tt.name, u.symbol, scale, tt.wantSymbol, tt.wantScale)
		}
	}
}

func TestLookupRejectsUnknownNames(t *testing.T) {
	for _, name := range []string{"", "zzz", "kzz", "squared"} {
		if u, _, ok := lookup(name); ok {
			t.Errorf("lookup(%q) = %v, want not found", name, u)
		}
	}
}

func TestDegreeSignIsIgnored(t *testing.T) {
	if got := convert(t, "32 °F to °C"); got != "32 °F = 0 °C" {
		t.Errorf("32 °F to °C = %q", got)
	}
}

func TestTemperatureIsAffine(t *testing.T) {
	f := index["F"]
	if !f.affine() {
		t.Fatal("F should be affine")
	}
	if got := f.toBase(32); got != 273.15 {
		t.Errorf("32 F = %v K, want 273.15", got)
	}
	if got := f.fromBase(273.15); got != 32 {
		t.Errorf("273.15 K = %v F, want 32", got)
	}
	c := index["C"]
	if got := c.toBase(100); got != 373.15 {
		t.Errorf("100 C = %v K, want 373.15", got)
	}
	if got := c.fromBase(0); got != -273.15 {
		t.Errorf("0 K = %v C, want -273.15", got)
	}
	if index["m"].affine() {
		t.Error("m should not be affine")
	}
}

func TestDimsString(t *testing.T) {
	tests := []struct {
		d    dims
		want string
	}{
		{dimNone, "dimensionless"},
		{dimLength, "length"},
		{dimPressure, "pressure"},
		{dims{time: 1, length: -1}, "time/length"},
		{dims{length: -2}, "1/length^2"},
		{dims{mass: 1, time: -1}, "mass/time"},
		{dims{mass: 1, length: -1, time: -2}, "pressure"},
		{dims{mass: 1, length: -2, time: -2}, "mass/(length^2*time^2)"},
	}
	for _, tt := range tests {
		if got := tt.d.String(); got != tt.want {
			t.Errorf("dims%+v = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestDimsAdd(t *testing.T) {
	got := dimLength.add(dimTime, -1)
	if got != dimSpeed {
		t.Errorf("length + -1*time = %+v, want %+v", got, dimSpeed)
	}
	if got := dimLength.add(dimLength, 1); got != dimArea {
		t.Errorf("length + length = %+v, want %+v", got, dimArea)
	}
}

func TestParseUnitExpr(t *testing.T) {
	tests := []struct {
		in     string
		dims   dims
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
		got, err := parseUnitExpr(tt.in)
		if err != nil {
			t.Errorf("parseUnitExpr(%q): %v", tt.in, err)
			continue
		}
		if got.dims != tt.dims {
			t.Errorf("parseUnitExpr(%q) dims = %+v, want %+v", tt.in, got.dims, tt.dims)
		}
		if math.Abs(got.factor-tt.factor) > 1e-12 {
			t.Errorf("parseUnitExpr(%q) factor = %v, want %v", tt.in, got.factor, tt.factor)
		}
	}
}

func TestParseUnitExprRejectsBadInput(t *testing.T) {
	for _, in := range []string{"", "zzz", "km//h", "/km", "km^", "km^0", "km/2", "2"} {
		if got, err := parseUnitExpr(in); err == nil {
			t.Errorf("parseUnitExpr(%q) = %+v, want an error", in, got)
		}
	}
}

func TestSplitAmount(t *testing.T) {
	tests := []struct {
		in       string
		want     float64
		wantRest string
	}{
		{"10 km", 10, "km"},
		{"10km", 10, "km"},
		{"-40 C", -40, "C"},
		{"+2.5 kg", 2.5, "kg"},
		{"1.5e3 m", 1500, "m"},
		{"10eV", 10, "eV"},
		{"1e-2 km", 0.01, "km"},
		{".5 m", 0.5, "m"},
	}
	for _, tt := range tests {
		v, rest, err := splitAmount(tt.in)
		if err != nil {
			t.Errorf("splitAmount(%q): %v", tt.in, err)
			continue
		}
		if v != tt.want || rest != tt.wantRest {
			t.Errorf("splitAmount(%q) = %v, %q, want %v, %q", tt.in, v, rest, tt.want, tt.wantRest)
		}
	}
	for _, bad := range []string{"", "km", "abc km", "1.2.3 m"} {
		if _, _, err := splitAmount(bad); err == nil {
			t.Errorf("splitAmount(%q) succeeded, want an error", bad)
		}
	}
}

func TestSplitConversion(t *testing.T) {
	tests := []struct{ in, left, right string }{
		{"10 km to mi", "10 km", "mi"},
		{"10 km into mi", "10 km", "mi"},
		{"10 km as mi", "10 km", "mi"},
		{"10 km mi", "10 km", "mi"},
		{"10km->mi", "10km", "mi"},
		{"10 km => mi", "10 km", "mi"},
		{"10 km → mi", "10 km", "mi"},
		{"100 km/h to miles per hour", "100 km/h", "miles per hour"},
	}
	for _, tt := range tests {
		left, right, err := splitConversion(tt.in)
		if err != nil {
			t.Errorf("splitConversion(%q): %v", tt.in, err)
			continue
		}
		if left != tt.left || right != tt.right {
			t.Errorf("splitConversion(%q) = %q, %q, want %q, %q", tt.in, left, right, tt.left, tt.right)
		}
	}
	for _, bad := range []string{"10 km", "to mi", "10 km to", "->mi", "10 km->"} {
		if _, _, err := splitConversion(bad); err == nil {
			t.Errorf("splitConversion(%q) succeeded, want an error", bad)
		}
	}
}

func TestSuggestAndDistance(t *testing.T) {
	if got := suggest("fht"); got != "ft" {
		t.Errorf("suggest(fht) = %q, want ft", got)
	}
	if got := suggest("km"); got != "" {
		t.Errorf("suggest(km) = %q, want no suggestion", got)
	}
	if got := suggest("zzzzzz"); got != "" {
		t.Errorf("suggest(zzzzzz) = %q, want no suggestion", got)
	}
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"kitten", "sitting", 3},
		{"ft", "fht", 1},
		{"same", "same", 0},
	}
	for _, tt := range tests {
		if got := distance(tt.a, tt.b); got != tt.want {
			t.Errorf("distance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
