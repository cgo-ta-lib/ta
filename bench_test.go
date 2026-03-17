package ta_test

import (
	"fmt"
	"math"
	"testing"

	ta "github.com/TA-Lib/ta-lib-cgo"
)

// makeClose generates a sine-wave close price series of length n.
func makeClose(n int) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = 100.0 + 10.0*math.Sin(float64(i)*0.1)
	}
	return s
}

// makeOHLC generates parallel high/low/close slices of length n.
func makeOHLC(n int) (high, low, close []float64) {
	high = make([]float64, n)
	low = make([]float64, n)
	close = make([]float64, n)
	for i := range high {
		base := 100.0 + 10.0*math.Sin(float64(i)*0.1)
		high[i] = base + 1.0
		low[i] = base - 1.0
		close[i] = base
	}
	return
}

var benchSizes = []int{1_000, 100_000}

// -- SMA --

func BenchmarkSma(b *testing.B) {
	for _, n := range benchSizes {
		data := makeClose(n)
		b.Run(fmt.Sprintf("%dk/alloc", n/1000), func(b *testing.B) {
			for b.Loop() {
				ta.Sma(data, 20, nil)
			}
		})
		buf := make([]float64, n)
		b.Run(fmt.Sprintf("%dk/reuse", n/1000), func(b *testing.B) {
			for b.Loop() {
				ta.Sma(data, 20, buf)
			}
		})
	}
}

// -- MACD --

func BenchmarkMacd(b *testing.B) {
	for _, n := range benchSizes {
		data := makeClose(n)
		b.Run(fmt.Sprintf("%dk/alloc", n/1000), func(b *testing.B) {
			for b.Loop() {
				ta.Macd(data, 12, 26, 9, nil, nil, nil)
			}
		})
		macdBuf := make([]float64, n)
		sigBuf := make([]float64, n)
		histBuf := make([]float64, n)
		b.Run(fmt.Sprintf("%dk/reuse", n/1000), func(b *testing.B) {
			for b.Loop() {
				ta.Macd(data, 12, 26, 9, macdBuf, sigBuf, histBuf)
			}
		})
	}
}

// -- ATR --

func BenchmarkAtr(b *testing.B) {
	for _, n := range benchSizes {
		high, low, close := makeOHLC(n)
		b.Run(fmt.Sprintf("%dk/alloc", n/1000), func(b *testing.B) {
			for b.Loop() {
				ta.Atr(high, low, close, 14, nil)
			}
		})
		buf := make([]float64, n)
		b.Run(fmt.Sprintf("%dk/reuse", n/1000), func(b *testing.B) {
			for b.Loop() {
				ta.Atr(high, low, close, 14, buf)
			}
		})
	}
}
