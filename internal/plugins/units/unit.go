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
	"fmt"
	"math"
	"sort"
	"strings"
)

// dims is an exponent vector over the base dimensions. Everything else — area,
// speed, energy — is expressed in terms of these six, which is what makes
// compound units and mismatch errors work.
type dims struct {
	length, mass, time, temp, data, angle int
}

// add returns d + n*o.
func (d dims) add(o dims, n int) dims {
	return dims{
		length: d.length + n*o.length,
		mass:   d.mass + n*o.mass,
		time:   d.time + n*o.time,
		temp:   d.temp + n*o.temp,
		data:   d.data + n*o.data,
		angle:  d.angle + n*o.angle,
	}
}

var (
	dimLength   = dims{length: 1}
	dimMass     = dims{mass: 1}
	dimTime     = dims{time: 1}
	dimTemp     = dims{temp: 1}
	dimData     = dims{data: 1}
	dimAngle    = dims{angle: 1}
	dimNone     = dims{}
	dimArea     = dims{length: 2}
	dimVolume   = dims{length: 3}
	dimSpeed    = dims{length: 1, time: -1}
	dimAccel    = dims{length: 1, time: -2}
	dimFreq     = dims{time: -1}
	dimForce    = dims{mass: 1, length: 1, time: -2}
	dimEnergy   = dims{mass: 1, length: 2, time: -2}
	dimPower    = dims{mass: 1, length: 2, time: -3}
	dimPressure = dims{mass: 1, length: -1, time: -2}
)

// dimNames gives the common derived dimensions a readable name. Anything not
// listed is rendered from its exponents by dims.String.
var dimNames = map[dims]string{
	dimNone:     "dimensionless",
	dimLength:   "length",
	dimMass:     "mass",
	dimTime:     "time",
	dimTemp:     "temperature",
	dimData:     "data",
	dimAngle:    "angle",
	dimArea:     "area",
	dimVolume:   "volume",
	dimSpeed:    "speed",
	dimAccel:    "acceleration",
	dimFreq:     "frequency",
	dimForce:    "force",
	dimEnergy:   "energy",
	dimPower:    "power",
	dimPressure: "pressure",
}

var baseDimNames = []struct {
	name string
	get  func(dims) int
	set  func(*dims, int)
}{
	{"length", func(d dims) int { return d.length }, func(d *dims, n int) { d.length = n }},
	{"mass", func(d dims) int { return d.mass }, func(d *dims, n int) { d.mass = n }},
	{"time", func(d dims) int { return d.time }, func(d *dims, n int) { d.time = n }},
	{"temperature", func(d dims) int { return d.temp }, func(d *dims, n int) { d.temp = n }},
	{"data", func(d dims) int { return d.data }, func(d *dims, n int) { d.data = n }},
	{"angle", func(d dims) int { return d.angle }, func(d *dims, n int) { d.angle = n }},
}

// String names a dimension vector, e.g. "energy" or "mass/(length*time^2)".
func (d dims) String() string {
	if name, ok := dimNames[d]; ok {
		return name
	}
	var pos, neg []string
	for _, b := range baseDimNames {
		switch e := b.get(d); {
		case e == 0:
		case e > 0:
			pos = append(pos, term(b.name, e))
		default:
			neg = append(neg, term(b.name, -e))
		}
	}
	switch {
	case len(pos) == 0 && len(neg) == 0:
		return "dimensionless"
	case len(neg) == 0:
		return strings.Join(pos, "*")
	case len(pos) == 0 && len(neg) == 1:
		return "1/" + neg[0]
	case len(pos) == 0:
		return "1/(" + strings.Join(neg, "*") + ")"
	case len(neg) == 1:
		return strings.Join(pos, "*") + "/" + neg[0]
	default:
		return strings.Join(pos, "*") + "/(" + strings.Join(neg, "*") + ")"
	}
}

func term(name string, exp int) string {
	if exp == 1 {
		return name
	}
	return fmt.Sprintf("%s^%d", name, exp)
}

// unit is one named measurement. A value in this unit reaches the base unit of
// its dimension as (value + pre) * scale + post; plain units only use scale,
// while the temperature scales also use pre and post.
type unit struct {
	family  string
	symbol  string
	aliases []string
	dims    dims
	scale   float64
	pre     float64
	post    float64
}

// affine reports whether converting through this unit needs offsets, which only
// works when the unit stands alone rather than inside a compound expression.
func (u *unit) affine() bool { return u.pre != 0 || u.post != 0 }

// toBase converts a value measured in this unit into base units.
func (u *unit) toBase(v float64) float64 { return (v+u.pre)*u.scale + u.post }

// fromBase is the inverse of toBase.
func (u *unit) fromBase(v float64) float64 { return (v-u.post)/u.scale - u.pre }

// prefixes are accepted in front of a unit name: SI decades plus the binary
// multipliers for data. Lookup tries them longest-first so "dam" is deca-metre
// rather than something invented from "d".
var prefixes = map[string]float64{
	"y": 1e-24, "z": 1e-21, "a": 1e-18, "f": 1e-15, "p": 1e-12, "n": 1e-9,
	"u": 1e-6, "µ": 1e-6, "m": 1e-3, "c": 1e-2, "d": 1e-1, "da": 1e1,
	"h": 1e2, "k": 1e3, "M": 1e6, "G": 1e9, "T": 1e12, "P": 1e15, "E": 1e18,
	"Z": 1e21, "Y": 1e24,
	"Ki": 1 << 10, "Mi": 1 << 20, "Gi": 1 << 30, "Ti": 1 << 40, "Pi": 1 << 50,
}

// prefixOrder lists the prefixes longest-first, so a two-letter prefix wins the
// tie against a one-letter one.
var prefixOrder = sortedKeysByLength(prefixes)

// wordPrefixes are the spelled-out decades, so "kilometres" and "megabytes"
// resolve the same way their abbreviated forms do.
var wordPrefixes = map[string]float64{
	"quecto": 1e-30, "ronto": 1e-27, "yocto": 1e-24, "zepto": 1e-21, "atto": 1e-18,
	"femto": 1e-15, "pico": 1e-12, "nano": 1e-9, "micro": 1e-6, "milli": 1e-3,
	"centi": 1e-2, "deci": 1e-1, "deca": 1e1, "hecto": 1e2, "kilo": 1e3,
	"mega": 1e6, "giga": 1e9, "tera": 1e12, "peta": 1e15, "exa": 1e18,
	"zetta": 1e21, "yotta": 1e24,
	"kibi": 1 << 10, "mebi": 1 << 20, "gibi": 1 << 30, "tebi": 1 << 40, "pebi": 1 << 50,
}

// wordPrefixOrder lists the spelled-out prefixes longest-first.
var wordPrefixOrder = sortedKeysByLength(wordPrefixes)

func sortedKeysByLength[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	return out
}

// table is every known unit. Names are matched case-sensitively (kB means
// kilobyte, kb means kilobit), so aliases spell out the spellings worth keeping.
var table = buildTable()

func buildTable() []*unit {
	var us []*unit
	add := func(family string, d dims, scale float64, symbol string, aliases ...string) {
		us = append(us, &unit{family: family, dims: d, scale: scale, symbol: symbol, aliases: aliases})
	}
	addAffine := func(family string, d dims, scale, pre, post float64, symbol string, aliases ...string) {
		us = append(us, &unit{family: family, dims: d, scale: scale, pre: pre, post: post, symbol: symbol, aliases: aliases})
	}

	// Length — base metre.
	add("length", dimLength, 1, "m", "metre", "meter")
	add("length", dimLength, 1609.344, "mi", "mile")
	add("length", dimLength, 0.9144, "yd", "yard")
	add("length", dimLength, 0.3048, "ft", "foot", "feet")
	add("length", dimLength, 0.0254, "in", "inch", "inches")
	add("length", dimLength, 2.54e-5, "mil", "thou")
	add("length", dimLength, 1852, "nmi", "nauticalmile")
	add("length", dimLength, 201.168, "furlong")
	add("length", dimLength, 20.1168, "chain")
	add("length", dimLength, 5.0292, "rod", "pole", "perch")
	add("length", dimLength, 0.1016, "hand")
	add("length", dimLength, 1e-10, "angstrom")
	add("length", dimLength, 1.495978707e11, "au", "astronomicalunit")
	add("length", dimLength, 9.4607304725808e15, "ly", "lightyear")
	add("length", dimLength, 3.0856775814913673e16, "pc", "parsec")

	// Mass — base kilogram.
	add("mass", dimMass, 1, "kg", "kilogram")
	add("mass", dimMass, 1e-3, "g", "gram")
	add("mass", dimMass, 1000, "t", "tonne", "metricton")
	add("mass", dimMass, 907.18474, "ton", "shortton")
	add("mass", dimMass, 1016.0469088, "longton")
	add("mass", dimMass, 0.45359237, "lb", "pound")
	add("mass", dimMass, 0.028349523125, "oz", "ounce")
	add("mass", dimMass, 6.35029318, "stone", "st")
	add("mass", dimMass, 6.479891e-5, "grain", "gr")
	add("mass", dimMass, 2e-4, "carat", "ct")
	add("mass", dimMass, 14.593902937206364, "slug")

	// Time — base second.
	add("time", dimTime, 1, "s", "second")
	add("time", dimTime, 60, "min", "minute")
	add("time", dimTime, 3600, "h", "hour", "hr")
	add("time", dimTime, 86400, "d", "day")
	add("time", dimTime, 604800, "week", "wk")
	add("time", dimTime, 1209600, "fortnight")
	add("time", dimTime, 31557600, "year", "yr", "julianyear")
	add("time", dimTime, 315576000, "decade")
	add("time", dimTime, 3155760000, "century")

	// Temperature — base kelvin, and the only affine units.
	add("temperature", dimTemp, 1, "K", "kelvin")
	addAffine("temperature", dimTemp, 1, 0, 273.15, "C", "c", "celsius")
	addAffine("temperature", dimTemp, 5.0/9.0, -32, 273.15, "F", "f", "fahrenheit")
	addAffine("temperature", dimTemp, 5.0/9.0, 0, 0, "R", "rankine")

	// Area — base square metre.
	add("area", dimArea, 1e4, "ha", "hectare")
	add("area", dimArea, 100, "are")
	add("area", dimArea, 4046.8564224, "acre")

	// Volume — base cubic metre.
	add("volume", dimVolume, 1e-3, "L", "l", "litre", "liter")
	add("volume", dimVolume, 1e-6, "cc")
	add("volume", dimVolume, 3.785411784e-3, "gal", "gallon", "usgal")
	add("volume", dimVolume, 4.54609e-3, "impgal", "ukgal")
	add("volume", dimVolume, 9.46352946e-4, "qt", "quart")
	add("volume", dimVolume, 1.1365225e-3, "impqt")
	add("volume", dimVolume, 4.73176473e-4, "pt", "pint")
	add("volume", dimVolume, 5.6826125e-4, "imppint")
	add("volume", dimVolume, 2.95735295625e-5, "floz", "fluidounce")
	add("volume", dimVolume, 2.84130625e-5, "impfloz")
	add("volume", dimVolume, 2.365882365e-4, "cup")
	add("volume", dimVolume, 2.84130625e-4, "impcup")
	add("volume", dimVolume, 1.478676478125e-5, "tbsp", "tablespoon")
	add("volume", dimVolume, 4.92892159375e-6, "tsp", "teaspoon")
	add("volume", dimVolume, 0.158987294928, "bbl", "barrel")

	// Speed — base metre per second. Compound units cover km/h and ft/s, so
	// only the named shortcuts live here.
	add("speed", dimSpeed, 0.44704, "mph")
	add("speed", dimSpeed, 1.0/3.6, "kph", "kmh")
	add("speed", dimSpeed, 1852.0/3600.0, "knot", "kt")

	// Data — base bit.
	add("data", dimData, 1, "bit", "b")
	add("data", dimData, 8, "B", "byte")
	add("data", dimData, 4, "nibble")

	// Angle — base radian.
	add("angle", dimAngle, 1, "rad", "radian")
	add("angle", dimAngle, 2*math.Pi, "turn", "revolution")
	add("angle", dimAngle, math.Pi/180, "deg", "degree")
	add("angle", dimAngle, math.Pi/200, "grad", "gon")
	add("angle", dimAngle, math.Pi/10800, "arcmin", "arcminute")
	add("angle", dimAngle, math.Pi/648000, "arcsec", "arcsecond")
	add("angle", dimAngle, math.Pi/6.48e8, "mas", "milliarcsecond")

	// Frequency — base hertz, which is a reciprocal second.
	add("frequency", dimFreq, 1, "Hz", "hertz")

	// Energy — base joule.
	add("energy", dimEnergy, 1, "J", "joule")
	add("energy", dimEnergy, 4.184, "cal", "calorie")
	add("energy", dimEnergy, 4184, "Cal", "foodcalorie")
	add("energy", dimEnergy, 3600, "Wh", "watt-hour")
	add("energy", dimEnergy, 1e-7, "erg")
	add("energy", dimEnergy, 1055.05585262, "BTU", "btu")
	add("energy", dimEnergy, 1.3558179483314004, "ftlb")
	add("energy", dimEnergy, 1.602176634e-19, "eV")

	// Power — base watt.
	add("power", dimPower, 1, "W", "watt")
	add("power", dimPower, 745.699872, "hp", "horsepower")
	add("power", dimPower, 735.49875, "PS", "metrichorsepower")

	// Pressure — base pascal.
	add("pressure", dimPressure, 1, "Pa", "pascal")
	add("pressure", dimPressure, 1e5, "bar")
	add("pressure", dimPressure, 101325, "atm")
	add("pressure", dimPressure, 101325.0/760, "torr", "mmHg")
	add("pressure", dimPressure, 6894.757293168361, "psi")
	add("pressure", dimPressure, 3386.388640341, "inHg")

	// Force — base newton.
	add("force", dimForce, 1, "N", "newton")
	add("force", dimForce, 1e-5, "dyn", "dyne")
	add("force", dimForce, 4.4482216152605, "lbf")
	add("force", dimForce, 9.80665, "kgf")

	return us
}

// index maps every accepted spelling to its unit.
var index = buildIndex()

func buildIndex() map[string]*unit {
	m := make(map[string]*unit)
	for _, u := range table {
		for _, name := range append([]string{u.symbol}, u.aliases...) {
			if prev, dup := m[name]; dup && prev != u {
				panic("units: duplicate unit name " + name)
			}
			m[name] = u
		}
	}
	return m
}

// lookup resolves a spelling to a unit plus any extra scale from a prefix. It
// tries the exact name, then a prefix (so km, MHz and GiB work), then a spelled-
// out prefix (kilometres, megabytes), then a plural.
func lookup(name string) (*unit, float64, bool) {
	if name == "" {
		return nil, 0, false
	}
	if u := index[name]; u != nil {
		return u, 1, true
	}
	for _, p := range prefixOrder {
		if u, scale, ok := splitPrefix(name, p, prefixes[p]); ok {
			return u, scale, true
		}
	}
	lower := strings.ToLower(name)
	for _, p := range wordPrefixOrder {
		if u, scale, ok := splitPrefix(lower, p, wordPrefixes[p]); ok {
			return u, scale, true
		}
	}
	if s := singular(name); s != name {
		if u := index[s]; u != nil {
			return u, 1, true
		}
	}
	return nil, 0, false
}

// splitPrefix peels a prefix off the front of name and looks up what is left,
// tolerating a trailing plural on the unit part.
func splitPrefix(name, prefix string, scale float64) (*unit, float64, bool) {
	if !strings.HasPrefix(name, prefix) || len(name) == len(prefix) {
		return nil, 0, false
	}
	rest := name[len(prefix):]
	if u := index[rest]; u != nil {
		return u, scale, true
	}
	if s := singular(rest); s != rest {
		if u := index[s]; u != nil {
			return u, scale, true
		}
	}
	return nil, 0, false
}

// singular drops a trailing plural, which covers miles, hectares, joules and
// friends without listing every one of them.
func singular(name string) string {
	if strings.HasSuffix(name, "s") && len(name) > 1 {
		return name[:len(name)-1]
	}
	return name
}

// suggest guesses what a misspelled unit meant, for a helpful error message. It
// stays quiet for real unit names and for anything too far from a known symbol.
func suggest(name string) string {
	if _, _, ok := lookup(name); ok {
		return ""
	}
	limit := 2
	if len([]rune(name)) <= 3 {
		limit = 1
	}
	best, bestDist := "", limit+1
	for _, u := range table {
		if d := distance(strings.ToLower(name), strings.ToLower(u.symbol)); d < bestDist {
			best, bestDist = u.symbol, d
		}
	}
	return best
}

// distance is a small Levenshtein edit distance over runes.
func distance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			cur[j] = min(min(cur[j-1]+1, prev[j]+1), prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(br)]
}

// families is the order units are listed in help output.
var families = []string{
	"length", "mass", "time", "temperature", "area", "volume", "speed", "data",
	"angle", "frequency", "energy", "power", "pressure", "force",
}

// baseUnits names the anchor of each family: converting to it expresses a value
// in that family's base units.
var baseUnits = map[string]string{
	"length": "m", "mass": "kg", "time": "s", "temperature": "K", "area": "m^2",
	"volume": "m^3", "speed": "m/s", "data": "bit", "angle": "rad",
	"frequency": "Hz", "energy": "J", "power": "W", "pressure": "Pa", "force": "N",
}

// listing renders one line per family with its base unit and symbols.
func listing() string {
	byFamily := make(map[string][]string, len(families))
	for _, u := range table {
		byFamily[u.family] = append(byFamily[u.family], u.symbol)
	}
	var b strings.Builder
	for _, family := range families {
		units := byFamily[family]
		sort.Strings(units)
		label := family + " (base " + baseUnits[family] + ")"
		fmt.Fprintf(&b, "%-24s %s\n", label, strings.Join(units, ", "))
	}
	return strings.TrimSuffix(b.String(), "\n")
}
