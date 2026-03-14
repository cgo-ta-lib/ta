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

// loadInputCSV reads testdata/input.csv and returns OHLCV slices.
// Skips the test if the file does not exist.
func loadInputCSV(t *testing.T) (open, high, low, closePrice, volume []float64) {
	t.Helper()
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
	// Skip header.
	if _, err := r.Read(); err != nil {
		t.Fatal(err)
	}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		open = append(open, mustParseFloat(t, rec[0]))
		high = append(high, mustParseFloat(t, rec[1]))
		low = append(low, mustParseFloat(t, rec[2]))
		closePrice = append(closePrice, mustParseFloat(t, rec[3]))
		volume = append(volume, mustParseFloat(t, rec[4]))
	}
	return
}

// loadExpectedCSV reads testdata/expected/<name> and returns one slice per column.
// Skips the test if the file does not exist.
func loadExpectedCSV(t *testing.T, name string) [][]float64 {
	t.Helper()
	path := filepath.Join(testdataDir(), "expected", name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("testdata/expected/%s not found; run: uv run ./scripts/gen_fixtures.py", name)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		t.Fatal(err)
	}
	cols := make([][]float64, len(header))

	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		for i, v := range rec {
			cols[i] = append(cols[i], mustParseFloat(t, v))
		}
	}
	return cols
}

func mustParseFloat(t *testing.T, s string) float64 {
	t.Helper()
	if s == "NaN" {
		return math.NaN()
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("parseFloat(%q): %v", s, err)
	}
	return v
}

// assertGolden compares got against want element-by-element within tol.
// Both NaN positions must match; non-NaN values must be within tol.
func assertGolden(t *testing.T, label string, got, want []float64, tol float64) {
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
			t.Errorf("%s[%d]: got %.10f, want %.10f (diff %.2e > tol %.2e)",
				label, i, g, w, math.Abs(g-w), tol)
		}
	}
}

// --- Golden tests ---

func TestSmaGolden(t *testing.T) {
	_, _, _, closePrice, _ := loadInputCSV(t)
	cols := loadExpectedCSV(t, "sma_10.csv")

	got := ta.Sma(closePrice, 10, nil)
	assertGolden(t, "Sma(close,10)", got, cols[0], 1e-8)
}

func TestMacdGolden(t *testing.T) {
	_, _, _, closePrice, _ := loadInputCSV(t)
	cols := loadExpectedCSV(t, "macd_12_26_9.csv")

	macd, signal, hist := ta.Macd(closePrice, 12, 26, 9, nil, nil, nil)
	assertGolden(t, "Macd.macd", macd, cols[0], 1e-8)
	assertGolden(t, "Macd.signal", signal, cols[1], 1e-8)
	assertGolden(t, "Macd.hist", hist, cols[2], 1e-8)
}

func TestAtrGolden(t *testing.T) {
	_, high, low, closePrice, _ := loadInputCSV(t)
	cols := loadExpectedCSV(t, "atr_14.csv")

	got := ta.Atr(high, low, closePrice, 14, nil)
	assertGolden(t, "Atr(14)", got, cols[0], 1e-8)
}
