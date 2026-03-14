# cgo-ta-lib

Go bindings for [TA-Lib](https://github.com/TA-Lib/ta-lib) that embed the C source directly —
no system library installation required. Users need only `go get`.  A working CGO environment is the only requirement, works out of the box with typical Go toolchain installations.

## Quick start

```go
import ta "github.com/cgo-ta-lib/ta"

// SMA over 10 prices with a 3-bar period.
prices := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
result := ta.Sma(prices, 3, nil)
// result: [NaN, NaN, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0]

// MACD.
macd, signal, hist := ta.Macd(prices, 12, 26, 9, nil, nil, nil)
```

## outBuf pattern

Pass a slice as the last argument(s) to reuse an existing buffer and avoid allocation:

```go
buf := make([]float64, 0, 1000) // allocate once

for _, batch := range batches {
    result := ta.Sma(batch, 20, buf) // reuses buf if cap >= len(batch)
    process(result)
    buf = result // keep the buffer for next iteration
}
```

If `outBuf` is nil or its capacity is less than the input length, a new slice is allocated.

## Lookback and NaN padding

All output slices have the same length as the input. The first `Lookback` positions are filled
with `NaN` (the "warmup" period where insufficient data exists for a valid output).

```go
lb := ta.SmaLookback(3) // returns 2
// The first 2 values of Sma(..., 3, nil) will be NaN.
```

Use lookback functions to sub-slice into valid data:

```go
result := ta.Sma(prices, 20, nil)
lb := ta.SmaLookback(20)
validResult := result[lb:] // only valid (non-NaN) values
```

## Platform support

Requires CGO. Tested on macOS and Linux. Windows support (MinGW) is planned but untested.

## Rationale

Existing Go TA-Lib approaches have significant trade-offs:

- **Pure-Go reimplementations** diverge from the reference C implementation and require ongoing
  maintenance to stay in sync. Any upstream bug fix or edge-case change must be manually ported.
- **System-library wrappers** require TA-Lib to be pre-installed (`apt install`, `brew install`,
  etc.), complicating builds, CI, and distribution — especially across macOS and Linux.

This library takes a different approach: the TA-Lib C source is embedded directly as an
amalgamation file (`talib_amalgamation.c`), committed to the repository. CGO compiles it as
part of the normal `go build`. Algorithm correctness is guaranteed by the upstream C
implementation, and you get a single `go get` with no external dependencies.

The API is designed for high-frequency and batch processing workloads. Callers control output
allocation via the `outBuf` parameter pattern — buffers can be reused across calls to eliminate
allocations in tight loops.

## Tests

A sensible set of tests are included, some of them generated.  The objective with the test suite is to provide some decent coverage that provides some proof this thing actually works, while not introducing a significant maintenance burden of trying to exhaustively prove what TA-Lib already implements and handles.

## Maintenenace - Updating TA-Lib

If you wish to do workon this wrapper library or update it, you can:

- Edit the version tag in the `//go:generate` directive in `talib.go` as needed.
- Run `go generate` from the repository root.
- Run `go test -v -count=1 ./...` to make sure it's all working
- Commit `include/`, `talib_amalgamation.c`, and `*_gen.go` files.

The `go generate` step requires additional tooling including CMake, `uv` and Python.  It will download the TA-Lib source and (re)generate the various function wrappers and test code.  Python is used to generate sample outputs using it's own TA-Lib bindings which we then verify in Go tests.

Note that it's always possible some structural change in a new TA-Lib version will require the generator code to be updated.  YMMV.
