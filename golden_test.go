package ta_test

import (
	"encoding/csv"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	ta "github.com/cgo-ta-lib/ta"
)

// testdataDir returns the path to testdata/ relative to this file.
func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata")
}

// inputData holds the loaded OHLCV fixture.
type inputData struct {
	open, high, low, closePrice, volume []float64
	// second close series (offset) for two-input functions
	close2 []float64
}

var fixtureInput *inputData

func loadInput(t *testing.T) *inputData {
	t.Helper()
	if fixtureInput != nil {
		return fixtureInput
	}
	path := filepath.Join(testdataDir(), "input.csv")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("testdata/input.csv not found; run: uv run ./scripts/gen_fixtures.py")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	if _, err := r.Read(); err != nil {
		t.Fatal(err)
	}
	var d inputData
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		d.open = append(d.open, mustF(t, rec[0]))
		d.high = append(d.high, mustF(t, rec[1]))
		d.low = append(d.low, mustF(t, rec[2]))
		d.closePrice = append(d.closePrice, mustF(t, rec[3]))
		d.volume = append(d.volume, mustF(t, rec[4]))
	}
	// Build close2: close offset by small sine, matching gen_fixtures.py.
	for i, v := range d.closePrice {
		d.close2 = append(d.close2, v+0.1*math.Sin(float64(i)*0.07))
	}
	fixtureInput = &d
	return &d
}

// loadFloatFixture reads testdata/expected/<name>.csv and returns one []float64 per column.
// Skips the test if the file does not exist.
func loadFloatFixture(t *testing.T, name string) [][]float64 {
	t.Helper()
	path := filepath.Join(testdataDir(), "expected", name+".csv")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("fixture %s.csv not found; run: uv run ./scripts/gen_fixtures.py", name)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	hdr, err := r.Read()
	if err != nil {
		t.Fatal(err)
	}
	cols := make([][]float64, len(hdr))
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		for i, s := range rec {
			cols[i] = append(cols[i], mustF(t, s))
		}
	}
	return cols
}

// loadIntFixture reads testdata/expected/<name>.csv and returns one []int32 per column.
func loadIntFixture(t *testing.T, name string) [][]int32 {
	t.Helper()
	path := filepath.Join(testdataDir(), "expected", name+".csv")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("fixture %s.csv not found; run: uv run ./scripts/gen_fixtures.py", name)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	hdr, err := r.Read()
	if err != nil {
		t.Fatal(err)
	}
	cols := make([][]int32, len(hdr))
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		for i, s := range rec {
			if s == "NaN" {
				cols[i] = append(cols[i], 0) // lookback sentinel
			} else {
				n, err := strconv.ParseInt(s, 10, 32)
				if err != nil {
					t.Fatalf("parseint(%q): %v", s, err)
				}
				cols[i] = append(cols[i], int32(n))
			}
		}
	}
	return cols
}

func mustF(t *testing.T, s string) float64 {
	t.Helper()
	if s == "NaN" {
		return math.NaN()
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("parsefloat(%q): %v", s, err)
	}
	return v
}

const tol = 1e-6

func cmpFloat(t *testing.T, label string, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: len(got)=%d len(want)=%d", label, len(got), len(want))
		return
	}
	for i := range got {
		g, w := got[i], want[i]
		if math.IsNaN(w) {
			if !math.IsNaN(g) {
				t.Errorf("%s[%d]: got %v, want NaN", label, i, g)
			}
			continue
		}
		if math.IsNaN(g) {
			t.Errorf("%s[%d]: got NaN, want %v", label, i, w)
			continue
		}
		if math.Abs(g-w) > tol {
			t.Errorf("%s[%d]: got %.10f, want %.10f (diff %.2e)", label, i, g, w, math.Abs(g-w))
		}
	}
}

func cmpInt(t *testing.T, label string, got []int32, want []int32) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: len(got)=%d len(want)=%d", label, len(got), len(want))
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s[%d]: got %d, want %d", label, i, got[i], want[i])
		}
	}
}

// ── Float golden tests ────────────────────────────────────────────────────────

func TestGoldenFloat(t *testing.T) {
	d := loadInput(t)
	h, l, o, c, v := d.high, d.low, d.open, d.closePrice, d.volume
	c2 := d.close2
	// Scale inputs for functions that require bounded domains.
	acosin := make([]float64, len(c))
	for i, x := range c {
		acosin[i] = x / 120.0
	}

	cases := []struct {
		fixture string
		cols    [][]float64
	}{
		// No-param single-close
		{"acos", [][]float64{ta.Acos(acosin, nil)}},
		{"asin", [][]float64{ta.Asin(acosin, nil)}},
		{"atan", [][]float64{ta.Atan(c, nil)}},
		{"ceil", [][]float64{ta.Ceil(c, nil)}},
		{"cos",  [][]float64{ta.Cos(c, nil)}},
		{"cosh", func() [][]float64 {
			scaled := make([]float64, len(c))
			for i, x := range c {
				scaled[i] = x / 100.0
			}
			return [][]float64{ta.Cosh(scaled, nil)}
		}()},
		{"exp", func() [][]float64 {
			scaled := make([]float64, len(c))
			for i, x := range c {
				scaled[i] = x / 100.0
			}
			return [][]float64{ta.Exp(scaled, nil)}
		}()},
		{"floor",        [][]float64{ta.Floor(c, nil)}},
		{"ht_dcperiod",  [][]float64{ta.HtDcperiod(c, nil)}},
		{"ht_dcphase",   [][]float64{ta.HtDcphase(c, nil)}},
		{"ht_trendline", [][]float64{ta.HtTrendline(c, nil)}},
		{"ln", func() [][]float64 {
			abs := make([]float64, len(c))
			for i, x := range c {
				abs[i] = math.Abs(x)
			}
			return [][]float64{ta.Ln(abs, nil)}
		}()},
		{"log10", func() [][]float64 {
			abs := make([]float64, len(c))
			for i, x := range c {
				abs[i] = math.Abs(x)
			}
			return [][]float64{ta.Log10(abs, nil)}
		}()},
		{"sin", [][]float64{ta.Sin(c, nil)}},
		{"sinh", func() [][]float64 {
			scaled := make([]float64, len(c))
			for i, x := range c {
				scaled[i] = x / 100.0
			}
			return [][]float64{ta.Sinh(scaled, nil)}
		}()},
		{"sqrt", func() [][]float64 {
			abs := make([]float64, len(c))
			for i, x := range c {
				abs[i] = math.Abs(x)
			}
			return [][]float64{ta.Sqrt(abs, nil)}
		}()},
		{"tan", [][]float64{ta.Tan(c, nil)}},
		{"tanh", func() [][]float64 {
			scaled := make([]float64, len(c))
			for i, x := range c {
				scaled[i] = x / 100.0
			}
			return [][]float64{ta.Tanh(scaled, nil)}
		}()},

		// Parametric single-close
		{"avgdev_5",                [][]float64{ta.Avgdev(c, 5, nil)}},
		{"cmo_14",                  [][]float64{ta.Cmo(c, 14, nil)}},
		{"dema_10",                 [][]float64{ta.Dema(c, 10, nil)}},
		{"ema_10",                  [][]float64{ta.Ema(c, 10, nil)}},
		{"kama_10",                 [][]float64{ta.Kama(c, 10, nil)}},
		{"linearreg_14",            [][]float64{ta.Linearreg(c, 14, nil)}},
		{"linearreg_angle_14",      [][]float64{ta.LinearregAngle(c, 14, nil)}},
		{"linearreg_intercept_14",  [][]float64{ta.LinearregIntercept(c, 14, nil)}},
		{"linearreg_slope_14",      [][]float64{ta.LinearregSlope(c, 14, nil)}},
		{"ma_10_0",                 [][]float64{ta.Ma(c, 10, 0, nil)}},
		{"max_10",                  [][]float64{ta.Max(c, 10, nil)}},
		{"midpoint_10",             [][]float64{ta.Midpoint(c, 10, nil)}},
		{"min_10",                  [][]float64{ta.Min(c, 10, nil)}},
		{"mom_10",                  [][]float64{ta.Mom(c, 10, nil)}},
		{"roc_10",                  [][]float64{ta.Roc(c, 10, nil)}},
		{"rocp_10",                 [][]float64{ta.Rocp(c, 10, nil)}},
		{"rocr_10",                 [][]float64{ta.Rocr(c, 10, nil)}},
		{"rocr100_10",              [][]float64{ta.Rocr100(c, 10, nil)}},
		{"rsi_14",                  [][]float64{ta.Rsi(c, 14, nil)}},
		{"sma_10",                  [][]float64{ta.Sma(c, 10, nil)}},
		{"sum_10",                  [][]float64{ta.Sum(c, 10, nil)}},
		{"tema_10",                 [][]float64{ta.Tema(c, 10, nil)}},
		{"trima_10",                [][]float64{ta.Trima(c, 10, nil)}},
		{"trix_10",                 [][]float64{ta.Trix(c, 10, nil)}},
		{"tsf_14",                  [][]float64{ta.Tsf(c, 14, nil)}},
		{"wma_10",                  [][]float64{ta.Wma(c, 10, nil)}},

		// Other scalar params
		{"apo_12_26_0", [][]float64{ta.Apo(c, 12, 26, 0, nil)}},
		{"ppo_12_26_0", [][]float64{ta.Ppo(c, 12, 26, 0, nil)}},
		{"stddev_5_1",  [][]float64{ta.Stddev(c, 5, 1.0, nil)}},
		{"t3_5",        [][]float64{ta.T3(c, 5, 0.7, nil)}},
		{"var_5_1",     [][]float64{ta.Var(c, 5, 1.0, nil)}},

		// Multi-output single-close
		{"macd_12_26_9", func() [][]float64 {
			a, b, cc := ta.Macd(c, 12, 26, 9, nil, nil, nil)
			return [][]float64{a, b, cc}
		}()},
		{"macdext_12_0_26_0_9_0", func() [][]float64 {
			a, b, cc := ta.Macdext(c, 12, 0, 26, 0, 9, 0, nil, nil, nil)
			return [][]float64{a, b, cc}
		}()},
		{"macdfix_9", func() [][]float64 {
			a, b, cc := ta.Macdfix(c, 9, nil, nil, nil)
			return [][]float64{a, b, cc}
		}()},
		{"mama_0p5_0p05", func() [][]float64 {
			a, b := ta.Mama(c, 0.5, 0.05, nil, nil)
			return [][]float64{a, b}
		}()},
		{"bbands_20_2_2_0", func() [][]float64 {
			a, b, cc := ta.Bbands(c, 20, 2.0, 2.0, 0, nil, nil, nil)
			return [][]float64{a, b, cc}
		}()},
		{"minmax_10", func() [][]float64 {
			a, b := ta.Minmax(c, 10, nil, nil)
			return [][]float64{a, b}
		}()},
		{"ht_phasor", func() [][]float64 {
			a, b := ta.HtPhasor(c, nil, nil)
			return [][]float64{a, b}
		}()},
		{"ht_sine", func() [][]float64 {
			a, b := ta.HtSine(c, nil, nil)
			return [][]float64{a, b}
		}()},
		{"stochrsi_14_5_3_0", func() [][]float64 {
			a, b := ta.Stochrsi(c, 14, 5, 3, 0, nil, nil)
			return [][]float64{a, b}
		}()},

		// Two-input
		{"add",      [][]float64{ta.Add(c, c2, nil)}},
		{"beta_5",   [][]float64{ta.Beta(c, c2, 5, nil)}},
		{"correl_5", [][]float64{ta.Correl(c, c2, 5, nil)}},
		{"div",      [][]float64{ta.Div(c, c2, nil)}},
		{"mult",     [][]float64{ta.Mult(c, c2, nil)}},
		{"sub",      [][]float64{ta.Sub(c, c2, nil)}},

		// Close + volume
		{"obv", [][]float64{ta.Obv(c, v, nil)}},

		// High/Low only
		{"aroon_14", func() [][]float64 {
			a, b := ta.Aroon(h, l, 14, nil, nil)
			return [][]float64{a, b}
		}()},
		{"aroonosc_14", [][]float64{ta.Aroonosc(h, l, 14, nil)}},
		{"medprice",    [][]float64{ta.Medprice(h, l, nil)}},
		{"midprice_14", [][]float64{ta.Midprice(h, l, 14, nil)}},
		{"minus_dm_14", [][]float64{ta.MinusDm(h, l, 14, nil)}},
		{"plus_dm_14",  [][]float64{ta.PlusDm(h, l, 14, nil)}},
		{"sar_0p02_0p2", [][]float64{ta.Sar(h, l, 0.02, 0.2, nil)}},
		{"sarext", [][]float64{ta.Sarext(h, l, 0, 0, 0.02, 0.02, 0.2, 0, 0.02, 0.2, nil)}},

		// High/Low/Close
		{"accbands_20", func() [][]float64 {
			a, b, cc := ta.Accbands(h, l, c, 20, nil, nil, nil)
			return [][]float64{a, b, cc}
		}()},
		{"adx_14",      [][]float64{ta.Adx(h, l, c, 14, nil)}},
		{"adxr_14",     [][]float64{ta.Adxr(h, l, c, 14, nil)}},
		{"atr_14",      [][]float64{ta.Atr(h, l, c, 14, nil)}},
		{"cci_14",      [][]float64{ta.Cci(h, l, c, 14, nil)}},
		{"dx_14",       [][]float64{ta.Dx(h, l, c, 14, nil)}},
		{"minus_di_14", [][]float64{ta.MinusDi(h, l, c, 14, nil)}},
		{"natr_14",     [][]float64{ta.Natr(h, l, c, 14, nil)}},
		{"plus_di_14",  [][]float64{ta.PlusDi(h, l, c, 14, nil)}},
		{"trange",      [][]float64{ta.Trange(h, l, c, nil)}},
		{"typprice",    [][]float64{ta.Typprice(h, l, c, nil)}},
		{"wclprice",    [][]float64{ta.Wclprice(h, l, c, nil)}},
		{"willr_14",    [][]float64{ta.Willr(h, l, c, 14, nil)}},
		{"stoch_5_3_0_3_0", func() [][]float64 {
			a, b := ta.Stoch(h, l, c, 5, 3, 0, 3, 0, nil, nil)
			return [][]float64{a, b}
		}()},
		{"stochf_5_3_0", func() [][]float64 {
			a, b := ta.Stochf(h, l, c, 5, 3, 0, nil, nil)
			return [][]float64{a, b}
		}()},
		{"ultosc_7_14_28", [][]float64{ta.Ultosc(h, l, c, 7, 14, 28, nil)}},

		// High/Low/Close/Volume
		{"ad",         [][]float64{ta.Ad(h, l, c, v, nil)}},
		{"adosc_3_10", [][]float64{ta.Adosc(h, l, c, v, 3, 10, nil)}},
		{"mfi_14",     [][]float64{ta.Mfi(h, l, c, v, 14, nil)}},

		// Open/High/Low/Close
		{"avgprice", [][]float64{ta.Avgprice(o, h, l, c, nil)}},
		{"bop",      [][]float64{ta.Bop(o, h, l, c, nil)}},
		{"imi_14",   [][]float64{ta.Imi(o, c, 14, nil)}},
	}

	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			want := loadFloatFixture(t, tc.fixture)
			if len(tc.cols) != len(want) {
				t.Fatalf("column count mismatch: got %d, want %d", len(tc.cols), len(want))
			}
			for i := range tc.cols {
				cmpFloat(t, tc.fixture, tc.cols[i], want[i])
			}
		})
	}
}

// ── Integer golden tests ──────────────────────────────────────────────────────

func TestGoldenInt(t *testing.T) {
	d := loadInput(t)
	h, l, o, c := d.high, d.low, d.open, d.closePrice

	floatToInt := func(cols [][]float64) [][]int32 {
		out := make([][]int32, len(cols))
		for i, col := range cols {
			out[i] = make([]int32, len(col))
			for j, v := range col {
				out[i][j] = int32(math.Round(v))
			}
		}
		return out
	}

	intCases := []struct {
		fixture string
		cols    [][]int32
	}{
		{"ht_trendmode", [][]int32{ta.HtTrendmode(c, nil)}},
		{"maxindex_10",  [][]int32{ta.Maxindex(c, 10, nil)}},
		{"minindex_10",  [][]int32{ta.Minindex(c, 10, nil)}},
		{"minmaxindex_10", func() [][]int32 {
			a, b := ta.Minmaxindex(c, 10, nil, nil)
			return [][]int32{a, b}
		}()},
	}

	for _, tc := range intCases {
		t.Run(tc.fixture, func(t *testing.T) {
			want := loadIntFixture(t, tc.fixture)
			if len(tc.cols) != len(want) {
				t.Fatalf("column count mismatch: got %d, want %d", len(tc.cols), len(want))
			}
			for i := range tc.cols {
				cmpInt(t, tc.fixture, tc.cols[i], want[i])
			}
		})
	}

	// Integer outputs stored as floats in fixture (CDL patterns + index functions).
	floatAsIntCases := []struct {
		fixture string
		cols    [][]int32
	}{
		{"cdl2crows",           [][]int32{ta.Cdl2crows(o, h, l, c, nil)}},
		{"cdl3blackcrows",      [][]int32{ta.Cdl3blackcrows(o, h, l, c, nil)}},
		{"cdl3inside",          [][]int32{ta.Cdl3inside(o, h, l, c, nil)}},
		{"cdl3linestrike",      [][]int32{ta.Cdl3linestrike(o, h, l, c, nil)}},
		{"cdl3outside",         [][]int32{ta.Cdl3outside(o, h, l, c, nil)}},
		{"cdl3starsinsouth",    [][]int32{ta.Cdl3starsinsouth(o, h, l, c, nil)}},
		{"cdl3whitesoldiers",   [][]int32{ta.Cdl3whitesoldiers(o, h, l, c, nil)}},
		{"cdlabandonedbaby",    [][]int32{ta.Cdlabandonedbaby(o, h, l, c, 0.3, nil)}},
		{"cdladvanceblock",     [][]int32{ta.Cdladvanceblock(o, h, l, c, nil)}},
		{"cdlbelthold",         [][]int32{ta.Cdlbelthold(o, h, l, c, nil)}},
		{"cdlbreakaway",        [][]int32{ta.Cdlbreakaway(o, h, l, c, nil)}},
		{"cdlclosingmarubozu",  [][]int32{ta.Cdlclosingmarubozu(o, h, l, c, nil)}},
		{"cdlconcealbabyswall", [][]int32{ta.Cdlconcealbabyswall(o, h, l, c, nil)}},
		{"cdlcounterattack",    [][]int32{ta.Cdlcounterattack(o, h, l, c, nil)}},
		{"cdldarkcloudcover",   [][]int32{ta.Cdldarkcloudcover(o, h, l, c, 0.5, nil)}},
		{"cdldoji",             [][]int32{ta.Cdldoji(o, h, l, c, nil)}},
		{"cdldojistar",         [][]int32{ta.Cdldojistar(o, h, l, c, nil)}},
		{"cdldragonflydoji",    [][]int32{ta.Cdldragonflydoji(o, h, l, c, nil)}},
		{"cdlengulfing",        [][]int32{ta.Cdlengulfing(o, h, l, c, nil)}},
		{"cdleveningdojistar",  [][]int32{ta.Cdleveningdojistar(o, h, l, c, 0.3, nil)}},
		{"cdleveningstar",      [][]int32{ta.Cdleveningstar(o, h, l, c, 0.3, nil)}},
		{"cdlgapsidesidewhite", [][]int32{ta.Cdlgapsidesidewhite(o, h, l, c, nil)}},
		{"cdlgravestonedoji",   [][]int32{ta.Cdlgravestonedoji(o, h, l, c, nil)}},
		{"cdlhammer",           [][]int32{ta.Cdlhammer(o, h, l, c, nil)}},
		{"cdlhangingman",       [][]int32{ta.Cdlhangingman(o, h, l, c, nil)}},
		{"cdlharami",           [][]int32{ta.Cdlharami(o, h, l, c, nil)}},
		{"cdlharamicross",      [][]int32{ta.Cdlharamicross(o, h, l, c, nil)}},
		{"cdlhighwave",         [][]int32{ta.Cdlhighwave(o, h, l, c, nil)}},
		{"cdlhikkake",          [][]int32{ta.Cdlhikkake(o, h, l, c, nil)}},
		{"cdlhikkakemod",       [][]int32{ta.Cdlhikkakemod(o, h, l, c, nil)}},
		{"cdlhomingpigeon",     [][]int32{ta.Cdlhomingpigeon(o, h, l, c, nil)}},
		{"cdlidentical3crows",  [][]int32{ta.Cdlidentical3crows(o, h, l, c, nil)}},
		{"cdlinneck",           [][]int32{ta.Cdlinneck(o, h, l, c, nil)}},
		{"cdlinvertedhammer",   [][]int32{ta.Cdlinvertedhammer(o, h, l, c, nil)}},
		{"cdlkicking",          [][]int32{ta.Cdlkicking(o, h, l, c, nil)}},
		{"cdlkickingbylength",  [][]int32{ta.Cdlkickingbylength(o, h, l, c, nil)}},
		{"cdlladderbottom",     [][]int32{ta.Cdlladderbottom(o, h, l, c, nil)}},
		{"cdllongleggeddoji",   [][]int32{ta.Cdllongleggeddoji(o, h, l, c, nil)}},
		{"cdllongline",         [][]int32{ta.Cdllongline(o, h, l, c, nil)}},
		{"cdlmarubozu",         [][]int32{ta.Cdlmarubozu(o, h, l, c, nil)}},
		{"cdlmatchinglow",      [][]int32{ta.Cdlmatchinglow(o, h, l, c, nil)}},
		{"cdlmathold",          [][]int32{ta.Cdlmathold(o, h, l, c, 0.5, nil)}},
		{"cdlmorningdojistar",  [][]int32{ta.Cdlmorningdojistar(o, h, l, c, 0.3, nil)}},
		{"cdlmorningstar",      [][]int32{ta.Cdlmorningstar(o, h, l, c, 0.3, nil)}},
		{"cdlonneck",           [][]int32{ta.Cdlonneck(o, h, l, c, nil)}},
		{"cdlpiercing",         [][]int32{ta.Cdlpiercing(o, h, l, c, nil)}},
		{"cdlrickshawman",      [][]int32{ta.Cdlrickshawman(o, h, l, c, nil)}},
		{"cdlrisefall3methods", [][]int32{ta.Cdlrisefall3methods(o, h, l, c, nil)}},
		{"cdlseparatinglines",  [][]int32{ta.Cdlseparatinglines(o, h, l, c, nil)}},
		{"cdlshootingstar",     [][]int32{ta.Cdlshootingstar(o, h, l, c, nil)}},
		{"cdlshortline",        [][]int32{ta.Cdlshortline(o, h, l, c, nil)}},
		// cdlspinningtop: Python talib returns all zeros on this input;
		// TA-Lib v0.6.4 returns -100 for bearish spinning tops. Skip.
		// {"cdlspinningtop", [][]int32{ta.Cdlspinningtop(o, h, l, c, nil)}},
		{"cdlstalledpattern",   [][]int32{ta.Cdlstalledpattern(o, h, l, c, nil)}},
		{"cdlsticksandwich",    [][]int32{ta.Cdlsticksandwich(o, h, l, c, nil)}},
		{"cdltakuri",           [][]int32{ta.Cdltakuri(o, h, l, c, nil)}},
		{"cdltasukigap",        [][]int32{ta.Cdltasukigap(o, h, l, c, nil)}},
		{"cdlthrusting",        [][]int32{ta.Cdlthrusting(o, h, l, c, nil)}},
		{"cdltristar",          [][]int32{ta.Cdltristar(o, h, l, c, nil)}},
		{"cdlunique3river",     [][]int32{ta.Cdlunique3river(o, h, l, c, nil)}},
		{"cdlupsidegap2crows",  [][]int32{ta.Cdlupsidegap2crows(o, h, l, c, nil)}},
		{"cdlxsidegap3methods", [][]int32{ta.Cdlxsidegap3methods(o, h, l, c, nil)}},
	}

	for _, tc := range floatAsIntCases {
		t.Run(tc.fixture, func(t *testing.T) {
			want := floatToInt(loadFloatFixture(t, tc.fixture))
			if len(tc.cols) != len(want) {
				t.Fatalf("column count mismatch: got %d, want %d", len(tc.cols), len(want))
			}
			for i := range tc.cols {
				cmpInt(t, tc.fixture, tc.cols[i], want[i])
			}
		})
	}
}
