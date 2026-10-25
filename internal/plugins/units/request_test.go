package units

import (
	"testing"
)

func TestDegreeSignIsIgnored(t *testing.T) {
	if got := convert(t, "32 °F to °C"); got != "32 °F = 0 °C" {
		t.Errorf("32 °F to °C = %q", got)
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
