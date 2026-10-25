package units

import (
	"strings"
	"testing"

	"gobot/internal/units"
)

// convert runs the command the way the REPL would.
func convert(t *testing.T, line string) string {
	t.Helper()
	out := New().handle(strings.Fields(line))
	if out == "" {
		t.Fatalf("convert %s: empty reply", line)
	}
	return out
}

func TestConversions(t *testing.T) {
	tests := []struct{ in, want string }{
		// Length.
		{"10 km to mi", "10 km = 6.21371192237 mi"},
		{"1 mi to ft", "1 mi = 5280 ft"},
		{"1 mile to metres", "1 mile = 1609.344 metres"},
		{"2 kilometres to mi", "2 kilometres = 1.24274238447 mi"},
		{"5 megabytes to kB", "5 megabytes = 5000 kB"},
		{"1 kibibyte to bytes", "1 kibibyte = 1024 bytes"},
		{"1 feet to inches", "1 feet = 12 inches"},
		{"100 cm to mm", "100 cm = 1000 mm"},
		{"1 µm to nm", "1 µm = 1000 nm"},
		{"1 dam to m", "1 dam = 10 m"},
		{"1 furlong to chains", "1 furlong = 10 chains"},
		{"1 hand to mm", "1 hand = 101.6 mm"},
		{"1 lightyear to km", "1 lightyear = 9.46073047258e+12 km"},

		// Compound units.
		{"100 km/h to mph", "100 km/h = 62.1371192237 mph"},
		{"100 km/h to m/s", "100 km/h = 27.7777777778 m/s"},
		{"10 miles per hour to km/h", "10 miles per hour = 16.09344 km/h"},
		{"10 kmh to mph", "10 kmh = 6.21371192237 mph"},
		{"2 ft^2 to in^2", "2 ft^2 = 288 in^2"},
		{"1 m^2 to ft^2", "1 m^2 = 10.7639104167 ft^2"},
		{"1 lb/ft^3 to kg/m^3", "1 lb/ft^3 = 16.018463374 kg/m^3"},
		{"1 lb per ft^3 to kg/m^3", "1 lb per ft^3 = 16.018463374 kg/m^3"},
		{"1 kg*m/s^2 to N", "1 kg*m/s^2 = 1 N"},
		{"1 s^-1 to Hz", "1 s^-1 = 1 Hz"},

		// Mass, time, volume.
		{"1 stone to lb", "1 stone = 14 lb"},
		{"2.5 kg to g", "2.5 kg = 2500 g"},
		{"1 day to s", "1 day = 86400 s"},
		{"1.5 h to min", "1.5 h = 90 min"},
		{"1 fortnight to h", "1 fortnight = 336 h"},
		{"1 century to weeks", "1 century = 5217.85714286 weeks"},
		{"5 L to cm^3", "5 L = 5000 cm^3"},
		{"100 cm3 to mL", "100 cm3 = 100 mL"},
		{"1 gallon to litres", "1 gallon = 3.785411784 litres"},
		{"1 impgal to L", "1 impgal = 4.54609 L"},
		{"1 pint to mL", "1 pint = 473.176473 mL"},
		{"12 tsp to tbsp", "12 tsp = 4 tbsp"},
		{"1 acre to ha", "1 acre = 0.40468564224 ha"},

		// Temperature, which is affine rather than a plain scale.
		{"32 F to C", "32 F = 0 C"},
		{"100 C to F", "100 C = 212 F"},
		{"0 C to F", "0 C = 32 F"},
		{"-40 C to F", "-40 C = -40 F"},
		{"100 F to C", "100 F = 37.7777777778 C"},
		{"0 C to K", "0 C = 273.15 K"},
		{"32 F to K", "32 F = 273.15 K"},

		// Physics families.
		{"1 kWh to MJ", "1 kWh = 3.6 MJ"},
		{"1 hp to W", "1 hp = 745.699872 W"},
		{"1 PS to hp", "1 PS = 0.986320070067 hp"},
		{"1 atm to psi", "1 atm = 14.6959487755 psi"},
		{"1 inHg to kPa", "1 inHg = 3.38638864034 kPa"},
		{"1 eV to J", "1 eV = 1.602176634e-19 J"},
		{"1 BTU to kJ", "1 BTU = 1.05505585262 kJ"},
		{"1 N to dyn", "1 N = 100000 dyn"},
		{"1 kgf to N", "1 kgf = 9.80665 N"},
		{"1 MHz to kHz", "1 MHz = 1000 kHz"},
		{"1 turn to deg", "1 turn = 360 deg"},
		{"1 rad to deg", "1 rad = 57.2957795131 deg"},
		{"1 mas to arcsec", "1 mas = 0.001 arcsec"},

		// Data, where case decides bits versus bytes.
		{"1 GiB to MB", "1 GiB = 1073.741824 MB"},
		{"1 KiB to bit", "1 KiB = 8192 bit"},
		{"1 Tb to TB", "1 Tb = 0.125 TB"},
		{"1 kB to bit", "1 kB = 8000 bit"},
		{"1 kb to bit", "1 kb = 1000 bit"},
		{"1 nibble to bit", "1 nibble = 4 bit"},
	}
	for _, tt := range tests {
		if got := convert(t, tt.in); got != tt.want {
			t.Errorf("convert %s\n got %s\nwant %s", tt.in, got, tt.want)
		}
	}
}

func TestRequestSyntax(t *testing.T) {
	tests := []struct{ in, want string }{
		{"10 km mi", "10 km = 6.21371192237 mi"},
		{"10 km into mi", "10 km = 6.21371192237 mi"},
		{"10 km as mi", "10 km = 6.21371192237 mi"},
		{"10km->mi", "10 km = 6.21371192237 mi"},
		{"10 km => mi", "10 km = 6.21371192237 mi"},
		{"10 km → mi", "10 km = 6.21371192237 mi"},
		{"10.0 km to mi", "10 km = 6.21371192237 mi"},
		{"1e-2 km to m", "0.01 km = 10 m"},
	}
	for _, tt := range tests {
		if got := convert(t, tt.in); got != tt.want {
			t.Errorf("convert %s\n got %s\nwant %s", tt.in, got, tt.want)
		}
	}
}

func TestConversionErrors(t *testing.T) {
	tests := []struct{ in, want string }{
		{"10 km to kg", "error: cannot convert km (length) to kg (mass)"},
		{"1 C to km", "error: cannot convert C (temperature) to km (length)"},
		{"10 km/h to s/km", "error: cannot convert km/h (speed) to s/km (time/length)"},
		{"5 C/F to K", `error: F can only be converted on its own, not inside "C/F"`},
		{"1 kC to K", `error: kC can only be converted on its own, not inside "kC"`},
		{"kJ to J", `error: expected an amount before "kJ"`},
		{"abc km to mi", `error: expected an amount before "abc km"`},
		{"1.2.3 km to mi", `error: "1.2.3" is not a number`},
		{"10 km to", "error: nothing to convert to"},
		{"10 km->", "error: nothing to convert to"},
		{"to mi", "error: nothing to convert"},
		{"->mi", "error: nothing to convert"},
		{"10 km", "error: usage: convert <amount> <unit> to <unit> — try `convert help`"},
		{"10 zzz to mi", `error: unknown unit "zzz"`},
		{"10 km to zzz", `error: unknown unit "zzz"`},
		{"1 km//h to mi", `error: repeated "/" in "km//h"`},
		{"1 /km to mi", `error: "/km" does not start with a unit`},
		{"1 km^ to mi", `error: expected an exponent after "^" in "km^"`},
		{"1 km^0 to mi", `error: "km" raised to the power of zero is not a unit`},
		{"1 km/2 to mi", `error: unexpected number "2" in "km/2"`},
		{"1 km to mi extra", `error: unknown unit "extra"`},
	}
	for _, tt := range tests {
		if got := convert(t, tt.in); got != tt.want {
			t.Errorf("convert %s\n got %s\nwant %s", tt.in, got, tt.want)
		}
	}
}

func TestSuggestsCloseUnitNames(t *testing.T) {
	if got := convert(t, "10 fht to mi"); got != `error: unknown unit "fht", did you mean "ft"?` {
		t.Errorf("fht = %s", got)
	}
	if got := convert(t, "10 km to mii"); got != `error: unknown unit "mii", did you mean "mi"?` {
		t.Errorf("mii = %s", got)
	}
}

func TestHelpAndUnitsListing(t *testing.T) {
	p := New()
	if got := p.handle(nil); got != usage {
		t.Errorf("bare convert = %q, want usage", got)
	}
	for _, word := range []string{"help", "?"} {
		got := p.handle(strings.Fields(word))
		for _, want := range []string{"convert 100 km/h to mph", "compound units", "temperature scales"} {
			if !strings.Contains(got, want) {
				t.Errorf("convert %s is missing %q:\n%s", word, want, got)
			}
		}
	}
	for _, word := range []string{"units", "list"} {
		got := p.handle(strings.Fields(word))
		for _, want := range []string{"length", "temperature", "Hz", "fortnight", "psi"} {
			if !strings.Contains(got, want) {
				t.Errorf("convert %s is missing %q:\n%s", word, want, got)
			}
		}
	}
}

func TestPluginMetadata(t *testing.T) {
	p := New()
	if got := p.Name(); got != "units" {
		t.Errorf("Name() = %q, want units", got)
	}
	if p.Description() == "" {
		t.Error("Description() is empty")
	}
	cmds := p.Commands()
	if len(cmds) != 2 {
		t.Fatalf("Commands() returned %d commands, want 2", len(cmds))
	}
	for _, cmd := range cmds {
		if cmd.Name != "convert" && cmd.Name != "units" {
			t.Errorf("unexpected command %q", cmd.Name)
		}
		if cmd.Description == "" || cmd.Handler == nil {
			t.Errorf("command %q is incomplete", cmd.Name)
		}
	}
}

// TestListingMatchesEngine keeps the command's `convert units` output in step
// with the engine it is drawn from.
func TestListingMatchesEngine(t *testing.T) {
	got := New().handle([]string{"units"})
	if got != units.Listing() {
		t.Errorf("listing does not match the engine:\n%s", got)
	}
}
