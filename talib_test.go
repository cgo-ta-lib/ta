package ta_test

import (
	"math"
	"testing"

	ta "github.com/TA-Lib/ta-lib-cgo"
)

// prices is a synthetic input with enough bars to exercise all lookback periods.
var prices = func() []float64 {
	s := make([]float64, 200)
	for i := range s {
		// Simple sine-wave-like series to avoid constant input edge cases.
		s[i] = 100.0 + 10.0*math.Sin(float64(i)*0.1)
	}
	return s
}()

// ohlcv returns parallel OHLCV slices of the same length as prices.
func ohlcv(n int) (high, low, open, closePrice, volume []float64) {
	high = make([]float64, n)
	low = make([]float64, n)
	open = make([]float64, n)
	closePrice = make([]float64, n)
	volume = make([]float64, n)
	for i := range high {
		base := 100.0 + 10.0*math.Sin(float64(i)*0.1)
		high[i] = base + 1.0
		low[i] = base - 1.0
		open[i] = base + 0.3
		closePrice[i] = base - 0.3
		volume[i] = 1e6 + float64(i)*1000
	}
	return
}

func isNaN(f float64) bool { return math.IsNaN(f) }

// assertShapeInvariants checks that out:
//   - has the same length as in
//   - out[:lookback] are all NaN
//   - out[lookback:] are all finite (not NaN, not Inf)
func assertShapeInvariants(t *testing.T, name string, in, out []float64, lookback int) {
	t.Helper()
	if len(out) != len(in) {
		t.Errorf("%s: output length %d != input length %d", name, len(out), len(in))
		return
	}
	for i := 0; i < lookback && i < len(out); i++ {
		if !isNaN(out[i]) {
			t.Errorf("%s: out[%d] = %v, want NaN (lookback prefix)", name, i, out[i])
		}
	}
	for i := lookback; i < len(out); i++ {
		if isNaN(out[i]) || math.IsInf(out[i], 0) {
			t.Errorf("%s: out[%d] = %v, want finite value", name, i, out[i])
		}
	}
}

// --- Checkpoint 1a: shape invariants for common single-output functions ---

func TestSmaShape(t *testing.T) {
	period := 10
	out := ta.Sma(prices, period, nil)
	lb := ta.SmaLookback(period)
	assertShapeInvariants(t, "Sma", prices, out, lb)
}

func TestEmaShape(t *testing.T) {
	period := 10
	out := ta.Ema(prices, period, nil)
	lb := ta.EmaLookback(period)
	assertShapeInvariants(t, "Ema", prices, out, lb)
}

func TestRsiShape(t *testing.T) {
	period := 14
	out := ta.Rsi(prices, period, nil)
	lb := ta.RsiLookback(period)
	assertShapeInvariants(t, "Rsi", prices, out, lb)
}

func TestAtrShape(t *testing.T) {
	high, low, _, closePrice, _ := ohlcv(len(prices))
	period := 14
	out := ta.Atr(high, low, closePrice, period, nil)
	lb := ta.AtrLookback(period)
	assertShapeInvariants(t, "Atr", high, out, lb)
}

func TestAdxShape(t *testing.T) {
	high, low, _, closePrice, _ := ohlcv(len(prices))
	period := 14
	out := ta.Adx(high, low, closePrice, period, nil)
	lb := ta.AdxLookback(period)
	assertShapeInvariants(t, "Adx", high, out, lb)
}

// --- Checkpoint 1b: shape invariants for multi-output functions ---

func TestMacdShape(t *testing.T) {
	macd, signal, hist := ta.Macd(prices, 12, 26, 9, nil, nil, nil)
	lb := ta.MacdLookback(12, 26, 9)
	assertShapeInvariants(t, "Macd.macd", prices, macd, lb)
	assertShapeInvariants(t, "Macd.signal", prices, signal, lb)
	assertShapeInvariants(t, "Macd.hist", prices, hist, lb)
}

func TestBbandsShape(t *testing.T) {
	upper, middle, lower := ta.Bbands(prices, 20, 2.0, 2.0, 0, nil, nil, nil)
	lb := ta.BbandsLookback(20, 2.0, 2.0, 0)
	assertShapeInvariants(t, "Bbands.upper", prices, upper, lb)
	assertShapeInvariants(t, "Bbands.middle", prices, middle, lb)
	assertShapeInvariants(t, "Bbands.lower", prices, lower, lb)
}

func TestAroonShape(t *testing.T) {
	high, low, _, _, _ := ohlcv(len(prices))
	period := 14
	down, up := ta.Aroon(high, low, period, nil, nil)
	lb := ta.AroonLookback(period)
	assertShapeInvariants(t, "Aroon.down", high, down, lb)
	assertShapeInvariants(t, "Aroon.up", high, up, lb)
}

// --- Checkpoint 1c: outBuf reuse ---

func TestOutBufReuse(t *testing.T) {
	buf := make([]float64, len(prices))
	out := ta.Sma(prices, 10, buf)
	// Must return the same backing array.
	if &out[0] != &buf[0] {
		t.Error("Sma: outBuf with sufficient cap should be reused, but got a new allocation")
	}
}

func TestOutBufReallocWhenTooSmall(t *testing.T) {
	small := make([]float64, 1) // cap < len(prices)
	out := ta.Sma(prices, 10, small)
	if &out[0] == &small[0] {
		t.Error("Sma: outBuf with insufficient cap should trigger new allocation")
	}
	if len(out) != len(prices) {
		t.Errorf("Sma: reallocated output has length %d, want %d", len(out), len(prices))
	}
}

// --- Checkpoint 1d: panic on bad input ---

func TestPanicOnEmptyInput(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Sma(empty): expected panic, got none")
		}
		err, ok := r.(*ta.TALibError)
		if !ok {
			t.Fatalf("Sma(empty): panic value is %T, want *ta.TALibError", r)
		}
		if err.RetCode != 2 {
			t.Errorf("Sma(empty): RetCode = %d, want 2 (TA_BAD_PARAM)", err.RetCode)
		}
	}()
	ta.Sma([]float64{}, 10, nil)
}

// --- Checkpoint 1e: lookback sanity ---

func TestSmaLookback(t *testing.T) {
	// SMA(n) lookback is n-1 for n >= 2 (period=1 is invalid in TA-Lib).
	for _, tc := range []struct{ period, want int }{
		{2, 1},
		{3, 2},
		{10, 9},
		{20, 19},
	} {
		got := ta.SmaLookback(tc.period)
		if got != tc.want {
			t.Errorf("SmaLookback(%d) = %d, want %d", tc.period, got, tc.want)
		}
	}
}

// --- Checkpoint 1f: known SMA values (hand-verifiable) ---

func TestSmaKnownValues(t *testing.T) {
	in := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	out := ta.Sma(in, 3, nil)

	if len(out) != 10 {
		t.Fatalf("len(out) = %d, want 10", len(out))
	}

	// First 2 values (lookback = 2) must be NaN.
	for i := 0; i < 2; i++ {
		if !isNaN(out[i]) {
			t.Errorf("out[%d] = %v, want NaN", i, out[i])
		}
	}

	// SMA(3): average of 3 consecutive values.
	expected := []float64{2, 3, 4, 5, 6, 7, 8, 9}
	for i, want := range expected {
		got := out[i+2]
		if math.Abs(got-want) > 1e-10 {
			t.Errorf("out[%d] = %v, want %v", i+2, got, want)
		}
	}
}
