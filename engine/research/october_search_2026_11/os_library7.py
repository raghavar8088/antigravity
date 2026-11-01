"""Library v7: an endless random sampler for the automatic October search.

When the fixed libraries are used up, os_auto keeps drawing new parameter
combinations from these ranges (never repeating an id) until it is stopped.

BZX  Bollinger fade, any timeframe 30m-4h, 10-50 bars, 1.2-3.0 sd
RZX  RSI fade, 15m-4h, period 5-35, threshold 15-40, exit 40-70
KZX  Keltner fade, 30m-4h, EMA 10-50, 1.0-3.0 ATR
ZEX  EMA z-score fade, 15m-4h, EMA 10-60, 1.5-3.5 sd
HZX  hour-of-day windows: any start, 1-12 h, buy or sell, 7 filters, all/weekdays/weekends
MXX  4h Bollinger extreme + fast RSI trigger inside an hour window
All take a volume filter (none / high / low) and, where it applies, a session.
"""
import numpy as np
import pandas as pd

from os_families2 import atr_bars, rsi
from os_library import SESS, bar_orders
from os_library4 import f_hz, vol_ok
from sl_core import DAY, H

TFS = (15, 30, 45, 60, 90, 120, 180, 240)
VF = ("none", "high", "low")
FAMS = ("BZX", "RZX", "KZX", "ZEX", "HZX", "MXX")


def r1(rng, lo, hi, step):
    return round(lo + step * rng.randrange(int(round((hi - lo) / step)) + 1), 2)


def sample(rng):
    fam = FAMS[rng.randrange(len(FAMS))]
    hold = (2, 3, 4, 6, 8, 10, 12, 16, 20)
    if fam == "BZX":
        p = (rng.choice(TFS[1:]), rng.randrange(10, 51), r1(rng, 1.2, 3.0, 0.1), r1(rng, 1.0, 5.0, 0.5), rng.choice(hold), rng.choice(VF), rng.choice(SESS))
    elif fam == "RZX":
        p = (rng.choice(TFS), rng.randrange(5, 36), rng.randrange(15, 41), rng.choice((40, 45, 50, 55, 60, 65, 70)), r1(rng, 1.0, 4.0, 0.5),
             rng.choice(hold), rng.choice(VF), rng.choice(SESS))
    elif fam == "KZX":
        p = (rng.choice(TFS[1:]), rng.randrange(10, 51), r1(rng, 1.0, 3.0, 0.1), r1(rng, 1.0, 5.0, 0.5), rng.choice(hold), rng.choice(("ema", "none")),
             rng.choice(VF), rng.choice(SESS))
    elif fam == "ZEX":
        p = (rng.choice(TFS), rng.randrange(10, 61), r1(rng, 1.5, 3.5, 0.1), r1(rng, 1.0, 4.0, 0.5), rng.choice(hold), rng.choice(VF), rng.choice(SESS))
    elif fam == "HZX":
        hh = rng.randrange(0, 24)
        p = (hh, rng.randrange(1, min(12, 24 - hh) + 1), rng.choice(("short", "long")),
             rng.choice(("none", "vwap", "rsi1h", "day", "trend10", "highvol", "lowvol")), rng.choice(("all", "wkday", "wkend")))
    else:
        p = (r1(rng, 1.4, 2.4, 0.1), rng.choice((15, 30, 60)), rng.randrange(10, 29), rng.randrange(20, 41), rng.randrange(0, 24, 2),
             rng.choice((6, 8, 12, 24)), r1(rng, 1.0, 3.0, 0.5), rng.choice((4, 6, 8, 12)))
    return fam, p


def f_bzx(cx, tf, n, k, st, hold, vf, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    m, sd = c.rolling(n).mean(), c.rolling(n).std(ddof=0)
    ok = vol_ok(b, vf)
    return bar_orders(cx, tf, (c < m - k * sd).to_numpy() & ok, (c > m + k * sd).to_numpy() & ok, (c - st * a).to_numpy(), (c + st * a).to_numpy(),
                      sess, hold, tgt_l=m.to_numpy(), tgt_s=m.to_numpy())


def f_rzx(cx, tf, n, lo, ex, st, hold, vf, sess):
    b = cx.bars(tf)
    r = rsi(b["c"], n)
    a = atr_bars(b, 14)
    c = b["c"]
    tc = b["tc"].to_numpy()
    ok = vol_ok(b, vf)
    return bar_orders(cx, tf, (r < lo).to_numpy() & ok, (r > 100 - lo).to_numpy() & ok, (c - st * a).to_numpy(), (c + st * a).to_numpy(), sess, hold,
                      exits_l=tc[(r >= ex).to_numpy()], exits_s=tc[(r <= 100 - ex).to_numpy()])


def f_kzx(cx, tf, n, k, st, hold, tgt, vf, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    e = c.ewm(span=n, adjust=False).mean()
    ok = vol_ok(b, vf)
    t = e.to_numpy() if tgt == "ema" else None
    return bar_orders(cx, tf, (c < e - k * a).to_numpy() & ok, (c > e + k * a).to_numpy() & ok, (c - st * a).to_numpy(), (c + st * a).to_numpy(),
                      sess, hold, tgt_l=t, tgt_s=t)


def f_zex(cx, tf, n, z, st, hold, vf, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    e = c.ewm(span=n, adjust=False).mean()
    dev = c - e
    zz = dev / dev.rolling(n).std(ddof=0).replace(0, np.nan)
    tc = b["tc"].to_numpy()
    ok = vol_ok(b, vf)
    return bar_orders(cx, tf, (zz < -z).to_numpy() & ok, (zz > z).to_numpy() & ok, (c - st * a).to_numpy(), (c + st * a).to_numpy(), sess, hold,
                      exits_l=tc[(zz >= 0).to_numpy()], exits_s=tc[(zz <= 0).to_numpy()])


def f_mxx(cx, k, ttf, n, thr, h0, span, st, hold):
    b4 = cx.bars(240)
    c4 = b4["c"]
    m, sd = c4.rolling(20).mean(), c4.rolling(20).std(ddof=0)
    low_at = pd.Series((c4 < m - k * sd).to_numpy(), index=b4["tc"].to_numpy())
    high_at = pd.Series((c4 > m + k * sd).to_numpy(), index=b4["tc"].to_numpy())
    bt = cx.bars(ttf)
    tct = bt["tc"].to_numpy()
    lo_ok = low_at.reindex(tct, method="ffill").fillna(False).to_numpy(bool)
    hi_ok = high_at.reindex(tct, method="ffill").fillna(False).to_numpy(bool)
    inwin = (((tct % DAY) // H - h0) % 24) < span
    r = rsi(bt["c"], n).to_numpy()
    c, a = bt["c"], atr_bars(bt, 14)
    return bar_orders(cx, ttf, lo_ok & (r < thr) & inwin, hi_ok & (r > 100 - thr) & inwin, (c - st * a).to_numpy(), (c + st * a).to_numpy(), "all", hold)


def f_hzx(cx, hh, dur, side, filt, days):
    if days != "wkend":
        return f_hz(cx, hh, dur, side, filt, days)
    return [o for o in f_hz(cx, hh, dur, side, filt, "all") if cx.f.weekday[cx.kday[o["ts"] // DAY * DAY]] >= 5]


FN7 = {"BZX": f_bzx, "RZX": f_rzx, "KZX": f_kzx, "ZEX": f_zex, "HZX": f_hzx, "MXX": f_mxx}
NAMES7 = {"BZX": "Bollinger fade", "RZX": "RSI fade", "KZX": "Keltner channel", "ZEX": "EMA z-score fade", "HZX": "Hour-of-day",
          "MXX": "4h band + fast RSI"}


def describe7(fam, p):
    S = {"all": "all day", "us": "US hours", "asia": "Asian hours", "eu": "European hours"}
    V = {"none": "", "high": ", high-volume bars only", "low": ", low-volume bars only"}
    if fam == "BZX":
        tf, n, k, st, hold, vf, sess = p
        return (f"Bollinger({n},{k}) {tf}m fade, stop {st} ATR, {hold} h{V[vf]}, {S[sess]}",
                f"Buy a {tf}-min close under the lower band (short over the upper); target the middle band, stop {st} ATR, up to {hold} h{V[vf]}.")
    if fam == "RZX":
        tf, n, lo, ex, st, hold, vf, sess = p
        return (f"RSI({n}) {tf}m fade <{lo}/>{100 - lo}, exit {ex}, stop {st} ATR, {hold} h{V[vf]}, {S[sess]}",
                f"On {tf}-min bars buy RSI({n}) under {lo} (short over {100 - lo}); exit at {ex}, after {hold} h or at a {st} ATR stop{V[vf]}.")
    if fam == "KZX":
        tf, n, k, st, hold, tgt, vf, sess = p
        return (f"Keltner(EMA{n}, {k} ATR) {tf}m fade, stop {st} ATR, {hold} h{', target EMA' if tgt == 'ema' else ''}{V[vf]}, {S[sess]}",
                f"Fade a {tf}-min close beyond EMA{n} +/- {k} ATR; stop {st} ATR, up to {hold} h{V[vf]}.")
    if fam == "ZEX":
        tf, n, z, st, hold, vf, sess = p
        return (f"EMA{n} z-score {z} fade {tf}m, stop {st} ATR, {hold} h{V[vf]}, {S[sess]}",
                f"Fade price {z} standard deviations from its EMA{n} on {tf}-min bars; exit on the cross back, after {hold} h or at a {st} ATR stop.")
    if fam == "HZX":
        from os_library4 import describe4
        t, r = describe4("HZ", (p[0], p[1], p[2], p[3], "wkday" if p[4] == "wkday" else "all"))
        if p[4] == "wkend":
            t, r = t.replace("every day", "weekends"), r.replace("Every day", "Weekends")
        return t, r
    if fam == "MXX":
        k, ttf, n, thr, h0, span, st, hold = p
        return (f"4h Bollinger({k}) extreme + {ttf}m RSI({n}) {thr}, {h0:02d}:00 UTC +{span} h",
                f"When the last 4h close is beyond its Bollinger {k} band, fade a {ttf}-min RSI({n}) extreme ({thr}/{100 - thr}) between "
                f"{h0:02d}:00 and {(h0 + span) % 24:02d}:00 UTC; stop {st} ATR, up to {hold} h.")
    return fam, ""
