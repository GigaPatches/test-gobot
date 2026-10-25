package calc

import (
	"errors"
	"fmt"
	"math"

	"gobot/internal/units"
)

// quantity is a value with an optional unit attached. A bare number is a quantity
// with no unit, so the evaluator has one value type and units survive arithmetic
// for free: 100 km / 2 h is 50 km/h because the units divide the same way the
// numbers do.
type quantity struct {
	value float64
	unit  units.Unit
}

// plain wraps a bare number.
func plain(v float64) quantity { return quantity{value: v, unit: units.Plain} }

// isPlain reports a value carrying no unit.
func (q quantity) isPlain() bool { return q.unit.IsEmpty() }

// String prints the value with its unit — "6.21371192237 mi" — or just the
// number when there is no unit.
func (q quantity) String() string {
	text := formatFloat(q.value)
	if unit := q.unit.String(); unit != "" {
		return text + " " + unit
	}
	return text
}

// normalized folds a temperature offset into base units. Arithmetic needs this:
// adding 5 C and 3 C as if they were plain numbers would be meaningless, while
// adding them through kelvin is at least dimensionally honest.
func (q quantity) normalized() quantity {
	if !q.unit.Affine() {
		return q
	}
	return quantity{value: q.unit.ToBase(q.value), unit: units.BaseUnit(q.unit.Dims)}
}

// in re-expresses the quantity in another unit.
func (q quantity) in(target units.Unit) (quantity, error) {
	if q.unit.IsEmpty() {
		return quantity{}, fmt.Errorf("cannot convert %s to %s, it has no unit", q, target)
	}
	v, err := q.unit.ConvertTo(target, q.value)
	if err != nil {
		return quantity{}, err
	}
	return quantity{value: v, unit: target}, nil
}

// add keeps the left side's unit, so 5 m + 3 ft is 5.9144 m.
func (q quantity) add(o quantity) (quantity, error) {
	left, right, err := q.align(o, "add", "to")
	if err != nil {
		return quantity{}, err
	}
	return quantity{value: left.value + right.value, unit: left.unit}, nil
}

// sub keeps the left side's unit.
func (q quantity) sub(o quantity) (quantity, error) {
	left, right, err := q.align(o, "subtract", "from")
	if err != nil {
		return quantity{}, err
	}
	return quantity{value: left.value - right.value, unit: left.unit}, nil
}

// mod keeps the left side's unit too: 7 m mod 3 m is 1 m.
func (q quantity) mod(o quantity) (quantity, error) {
	left, right, err := q.align(o, "divide", "by")
	if err != nil {
		return quantity{}, err
	}
	if right.value == 0 {
		return quantity{}, errors.New("modulo by zero")
	}
	return quantity{value: math.Mod(left.value, right.value), unit: left.unit}, nil
}

// align puts both sides in the left side's unit, refusing a dimension mismatch.
func (q quantity) align(o quantity, verb, prep string) (quantity, quantity, error) {
	if q.isPlain() && o.isPlain() {
		return q, o, nil
	}
	left, right := q, o
	if !left.unit.SameDims(right.unit) {
		return quantity{}, quantity{}, fmt.Errorf("cannot %s %s (%s) %s %s (%s)",
			verb, right, right.unit.Dims, prep, left, left.unit.Dims)
	}
	if left.unit.SameScale(right.unit) {
		return left, right, nil
	}
	// Two temperature scales have to meet in kelvin before anything can be added
	// to them; the same scale adds as it stands, which is what a temperature
	// difference needs: 5 C + 3 C is 8 C, not 554.3 K.
	left, right = left.normalized(), right.normalized()
	right.value = left.unit.FromBase(right.unit.ToBase(right.value))
	return left, right, nil
}

// mul combines units as well as values: 2 kg * 3 m/s^2 is 6 kg*m/s^2. A unit
// literal arrives here as a quantity of one, so scaling it must not fold a
// temperature offset in: 32 F is 32 degrees Fahrenheit, not 32 kelvin.
func (q quantity) mul(o quantity) quantity {
	return quantity{value: q.value * o.value, unit: q.unit.Times(o.unit)}
}

// div combines units as well as values: 100 km / 2 h is 50 km/h.
func (q quantity) div(o quantity) (quantity, error) {
	if o.value == 0 {
		return quantity{}, errors.New("division by zero")
	}
	return quantity{value: q.value / o.value, unit: q.unit.DividedBy(o.unit)}, nil
}

// pow raises both the value and its unit, which only works for a whole power:
// (2 m)^2 is 4 m^2, while 2 m^0.5 is not a unit.
func (q quantity) pow(o quantity) (quantity, error) {
	if !o.isPlain() {
		return quantity{}, fmt.Errorf("cannot raise %s to the power of %s", q, o)
	}
	base := q.normalized()
	if base.isPlain() {
		return plain(math.Pow(base.value, o.value)), nil
	}
	n, ok := wholeNumber(o.value)
	if !ok {
		return quantity{}, fmt.Errorf("cannot raise %s to %s, the power of a unit has to be a whole number",
			q, formatFloat(o.value))
	}
	unit, err := base.unit.Power(n)
	if err != nil {
		return quantity{}, err
	}
	return quantity{value: math.Pow(base.value, float64(n)), unit: unit}, nil
}

// root takes an n-th root of a quantity. A unit has to divide evenly — sqrt(16 m^2)
// is 4 m — and when it does not, the same is tried in base units, which is what
// makes sqrt(1 ha) come out as 100 m.
func (q quantity) root(n int) (quantity, error) {
	q = q.normalized()
	if unit, err := q.unit.Root(n); err == nil {
		return quantity{value: q.value, unit: unit}, nil
	}
	if q.isPlain() {
		return q, nil
	}
	base := units.BaseUnit(q.unit.Dims)
	unit, err := base.Root(n)
	if err != nil {
		return quantity{}, fmt.Errorf("%s is not a unit that can be rooted", q)
	}
	return quantity{value: q.unit.ToBase(q.value), unit: unit}, nil
}

// wholeNumber reports whether v can be used as an exponent on a unit. The bounds
// keep a runaway 10^300 from turning into an absurd exponent vector.
func wholeNumber(v float64) (int, bool) {
	if math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) || v > 1e6 || v < -1e6 {
		return 0, false
	}
	return int(v), true
}

// angleDims is the dimension trig functions accept in place of a bare number.
var angleDims = units.Dims{Angle: 1}

// unitPolicy decides how a function treats arguments that carry units: it
// unwraps them to plain numbers and reports which unit the result carries.
// Keeping that separate from the math leaves the function table a list of
// ordinary float64 functions, which stay easy to read and to test.
type unitPolicy func(p *Plugin, name string, args []quantity) ([]float64, units.Unit, error)

// plainArgs refuses any argument that carries a unit.
func plainArgs(_ *Plugin, name string, args []quantity) ([]float64, units.Unit, error) {
	vals := make([]float64, len(args))
	for i, a := range args {
		if !a.isPlain() {
			return nil, units.Plain, fmt.Errorf("%s needs a plain number, not %s", name, a)
		}
		vals[i] = a.value
	}
	return vals, units.Plain, nil
}

// keepArgs lets the first argument carry any unit, which the result keeps; the
// rest have to be plain. This is abs, floor, round and friends.
func keepArgs(p *Plugin, name string, args []quantity) ([]float64, units.Unit, error) {
	first := args[0].normalized()
	rest, _, err := plainArgs(p, name, args[1:])
	if err != nil {
		return nil, units.Plain, err
	}
	return append([]float64{first.value}, rest...), first.unit, nil
}

// sameArgs requires every argument to measure the same thing and converts them
// all into the first argument's unit, so min(1 m, 3 ft) is 1 m.
func sameArgs(_ *Plugin, name string, args []quantity) ([]float64, units.Unit, error) {
	left := args[0].normalized()
	vals := make([]float64, len(args))
	vals[0] = left.value
	for i, a := range args[1:] {
		right := a.normalized()
		if !left.unit.SameDims(right.unit) {
			return nil, units.Plain, fmt.Errorf("%s needs the same units, got %s (%s) and %s (%s)",
				name, left, left.unit.Dims, right, right.unit.Dims)
		}
		vals[i+1] = left.unit.FromBase(right.unit.ToBase(right.value))
	}
	return vals, left.unit, nil
}

// angleIn reads the first argument as an angle: a quantity such as 45 deg is
// converted to radians, while a bare number is read through mode.
func angleIn(mode angleMode, name string, args []quantity) ([]float64, units.Unit, error) {
	vals := make([]float64, 0, len(args))
	first := args[0]
	switch {
	case first.isPlain():
		vals = append(vals, mode.toRadians(first.value))
	case first.unit.Dims == angleDims:
		vals = append(vals, first.unit.ToBase(first.value))
	default:
		return nil, units.Plain, fmt.Errorf("%s needs an angle, not %s", name, first)
	}
	for _, a := range args[1:] {
		if !a.isPlain() {
			return nil, units.Plain, fmt.Errorf("%s needs a plain number, not %s", name, a)
		}
		vals = append(vals, a.value)
	}
	return vals, units.Plain, nil
}

// anglePolicy reads the first argument in the current angle mode, so sin(90)
// means different things in deg and rad mode.
func anglePolicy(p *Plugin, name string, args []quantity) ([]float64, units.Unit, error) {
	return angleIn(p.mode, name, args)
}

// radiansPolicy reads the first argument as an angle regardless of the mode, for
// deg and rad, which convert between scales rather than using the current one.
func radiansPolicy(_ *Plugin, name string, args []quantity) ([]float64, units.Unit, error) {
	return angleIn(rad, name, args)
}

// rootPolicy is sqrt and cbrt: the value takes its root through the function
// itself, and the unit's exponents are divided here.
func rootPolicy(n int) unitPolicy {
	return func(_ *Plugin, name string, args []quantity) ([]float64, units.Unit, error) {
		q, err := args[0].root(n)
		if err != nil {
			return nil, units.Plain, fmt.Errorf("%s of %s is not a unit", name, args[0])
		}
		return []float64{q.value}, q.unit, nil
	}
}

// rootArgs is root(n, x), where the degree comes from the first argument.
func rootArgs(p *Plugin, name string, args []quantity) ([]float64, units.Unit, error) {
	degree, _, err := plainArgs(p, name, args[:1])
	if err != nil {
		return nil, units.Plain, err
	}
	n, ok := wholeNumber(degree[0])
	x := args[1]
	if !ok || n < 1 {
		// A zero or fractional degree is rootSafe's business, unless the value
		// carries a unit, in which case there is nothing sensible to do with it.
		if x.isPlain() {
			return []float64{degree[0], x.value}, units.Plain, nil
		}
		return nil, units.Plain, fmt.Errorf("%s needs a whole degree, got %s", name, formatFloat(degree[0]))
	}
	rooted, err := x.root(n)
	if err != nil {
		return nil, units.Plain, fmt.Errorf("%s of %s is not a unit", name, x)
	}
	return []float64{degree[0], rooted.value}, rooted.unit, nil
}

// powArgs is pow(x, y): a unit can only be raised to a whole power.
func powArgs(_ *Plugin, name string, args []quantity) ([]float64, units.Unit, error) {
	if !args[1].isPlain() {
		return nil, units.Plain, fmt.Errorf("%s cannot use %s as a power", name, args[1])
	}
	base := args[0].normalized()
	if base.isPlain() {
		return []float64{base.value, args[1].value}, units.Plain, nil
	}
	n, ok := wholeNumber(args[1].value)
	if !ok {
		return nil, units.Plain, fmt.Errorf("cannot raise %s to %s, the power of a unit has to be a whole number",
			args[0], formatFloat(args[1].value))
	}
	unit, err := base.unit.Power(n)
	if err != nil {
		return nil, units.Plain, err
	}
	return []float64{base.value, float64(n)}, unit, nil
}
