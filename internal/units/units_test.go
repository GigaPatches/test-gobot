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

func TestTemperatureIsAffine(t *testing.T) {
	f, ok := Literal("F")
	if !ok {
		t.Fatal("Literal(F) failed")
	}
	if !f.Affine() {
		t.Fatal("F should be affine")
	}
	if got := f.ToBase(32); got != 273.15 {
		t.Errorf("32 F = %v K, want 273.15", got)
	}
	if got := f.FromBase(273.15); got != 32 {
		t.Errorf("273.15 K = %v F, want 32", got)
	}
	c, _ := Literal("C")
	if got := c.ToBase(100); got != 373.15 {
		t.Errorf("100 C = %v K, want 373.15", got)
	}
	if got := c.FromBase(0); got != -273.15 {
		t.Errorf("0 K = %v C, want -273.15", got)
	}
	if m, _ := Literal("m"); m.Affine() {
		t.Error("m should not be affine")
	}
}

func TestDimsString(t *testing.T) {
	tests := []struct {
		d    Dims
		want string
	}{
		{dimNone, "dimensionless"},
		{dimLength, "length"},
		{dimPressure, "pressure"},
		{Dims{Time: 1, Length: -1}, "time/length"},
		{Dims{Length: -2}, "1/length^2"},
		{Dims{Mass: 1, Time: -1}, "mass/time"},
		{Dims{Mass: 1, Length: -1, Time: -2}, "pressure"},
		{Dims{Mass: 1, Length: -2, Time: -2}, "mass/(length^2*time^2)"},
	}
	for _, tt := range tests {
		if got := tt.d.String(); got != tt.want {
			t.Errorf("Dims%+v = %q, want %q", tt.d, got, tt.want)
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

func TestSuggestAndDistance(t *testing.T) {
	if got := Suggest("fht"); got != "ft" {
		t.Errorf("Suggest(fht) = %q, want ft", got)
	}
	if got := Suggest("km"); got != "" {
		t.Errorf("Suggest(km) = %q, want no suggestion", got)
	}
	if got := Suggest("zzzzzz"); got != "" {
		t.Errorf("Suggest(zzzzzz) = %q, want no suggestion", got)
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

// TestEveryUnitRoundTrips converts one of each unit to itself, which catches a
// mistyped scale, an unreachable alias or a dimension that does not line up.
func TestEveryUnitRoundTrips(t *testing.T) {
	for _, u := range table {
		from, ok := Literal(u.symbol)
		if !ok {
			t.Errorf("Literal(%q) failed", u.symbol)
			continue
		}
		got, err := from.ConvertTo(from, 1)
		if err != nil {
			t.Errorf("round trip for %s: %v", u.symbol, err)
			continue
		}
		if math.Abs(got-1) > 1e-9 {
			t.Errorf("1 %s = %v %s", u.symbol, got, u.symbol)
		}
	}
}

// TestEveryUnitReachesItsBase checks that each unit converts to the base unit of
// its family, so no family is missing its anchor.
func TestEveryUnitReachesItsBase(t *testing.T) {
	for _, u := range table {
		want, ok := BaseUnits[u.family]
		if !ok {
			t.Errorf("unit %s has family %q, which has no base unit", u.symbol, u.family)
			continue
		}
		base, err := Parse(want)
		if err != nil {
			t.Errorf("Parse(%q): %v", want, err)
			continue
		}
		from, _ := Literal(u.symbol)
		if _, err := from.ConvertTo(base, 1); err != nil {
			t.Errorf("convert 1 %s to %s: %v", u.symbol, want, err)
		}
	}
}

func TestTableSanity(t *testing.T) {
	seen := map[string]bool{}
	for _, u := range table {
		if u.symbol == "" || u.family == "" {
			t.Errorf("unit %+v needs a symbol and a family", u)
		}
		if seen[u.symbol] {
			t.Errorf("symbol %q is registered twice", u.symbol)
		}
		seen[u.symbol] = true
		if u.scale <= 0 {
			t.Errorf("unit %s has a non-positive scale %v", u.symbol, u.scale)
		}
		if !familyListed(u.family) {
			t.Errorf("family %q is not listed in help output", u.family)
		}
	}
}

func familyListed(family string) bool {
	for _, f := range families {
		if f == family {
			return true
		}
	}
	return false
}
