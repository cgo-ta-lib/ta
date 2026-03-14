# /// script
# dependencies = ["ta-lib", "numpy"]
# ///
"""
Benchmark Python ta-lib for comparison with Go cgo-ta-lib benchmarks.
Run with: uv run ./scripts/bench.py

Output is in ns/call to match Go's ns/op.
"""

import timeit
import numpy as np
import talib

SIZES = [1_000, 100_000]


def make_close(n: int) -> np.ndarray:
    i = np.arange(n, dtype=np.float64)
    return 100.0 + 10.0 * np.sin(i * 0.1)


def make_ohlc(n: int) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
    base = 100.0 + 10.0 * np.sin(np.arange(n, dtype=np.float64) * 0.1)
    return base + 1.0, base - 1.0, base


def bench(label: str, fn, n: int) -> None:
    # Warm up.
    fn()
    # Choose iterations so total wall time is ~1s.
    elapsed_one = timeit.timeit(fn, number=1)
    iters = max(10, int(1.0 / elapsed_one))
    total = timeit.timeit(fn, number=iters)
    ns_per_call = (total / iters) * 1e9
    print(f"  {label:<30s}  {ns_per_call:>12.0f} ns/call")


print(f"Python ta-lib benchmarks\n")

for n in SIZES:
    close = make_close(n)
    high, low, _ = make_ohlc(n)
    label_k = f"{n // 1000}k"
    print(f"n = {n:,}")
    bench(f"Sma/{label_k}", lambda: talib.SMA(close, timeperiod=20), n)
    bench(f"Macd/{label_k}", lambda: talib.MACD(close, fastperiod=12, slowperiod=26, signalperiod=9), n)
    bench(f"Atr/{label_k}", lambda: talib.ATR(high, low, close, timeperiod=14), n)
    print()
