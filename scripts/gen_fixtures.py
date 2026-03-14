#!/usr/bin/env python3
# /// script
# requires-python = ">=3.10"
# dependencies = [
#   "TA-Lib",
#   "numpy",
# ]
# ///
"""
Generate golden fixture files for cgo-ta-lib tests using the Python ta-lib
wrapper as an independent reference implementation.

Both Python ta-lib and Go cgo-ta-lib wrap the same upstream C library, so
this cross-checks that the CGO bindings (index math, NaN padding, buffer
handling) are correct rather than verifying algorithmic correctness.

Usage (requires uv):
    uv run ./scripts/gen_fixtures.py

Output:
    testdata/input.csv
    testdata/expected/sma_10.csv
    testdata/expected/macd_12_26_9.csv
    testdata/expected/atr_14.csv
"""

import csv
import math
import os
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(SCRIPT_DIR)
TESTDATA_DIR = os.path.join(REPO_ROOT, "testdata")
EXPECTED_DIR = os.path.join(TESTDATA_DIR, "expected")
N = 200  # number of bars; must match cmd/gentestdata/main.go


def synthetic_ohlcv(n):
    """Deterministic OHLCV data matching cmd/gentestdata/main.go."""
    opens, highs, lows, closes, volumes = [], [], [], [], []
    for i in range(n):
        base = 100.0 + 10.0 * math.sin(i * 0.1)
        opens.append(base + 0.3)
        highs.append(base + 1.0)
        lows.append(base - 1.0)
        closes.append(base - 0.3)
        volumes.append(1e6 + i * 1000)
    return opens, highs, lows, closes, volumes


def fmt_float(v):
    if math.isnan(v):
        return "NaN"
    return f"{v:.10f}"


def write_csv(path, headers, *cols):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(headers)
        for row in zip(*cols):
            w.writerow([fmt_float(v) for v in row])
    print(f"wrote {os.path.relpath(path, REPO_ROOT)}")


def write_input(opens, highs, lows, closes, volumes):
    write_csv(
        os.path.join(TESTDATA_DIR, "input.csv"),
        ["open", "high", "low", "close", "volume"],
        opens, highs, lows, closes, volumes,
    )


def generate_expected(highs, lows, closes):
    import numpy as np
    import talib

    h = np.array(highs)
    l = np.array(lows)
    c = np.array(closes)

    # SMA(10) of close.
    sma = talib.SMA(c, timeperiod=10)
    write_csv(
        os.path.join(EXPECTED_DIR, "sma_10.csv"),
        ["out"],
        sma.tolist(),
    )

    # MACD(12, 26, 9) of close.
    macd, signal, hist = talib.MACD(c, fastperiod=12, slowperiod=26, signalperiod=9)
    write_csv(
        os.path.join(EXPECTED_DIR, "macd_12_26_9.csv"),
        ["macd", "signal", "hist"],
        macd.tolist(), signal.tolist(), hist.tolist(),
    )

    # ATR(14) of high/low/close.
    atr = talib.ATR(h, l, c, timeperiod=14)
    write_csv(
        os.path.join(EXPECTED_DIR, "atr_14.csv"),
        ["out"],
        atr.tolist(),
    )


def main():
    opens, highs, lows, closes, volumes = synthetic_ohlcv(N)
    write_input(opens, highs, lows, closes, volumes)
    generate_expected(highs, lows, closes)


if __name__ == "__main__":
    main()
