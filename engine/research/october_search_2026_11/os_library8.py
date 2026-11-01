"""Library v8 for the endless October search: 23 more families.

Wider, randomly sampled versions of fixed families:
BBY Bollinger breakout, DNX Donchian breakout, RCX sharp move (follow or fade), SKX candle-streak fade,
CDX candle patterns, EPX EMA pullback (limit), KLX Keltner (follow or fade), ATY range-expansion bar,
SRX Parabolic SAR, DMX DMI cross, DWX day-of-week x hour, CZX 1h RSI fade by hour and VWAP,
E1X EMA crossover, STR Supertrend.
New ideas:
VBO  volatility breakout from a session open: OCO stop orders at open +/- k x yesterday's range (or daily ATR)
TSP  turtle soup: fade a new N-bar high/low whose bar closes back inside the old range
SQZ  Bollinger squeeze, then follow the first close outside the bands
VSP  volume-spike candle (volume and range both large), follow or fade
LLG  lead-lag: trade a coin that lagged BTC's last move (ETH leads BTC itself)
FND  hour-of-day position against (or with) the sign of the last funding print
PDF  previous-day high/low failure: fade a bar that pokes through and closes back inside
RNG  Asian-range edges faded with resting limit orders (maker fee), OCO
BLX  Bollinger / Keltner band fades with resting limit orders at the band (maker fee), OCO
OCO pairs are resolved the way the exchange would: the order whose level trades first fills, the other is
cancelled (same touch rules as sl_core.fill). os_auto fills ALL (symbol -> Ctx) so LLG can read the leader.
"""
import numpy as np
import pandas as pd

import os_families2 as N2
import os_library as _L
import os_library2 as _L2
import os_library3 as _L3
import os_library5 as _L5
import os_library7 as L7
from os_families2 import OCT0, OCT1, atr_bars
from os_library import SESS, bar_orders, window
from os_library4 import vol_ok
from sl_core import DAY, H, THROUGH, london_open, us_open

ALL = {}
TFS = L7.TFS
HOLD = (2, 3, 4, 6, 8, 10, 12, 16, 20)
WIDE = ("BBY", "DNX", "RCX", "SKX", "CDX", "EPX", "KLX", "ATY", "SRX", "DMX", "DWX", "CZX", "E1X", "STR")
NEW = ("VBO", "TSP", "SQZ", "VSP", "LLG", "FND", "PDF", "RNG", "BLX")
FAMS8 = WIDE + NEW
r1 = L7.r1


def sample(rng):
    """One family drawn evenly from the 14 of library 7 and the 23 here, then random parameters."""
    k = rng.randrange(len(L7.FAMS) + len(FAMS8))
    if k < len(L7.FAMS):
        return L7.sample(rng)
    fam, c = FAMS8[k - len(L7.FAMS)], rng.choice
    if fam == "BBY":
        p = (c(TFS), rng.randrange(10, 51), r1(rng, 1.0, 3.0, 0.1), c(("mid", "atr")), c((0.0, 1.0, 1.5, 2.0, 3.0)), c(("none", "trend")), c(SESS))
    elif fam == "DNX":
        p = (c(TFS), rng.randrange(5, 61), c(("opp", "atr")), c((0.0, 1.0, 1.5, 2.0, 3.0)), c(("none", "trend")), c(SESS))
    elif fam == "RCX":
        p = (c(TFS), rng.randrange(1, 13), r1(rng, 0.5, 4.0, 0.25), c((1, 2, 3, 4, 6, 8)), c(("mom", "rev")), c(("none", "trend")))
    elif fam == "SKX":
        p = (c(TFS), rng.randrange(3, 10), c(HOLD), r1(rng, 1.0, 4.0, 0.5), c(SESS))
    elif fam == "CDX":
        p = (c(TFS), c(("engulf", "pin", "inside")), c(("none", "trend")), r1(rng, 1.0, 4.0, 0.5), c(HOLD))
    elif fam == "EPX":
        p = (c(TFS), rng.randrange(10, 101, 5), r1(rng, 0.5, 3.0, 0.5), r1(rng, 1.0, 4.0, 0.5), c(SESS))
    elif fam == "KLX":
        p = (c(TFS), r1(rng, 1.0, 3.5, 0.1), c(("fade", "break")), r1(rng, 1.0, 4.0, 0.5), c(HOLD), c(SESS))
    elif fam == "ATY":
        p = (c(TFS), r1(rng, 1.2, 4.0, 0.1), c(("follow", "fade")), c((1, 2, 3, 4, 6, 8)), c(("none", "trend")))
    elif fam == "SRX":
        p = (c(TFS), c((0.005, 0.01, 0.015, 0.02, 0.025, 0.03, 0.04, 0.05)), c(SESS), c(("none", "trend")))
    elif fam == "DMX":
        p = (c(TFS), rng.randrange(7, 29), rng.randrange(10, 36), c(SESS), c(("none", "trend")))
    elif fam == "DWX":
        hh = rng.randrange(24)
        p = (rng.randrange(7), hh, rng.randrange(1, min(8, 24 - hh) + 1), c(("long", "short")))
    elif fam == "CZX":
        p = (rng.randrange(24), rng.randrange(2, 15), c(("any", "with", "against")), rng.randrange(7, 36), rng.randrange(15, 41),
             c((45, 50, 55, 60, 65, 70)))
    elif fam == "E1X":
        f = rng.randrange(3, 31)
        p = (c((5, 15, 30, 60, 120, 240)), f, f + rng.randrange(5, 71), c(("all", "us")), c(("none", "trend")))
    elif fam == "STR":
        p = (c((5, 15, 30, 60, 120, 240)), rng.randrange(7, 31), r1(rng, 1.0, 5.0, 0.25), c(("all", "us")), c(("none", "trend")))
    elif fam == "VBO":
        p = (c(("utc", "lon", "us")), r1(rng, 0.1, 1.2, 0.05), c(("range", "atr")), c((0, 1, 2)), c(("anchor", "half", "atr")),
             c((0.0, 1.0, 1.5, 2.0, 3.0)), c(("none", "trend", "nr4")))
    elif fam == "TSP":
        p = (c(TFS), rng.randrange(5, 61), r1(rng, 0.0, 1.0, 0.25), c(("mid", 1.0, 1.5, 2.0, 3.0)), c(HOLD), c(L7.VF), c(SESS))
    elif fam == "SQZ":
        p = (c(TFS), rng.randrange(10, 41), r1(rng, 1.5, 2.5, 0.25), rng.randrange(5, 31, 5), rng.randrange(50, 201, 25), rng.randrange(1, 7),
             c(("mid", 1.0, 1.5, 2.0, 3.0)), c((0.0, 1.0, 1.5, 2.0, 3.0)), c(HOLD), c(SESS))
    elif fam == "VSP":
        p = (c(TFS), r1(rng, 1.5, 5.0, 0.25), r1(rng, 0.5, 3.0, 0.25), c(("follow", "fade")), r1(rng, 1.0, 3.0, 0.5), c(HOLD), c(SESS))
    elif fam == "LLG":
        p = (c((5, 15, 30, 60)), rng.randrange(1, 7), r1(rng, 1.0, 4.0, 0.25), c((0.0, 0.25, 0.5)), r1(rng, 1.0, 3.0, 0.5), c((1, 2, 3, 4, 6)),
             c(SESS))
    elif fam == "FND":
        p = (rng.randrange(24), rng.randrange(1, 13), c((0.0, 0.00005, 0.0001, 0.00015, 0.0002)), c(("fade", "follow")), c((0.0, 0.5, 1.0)))
    elif fam == "PDF":
        p = (c((5, 15, 30, 60)), r1(rng, 0.0, 1.0, 0.25), c(("open", "vwap", 1.0, 1.5, 2.0, 3.0)), c(HOLD), c(SESS))
    elif fam == "RNG":
        e = c((2, 4, 6, 7, 8))
        p = (e, rng.randrange(e + 2, 24), c((0.0, 0.1, 0.25, 0.5)), c((0.25, 0.5, 0.75, 1.0)), c(("mid", "far")))
    else:  # BLX
        p = (c(TFS), c(("bb", "kel")), rng.randrange(10, 51), r1(rng, 1.2, 3.0, 0.1), c((1, 2)), r1(rng, 1.0, 4.0, 0.5), c(HOLD), c(SESS))
    return fam, p


def first_touch(hit_a, hit_b):
    """Index and side (+1 = a, -1 = b) of the first minute either level trades; None if neither or both at once."""
    any_ = hit_a | hit_b
    if not any_.any():
        return None
    j = int(np.argmax(any_))
    if hit_a[j] and hit_b[j]:
        return None
    return j, (1 if hit_a[j] else -1)


def f_vbo(cx, anchor, k, basis, delay, stp, tr, filt):
    mk, f = cx.mk, cx.f
    out = []
    for d in cx.days:
        kd = cx.kday[d]
        if (anchor == "us" and f.weekday[kd] >= 5) or (filt == "nr4" and not f.nr4[kd]):
            continue
        rng_ = f.prev_rng[kd] if basis == "range" else f.atr[kd]
        if not rng_ > 0 or np.isnan(f.atr[kd]):
            continue
        a0 = d if anchor == "utc" else (london_open(d) if anchor == "lon" else us_open(d))
        anc = mk.o[int(mk.idx(a0))]
        up, dn = anc + k * rng_, anc - k * rng_
        ts, end = a0 + delay * H, d + DAY - H
        i0, i1 = (int(x) for x in mk.idx([ts, end]))
        hit = first_touch(mk.h[i0:i1] >= up, mk.l[i0:i1] <= dn)
        if hit is None:
            continue
        side = hit[1]
        if filt == "trend" and side != cx.trend(d):
            continue
        lvl = up if side > 0 else dn
        stop = anc if stp == "anchor" else (lvl - side * 0.5 * k * rng_ if stp == "half" else lvl - side * 0.5 * f.atr[kd])
        od = {"ts": int(ts), "side": side, "entry": "stop", "level": float(lvl), "valid_until": int(end), "stop": float(stop),
              "exit_by": int(d + DAY)}
        if tr:
            od["target_r"] = float(tr)
        out.append(od)
    return out


def f_tsp(cx, tf, n, buf, tgt, hold, vf, sess):
    b = cx.bars(tf)
    h, l, c = b["h"], b["l"], b["c"]
    a = atr_bars(b, 14)
    ph, pl = h.rolling(n).max().shift(1), l.rolling(n).min().shift(1)
    ok = vol_ok(b, vf)
    longs, shorts = ((l < pl) & (c > pl)).to_numpy() & ok, ((h > ph) & (c < ph)).to_numpy() & ok
    sl, ss = (l - buf * a).to_numpy(), (h + buf * a).to_numpy()
    if tgt == "mid":
        mid = ((ph + pl) / 2).to_numpy()
        return bar_orders(cx, tf, longs, shorts, sl, ss, sess, hold, tgt_l=mid, tgt_s=mid)
    return bar_orders(cx, tf, longs, shorts, sl, ss, sess, hold, target_r=float(tgt))


def f_sqz(cx, tf, n, k, q, look, w, stp, tr, hold, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    m, sd = c.rolling(n).mean(), c.rolling(n).std(ddof=0)
    bw = sd / m
    tight = (bw <= bw.rolling(look).quantile(q / 100)).astype(float)
    recent = tight.rolling(w).max().shift(1).fillna(0) > 0
    longs, shorts = ((c > m + k * sd) & recent).to_numpy(), ((c < m - k * sd) & recent).to_numpy()
    if stp == "mid":
        sl = ss = m.to_numpy()
    else:
        sl, ss = (c - stp * a).to_numpy(), (c + stp * a).to_numpy()
    return bar_orders(cx, tf, longs, shorts, sl, ss, sess, hold, target_r=tr or None)


def f_vsp(cx, tf, m, r, mode, st, hold, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14).shift(1)
    va = b["v"].rolling(20).mean().shift(1)
    big = (b["v"] > m * va) & ((b["h"] - b["l"]) > r * a)
    up, dn = (big & (c > b["o"])).to_numpy(), (big & (c < b["o"])).to_numpy()
    longs, shorts = (up, dn) if mode == "follow" else (dn, up)
    return bar_orders(cx, tf, longs, shorts, (c - st * a).to_numpy(), (c + st * a).to_numpy(), sess, hold)


def f_llg(cx, tf, nb, k, frac, st, hold, sess):
    lead = ALL.get("ETHUSDT" if cx.mk.sym == "BTCUSDT" else "BTCUSDT")
    if lead is None:
        return []
    b, bl = cx.bars(tf), lead.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    ml = (bl["c"] - bl["c"].shift(nb)) / atr_bars(bl, 14)
    ml = pd.Series(ml.to_numpy(), index=bl["tc"].to_numpy()).reindex(b["tc"].to_numpy()).to_numpy()
    ma = ((c - c.shift(nb)) / a).to_numpy()
    with np.errstate(invalid="ignore"):
        longs, shorts = (ml > k) & (ma < frac * ml), (ml < -k) & (ma > frac * ml)
    return bar_orders(cx, tf, longs, shorts, (c - st * a).to_numpy(), (c + st * a).to_numpy(), sess, hold)


def f_fnd(cx, hh, dur, thr, mode, st):
    mk, out = cx.mk, []
    for d in cx.days:
        ts = d + hh * H
        j = int(np.searchsorted(mk.fund_t, ts, side="left")) - 1  # the last print strictly before the entry
        if j < 0:
            continue
        fr = float(mk.fund_r[j])
        if fr == 0 or abs(fr) < thr:
            continue
        side = -int(np.sign(fr)) if mode == "fade" else int(np.sign(fr))
        stop = None
        if st:
            atr_d = cx.f.atr[cx.kday[d]]
            if np.isnan(atr_d):
                continue
            stop = float(cx.last_close(ts) - side * st * atr_d)
        out.append({"ts": int(ts), "side": side, "entry": "market", "stop": stop, "exit_by": int(ts + dur * H)})
    return out


def f_pdf(cx, tf, buf, tgt, hold, sess):
    b = cx.bars(tf)
    tc = b["tc"].to_numpy()
    a = atr_bars(b, 14)
    dh, dl = cx.mk.daily["h"].to_numpy(), cx.mk.daily["l"].to_numpy()
    kd = (tc // DAY * DAY - cx.mk.t0) // DAY
    km = np.clip(kd - 1, 0, len(dh) - 1)
    pdh, pdl = np.where(kd >= 1, dh[km], np.nan), np.where(kd >= 1, dl[km], np.nan)
    hv, lv, cv = b["h"].to_numpy(), b["l"].to_numpy(), b["c"].to_numpy()
    with np.errstate(invalid="ignore"):
        longs, shorts = (lv < pdl) & (cv > pdl), (hv > pdh) & (cv < pdh)
    sl, ss = (b["l"] - buf * a).to_numpy(), (b["h"] + buf * a).to_numpy()
    if tgt in ("open", "vwap"):
        t_ = cx.f.dopen[np.clip(kd, 0, len(dh) - 1)] if tgt == "open" else cx.vwap(tc)
        return bar_orders(cx, tf, longs, shorts, sl, ss, sess, hold, tgt_l=t_, tgt_s=t_)
    return bar_orders(cx, tf, longs, shorts, sl, ss, sess, hold, target_r=float(tgt))


def f_rng(cx, r_end, until, buf, stx, tgt):
    mk, out = cx.mk, []
    for d in cx.days:
        i0, i1, i2 = (int(x) for x in mk.idx([d, d + r_end * H, d + until * H]))
        rh, rl = mk.h[i0:i1].max(), mk.l[i0:i1].min()
        wd = rh - rl
        if not wd > 0:
            continue
        lb, ls = rl - buf * wd, rh + buf * wd
        hit = first_touch(mk.l[i1:i2] <= lb * (1 - THROUGH), mk.h[i1:i2] >= ls * (1 + THROUGH))
        if hit is None:
            continue
        side = hit[1]
        lvl = lb if side > 0 else ls
        tg = (rh + rl) / 2 if tgt == "mid" else (rh if side > 0 else rl)
        out.append({"ts": int(d + r_end * H), "side": side, "entry": "limit", "level": float(lvl), "valid_until": int(d + until * H),
                    "stop": float(lvl - side * stx * wd), "target": float(tg), "exit_by": int(d + DAY)})
    return out


def f_blx(cx, tf, band, n, k, vb, st, hold, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    if band == "bb":
        mid, sd = c.rolling(n).mean(), c.rolling(n).std(ddof=0)
        lo, hi = mid - k * sd, mid + k * sd
    else:
        mid = c.ewm(span=n, adjust=False).mean()
        lo, hi = mid - k * a, mid + k * a
    mk, tc = cx.mk, b["tc"].to_numpy()
    lo, hi, mid, a = lo.to_numpy(), hi.to_numpy(), mid.to_numpy(), a.to_numpy()
    out = []
    for i in np.nonzero((tc >= OCT0) & (tc < OCT1) & np.isfinite(lo) & np.isfinite(a))[0]:
        t = int(tc[i])
        w = window(cx, t // DAY * DAY, sess)
        if w is None or not (w[0] <= t < w[1] - 300):
            continue
        exit_by = int(min(t + hold * H, w[1]))
        vu = min(t + vb * tf * 60, exit_by)
        i0, i1 = (int(x) for x in mk.idx([t, vu]))
        hit = first_touch(mk.l[i0:i1] <= lo[i] * (1 - THROUGH), mk.h[i0:i1] >= hi[i] * (1 + THROUGH))
        if hit is None:
            continue
        side = hit[1]
        lvl = lo[i] if side > 0 else hi[i]
        out.append({"ts": t, "side": side, "entry": "limit", "level": float(lvl), "valid_until": int(vu), "stop": float(lvl - side * st * a[i]),
                    "target": float(mid[i]), "exit_by": exit_by})
    return out


FN8 = {"BBY": _L.f_bbb, "DNX": _L.f_don, "RCX": _L.f_roc, "SKX": _L.f_stk, "CDX": _L.f_cdl, "EPX": _L.f_ema, "KLX": _L2.f_kel,
       "ATY": _L3.f_atx, "SRX": _L3.f_sar, "DMX": _L3.f_dmi, "DWX": _L3.f_dow, "CZX": _L5.f_cz, "E1X": N2.n1, "STR": N2.n2,
       "VBO": f_vbo, "TSP": f_tsp, "SQZ": f_sqz, "VSP": f_vsp, "LLG": f_llg, "FND": f_fnd, "PDF": f_pdf, "RNG": f_rng, "BLX": f_blx}
BASE8 = {"BBY": ("BBB", _L.describe), "DNX": ("DON", _L.describe), "RCX": ("ROC", _L.describe), "SKX": ("STK", _L.describe),
         "CDX": ("CDL", _L.describe), "EPX": ("EMA", _L.describe), "KLX": ("KEL", _L2.describe2), "ATY": ("ATX", _L3.describe3),
         "SRX": ("SAR", _L3.describe3), "DMX": ("DMI", _L3.describe3), "DWX": ("DOW", _L3.describe3), "CZX": ("CZ", _L5.describe5)}
N2MAP = {"E1X": "N1", "STR": "N2"}
NAMES8 = {"BBY": "Bollinger breakout", "DNX": "Donchian breakout", "RCX": "Sharp-move momentum/reversal", "SKX": "Candle-streak fade",
          "CDX": "Candle pattern", "EPX": "EMA pullback", "KLX": "Keltner channel", "ATY": "Range-expansion bar", "SRX": "Parabolic SAR",
          "DMX": "DMI cross", "DWX": "Day-of-week window", "CZX": "RSI fade by hour and VWAP", "E1X": "EMA crossover", "STR": "Supertrend",
          "VBO": "Volatility breakout from the open", "TSP": "Turtle soup (failed breakout)",
          "SQZ": "Bollinger squeeze breakout", "VSP": "Volume spike", "LLG": "BTC lead-lag", "FND": "Funding-rate tilt",
          "PDF": "Previous-day high/low failure", "RNG": "Asian-range limit fade", "BLX": "Band limit fade (maker)"}


def describe8(fam, p):
    if fam in BASE8:
        code, desc = BASE8[fam]
        return desc(code, p)
    if fam in N2MAP:
        return N2.describe((N2MAP[fam],) + tuple(p))
    S = {"all": "all day", "us": "US hours", "asia": "Asian hours", "eu": "European hours"}
    V = {"none": "", "high": ", high-volume bars only", "low": ", low-volume bars only"}
    if fam == "VBO":
        anchor, k, basis, delay, stp, tr, filt = p
        an = {"utc": "00:00 UTC open", "lon": "London open", "us": "NYSE open"}[anchor]
        bs = "yesterday's range" if basis == "range" else "the 14-day ATR"
        sp = {"anchor": "back at the open", "half": "halfway back to the open", "atr": "0.5 daily ATR"}[stp]
        fl = {"none": "", "trend": ", with the 10-day trend", "nr4": ", after a narrow-range day"}[filt]
        return (f"Volatility breakout {k} x {'range' if basis == 'range' else 'ATR'} from the {an}" + (f" (+{delay} h)" if delay else "")
                + (f", {tr}R target" if tr else "") + fl,
                f"From the {an}" + (f" plus {delay} h" if delay else "") + f", rest a buy stop {k} x {bs} above the open and a sell stop the "
                f"same distance below; the first one hit is the trade (the other is cancelled); stop {sp}" + (f", target {tr}R" if tr else "")
                + ", flat at 00:00 UTC.")
    if fam == "TSP":
        tf, n, buf, tgt, hold, vf, sess = p
        tg = "channel middle" if tgt == "mid" else f"{tgt}R"
        return (f"Turtle soup {n}-bar {tf}m, stop {buf} ATR beyond, target {tg}, {hold} h{V[vf]}, {S[sess]}",
                f"When a {tf}-min bar makes a new {n}-bar low but closes back above the old low, buy (mirror at highs); stop {buf} ATR beyond "
                f"the bar's extreme, target {'the middle of the old channel' if tgt == 'mid' else tg}, up to {hold} h.")
    if fam == "SQZ":
        tf, n, k, q, look, w, stp, tr, hold, sess = p
        return (f"Bollinger({n},{k}) squeeze breakout {tf}m (width in lowest {q}% of {look} bars)" + (f", {tr}R target" if tr else "")
                + f", {S[sess]}",
                f"After the band width sat in its lowest {q}% of the last {look} bars within the previous {w} bars, follow the first {tf}-min "
                f"close outside the bands; stop {'at the middle band' if stp == 'mid' else f'{stp} ATR'}" + (f", target {tr}R" if tr else "")
                + f", up to {hold} h.")
    if fam == "VSP":
        tf, m, r, mode, st, hold, sess = p
        return (f"{mode.title()} a {tf}m volume spike ({m}x volume, {r} ATR range), stop {st} ATR, {hold} h, {S[sess]}",
                f"When a {tf}-min bar has more than {m}x its 20-bar average volume and a range over {r} ATR, {mode} its direction; stop {st} ATR, "
                f"up to {hold} h.")
    if fam == "LLG":
        tf, nb, k, frac, st, hold, sess = p
        return (f"Lead-lag: catch up with a {k} ATR BTC move over {nb} x {tf}m, lag <{frac}, {hold} h, {S[sess]}",
                f"When BTC (ETH, for BTC itself) has moved more than {k} ATR over the last {nb} {tf}-min bars and this coin has moved less than "
                f"{frac} of that in its own ATR, trade this coin in BTC's direction; stop {st} ATR, up to {hold} h.")
    if fam == "FND":
        hh, dur, thr, mode, st = p
        return (f"{hh:02d}:00 UTC for {dur} h {'against' if mode == 'fade' else 'with'} the last funding print"
                + (f" (|rate| >= {thr * 100:.3f}%)" if thr else "") + (f", stop {st} ATR" if st else ""),
                f"Every day at {hh:02d}:00 UTC take the side {'opposite to' if mode == 'fade' else 'of'} the sign of the latest funding rate"
                + (f" when it is at least {thr * 100:.3f}% per 8 h" if thr else "") + f"; close {dur} h later" + (f"; stop {st} daily ATR." if st else "."))
    if fam == "PDF":
        tf, buf, tgt, hold, sess = p
        tg = {"open": "the day's 00:00 open", "vwap": "VWAP"}.get(tgt, f"{tgt}R")
        return (f"Previous-day high/low failure {tf}m, stop {buf} ATR beyond, target {tg}, {hold} h, {S[sess]}",
                f"When a {tf}-min bar trades through yesterday's high but closes back below it, sell (mirror at the low); stop {buf} ATR beyond "
                f"the bar, target {tg}, up to {hold} h.")
    if fam == "RNG":
        r_end, until, buf, stx, tgt = p
        return (f"Asian range 00:00-{r_end:02d}:00 UTC limit fade until {until:02d}:00, edge +{buf}, stop {stx}, target "
                f"{'middle' if tgt == 'mid' else 'far edge'}",
                f"Range = 00:00-{r_end:02d}:00 UTC. Until {until:02d}:00 UTC rest a buy limit {buf} range-widths below the low and a sell limit "
                f"{buf} above the high (the first filled cancels the other; maker fee); stop {stx} range-widths beyond, target the "
                f"{'range middle' if tgt == 'mid' else 'far edge'}, flat at 00:00 UTC.")
    if fam == "BLX":
        tf, band, n, k, vb, st, hold, sess = p
        bn = f"Bollinger({n},{k})" if band == "bb" else f"Keltner(EMA{n}, {k} ATR)"
        return (f"{bn} {tf}m limit fade, {vb} bar(s), stop {st} ATR, {hold} h, {S[sess]}",
                f"After each {tf}-min close, rest a buy limit at the lower band and a sell limit at the upper band for {vb} bar(s) (the first "
                f"fill cancels the other; maker fee); target the middle, stop {st} ATR, up to {hold} h.")
    return fam, ""
