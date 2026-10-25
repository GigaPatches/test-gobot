package calc

import (
	"math"
	"testing"
)

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{4, "4"},
		{-4, "-4"},
		{0.5, "0.5"},
		{1234567, "1234567"},
		{1.0 / 3, "0.333333333333"},
		{1e21, "1e+21"},
		{1e-9, "1e-09"},
		{math.Inf(1), "Inf"},
		{math.Inf(-1), "-Inf"},
		{math.NaN(), "NaN"},
	}
	for _, tt := range tests {
		if got := formatFloat(tt.in); got != tt.want {
			t.Errorf("formatFloat(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTokenizeFoldsNamesAndSplitsExponents(t *testing.T) {
	toks, err := tokenize("X = -2.5E1 SIN(pi)")
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	want := []tokKind{tokIdent, tokOp, tokOp, tokNum, tokIdent, tokOp, tokIdent, tokOp, tokEOF}
	if len(toks) != len(want) {
		t.Fatalf("got %d tokens, want %d: %+v", len(toks), len(want), toks)
	}
	for i, k := range want {
		if toks[i].kind != k {
			t.Errorf("token %d = %v, want %v", i, toks[i].kind, k)
		}
	}
	if toks[0].text != "x" {
		t.Errorf("identifier = %q, want folded \"x\"", toks[0].text)
	}
	if toks[3].value != 25 {
		t.Errorf("number token = %v, want 25", toks[3].value)
	}
}

func TestScanNumberStopsBeforeALoneE(t *testing.T) {
	// 2e is 2 * e, so the literal stops at the digits; 2e3 is one number.
	tok, width, err := scanNumber("2e")
	if err != nil {
		t.Fatalf("scanNumber(\"2e\"): %v", err)
	}
	if width != 1 || tok.value != 2 {
		t.Errorf("scanNumber(\"2e\") = %v (width %d), want 2 with width 1", tok.value, width)
	}
	tok, width, err = scanNumber("2e3-1")
	if err != nil {
		t.Fatalf("scanNumber(\"2e3-1\"): %v", err)
	}
	if width != 3 || tok.value != 2000 {
		t.Errorf("scanNumber(\"2e3-1\") = %v (width %d), want 2000 with width 3", tok.value, width)
	}
}
