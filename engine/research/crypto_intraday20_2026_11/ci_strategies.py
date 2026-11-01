"""The 20 intraday crypto strategies. Rules and parameters are fixed here before
any backtest is run and are not tuned on the results; ci_run.py records this
file's SHA-256 with every result.

Each function takes a Market (and the dict of all markets) and returns entry
dicts for ci_core.simulate. Signals read only closed bars; see ci_core.
"""
import numpy as np
import pandas as pd

from ci_core import DAY, H, adx, atr, london_open_utc, round_step, rsi, us_open_utc


def _days(mk):
    return np.arange(mk.t0, mk.t[-1] + 1, DAY, dtype=np.int64)


def _first_cross(mk, start, stop, up, dn):
    """First minute in [start, stop) whose CLOSE is above `up` (+1) or below `dn` (-1).
    Returns (signal time = that minute's close, side) or None."""
    i0, i1 = int(mk.idx(start)), int(mk.idx(stop))
    c = mk.c[i0:i1]
    hit = (c > up) | (c < dn)
    if not hit.any():
        return None
    k = int(np.argmax(hit))
    return int(mk.t[i0 + k] + 60), (1 if c[k] > up else -1)


# S01 ─────────────────────────────────────────────────────────────────────────
def s01(mk, mkts):
    b = mk.bars(30)
    first = b[b["t"] % DAY == 0]
    out = []
    for t, o, c in zip(first["t"], first["o"], first["c"]):
        r = c / o - 1
        if r != 0:
            out.append({"ts": int(t + 23 * H + 1800), "side": int(np.sign(r)), "exit_by": int(t + DAY)})
    return out


# S02 ─────────────────────────────────────────────────────────────────────────
def s02(mk, mkts):
    d = mk.bars(1440)
    out = []
    for k in range(1, len(d)):
        day, o = int(d["t"].iat[k]), d["o"].iat[k]
        rng = d["h"].iat[k - 1] - d["l"].iat[k - 1]
        x = _first_cross(mk, day, day + DAY - 60, o + 0.5 * rng, o - 0.5 * rng)
        if x:
            out.append({"ts": x[0], "side": x[1]})
    return out


# S03 ─────────────────────────────────────────────────────────────────────────
def s03(mk, mkts):
    d = mk.bars(1440)
    out = []
    for k in range(4, len(d)):
        day, o = int(d["t"].iat[k]), d["o"].iat[k]
        p = d.iloc[k - 4:k]
        rng = max(p["h"].max() - p["c"].min(), p["c"].max() - p["l"].min())
        up, dn = o + 0.5 * rng, o - 0.5 * rng
        i0, i1 = int(mk.idx(day)), int(mk.idx(day + DAY - 60))
        c = mk.c[i0:i1]
        flips, pos = [], 0
        for j in np.nonzero((c > up) | (c < dn))[0]:
            s = 1 if c[j] > up else -1
            if s != pos:
                flips.append((int(mk.t[i0 + j] + 60), s))
                pos = s
        for n, (ts, s) in enumerate(flips):
            out.append({"ts": ts, "side": s, "exit_by": flips[n + 1][0] if n + 1 < len(flips) else None})
    return out


# S04 ─────────────────────────────────────────────────────────────────────────
def s04(mk, mkts):
    b5 = mk.bars(5)
    first = b5[b5["t"] % DAY == 0].reset_index(drop=True)
    d = mk.bars(1440).reset_index(drop=True)
    a14 = atr(d, 14).shift(1)  # through the previous day only
    rv = first["v"] / first["v"].rolling(14).mean().shift(1)
    out = []
    for k in range(len(first)):
        o, c, day = first["o"].iat[k], first["c"].iat[k], int(first["t"].iat[k])
        if c == o or not (rv.iat[k] >= 1.0) or np.isnan(a14.iat[k]):
            continue
        out.append({"ts": day + 300, "side": 1 if c > o else -1, "stop_dist": 0.10 * a14.iat[k]})
    return out


# S05 ─────────────────────────────────────────────────────────────────────────
def s05(mk, mkts):
    out = []
    for day in _days(mk):
        if pd.Timestamp(int(day), unit="s").weekday() >= 5:
            continue  # no US session on weekends
        op = us_open_utc(int(day))
        i0, i1 = int(mk.idx(op)), int(mk.idx(op + 1800))
        rh, rl = mk.h[i0:i1].max(), mk.l[i0:i1].min()
        if rh <= rl:
            continue
        x = _first_cross(mk, op + 1800, op + 4 * H + 1800, rh, rl)
        if x:
            ts, s = x
            out.append({"ts": ts, "side": s, "stop": rl if s > 0 else rh, "target_dist": 2 * (rh - rl),
                        "exit_by": op + 6 * H + 1800})
    return out


# S06 ─────────────────────────────────────────────────────────────────────────
def s06(mk, mkts):
    out = []
    for day in _days(mk):
        if pd.Timestamp(int(day), unit="s").weekday() >= 5:
            continue  # no London session on weekends
        lo = london_open_utc(int(day))
        i0, i1 = int(mk.idx(day)), int(mk.idx(lo))
        ah, al = mk.h[i0:i1].max(), mk.l[i0:i1].min()
        if ah <= al:
            continue
        x = _first_cross(mk, lo, lo + 5 * H, ah, al)
        if x:
            ts, s = x
            out.append({"ts": ts, "side": s, "stop": (ah + al) / 2, "target_dist": ah - al, "exit_by": lo + 9 * H})
    return out


# S07 ─────────────────────────────────────────────────────────────────────────
def s07(mk, mkts):
    b = mk.bars(15)
    tc = b["tc"].to_numpy()
    tc = tc[(tc % DAY != 0) & (tc % DAY <= 23 * H + 1800)]
    side = np.sign(mk.c[mk.idx(tc) - 1] - mk.vwap_before(tc))
    out = []
    for day in np.unique(tc // DAY):
        m = (tc // DAY) == day
        ts, s = tc[m], side[m]
        starts = [k for k in range(len(ts)) if s[k] != 0 and (k == 0 or s[k] != s[k - 1])]
        for n, k in enumerate(starts):
            nxt = next((ts[j] for j in range(k + 1, len(ts)) if s[j] != s[k]), None)
            out.append({"ts": int(ts[k]), "side": int(s[k]), "exit_by": int(nxt) if nxt is not None else None})
    return out


# S08 ─────────────────────────────────────────────────────────────────────────
def s08(mk, mkts):
    b = mk.bars(60)
    r = b["c"] / b["o"] - 1
    sig = r.rolling(720).std().shift(1)
    hit = r.abs() > 3 * sig
    out = []
    for k in np.nonzero(hit.to_numpy())[0]:
        s = int(np.sign(r.iat[k]))
        out.append({"ts": int(b["tc"].iat[k]), "side": s, "stop": b["o"].iat[k], "exit_by": int(b["tc"].iat[k] + 2 * H)})
    return out


# S09 ─────────────────────────────────────────────────────────────────────────
def s09(mk, mkts):
    b = mk.bars(5)
    r = b["c"] / b["o"] - 1
    sig = r.rolling(864).std().shift(1)
    out = []
    for k in np.nonzero((r.abs() > 4 * sig).to_numpy())[0]:
        o, c, hi, lo = b["o"].iat[k], b["c"].iat[k], b["h"].iat[k], b["l"].iat[k]
        jump = abs(c - o)
        s = -int(np.sign(c - o))
        stop = lo - 0.5 * jump if s > 0 else hi + 0.5 * jump
        out.append({"ts": int(b["tc"].iat[k]), "side": s, "stop": stop, "target": c + 0.5 * (o - c),
                    "exit_by": int(b["tc"].iat[k] + H)})
    return out


# S10 / S11 ───────────────────────────────────────────────────────────────────
def _flow_frame(mk):
    b = mk.bars(15).copy()
    b["atr"] = atr(b, 14)
    tc = b["tc"].to_numpy()
    ok = mk.idx(tc) - 30 >= 0
    imb, vol, ret = np.full(len(tc), np.nan), np.full(len(tc), np.nan), np.full(len(tc), np.nan)
    imb[ok], vol[ok], ret[ok] = mk.flow(tc[ok], 30)
    b["imb"], b["vol30"], b["ret30"] = imb, vol, ret
    # average 30-minute volume over the previous 7 days
    i = mk.idx(tc)
    a = np.maximum(i - 30 - 7 * 1440, 0)
    span = np.maximum((i - 30) - a, 1)
    b["avg30"] = np.where(i - 30 - 7 * 1440 >= 0, (mk.cv[np.maximum(i - 30, 0)] - mk.cv[a]) / span * 30, np.nan)
    return b


def s10(mk, mkts):
    b = _flow_frame(mk)
    loud = b["vol30"] >= 1.5 * b["avg30"]
    out = []
    for s, cond in ((1, (b["imb"] > 0.10) & (b["ret30"] > 0)), (-1, (b["imb"] < -0.10) & (b["ret30"] < 0))):
        for k in np.nonzero((cond & loud & b["atr"].notna()).to_numpy())[0]:
            out.append({"ts": int(b["tc"].iat[k]), "side": s, "stop_dist": b["atr"].iat[k], "exit_by": int(b["tc"].iat[k] + H)})
    return out


def s11(mk, mkts):
    b = _flow_frame(mk)
    loud = b["vol30"] >= 1.5 * b["avg30"]
    out = []
    for s, cond in ((-1, (b["imb"] > 0.15) & (b["ret30"] <= 0)), (1, (b["imb"] < -0.15) & (b["ret30"] >= 0))):
        for k in np.nonzero((cond & loud & b["atr"].notna()).to_numpy())[0]:
            out.append({"ts": int(b["tc"].iat[k]), "side": s, "stop_dist": b["atr"].iat[k], "exit_by": int(b["tc"].iat[k] + H)})
    return out


# S12 / S13 ───────────────────────────────────────────────────────────────────
def _with_step(mk, minutes):
    b = mk.bars(minutes).copy()
    d = mk.bars(1440)
    prev_close = pd.Series(d["c"].to_numpy(), index=d["t"].to_numpy() + DAY)  # known from the next day on
    b["u"] = (b["t"] // DAY * DAY).map(prev_close).map(lambda p: round_step(p) if p == p else np.nan)
    return b.dropna(subset=["u"])


def s12(mk, mkts):
    b = _with_step(mk, 15)
    out = []
    for o, c, u, tc in zip(b["o"], b["c"], b["u"], b["tc"]):
        up = np.ceil(o / u) * u
        dn = np.floor(o / u) * u
        if up == o:
            up += u
        if dn == o:
            dn -= u
        if c >= up:
            out.append({"ts": int(tc), "side": 1, "stop": up - 0.25 * u, "target": up + 0.75 * u, "exit_by": int(tc + 2 * H)})
        elif c <= dn:
            out.append({"ts": int(tc), "side": -1, "stop": dn + 0.25 * u, "target": dn - 0.75 * u, "exit_by": int(tc + 2 * H)})
    return out


def s13(mk, mkts):
    b = _with_step(mk, 15)
    out = []
    for o, hi, lo, c, u, tc in zip(b["o"], b["h"], b["l"], b["c"], b["u"], b["tc"]):
        r_up = np.ceil(o / u) * u
        r_dn = np.floor(o / u) * u
        if hi >= r_up - 0.1 * u and c <= r_up - 0.3 * u and c < o:
            out.append({"ts": int(tc), "side": -1, "stop": max(hi, r_up) + 0.2 * u, "target_dist": 0.6 * u,
                        "exit_by": int(tc + 2 * H)})
        elif lo <= r_dn + 0.1 * u and c >= r_dn + 0.3 * u and c > o:
            out.append({"ts": int(tc), "side": 1, "stop": min(lo, r_dn) - 0.2 * u, "target_dist": 0.6 * u,
                        "exit_by": int(tc + 2 * H)})
    return out


# S14 ─────────────────────────────────────────────────────────────────────────
def s14(mk, mkts):
    out = []
    prem = pd.Series(mk.prem)
    p30 = prem.rolling(30).mean().to_numpy()            # mean of the 30 minutes ending at index i
    q_hi = prem.rolling(7 * 1440, min_periods=7 * 720).quantile(0.9).to_numpy()
    q_lo = prem.rolling(7 * 1440, min_periods=7 * 720).quantile(0.1).to_numpy()
    for day in _days(mk):
        for settle in (8 * H, 16 * H, 24 * H):
            ts = int(day + settle - 1800)
            i = int(mk.idx(ts)) - 1  # last closed minute
            if i < 7 * 1440 or np.isnan(q_hi[i - 30]):
                continue
            exit_by = int(day + settle + 900) if settle < 24 * H else int(day + DAY - 60)
            ref_hi, ref_lo = q_hi[i - 30], q_lo[i - 30]   # distribution from before the window
            if p30[i] > ref_hi:
                out.append({"ts": ts, "side": -1, "exit_by": exit_by})
            elif p30[i] < ref_lo:
                out.append({"ts": ts, "side": 1, "exit_by": exit_by})
    return out


# S15 ─────────────────────────────────────────────────────────────────────────
def s15(mk, mkts):
    b = mk.bars(15)
    d = mk.bars(1440)
    pdh = pd.Series(d["h"].to_numpy(), index=d["t"].to_numpy() + DAY)
    pdl = pd.Series(d["l"].to_numpy(), index=d["t"].to_numpy() + DAY)
    day = b["t"] // DAY * DAY
    hi_prev, lo_prev = day.map(pdh), day.map(pdl)
    out, used = [], set()
    for k in range(len(b)):
        dk = int(day.iat[k])
        h_, l_, c_, tc = b["h"].iat[k], b["l"].iat[k], b["c"].iat[k], int(b["tc"].iat[k])
        if h_ > hi_prev.iat[k] and c_ < hi_prev.iat[k] and (dk, -1) not in used:
            used.add((dk, -1))
            stop = h_ * 1.001
            out.append({"ts": tc, "side": -1, "stop": stop, "target_dist": 2 * (stop - c_), "exit_by": tc + 4 * H})
        elif l_ < lo_prev.iat[k] and c_ > lo_prev.iat[k] and (dk, 1) not in used:
            used.add((dk, 1))
            stop = l_ * 0.999
            out.append({"ts": tc, "side": 1, "stop": stop, "target_dist": 2 * (c_ - stop), "exit_by": tc + 4 * H})
    return out


# S16 ─────────────────────────────────────────────────────────────────────────
def s16(mk, mkts):
    b = mk.bars(5)
    mid = b["c"].rolling(20).mean()
    sd = b["c"].rolling(20).std(ddof=0)
    h1 = mk.bars(60)
    ax = pd.Series(adx(h1, 14).to_numpy(), index=h1["tc"].to_numpy())
    # ADX of the last hourly bar closed at or before each 5m close
    adx_now = ax.reindex(b["tc"].to_numpy(), method="ffill").to_numpy()
    calm = adx_now < 20
    out = []
    for k in np.nonzero(calm)[0]:
        c, m_, s_ = b["c"].iat[k], mid.iat[k], sd.iat[k]
        if np.isnan(s_) or s_ == 0:
            continue
        tc = int(b["tc"].iat[k])
        if c < m_ - 2.5 * s_:
            out.append({"ts": tc, "side": 1, "target": m_, "stop_dist": 1.5 * 2.5 * s_, "exit_by": tc + 2 * H})
        elif c > m_ + 2.5 * s_:
            out.append({"ts": tc, "side": -1, "target": m_, "stop_dist": 1.5 * 2.5 * s_, "exit_by": tc + 2 * H})
    return out


# S17 ─────────────────────────────────────────────────────────────────────────
def s17(mk, mkts):
    b = mk.bars(15)
    r2 = rsi(b["c"], 2)
    sma = b["c"].rolling(200).mean()
    a = atr(b, 14)
    tc = b["tc"].to_numpy()
    exit_long = tc[(r2 > 70).to_numpy()]
    exit_short = tc[(r2 < 30).to_numpy()]
    out = []
    for k in range(len(b)):
        if np.isnan(sma.iat[k]) or np.isnan(a.iat[k]):
            continue
        c = b["c"].iat[k]
        if c > sma.iat[k] and r2.iat[k] < 5:
            out.append({"ts": int(tc[k]), "side": 1, "stop_dist": 2 * a.iat[k], "exit_by": int(tc[k] + 2 * H), "exits": exit_long})
        elif c < sma.iat[k] and r2.iat[k] > 95:
            out.append({"ts": int(tc[k]), "side": -1, "stop_dist": 2 * a.iat[k], "exit_by": int(tc[k] + 2 * H), "exits": exit_short})
    return out


# S18 ─────────────────────────────────────────────────────────────────────────
def s18(mk, mkts):
    if mk.sym == "BTCUSDT":
        return []
    a = mk.bars(5)
    btc = mkts["BTCUSDT"].bars(5).set_index("t")
    rb = (btc["c"] / btc["o"] - 1).reindex(a["t"].to_numpy()).to_numpy()
    ra = (a["c"] / a["o"] - 1).to_numpy()
    out = []
    for k in range(len(a)):
        if np.isnan(rb[k]):
            continue
        tc = int(a["tc"].iat[k])
        if rb[k] > 0.005 and ra[k] < 0.5 * rb[k]:
            out.append({"ts": tc, "side": 1, "stop_dist": 0.01 * a["c"].iat[k], "exit_by": tc + 900})
        elif rb[k] < -0.005 and ra[k] > 0.5 * rb[k]:
            out.append({"ts": tc, "side": -1, "stop_dist": 0.01 * a["c"].iat[k], "exit_by": tc + 900})
    return out


# S19 ─────────────────────────────────────────────────────────────────────────
def s19(mk, mkts):
    b = mk.bars(60)
    hh = b["h"].rolling(24).max().shift(1)
    ll = b["l"].rolling(24).min().shift(1)
    a = atr(b, 14)
    oi = mk.metrics["sum_open_interest"]
    tc = b["tc"].to_numpy()
    oi_end = oi[np.minimum(mk.idx(tc), mk.n - 1)]
    oi_start = oi[np.maximum(mk.idx(tc - H), 0)]
    with np.errstate(invalid="ignore", divide="ignore"):
        doi = np.where(oi_start > 0, oi_end / oi_start - 1, np.nan)
    out = []
    for k in range(len(b)):
        if np.isnan(hh.iat[k]) or np.isnan(a.iat[k]) or not (doi[k] >= 0.01):
            continue
        if b["c"].iat[k] > hh.iat[k]:
            out.append({"ts": int(tc[k]), "side": 1, "stop_dist": a.iat[k]})
        elif b["c"].iat[k] < ll.iat[k]:
            out.append({"ts": int(tc[k]), "side": -1, "stop_dist": a.iat[k]})
    return out


# S20 ─────────────────────────────────────────────────────────────────────────
def s20(mk, mkts):
    b = mk.bars(60)
    a = atr(b, 14)
    ratio = pd.Series(mk.metrics["count_long_short_ratio"])
    w = 7 * 1440
    hi = ratio.rolling(w, min_periods=w // 2).quantile(0.95).to_numpy()
    lo = ratio.rolling(w, min_periods=w // 2).quantile(0.05).to_numpy()
    r = ratio.to_numpy()
    tc = b["tc"].to_numpy()
    vw = mk.vwap_before(tc)
    out = []
    for k in range(len(b)):
        i = int(mk.idx(tc[k])) - 1
        if i < w or np.isnan(a.iat[k]) or np.isnan(vw[k]) or tc[k] % DAY == 0:
            continue
        c = b["c"].iat[k]
        if r[i] >= hi[i] and c < vw[k]:
            out.append({"ts": int(tc[k]), "side": -1, "stop_dist": 1.5 * a.iat[k], "exit_by": int(tc[k] + 4 * H)})
        elif r[i] <= lo[i] and c > vw[k]:
            out.append({"ts": int(tc[k]), "side": 1, "stop_dist": 1.5 * a.iat[k], "exit_by": int(tc[k] + 4 * H)})
    return out


STRATEGIES = [
    {"id": "S01", "fn": s01, "family": "Time-series momentum", "name": "First-half-hour momentum",
     "source": "Shen, Urquhart & Wang (2022), 'Bitcoin intraday time series momentum', Financial Review; Gao, Han, Li & Zhou (2018), 'Market intraday momentum', Journal of Financial Economics",
     "idea": "The return of the first half-hour of the UTC day predicts the return of the last half-hour.",
     "rules": "Take the 00:00-00:30 UTC return. At 23:30 UTC trade in its direction; exit at 00:00. No stop. One trade a day."},
    {"id": "S02", "fn": s02, "family": "Volatility breakout", "name": "Williams volatility breakout",
     "source": "Williams (1999), 'Long-Term Secrets to Short-Term Trading'; widely run on Bitcoin as the k = 0.5 volatility breakout",
     "idea": "A move of half of yesterday's range away from today's open signals a directional day.",
     "rules": "Long when a 1m close is above the 00:00 open + 0.5 x yesterday's range; short below the open - 0.5 x range. First trigger only; exit at 00:00. No stop."},
    {"id": "S03", "fn": s03, "family": "Volatility breakout", "name": "Dual Thrust",
     "source": "Michael Chalek's Dual Thrust system, a standard intraday breakout in futures and crypto trading",
     "idea": "Breakouts beyond a range built from the last four days continue within the day.",
     "rules": "Range = max(HH - LC, HC - LL) over the prior 4 days. Long above open + 0.5 x range, short below open - 0.5 x range, reversing on the opposite line; exit at 00:00."},
    {"id": "S04", "fn": s04, "family": "Opening range", "name": "First 5-minute direction",
     "source": "Zarattini, Barbon & Aziz (2024), 'A Profitable Day Trading Strategy for the U.S. Equity Market' (5-minute opening-range rule)",
     "idea": "The direction of the first 5-minute candle, on days with heavy early volume, carries through the session.",
     "rules": "If the 00:00-00:05 candle is up (down) and its volume is at least its 14-day average, buy (sell) at 00:05. Stop 10% of the 14-day ATR; exit at 00:00."},
    {"id": "S05", "fn": s05, "family": "Opening range", "name": "US-open range breakout",
     "source": "Crabel (1990), 'Day Trading with Short Term Price Patterns and Opening Range Breakout'; Eross, McGroarty, Urquhart & Wolfe (2019), 'The intraday dynamics of bitcoin', Research in International Business and Finance (activity peaks in US hours)",
     "idea": "Crypto volume and volatility peak when US equities open; the first 30 minutes set a range whose break carries on.",
     "rules": "Range = first 30 min after the NYSE open (13:30 or 14:30 UTC). Weekdays only. Trade the first 1m close outside it within 4 h; stop at the other side, target 2x the range, exit at the NYSE close."},
    {"id": "S06", "fn": s06, "family": "Opening range", "name": "Asia range, London breakout",
     "source": "FX 'London breakout' practice; Eross et al. (2019) on activity rising at the European open",
     "idea": "The quiet Asian-hours range breaks when Europe arrives, and the break continues.",
     "rules": "Range = 00:00 UTC to the London open (07:00 or 08:00 UTC). Weekdays only. Trade the first 1m close outside it within 5 h; stop at the range middle, target 1x the range, exit 9 h after the London open."},
    {"id": "S07", "fn": s07, "family": "Trend", "name": "Session VWAP trend",
     "source": "Zarattini & Aziz (2023), 'Volume Weighted Average Price (VWAP): The Holy Grail for Day Trading Systems'",
     "idea": "Price above the session VWAP means buyers are in control, below means sellers.",
     "rules": "Every 15 min from 00:15 to 23:30 UTC hold long if the close is above the session VWAP and short if below, flipping when it crosses; exit at 00:00."},
    {"id": "S08", "fn": s08, "family": "Overreaction", "name": "Abnormal-hour continuation",
     "source": "Caporale & Plastun (2019), 'Price overreactions in the cryptocurrency market', Journal of Economic Studies",
     "idea": "After an abnormally large hourly move, price keeps moving the same way for a while.",
     "rules": "If an hour's return exceeds 3 standard deviations of the prior 30 days of hourly returns, trade in its direction for 2 h; stop at that hour's open."},
    {"id": "S09", "fn": s09, "family": "Liquidity provision", "name": "Jump reversal",
     "source": "Nagel (2012), 'Evaporating liquidity', Review of Financial Studies (short-term reversal as paid liquidity provision)",
     "idea": "A violent 5-minute move is often forced flow that partly reverses once it is absorbed.",
     "rules": "If a 5m return exceeds 4 standard deviations of the prior 3 days, fade it. Target half the bar's move back, stop half the jump beyond its extreme, exit after 1 h."},
    {"id": "S10", "fn": s10, "family": "Order flow", "name": "Taker-flow momentum",
     "source": "Cont, Kukanov & Stoikov (2014), 'The price impact of order book events', Journal of Financial Econometrics; Silantyev (2019), 'Order flow analysis of cryptocurrency markets', Digital Finance",
     "idea": "Heavy, one-sided aggressive buying or selling keeps pushing price the same way.",
     "rules": "Every 15 min: if the last 30 min had taker imbalance above +0.10 (below -0.10), a return in the same direction, and volume at least 1.5x its 7-day average, follow it for 1 h. Stop 1 ATR (15m, 14)."},
    {"id": "S11", "fn": s11, "family": "Order flow", "name": "Absorption fade",
     "source": "Kyle (1985) price-impact framework; order-flow 'absorption' as practitioners read it",
     "idea": "Strong aggressive buying that fails to lift price is being absorbed by a large passive seller, so price turns down (and the mirror).",
     "rules": "Every 15 min: if the last 30 min had taker imbalance above +0.15 but no gain (below -0.15 but no loss) on 1.5x average volume, trade against the flow for 1 h. Stop 1 ATR (15m, 14)."},
    {"id": "S12", "fn": s12, "family": "Round numbers", "name": "Round-number breakout",
     "source": "Osler (2003), 'Currency orders and exchange rate dynamics', Journal of Finance (stop orders cluster just past round numbers); Urquhart (2017), 'Price clustering in Bitcoin', Economics Letters",
     "idea": "Crossing a round number triggers the stops parked just beyond it, which speeds the move up.",
     "rules": "Round step = largest 1-2-5 step at most 1% of yesterday's close. When a 15m bar closes through a round level, follow it: stop 0.25 step back, target 0.75 step beyond, exit after 2 h."},
    {"id": "S13", "fn": s13, "family": "Round numbers", "name": "Round-number rejection",
     "source": "Osler (2003): take-profit orders cluster at round numbers, so price tends to stall and reverse there",
     "idea": "A test of a round level that closes well away from it is a rejection worth fading.",
     "rules": "A 15m bar that reaches within 0.1 step of the round level above, then closes 0.3 step below it and down on the bar, is sold (mirror at support). Stop 0.2 step past the level, target 0.6 step, exit after 2 h."},
    {"id": "S14", "fn": s14, "family": "Perpetual mechanics", "name": "Pre-funding premium fade",
     "source": "He, Manela, Ross & von Wachter (2022), 'Fundamentals of Perpetual Futures' (funding ties the perpetual to spot)",
     "idea": "An unusually rich (cheap) perpetual just before a funding settlement gets sold (bought) by traders avoiding the payment and by arbitrage.",
     "rules": "30 min before each 08:00, 16:00 and 00:00 UTC settlement: if the 30-min mean premium is above the 90th (below the 10th) percentile of the prior 7 days, short (buy) the perpetual. Exit 15 min after the settlement, or at 23:59 for the 00:00 one."},
    {"id": "S15", "fn": s15, "family": "Liquidity sweep", "name": "Prior-day high/low sweep reversal",
     "source": "Osler (2003) on stop orders resting beyond prior highs and lows; the 'stop run' reversal",
     "idea": "A brief push through yesterday's high or low that closes back inside has run the stops and tends to reverse.",
     "rules": "A 15m bar that trades above yesterday's high but closes below it is sold (mirror at the low), once per side a day. Stop 0.1% beyond the bar's extreme, target 2x the risk, exit after 4 h."},
    {"id": "S16", "fn": s16, "family": "Mean reversion", "name": "Calm-market Bollinger fade",
     "source": "Bollinger (2001), 'Bollinger on Bollinger Bands'; Wilder (1978) ADX as the regime filter",
     "idea": "When the market is not trending, closes far outside the band snap back to the middle.",
     "rules": "5m Bollinger(20, 2.5). When the hourly ADX(14) is below 20, buy a close under the lower band (sell above the upper). Target the middle band, stop 1.5 band-widths away, exit after 2 h."},
    {"id": "S17", "fn": s17, "family": "Mean reversion", "name": "RSI(2) pullback in trend",
     "source": "Connors & Alvarez (2009), 'Short Term Trading Strategies That Work'",
     "idea": "A sharp short-term pullback inside a trend is bought (sold) and quickly recovers.",
     "rules": "15m bars. Above the 200-bar average, buy RSI(2) under 5 and exit when RSI(2) goes above 70; below it, sell RSI(2) over 95 and exit under 30. Stop 2 ATR(14), exit after 2 h."},
    {"id": "S18", "fn": s18, "family": "Cross-asset", "name": "BTC-to-alt catch-up",
     "source": "Bitcoin's lead in crypto price discovery; tested here as a hypothesis on 5-minute bars",
     "idea": "When BTC jumps and an alt has not followed yet, the alt catches up within minutes.",
     "rules": "Every 5 min: if BTC moved more than 0.5% and the alt moved less than half of that, trade the alt in BTC's direction for 15 min. Stop 1%. Alts only."},
    {"id": "S19", "fn": s19, "family": "Positioning", "name": "Open-interest-confirmed breakout",
     "source": "Bessembinder & Seguin (1993), 'Price volatility, trading volume, and market depth', Journal of Financial and Quantitative Analysis (open interest as new positioning)",
     "idea": "A breakout that brings in new positions (rising open interest) is real money behind the move.",
     "rules": "Hourly close above the prior 24-hour high (below the low) while open interest rose at least 1% in that hour: trade the break. Stop 1 ATR(1h, 14); exit at 00:00."},
    {"id": "S20", "fn": s20, "family": "Positioning", "name": "Crowded-account contrarian",
     "source": "Contrarian reading of retail positioning (Kumar & Lee (2006), 'Retail investor sentiment and return comovements', Journal of Finance); Binance's all-account long/short ratio",
     "idea": "When an extreme share of accounts is long (short) and price is already weak (strong), the crowd gets squeezed.",
     "rules": "Hourly: if the account long/short ratio is at its 7-day 95th percentile and price is below the session VWAP, short (mirror at the 5th percentile). Stop 1.5 ATR(1h, 14), exit after 4 h."},
]
