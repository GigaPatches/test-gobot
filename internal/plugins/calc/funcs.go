package calc

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"gobot/internal/units"
)

// function is one callable name inside an expression, together with the number
// of arguments it accepts, how it treats arguments that carry units, and the help
// text that documents it.
type function struct {
	min   int    // smallest accepted argument count
	max   int    // largest accepted argument count, or -1 for variadic
	doc   string // one-line summary shown by `calc help`
	fn    func(mode angleMode, args []float64) (float64, error)
	units unitPolicy // how to unwrap quantities, or nil for plain numbers only
}

// with attaches a unit policy, which is how the table below says which functions
// are happy to be handed a quantity such as 45 deg or 16 m^2.
func (f function) with(policy unitPolicy) function {
	f.units = policy
	return f
}

// apply unwraps quantities into the plain numbers fn expects and reports the
// unit the result should carry.
func (f function) apply(p *Plugin, name string, args []quantity) ([]float64, units.Unit, error) {
	if f.units == nil {
		return plainArgs(p, name, args)
	}
	return f.units(p, name, args)
}

// arity describes the accepted argument count for error messages.
func (f function) arity() string {
	switch {
	case f.max < 0 && f.min == 1:
		return "one or more values"
	case f.max < 0:
		return fmt.Sprintf("%d or more values", f.min)
	case f.min == f.max && f.min == 1:
		return "one value"
	case f.min == f.max:
		return fmt.Sprintf("%d values", f.min)
	default:
		return fmt.Sprintf("%d or %d values", f.min, f.max)
	}
}

// unary builds a one-argument function that ignores the angle mode.
func unary(doc string, f func(x float64) (float64, error)) function {
	return function{min: 1, max: 1, doc: doc, fn: func(_ angleMode, a []float64) (float64, error) {
		return f(a[0])
	}}
}

// unaryAngleOut builds a one-argument function whose result is reported in the
// current angle mode, such as atan.
func unaryAngleOut(doc string, f func(x float64) (float64, error)) function {
	return function{min: 1, max: 1, doc: doc, fn: func(m angleMode, a []float64) (float64, error) {
		v, err := f(a[0])
		return m.fromRadians(v), err
	}}
}

// binary builds a two-argument function.
func binary(doc string, f func(x, y float64) (float64, error)) function {
	return function{min: 2, max: 2, doc: doc, fn: func(_ angleMode, a []float64) (float64, error) {
		return f(a[0], a[1])
	}}
}

// nary builds a function taking at least min arguments.
func nary(min int, doc string, f func(mode angleMode, args []float64) (float64, error)) function {
	return function{min: min, max: -1, doc: doc, fn: f}
}

// funcs are the names callable inside an expression. Keys are lowercase because
// the tokenizer folds identifiers down.
var funcs = map[string]function{
	// Trigonometry. The angle policy has already read the argument as an angle —
	// a quantity such as 45 deg, or a bare number in the current mode — so these
	// are handed radians.
	"sin": unary("sin(x) — sine of x", func(x float64) (float64, error) {
		return math.Sin(x), nil
	}).with(anglePolicy),
	"cos": unary("cos(x) — cosine of x", func(x float64) (float64, error) {
		return math.Cos(x), nil
	}).with(anglePolicy),
	"tan": unary("tan(x) — tangent of x", func(x float64) (float64, error) {
		return math.Tan(x), nil
	}).with(anglePolicy),
	"asin": unaryAngleOut("asin(x) — inverse sine of x, for -1 <= x <= 1", asinSafe),
	"acos": unaryAngleOut("acos(x) — inverse cosine of x, for -1 <= x <= 1", acosSafe),
	"atan": unaryAngleOut("atan(x) — inverse tangent of x", func(x float64) (float64, error) {
		return math.Atan(x), nil
	}),
	"atan2": {min: 2, max: 2, doc: "atan2(y, x) — angle of the vector (x, y)",
		fn: func(m angleMode, a []float64) (float64, error) {
			return m.fromRadians(math.Atan2(a[0], a[1])), nil
		}},
	"sinh": unary("sinh(x) — hyperbolic sine of x", func(x float64) (float64, error) {
		return math.Sinh(x), nil
	}),
	"cosh": unary("cosh(x) — hyperbolic cosine of x", func(x float64) (float64, error) {
		return math.Cosh(x), nil
	}),
	"tanh": unary("tanh(x) — hyperbolic tangent of x", func(x float64) (float64, error) {
		return math.Tanh(x), nil
	}),

	// Exponentials and logarithms.
	"exp": unary("exp(x) — e raised to x", func(x float64) (float64, error) {
		return math.Exp(x), nil
	}),
	"ln":   unary("ln(x) — natural logarithm of x", lnSafe),
	"log2": unary("log2(x) — base-2 logarithm of x", log2Safe),
	"log": {min: 1, max: 2, doc: "log(x) or log(x, base) — logarithm of x, base 10 by default",
		fn: func(_ angleMode, a []float64) (float64, error) {
			if a[0] <= 0 {
				return 0, fmt.Errorf("log is undefined for %s", formatFloat(a[0]))
			}
			if len(a) == 1 {
				return math.Log10(a[0]), nil
			}
			if a[1] <= 0 || a[1] == 1 {
				return 0, fmt.Errorf("log base must be positive and not 1, got %s", formatFloat(a[1]))
			}
			return math.Log(a[0]) / math.Log(a[1]), nil
		}},
	"logb": binary("logb(x, base) — logarithm of x to the given base", logbSafe),

	// Powers and roots. A root divides the exponents of a unit, and a power has
	// to be a whole number for one.
	"sqrt": unary("sqrt(x) — square root of x", sqrtSafe).with(rootPolicy(2)),
	"cbrt": unary("cbrt(x) — cube root of x", func(x float64) (float64, error) {
		return math.Cbrt(x), nil
	}).with(rootPolicy(3)),
	"root": binary("root(n, x) — n-th root of x", rootSafe).with(rootArgs),
	"pow": binary("pow(x, y) — x raised to y, same as x^y", func(x, y float64) (float64, error) {
		return math.Pow(x, y), nil
	}).with(powArgs),
	"fact": unary("fact(n) — factorial of n, same as n!, defined for n >= 0", factorial),

	// Rounding and sign, which keep whatever unit they are given.
	"abs": unary("abs(x) — magnitude of x", func(x float64) (float64, error) {
		return math.Abs(x), nil
	}).with(keepArgs),
	"sign": unary("sign(x) — -1, 0 or 1 according to the sign of x", func(x float64) (float64, error) {
		return sign(x), nil
	}).with(keepArgs),
	"floor": unary("floor(x) — largest integer not greater than x", func(x float64) (float64, error) {
		return math.Floor(x), nil
	}).with(keepArgs),
	"ceil": unary("ceil(x) — smallest integer not less than x", func(x float64) (float64, error) {
		return math.Ceil(x), nil
	}).with(keepArgs),
	"trunc": unary("trunc(x) — x with its fractional part dropped", func(x float64) (float64, error) {
		return math.Trunc(x), nil
	}).with(keepArgs),
	"round": {min: 1, max: 2, doc: "round(x) or round(x, digits) — nearest integer, halves away from zero, or to digits places",
		units: keepArgs,
		fn: func(_ angleMode, a []float64) (float64, error) {
			places := 0.0
			if len(a) == 2 {
				places = a[1]
			}
			shift := math.Pow(10, places)
			return math.Round(a[0]*shift) / shift, nil
		}},

	// Combining and remainder, where every argument has to measure the same thing.
	"min": nary(1, "min(a, b, ...) — smallest argument", func(_ angleMode, a []float64) (float64, error) {
		out := a[0]
		for _, v := range a[1:] {
			out = math.Min(out, v)
		}
		return out, nil
	}).with(sameArgs),
	"max": nary(1, "max(a, b, ...) — largest argument", func(_ angleMode, a []float64) (float64, error) {
		out := a[0]
		for _, v := range a[1:] {
			out = math.Max(out, v)
		}
		return out, nil
	}).with(sameArgs),
	"hypot": nary(2, "hypot(a, b, ...) — Euclidean length of the arguments",
		func(_ angleMode, a []float64) (float64, error) {
			out := 0.0
			for _, v := range a {
				out = math.Hypot(out, v)
			}
			return out, nil
		}).with(sameArgs),
	"mod": binary("mod(x, y) — remainder of x divided by y, same as x % y", modSafe).with(sameArgs),

	// Angle conversion, independent of the current mode.
	"deg": unary("deg(x) — x radians expressed in degrees", func(x float64) (float64, error) {
		return x * 180 / math.Pi, nil
	}).with(radiansPolicy),
	"rad": unary("rad(x) — x degrees expressed in radians", func(x float64) (float64, error) {
		return x * math.Pi / 180, nil
	}).with(radiansPolicy),
}

// constants are the predefined names. They cannot be assigned to.
var constants = map[string]float64{
	"pi":  math.Pi,
	"tau": 2 * math.Pi,
	"e":   math.E,
	"phi": (1 + math.Sqrt(5)) / 2,
	"inf": math.Inf(1),
}

func asinSafe(x float64) (float64, error) {
	if x < -1 || x > 1 {
		return 0, fmt.Errorf("asin is undefined for %s, want -1 to 1", formatFloat(x))
	}
	return math.Asin(x), nil
}

func acosSafe(x float64) (float64, error) {
	if x < -1 || x > 1 {
		return 0, fmt.Errorf("acos is undefined for %s, want -1 to 1", formatFloat(x))
	}
	return math.Acos(x), nil
}

func lnSafe(x float64) (float64, error) {
	if x <= 0 {
		return 0, fmt.Errorf("ln is undefined for %s, want a positive value", formatFloat(x))
	}
	return math.Log(x), nil
}

func log2Safe(x float64) (float64, error) {
	if x <= 0 {
		return 0, fmt.Errorf("log2 is undefined for %s, want a positive value", formatFloat(x))
	}
	return math.Log2(x), nil
}

func logbSafe(x, base float64) (float64, error) {
	if x <= 0 {
		return 0, fmt.Errorf("logb is undefined for %s, want a positive value", formatFloat(x))
	}
	if base <= 0 || base == 1 {
		return 0, fmt.Errorf("logb base must be positive and not 1, got %s", formatFloat(base))
	}
	return math.Log(x) / math.Log(base), nil
}

func sqrtSafe(x float64) (float64, error) {
	if x < 0 {
		return 0, fmt.Errorf("sqrt of %s is not a real number", formatFloat(x))
	}
	return math.Sqrt(x), nil
}

func rootSafe(n, x float64) (float64, error) {
	if n == 0 {
		return 0, errors.New("root needs a non-zero degree")
	}
	if x < 0 && math.Mod(n, 2) == 0 {
		return 0, fmt.Errorf("%s-th root of %s is not a real number", formatFloat(n), formatFloat(x))
	}
	return math.Copysign(math.Pow(math.Abs(x), 1/n), x), nil
}

func modSafe(x, y float64) (float64, error) {
	if y == 0 {
		return 0, errors.New("modulo by zero")
	}
	return math.Mod(x, y), nil
}

// factorial extends n! to non-integers through the gamma function.
func factorial(x float64) (float64, error) {
	if x < 0 && x == math.Trunc(x) {
		return 0, fmt.Errorf("factorial is undefined for %s", formatFloat(x))
	}
	return math.Gamma(x + 1), nil
}

func sign(x float64) float64 {
	switch {
	case x > 0:
		return 1
	case x < 0:
		return -1
	default:
		return 0
	}
}

// sortedKeys returns a map's keys in ascending order, for stable help output.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
