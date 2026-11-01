"""Library v9 for the endless October search: 9 more families.

XSX  cross-coin ranking at any hour: buy the strongest (or weakest) k coins over a lookback, sell the other end
RBX  session-range breakout: OCO stop orders at the edges of the 00:00-to-r_end UTC range (complement of RNG)
VWB  VWAP band fade with resting limit orders at VWAP +/- k volume-weighted sd (maker fee), OCO, target VWAP
ORF  opening-range failure: after a break of a session's opening range closes back inside, trade the other way
FLX  flush reversal: fade an m-bar move larger than k ATR (optionally on heavy volume), target a retrace
RVF  relative value vs BTC: fade a z-score extreme of log(coin / BTC) (ETH for BTC itself), exit as it reverts
WOF  fade a stretch from the weekly (Monday 00:00 UTC) open beyond k daily ATR
DIV  RSI divergence: a new N-bar closing low with RSI above its prior low (mirror for highs)
BZR  Bollinger fade with a regime filter (ADX, yesterday's range vs its median, with/against the 10-day trend)
XSX is cross-coin: os_auto calls xs_orders2 with the whole coin set (ranks are taken within that set).
"""
import numpy as np
import pandas as pd

import os_library6 as _L6
import os_library7 as L7
import os_library8 as L8
from os_families2 import adx, atr_bars, rsi
from os_library import SESS, bar_orders, window
from os_library4 import vol_ok
from sl_core import DAY, H, THROUGH, london_open, us_open

TFS, HOLD, r1 = L7.TFS, L8.HOLD, L7.r1
FAMS9 = ("XSX", "RBX", "VWB", "ORF", "FLX", "RVF", "WOF", "DIV", "BZR")
REG = ("adx20", "adx25", "adx30", "calm", "wild", "with", "against")


def sample(rng):
    """One family drawn evenly from all 46 (libraries 7, 8 and 9), then random parameters."""
    before = len(L7.FAMS) + len(L8.FAMS8)
    k = rng.randrange(before + len(FAMS9))
    if k < before:
        return L8.sample(rng)
    fam, c = FAMS9[k - before], rng.choice
    if fam == "XSX":
        basis = c(("day", "8h", "4h", "2h", "1h", "30m"))
        p = (rng.randrange(1 if basis == "day" else 0, 24), basis, c(("mom", "rev")), c((1, 2)), c((1, 2, 3, 4, 6, 8, 12, 0)), c((0.0, 0.5, 1.0)))
    elif fam == "RBX":
        e = c((1, 2, 4, 6, 7, 8))
        p = (e, rng.randrange(e + 2, 24), c((0.0, 0.1, 0.25)), c(("mid", "far")), c((0.0, 1.0, 1.5, 2.0, 3.0)), c(("none", "trend", "nr4")))
    elif fam == "VWB":
        p = (c((5, 15, 30, 60)), r1(rng, 1.5, 3.5, 0.25), c((1, 2, 4, 6)), c((1, 2)), c((0.5, 1.0, 1.5, 2.0)), c((1, 2, 3, 4, 6, 8)), c(SESS))
    elif fam == "ORF":
        p = (c(("asia0", "lon", "nyse")), c((15, 30, 60)), c((1, 2, 3, 6, 12)), c((0.0, 0.1, 0.25, 0.5)), c(("mid", "far")))
    elif fam == "FLX":
        p = (c((5, 15, 30)), rng.randrange(1, 7), r1(rng, 1.5, 6.0, 0.5), c((0.0, 1.5, 2.0, 3.0)), c((0.0, 0.25, 0.5)), c((0.382, 0.5, 0.618, 1.0)),
             c((1, 2, 3, 4, 6)), c(SESS))
    elif fam == "RVF":
        p = (c((15, 30, 60, 120, 240)), rng.randrange(24, 301, 12), r1(rng, 1.5, 3.5, 0.25), c((0.0, 0.5, 1.0)), r1(rng, 1.0, 4.0, 0.5), c(HOLD),
             c(SESS))
    elif fam == "WOF":
        p = (r1(rng, 0.5, 3.0, 0.25), c((15, 30, 60)), c(("anchor", "half")), c((0.5, 1.0, 1.5)), c((4, 8, 0)), c(SESS))
    elif fam == "DIV":
        p = (c(TFS), rng.randrange(10, 61), rng.randrange(7, 22), rng.randrange(2, 13), c((50, 55, 60, 65)), r1(rng, 1.0, 4.0, 0.5), c(HOLD), c(SESS))
    else:  # BZR
        p = (c((60, 90, 120, 180, 240)), rng.randrange(10, 51), r1(rng, 1.2, 3.0, 0.1), r1(rng, 1.0, 5.0, 0.5), c(HOLD), c(L7.VF), c(REG), c(SESS))
    return fam, p


def oco_first(mk, t, vu, lo, hi):
    """First touch of OCO limit pairs (buy at lo, sell at hi, each needing a THROUGH trade-through, as in sl_core.fill)
    placed at minute times t and valid until vu: (rows, sides) for the pairs where exactly one side traded first."""
    if not len(t):
        return np.zeros(0, int), np.zeros(0, int)
    n = (vu - t) // 60
    if n.max() <= 0:
        return np.zeros(0, int), np.zeros(0, int)
    cols = np.arange(int(n.max()))
    idx = np.minimum(mk.idx(t)[:, None] + cols[None, :], mk.n - 1)
    live = cols[None, :] < n[:, None]
    buy = (mk.l[idx] <= (lo * (1 - THROUGH))[:, None]) & live
    sell = (mk.h[idx] >= (hi * (1 + THROUGH))[:, None]) & live
    any_ = buy | sell
    j = any_.argmax(1)
    r = np.arange(len(t))
    ok = any_.any(1) & ~(buy[r, j] & sell[r, j])
    return r[ok], np.where(buy[r, j], 1, -1)[ok]


def xs_orders2(ctxs, hh, basis, mode, k, hold, st):
    """{sym: orders} for a coin set; ranks are taken within that set."""
    syms = list(ctxs)
    out = {s: [] for s in syms}
    if len(syms) < 2 * k + 1:
        return out
    look = {"day": None, "8h": 8 * H, "4h": 4 * H, "2h": 2 * H, "1h": H, "30m": 1800}[basis]
    for d in ctxs[syms[0]].days:
        ts = d + hh * H
        rets = {}
        for s in syms:
            mk = ctxs[s].mk
            if d in mk.bad_days:
                break
            ref = mk.o[int(mk.idx(d))] if look is None else mk.c[int(mk.idx(ts - look)) - 1]
            rets[s] = mk.c[int(mk.idx(ts)) - 1] / ref - 1
        if len(rets) < len(syms):
            continue
        order = sorted(syms, key=lambda s: -rets[s])
        longs, shorts = (order[:k], order[-k:]) if mode == "mom" else (order[-k:], order[:k])
        end = d + DAY if hold == 0 else min(ts + hold * H, d + DAY)
        for side, group in ((1, longs), (-1, shorts)):
            for s in group:
                stop = None
                if st:
                    a = ctxs[s].f.atr[ctxs[s].kday[d]]
                    if np.isnan(a):
                        continue
                    stop = float(ctxs[s].mk.c[int(ctxs[s].mk.idx(ts)) - 1] - side * st * a)
                out[s].append({"ts": int(ts), "side": side, "entry": "market", "stop": stop, "exit_by": int(end)})
    return out


def f_rbx(cx, r_end, until, buf, stp, tr, filt):
    mk, out = cx.mk, []
    for d in cx.days:
        if filt == "nr4" and not cx.f.nr4[cx.kday[d]]:
            continue
        i0, i1, i2 = (int(x) for x in mk.idx([d, d + r_end * H, d + until * H]))
        rh, rl = mk.h[i0:i1].max(), mk.l[i0:i1].min()
        wd = rh - rl
        if not wd > 0:
            continue
        up, dn = rh + buf * wd, rl - buf * wd
        hit = L8.first_touch(mk.h[i1:i2] >= up, mk.l[i1:i2] <= dn)
        if hit is None:
            continue
        side = hit[1]
        if filt == "trend" and side != cx.trend(d):
            continue
        od = {"ts": int(d + r_end * H), "side": side, "entry": "stop", "level": float(up if side > 0 else dn), "valid_until": int(d + until * H),
              "stop": float((rh + rl) / 2 if stp == "mid" else (rl if side > 0 else rh)), "exit_by": int(d + DAY)}
        if tr:
            od["target_r"] = float(tr)
        out.append(od)
    return out


def vw_sq(cx):
    """Cumulative volume x typical price squared, cached on the context (for the VWAP band sd)."""
    if not hasattr(cx, "cpv2"):
        mk = cx.mk
        tp = (mk.h + mk.l + mk.c) / 3
        cx.cpv2 = np.concatenate([[0.0], np.cumsum(tp * tp * mk.v)])
    return cx.cpv2


def f_vwb(cx, tf, k, warm, vb, st, hold, sess):
    mk, cpv2 = cx.mk, vw_sq(cx)
    tc = cx.bars(tf)["tc"].to_numpy()
    out = []
    for d in cx.days:
        w = window(cx, d, sess)
        if w is None:
            continue
        t = tc[(tc >= max(w[0], d + warm * H)) & (tc < w[1] - 300)]
        if not len(t):
            continue
        ii, i0 = mk.idx(t), int(mk.idx(d))
        vol = cx.cv[ii] - cx.cv[i0]
        with np.errstate(invalid="ignore", divide="ignore"):
            vw = (cx.cpv[ii] - cx.cpv[i0]) / vol
            sd = np.sqrt(np.maximum((cpv2[ii] - cpv2[i0]) / vol - vw * vw, 0))
        ok = (vol > 0) & (sd > 0)
        t, vw, sd = t[ok], vw[ok], sd[ok]
        exit_by = np.minimum(t + hold * H, w[1])
        vu = np.minimum(t + vb * tf * 60, exit_by)
        lo, hi = vw - k * sd, vw + k * sd
        rows, sides = oco_first(mk, t, vu, lo, hi)
        for r, s in zip(rows, sides):
            lvl = lo[r] if s > 0 else hi[r]
            out.append({"ts": int(t[r]), "side": int(s), "entry": "limit", "level": float(lvl), "valid_until": int(vu[r]),
                        "stop": float(lvl - s * st * sd[r]), "target": float(vw[r]), "exit_by": int(exit_by[r])})
    return out


def f_orf(cx, which, rmin, rb, buf, tgt):
    mk, b, out = cx.mk, cx.bars(5), []
    tc5, h5, l5, c5 = (b[x].to_numpy() for x in ("tc", "h", "l", "c"))
    for d in cx.days:
        if which == "nyse" and cx.f.weekday[cx.kday[d]] >= 5:
            continue
        op = d if which == "asia0" else (london_open(d) if which == "lon" else us_open(d))
        i0, i1 = (int(x) for x in mk.idx([op, op + rmin * 60]))
        rh, rl = mk.h[i0:i1].max(), mk.l[i0:i1].min()
        wd = rh - rl
        if not wd > 0:
            continue
        m = np.nonzero((tc5 >= op + rmin * 60 + 300) & (tc5 <= min(op + 8 * H, d + DAY - H)))[0]
        cc = c5[m]
        brk = np.nonzero((cc > rh) | (cc < rl))[0]
        if not len(brk):
            continue
        j = int(brk[0])
        s = 1 if cc[j] > rh else -1  # the breakout's direction
        seg = cc[j + 1:j + 1 + rb]
        back = np.nonzero(seg <= rh if s > 0 else seg >= rl)[0]
        if not len(back):
            continue
        q = j + 1 + int(back[0])
        ext = h5[m[j:q + 1]].max() if s > 0 else l5[m[j:q + 1]].min()
        out.append({"ts": int(tc5[m[q]]), "side": -s, "entry": "market", "stop": float(ext + s * buf * wd),
                    "target": float((rh + rl) / 2 if tgt == "mid" else (rl if s > 0 else rh)), "exit_by": int(d + DAY)})
    return out


def f_flx(cx, tf, m, k, vm, buf, rt, hold, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14).shift(m)  # ATR before the move
    start = b["o"].shift(m - 1)
    mv = c - start
    ok = (b["v"].rolling(m).sum() > vm * m * b["v"].rolling(96).mean().shift(m)) if vm else pd.Series(True, index=b.index)
    dn, up = ((mv < -k * a) & ok).to_numpy(), ((mv > k * a) & ok).to_numpy()
    tg = (c + rt * (start - c)).to_numpy()
    return bar_orders(cx, tf, dn, up, (b["l"].rolling(m).min() - buf * a).to_numpy(), (b["h"].rolling(m).max() + buf * a).to_numpy(), sess, hold,
                      tgt_l=tg, tgt_s=tg)


def f_rvf(cx, tf, n, zt, ex, st, hold, sess):
    lead = L8.ALL.get("ETHUSDT" if cx.mk.sym == "BTCUSDT" else "BTCUSDT")
    if lead is None:
        return []
    b, bl = cx.bars(tf), lead.bars(tf)
    cl = pd.Series(bl["c"].to_numpy(), index=bl["tc"].to_numpy()).reindex(b["tc"].to_numpy()).to_numpy()
    s = pd.Series(np.log(b["c"].to_numpy() / cl))
    z = ((s - s.rolling(n).mean()) / s.rolling(n).std(ddof=0)).to_numpy()
    c, a = b["c"], atr_bars(b, 14)
    tc = b["tc"].to_numpy()
    with np.errstate(invalid="ignore"):
        longs, shorts, exl, exs = z < -zt, z > zt, tc[z >= -ex], tc[z <= ex]
    return bar_orders(cx, tf, longs, shorts, (c - st * a).to_numpy(), (c + st * a).to_numpy(), sess, hold, exits_l=exl, exits_s=exs)


def f_wof(cx, k, tf, tgt, st, hold, sess):
    o = cx.mk.daily["o"].to_numpy()

    def week_open(kd):
        m0 = kd - int(cx.f.weekday[kd])  # Monday = 0
        return o[m0] if m0 >= 0 else np.nan
    return _L6.anchor_fade(cx, week_open, k, tf, tgt, st, hold, sess)


def f_div(cx, tf, n_lo, n, dd, ex, st, hold, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    r = rsi(c, n)
    longs = ((c <= c.rolling(n_lo).min()) & (r > r.rolling(n_lo).min().shift(1) + dd)).to_numpy()
    shorts = ((c >= c.rolling(n_lo).max()) & (r < r.rolling(n_lo).max().shift(1) - dd)).to_numpy()
    tc = b["tc"].to_numpy()
    return bar_orders(cx, tf, longs, shorts, (c - st * a).to_numpy(), (c + st * a).to_numpy(), sess, hold,
                      exits_l=tc[(r >= ex).to_numpy()], exits_s=tc[(r <= 100 - ex).to_numpy()])


def regime(cx, b, reg):
    """(ok for longs, ok for shorts) per bar; day values come from days before the bar's order day."""
    if reg.startswith("adx"):
        ok = (adx(b, 14) < int(reg[3:])).to_numpy()
        return ok, ok
    kd = np.clip((b["tc"].to_numpy() // DAY * DAY - cx.mk.t0) // DAY, 0, len(cx.f.days) - 1)
    if reg in ("calm", "wild"):
        pr = cx.f.prev_rng
        med = pd.Series(pr).rolling(20, min_periods=10).median().to_numpy()
        with np.errstate(invalid="ignore"):
            ok = (pr < med)[kd] if reg == "calm" else (pr > med)[kd]
        return ok, ok
    tr = cx.f.trend[kd]
    return (tr > 0, tr < 0) if reg == "with" else (tr < 0, tr > 0)


def f_bzr(cx, tf, n, k, st, hold, vf, reg, sess):
    b = cx.bars(tf)
    c, a = b["c"], atr_bars(b, 14)
    m, sd = c.rolling(n).mean(), c.rolling(n).std(ddof=0)
    v = vol_ok(b, vf)
    ok_l, ok_s = regime(cx, b, reg)
    return bar_orders(cx, tf, (c < m - k * sd).to_numpy() & v & ok_l, (c > m + k * sd).to_numpy() & v & ok_s, (c - st * a).to_numpy(),
                      (c + st * a).to_numpy(), sess, hold, tgt_l=m.to_numpy(), tgt_s=m.to_numpy())


FN9 = {"XSX": xs_orders2, "RBX": f_rbx, "VWB": f_vwb, "ORF": f_orf, "FLX": f_flx, "RVF": f_rvf, "WOF": f_wof, "DIV": f_div, "BZR": f_bzr}
CROSS9 = {"XSX": xs_orders2}
NAMES9 = {"XSX": "Cross-coin intraday", "RBX": "Session-range breakout", "VWB": "VWAP band limit fade (maker)", "ORF": "Opening-range failure",
          "FLX": "Flush reversal", "RVF": "Relative value vs BTC", "WOF": "Fade from the weekly open", "DIV": "RSI divergence",
          "BZR": "Bollinger fade + regime filter"}


def describe9(fam, p):
    S = {"all": "all day", "us": "US hours", "asia": "Asian hours", "eu": "European hours"}
    V = {"none": "", "high": ", high-volume bars only", "low": ", low-volume bars only"}
    if fam == "XSX":
        hh, basis, mode, k, hold, st = p
        lb = {"day": "since 00:00 UTC", "8h": "over the last 8 h", "4h": "over the last 4 h", "2h": "over the last 2 h", "1h": "over the last hour",
              "30m": "over the last 30 min"}[basis]
        return (f"Cross-coin {'momentum' if mode == 'mom' else 'reversal'} at {hh:02d}:00 UTC ({basis}), {k} each side, "
                + (f"{hold} h" if hold else "to 00:00") + (f", stop {st} ATR" if st else ""),
                f"At {hh:02d}:00 UTC rank the coins by return {lb}; {'buy the strongest' if mode == 'mom' else 'buy the weakest'} {k} and "
                f"{'sell the weakest' if mode == 'mom' else 'sell the strongest'} {k}; close " + (f"{hold} h later" if hold else "at 00:00 UTC")
                + (f", stop {st} daily ATR." if st else "."))
    if fam == "RBX":
        r_end, until, buf, stp, tr, filt = p
        fl = {"none": "", "trend": ", with the 10-day trend", "nr4": ", after a narrow-range day"}[filt]
        return (f"00:00-{r_end:02d}:00 UTC range breakout until {until:02d}:00, edge +{buf}, stop {'middle' if stp == 'mid' else 'far side'}"
                + (f", {tr}R target" if tr else "") + fl,
                f"Range = 00:00-{r_end:02d}:00 UTC. Until {until:02d}:00 UTC rest a buy stop {buf} range-widths above the high and a sell stop "
                f"{buf} below the low (the first hit cancels the other); stop at the range {'middle' if stp == 'mid' else 'far side'}"
                + (f", target {tr}R" if tr else "") + ", flat at 00:00 UTC.")
    if fam == "VWB":
        tf, k, warm, vb, st, hold, sess = p
        return (f"VWAP {k}-sd band limit fade on {tf}m (from {warm} h into the day), stop {st} sd, {hold} h, {S[sess]}",
                f"After each {tf}-min close at least {warm} h after 00:00 UTC, rest a buy limit at VWAP - {k} sd and a sell limit at VWAP + {k} sd "
                f"for {vb} bar(s) (the first fill cancels the other; maker fee); target VWAP, stop {st} sd beyond, up to {hold} h.")
    if fam == "ORF":
        which, rmin, rb, buf, tgt = p
        nm = {"asia0": "00:00 UTC", "lon": "London", "nyse": "NYSE"}[which]
        return (f"{nm} {rmin}-min opening-range failure (back inside within {rb} x 5m), stop +{buf}, target "
                f"{'middle' if tgt == 'mid' else 'far edge'}",
                f"Range = the first {rmin} min after the {nm} open. If a 5-min close breaks out and a close within the next {rb} bars is back "
                f"inside, trade the other way; stop {buf} range-widths beyond the breakout extreme, target the range "
                f"{'middle' if tgt == 'mid' else 'far edge'}, flat at 00:00 UTC.")
    if fam == "FLX":
        tf, m, k, vm, buf, rt, hold, sess = p
        return (f"Flush reversal: fade a {k} ATR move over {m} x {tf}m" + (f" on {vm}x volume" if vm else "") + f", target {rt} retrace, "
                f"{hold} h, {S[sess]}",
                f"When {m} {tf}-min bars move more than {k} ATR" + (f" on more than {vm}x their usual volume" if vm else "")
                + f", trade the other way; target a {rt} retrace of the move, stop {buf} ATR beyond its extreme, up to {hold} h.")
    if fam == "RVF":
        tf, n, zt, ex, st, hold, sess = p
        return (f"Relative value vs BTC: fade a {zt}-sd move of coin/BTC ({n} x {tf}m), exit at {ex} sd, stop {st} ATR, {hold} h, {S[sess]}",
                f"Track log(coin / BTC) (ETH for BTC itself) on {tf}-min bars; when it is more than {zt} standard deviations from its {n}-bar mean, "
                f"trade the coin back toward it; exit when within {ex} sd, after {hold} h or at a {st} ATR stop.")
    if fam == "WOF":
        k, tf, tgt, st, hold, sess = p
        return (f"Fade {k} ATR from the weekly open ({tf}m check), target {'the open' if tgt == 'anchor' else 'halfway'}, {S[sess]}",
                f"When a {tf}-min close is more than {k} daily ATR from this week's Monday 00:00 UTC open, trade back toward it; stop {st} ATR beyond, "
                + (f"up to {hold} h." if hold else "to the end of the window."))
    if fam == "DIV":
        tf, n_lo, n, dd, ex, st, hold, sess = p
        return (f"RSI({n}) divergence {tf}m ({n_lo}-bar extreme, {dd} pts), exit {ex}, stop {st} ATR, {hold} h, {S[sess]}",
                f"When a {tf}-min close is the lowest of the last {n_lo} bars but RSI({n}) is at least {dd} points above its lowest reading of the "
                f"previous {n_lo} bars, buy (mirror at highs); exit at RSI {ex}, after {hold} h or at a {st} ATR stop.")
    if fam == "BZR":
        tf, n, k, st, hold, vf, reg, sess = p
        rg = {"adx20": "ADX under 20", "adx25": "ADX under 25", "adx30": "ADX under 30", "calm": "after a calmer-than-usual day",
              "wild": "after a wider-than-usual day", "with": "only with the 10-day trend", "against": "only against the 10-day trend"}[reg]
        return (f"Bollinger({n},{k}) {tf}m fade, {rg}, stop {st} ATR, {hold} h{V[vf]}, {S[sess]}",
                f"Buy a {tf}-min close under the lower band (short over the upper), {rg}; target the middle band, stop {st} ATR, up to {hold} h{V[vf]}.")
    return fam, ""
