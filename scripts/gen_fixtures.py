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

Usage (requires uv):
    uv run ./scripts/gen_fixtures.py

Output:
    testdata/input.csv
    testdata/expected/<indicator>.csv
"""

import csv
import math
import os

import numpy as np
import talib

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(SCRIPT_DIR)
TESTDATA_DIR = os.path.join(REPO_ROOT, "testdata")
EXPECTED_DIR = os.path.join(TESTDATA_DIR, "expected")
N = 200


def synthetic_ohlcv(n):
    opens, highs, lows, closes, volumes = [], [], [], [], []
    for i in range(n):
        base = 100.0 + 10.0 * math.sin(i * 0.1)
        opens.append(base + 0.3)
        highs.append(base + 1.0)
        lows.append(base - 1.0)
        closes.append(base - 0.3)
        volumes.append(1e6 + i * 1000)
    return opens, highs, lows, closes, volumes


def fmt(v):
    if isinstance(v, (int, np.integer)):
        return str(int(v))
    if math.isnan(float(v)):
        return "NaN"
    return f"{float(v):.10f}"


def write_fixture(name, headers, *cols):
    os.makedirs(EXPECTED_DIR, exist_ok=True)
    path = os.path.join(EXPECTED_DIR, name + ".csv")
    with open(path, "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(headers)
        for row in zip(*cols):
            w.writerow([fmt(v) for v in row])


def write_input(opens, highs, lows, closes, volumes):
    os.makedirs(TESTDATA_DIR, exist_ok=True)
    path = os.path.join(TESTDATA_DIR, "input.csv")
    with open(path, "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["open", "high", "low", "close", "volume"])
        for row in zip(opens, highs, lows, closes, volumes):
            w.writerow([f"{v:.10f}" for v in row])
    print(f"wrote testdata/input.csv")


def gen(name, headers, *arrays):
    write_fixture(name, headers, *[a.tolist() for a in arrays])
    print(f"wrote testdata/expected/{name}.csv")


def main():
    opens_l, highs_l, lows_l, closes_l, volumes_l = synthetic_ohlcv(N)
    write_input(opens_l, highs_l, lows_l, closes_l, volumes_l)

    o = np.array(opens_l)
    h = np.array(highs_l)
    l = np.array(lows_l)
    c = np.array(closes_l)
    v = np.array(volumes_l)
    # Second close series (shifted by small offset) for two-input functions.
    c2 = np.array([x + 0.1 * math.sin(i * 0.07) for i, x in enumerate(closes_l)])

    # ── Single close input, no params ─────────────────────────────────────────
    # Only valid for positive inputs; use abs(c) for log functions.
    ac = np.abs(c)
    gen("acos",    ["out"], talib.ACOS(ac / 120.0))   # scale to [-1,1]
    gen("asin",    ["out"], talib.ASIN(ac / 120.0))
    gen("atan",    ["out"], talib.ATAN(c))
    gen("ceil",    ["out"], talib.CEIL(c))
    gen("cos",     ["out"], talib.COS(c))
    gen("cosh",    ["out"], talib.COSH(c / 100.0))    # scale to avoid overflow
    gen("exp",     ["out"], talib.EXP(c / 100.0))     # scale to avoid overflow
    gen("floor",   ["out"], talib.FLOOR(c))
    gen("ht_dcperiod",  ["out"], talib.HT_DCPERIOD(c))
    gen("ht_dcphase",   ["out"], talib.HT_DCPHASE(c))
    gen("ht_trendline", ["out"], talib.HT_TRENDLINE(c))
    gen("ln",      ["out"], talib.LN(ac))
    gen("log10",   ["out"], talib.LOG10(ac))
    gen("sin",     ["out"], talib.SIN(c))
    gen("sinh",    ["out"], talib.SINH(c / 100.0))    # scale to avoid overflow
    gen("sqrt",    ["out"], talib.SQRT(ac))
    gen("tan",     ["out"], talib.TAN(c))
    gen("tanh",    ["out"], talib.TANH(c / 100.0))    # scale to avoid overflow

    # ── Single close input, period param ──────────────────────────────────────
    gen("avgdev_5",          ["out"], talib.AVGDEV(c, 5))
    gen("cmo_14",            ["out"], talib.CMO(c, 14))
    gen("dema_10",           ["out"], talib.DEMA(c, 10))
    gen("ema_10",            ["out"], talib.EMA(c, 10))
    gen("kama_10",           ["out"], talib.KAMA(c, 10))
    gen("linearreg_14",      ["out"], talib.LINEARREG(c, 14))
    gen("linearreg_angle_14",["out"], talib.LINEARREG_ANGLE(c, 14))
    gen("linearreg_intercept_14", ["out"], talib.LINEARREG_INTERCEPT(c, 14))
    gen("linearreg_slope_14",["out"], talib.LINEARREG_SLOPE(c, 14))
    gen("ma_10_0",           ["out"], talib.MA(c, 10, 0))
    gen("max_10",            ["out"], talib.MAX(c, 10))
    gen("midpoint_10",       ["out"], talib.MIDPOINT(c, 10))
    gen("min_10",            ["out"], talib.MIN(c, 10))
    gen("mom_10",            ["out"], talib.MOM(c, 10))
    gen("roc_10",            ["out"], talib.ROC(c, 10))
    gen("rocp_10",           ["out"], talib.ROCP(c, 10))
    gen("rocr_10",           ["out"], talib.ROCR(c, 10))
    gen("rocr100_10",        ["out"], talib.ROCR100(c, 10))
    gen("rsi_14",            ["out"], talib.RSI(c, 14))
    gen("sma_10",            ["out"], talib.SMA(c, 10))
    gen("sum_10",            ["out"], talib.SUM(c, 10))
    gen("tema_10",           ["out"], talib.TEMA(c, 10))
    gen("trima_10",          ["out"], talib.TRIMA(c, 10))
    gen("trix_10",           ["out"], talib.TRIX(c, 10))
    gen("tsf_14",            ["out"], talib.TSF(c, 14))
    gen("wma_10",            ["out"], talib.WMA(c, 10))

    # ── Single close input, other scalar params ────────────────────────────────
    gen("apo_12_26_0",  ["out"], talib.APO(c, 12, 26, 0))
    gen("ppo_12_26_0",  ["out"], talib.PPO(c, 12, 26, 0))
    gen("stddev_5_1",   ["out"], talib.STDDEV(c, 5, 1.0))
    gen("t3_5",         ["out"], talib.T3(c, 5, 0.7))
    gen("var_5_1",      ["out"], talib.VAR(c, 5, 1.0))

    # ── Single close input, multi float output ─────────────────────────────────
    macd, sig, hist = talib.MACD(c, 12, 26, 9)
    gen("macd_12_26_9", ["macd", "signal", "hist"], macd, sig, hist)

    macd2, sig2, hist2 = talib.MACDEXT(c, 12, 0, 26, 0, 9, 0)
    gen("macdext_12_0_26_0_9_0", ["macd", "signal", "hist"], macd2, sig2, hist2)

    macd3, sig3, hist3 = talib.MACDFIX(c, 9)
    gen("macdfix_9", ["macd", "signal", "hist"], macd3, sig3, hist3)

    mama, fama = talib.MAMA(c, 0.5, 0.05)
    gen("mama_0p5_0p05", ["mama", "fama"], mama, fama)

    upper, middle, lower = talib.BBANDS(c, 20, 2.0, 2.0, 0)
    gen("bbands_20_2_2_0", ["upper", "middle", "lower"], upper, middle, lower)

    mn, mx = talib.MINMAX(c, 10)
    gen("minmax_10", ["min", "max"], mn, mx)

    inphase, quad = talib.HT_PHASOR(c)
    gen("ht_phasor", ["inphase", "quadrature"], inphase, quad)

    sine, leadsine = talib.HT_SINE(c)
    gen("ht_sine", ["sine", "leadsine"], sine, leadsine)

    stochrsi_k, stochrsi_d = talib.STOCHRSI(c, 14, 5, 3, 0)
    gen("stochrsi_14_5_3_0", ["fastk", "fastd"], stochrsi_k, stochrsi_d)

    # ── Single close input, integer output ────────────────────────────────────
    gen("ht_trendmode", ["out"], talib.HT_TRENDMODE(c))
    gen("maxindex_10",  ["out"], talib.MAXINDEX(c, 10))
    gen("minindex_10",  ["out"], talib.MININDEX(c, 10))

    # ── Single close input, multi integer output ───────────────────────────────
    mnidx, mxidx = talib.MINMAXINDEX(c, 10)
    gen("minmaxindex_10", ["minidx", "maxidx"], mnidx, mxidx)

    # ── Two close inputs ───────────────────────────────────────────────────────
    gen("add",       ["out"], talib.ADD(c, c2))
    gen("beta_5",    ["out"], talib.BETA(c, c2, 5))
    gen("correl_5",  ["out"], talib.CORREL(c, c2, 5))
    gen("div",       ["out"], talib.DIV(c, c2))
    gen("mult",      ["out"], talib.MULT(c, c2))
    gen("sub",       ["out"], talib.SUB(c, c2))

    # ── Close + volume ─────────────────────────────────────────────────────────
    gen("obv", ["out"], talib.OBV(c, v))

    # ── High/Low only ──────────────────────────────────────────────────────────
    aroon_down, aroon_up = talib.AROON(h, l, 14)
    gen("aroon_14", ["down", "up"], aroon_down, aroon_up)

    gen("aroonosc_14", ["out"], talib.AROONOSC(h, l, 14))
    gen("medprice",    ["out"], talib.MEDPRICE(h, l))
    gen("midprice_14", ["out"], talib.MIDPRICE(h, l, 14))
    gen("minus_dm_14", ["out"], talib.MINUS_DM(h, l, 14))
    gen("plus_dm_14",  ["out"], talib.PLUS_DM(h, l, 14))
    gen("sar_0p02_0p2",["out"], talib.SAR(h, l, 0.02, 0.2))
    gen("sarext",      ["out"], talib.SAREXT(h, l, 0, 0, 0.02, 0.02, 0.2, 0, 0.02, 0.2))

    # ── High/Low/Close ─────────────────────────────────────────────────────────
    gen("accbands_20",  ["upper", "middle", "lower"],
        *talib.ACCBANDS(h, l, c, 20))
    gen("adx_14",       ["out"], talib.ADX(h, l, c, 14))
    gen("adxr_14",      ["out"], talib.ADXR(h, l, c, 14))
    gen("atr_14",       ["out"], talib.ATR(h, l, c, 14))
    gen("cci_14",       ["out"], talib.CCI(h, l, c, 14))
    gen("dx_14",        ["out"], talib.DX(h, l, c, 14))
    gen("minus_di_14",  ["out"], talib.MINUS_DI(h, l, c, 14))
    gen("natr_14",      ["out"], talib.NATR(h, l, c, 14))
    gen("plus_di_14",   ["out"], talib.PLUS_DI(h, l, c, 14))
    gen("trange",       ["out"], talib.TRANGE(h, l, c))
    gen("typprice",     ["out"], talib.TYPPRICE(h, l, c))
    gen("wclprice",     ["out"], talib.WCLPRICE(h, l, c))
    gen("willr_14",     ["out"], talib.WILLR(h, l, c, 14))

    slowk, slowd = talib.STOCH(h, l, c, 5, 3, 0, 3, 0)
    gen("stoch_5_3_0_3_0", ["slowk", "slowd"], slowk, slowd)

    fastk, fastd = talib.STOCHF(h, l, c, 5, 3, 0)
    gen("stochf_5_3_0", ["fastk", "fastd"], fastk, fastd)

    gen("ultosc_7_14_28", ["out"], talib.ULTOSC(h, l, c, 7, 14, 28))

    # ── High/Low/Close/Volume ──────────────────────────────────────────────────
    gen("ad",         ["out"], talib.AD(h, l, c, v))
    gen("adosc_3_10", ["out"], talib.ADOSC(h, l, c, v, 3, 10))
    gen("mfi_14",     ["out"], talib.MFI(h, l, c, v, 14))

    # ── Open/High/Low/Close ────────────────────────────────────────────────────
    gen("avgprice",  ["out"], talib.AVGPRICE(o, h, l, c))
    gen("bop",       ["out"], talib.BOP(o, h, l, c))
    gen("imi_14",    ["out"], talib.IMI(o, c, 14))

    # ── Candlestick patterns (all OHLC → int) ─────────────────────────────────
    cdl_funcs = [
        ("cdl2crows",           talib.CDL2CROWS),
        ("cdl3blackcrows",      talib.CDL3BLACKCROWS),
        ("cdl3inside",          talib.CDL3INSIDE),
        ("cdl3linestrike",      talib.CDL3LINESTRIKE),
        ("cdl3outside",         talib.CDL3OUTSIDE),
        ("cdl3starsinsouth",    talib.CDL3STARSINSOUTH),
        ("cdl3whitesoldiers",   talib.CDL3WHITESOLDIERS),
        ("cdlabandonedbaby",    lambda o,h,l,c: talib.CDLABANDONEDBABY(o,h,l,c, 0.3)),
        ("cdladvanceblock",     talib.CDLADVANCEBLOCK),
        ("cdlbelthold",         talib.CDLBELTHOLD),
        ("cdlbreakaway",        talib.CDLBREAKAWAY),
        ("cdlclosingmarubozu",  talib.CDLCLOSINGMARUBOZU),
        ("cdlconcealbabyswall", talib.CDLCONCEALBABYSWALL),
        ("cdlcounterattack",    talib.CDLCOUNTERATTACK),
        ("cdldarkcloudcover",   lambda o,h,l,c: talib.CDLDARKCLOUDCOVER(o,h,l,c, 0.5)),
        ("cdldoji",             talib.CDLDOJI),
        ("cdldojistar",         talib.CDLDOJISTAR),
        ("cdldragonflydoji",    talib.CDLDRAGONFLYDOJI),
        ("cdlengulfing",        talib.CDLENGULFING),
        ("cdleveningdojistar",  lambda o,h,l,c: talib.CDLEVENINGDOJISTAR(o,h,l,c, 0.3)),
        ("cdleveningstar",      lambda o,h,l,c: talib.CDLEVENINGSTAR(o,h,l,c, 0.3)),
        ("cdlgapsidesidewhite", talib.CDLGAPSIDESIDEWHITE),
        ("cdlgravestonedoji",   talib.CDLGRAVESTONEDOJI),
        ("cdlhammer",           talib.CDLHAMMER),
        ("cdlhangingman",       talib.CDLHANGINGMAN),
        ("cdlharami",           talib.CDLHARAMI),
        ("cdlharamicross",      talib.CDLHARAMICROSS),
        ("cdlhighwave",         talib.CDLHIGHWAVE),
        ("cdlhikkake",          talib.CDLHIKKAKE),
        ("cdlhikkakemod",       talib.CDLHIKKAKEMOD),
        ("cdlhomingpigeon",     talib.CDLHOMINGPIGEON),
        ("cdlidentical3crows",  talib.CDLIDENTICAL3CROWS),
        ("cdlinneck",           talib.CDLINNECK),
        ("cdlinvertedhammer",   talib.CDLINVERTEDHAMMER),
        ("cdlkicking",          talib.CDLKICKING),
        ("cdlkickingbylength",  talib.CDLKICKINGBYLENGTH),
        ("cdlladderbottom",     talib.CDLLADDERBOTTOM),
        ("cdllongleggeddoji",   talib.CDLLONGLEGGEDDOJI),
        ("cdllongline",         talib.CDLLONGLINE),
        ("cdlmarubozu",         talib.CDLMARUBOZU),
        ("cdlmatchinglow",      talib.CDLMATCHINGLOW),
        ("cdlmathold",          lambda o,h,l,c: talib.CDLMATHOLD(o,h,l,c, 0.5)),
        ("cdlmorningdojistar",  lambda o,h,l,c: talib.CDLMORNINGDOJISTAR(o,h,l,c, 0.3)),
        ("cdlmorningstar",      lambda o,h,l,c: talib.CDLMORNINGSTAR(o,h,l,c, 0.3)),
        ("cdlonneck",           talib.CDLONNECK),
        ("cdlpiercing",         talib.CDLPIERCING),
        ("cdlrickshawman",      talib.CDLRICKSHAWMAN),
        ("cdlrisefall3methods", talib.CDLRISEFALL3METHODS),
        ("cdlseparatinglines",  talib.CDLSEPARATINGLINES),
        ("cdlshootingstar",     talib.CDLSHOOTINGSTAR),
        ("cdlshortline",        talib.CDLSHORTLINE),
        ("cdlspinningtop",      talib.CDLSPINNINGTOP),
        ("cdlstalledpattern",   talib.CDLSTALLEDPATTERN),
        ("cdlsticksandwich",    talib.CDLSTICKSANDWICH),
        ("cdltakuri",           talib.CDLTAKURI),
        ("cdltasukigap",        talib.CDLTASUKIGAP),
        ("cdlthrusting",        talib.CDLTHRUSTING),
        ("cdltristar",          talib.CDLTRISTAR),
        ("cdlunique3river",     talib.CDLUNIQUE3RIVER),
        ("cdlupsidegap2crows",  talib.CDLUPSIDEGAP2CROWS),
        ("cdlxsidegap3methods", talib.CDLXSIDEGAP3METHODS),
    ]
    for name, fn in cdl_funcs:
        gen(name, ["out"], fn(o, h, l, c))


if __name__ == "__main__":
    main()
