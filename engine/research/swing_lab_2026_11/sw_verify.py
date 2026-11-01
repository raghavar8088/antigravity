"""Independent checks of the swing lab's finalists.

1. Positions and P&L recomputed by explicit day-by-day loops written apart from
   the vectorised code in sw_lab.py; daily P&L must match to the cent.
2. No look-ahead: every finalist re-run on data cut at 2024-07-01; positions and
   P&L before the cut must be identical.

Usage: python -I sw_verify.py
"""
import json
import math
import os
import sys

import numpy as np
import pandas as pd

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import sw_lab as S  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
CUT = pd.Timestamp("2024-07-01")


def sma_at(x, i, n):
    w = x[i - n + 1:i + 1]
    return np.nan if i - n + 1 < 0 or np.isnan(w).any() else w.mean()


def ref_positions(P, vid):
    """Positions for day i decided with data through day i-1, by plain loops."""
    p = vid.split("|")
    fam = p[0]
    idx = P.c.index
    n = len(idx)
    if fam in ("T1", "T2", "T4", "S1"):
        out = {}
        for s in S.COINS6:
            c = P.c[s].to_numpy()
            pos = np.zeros(n)
            hold = 0.0
            for i in range(1, n):
                j = i - 1  # yesterday's close
                if fam == "T1":
                    m = sma_at(c, j, int(p[1]))
                    sig = 0.0 if np.isnan(m) or np.isnan(c[j]) else float(np.sign(c[j] - m))
                elif fam == "T2":
                    f, sl = (int(x) for x in p[1].split("/"))
                    a, b = sma_at(c, j, f), sma_at(c, j, sl)
                    sig = 0.0 if np.isnan(a) or np.isnan(b) else float(np.sign(a - b))
                elif fam == "T4":
                    L = int(p[1])
                    sig = 0.0 if j - L < 0 or np.isnan(c[j]) or np.isnan(c[j - L]) else float(np.sign(c[j] / c[j - L] - 1))
                    if p[2] == "weekly":
                        if idx[i].dayofweek == 0:
                            hold = sig
                        sig = hold
                else:
                    if j < 21:
                        sig = 0.0
                    else:
                        r = c[1:j + 1] / c[:j] - 1
                        rr = r[-1]
                        sd = np.std(r[-20:], ddof=1)
                        sig = float(np.sign(rr)) if (not np.isnan(rr) and abs(rr) > float(p[1]) * sd) else 0.0
                        if p[2] == "rev":
                            sig = -sig
                if fam != "S1" and p[-1] == "LO":
                    sig = max(sig, 0.0)
                pos[i] = sig
            out[s] = pos
        return pd.DataFrame(out, index=idx)
    if fam == "T3":
        n_in, n_out = (int(x) for x in p[1].split("/"))
        out = {}
        for s in S.COINS6:
            c, h, l = (getattr(P, a)[s].to_numpy() for a in ("c", "h", "l"))
            pos, res = 0, np.zeros(n)
            for i in range(n):
                res[i] = pos
                if i < n_in:
                    continue
                wh, wl = h[i - n_in:i], l[i - n_in:i]
                if np.isnan(c[i]) or np.isnan(wh).any() or np.isnan(wl).any():
                    continue
                exit_lo, exit_hi = l[i - n_out:i].min(), h[i - n_out:i].max()
                if pos == 1 and c[i] < exit_lo:
                    pos = 0
                elif pos == -1 and c[i] > exit_hi:
                    pos = 0
                if pos == 0:
                    if c[i] > wh.max():
                        pos = 1
                    elif c[i] < wl.min() and p[2] == "LS":
                        pos = -1
            out[s] = res
        return pd.DataFrame(out, index=idx)
    if fam == "M1":
        thr, filt, mode = int(p[1]), p[2], p[3]
        out = {}
        for s in S.COINS6:
            c = P.c[s].to_numpy()
            up = dn = np.nan
            res = np.zeros(n)
            pos, age = 0, 0
            for i in range(n):
                res[i] = pos
                if i > 0 and not np.isnan(c[i]) and not np.isnan(c[i - 1]):
                    d = c[i] - c[i - 1]
                    g, ls_ = max(d, 0.0), max(-d, 0.0)
                    up = g if np.isnan(up) else 0.5 * up + 0.5 * g
                    dn = ls_ if np.isnan(dn) else 0.5 * dn + 0.5 * ls_
                if np.isnan(up) or np.isnan(dn) or dn == 0:
                    continue
                r = 100 - 100 / (1 + up / dn)
                m = sma_at(c, i, 200)
                if filt == "sma200" and np.isnan(m):
                    continue
                if pos != 0:
                    age += 1
                    if (pos == 1 and r > 70) or (pos == -1 and r < 30) or age >= 5:
                        pos, age = 0, 0
                if pos == 0:
                    if r < thr and (filt == "none" or c[i] > m):
                        pos, age = 1, 0
                    elif r > 100 - thr and (filt == "none" or c[i] < m) and mode == "LS":
                        pos, age = -1, 0
            out[s] = res
        return pd.DataFrame(out, index=idx)
    # cross-coin families
    k = int(p[2])
    coins = S.COINS12
    c = P.c[coins].to_numpy()
    fr = P.funding_real[coins].to_numpy()
    out = np.zeros((n, len(coins)))
    last = None
    for i in range(1, n):
        j = i - 1
        reb = (p[3] if fam == "X3" else ("daily" if fam == "X2" and int(p[1]) < 7 else "weekly"))
        if reb == "daily" or idx[i].dayofweek == 0 or last is None:
            score = {}
            for q, s in enumerate(coins):
                if fam == "X3":
                    D = int(p[1])
                    w = fr[j - D + 1:j + 1, q] if j - D + 1 >= 0 else np.array([np.nan])
                    v = np.nan if np.isnan(w).any() else w.mean()
                else:
                    L = int(p[1])
                    v = np.nan if j - L < 0 else c[j, q] / c[j - L, q] - 1
                if not np.isnan(v):
                    score[s] = v
            if len(score) >= 2 * k + 1:
                order = sorted(score, key=lambda s: -score[s])
                flip = fam in ("X2", "X3")
                last = {s: 0.0 for s in coins}
                for s in order[:k]:
                    last[s] = -1.0 if flip else 1.0
                for s in order[-k:]:
                    last[s] = 1.0 if flip else -1.0
            else:
                last = None
        if last:
            out[i] = [last[s] for s in coins]
    return pd.DataFrame(out, index=idx, columns=coins)


def ref_pnl(P, pos):
    cols = list(pos.columns)
    o, c = P.o[cols].to_numpy(), P.c[cols].to_numpy()
    fund = P.funding[cols].to_numpy()
    cost = P.cost[cols].to_numpy()
    pv = pos.to_numpy()
    n = len(pos)
    out = np.zeros((n, len(cols)))
    for q in range(len(cols)):
        prev = 0.0
        for i in range(n):
            x = pv[i, q] if not np.isnan(c[i, q]) else 0.0
            r = o[i + 1, q] / o[i, q] - 1 if i + 1 < n and not np.isnan(o[i, q]) and not np.isnan(o[i + 1, q]) else 0.0
            out[i, q] = S.N * (x * r - abs(x - prev) * cost[q] - x * fund[i, q])
            prev = x
    return pd.DataFrame(out, index=pos.index, columns=cols)


def main():
    s = json.load(open(os.path.join(HERE, "results", "summary.json"), encoding="utf-8"))
    fins = [f["variant"] for f in s["finalists"]]
    P, Pc = S.Panel(), S.Panel(cut=CUT)
    lookup = {v: (fn, args) for vs in S.FAMILIES.values() for v, fn, args in vs}
    bad_pos, bad_pnl, bad_cut = [], [], []
    for v in fins:
        fn, args = lookup[v]
        pos = fn(P, *args).fillna(0.0)
        pnl = P.pnl(pos)
        ref = ref_positions(P, v).reindex(columns=pos.columns).fillna(0.0)
        listed = P.c[pos.columns].notna()
        if not np.allclose(pos.where(listed, 0.0).to_numpy(), ref.where(listed, 0.0).to_numpy()):
            diff = (pos.where(listed, 0.0) != ref.where(listed, 0.0)).any(axis=1)
            bad_pos.append(f"{v}: positions differ on {int(diff.sum())} days, first {diff[diff].index[0].date()}")
        rp = ref_pnl(P, pos.where(listed, 0.0))
        if not np.allclose(pnl.to_numpy(), rp.to_numpy(), atol=0.01):
            bad_pnl.append(f"{v}: P&L differs, total {pnl.to_numpy().sum():.2f} vs {rp.to_numpy().sum():.2f}")
        pc = fn(Pc, *args).fillna(0.0)
        pnc = Pc.pnl(pc)
        a = pnl[pnl.index < CUT - pd.Timedelta(days=1)]
        b = pnc[pnc.index < CUT - pd.Timedelta(days=1)]
        if not np.allclose(a.to_numpy(), b.reindex(a.index).to_numpy(), atol=1e-6):
            bad_cut.append(f"{v}: P&L before the cut changes when later data is removed")
        print(v, "checked", flush=True)
    out = {"finalists": fins, "position_mismatches": bad_pos, "pnl_mismatches": bad_pnl,
           "truncation_cut": str(CUT.date()), "truncation_mismatches": bad_cut}
    json.dump(out, open(os.path.join(HERE, "results", "verify.json"), "w", encoding="utf-8"), indent=1)
    print(json.dumps(out, indent=1))
    return 0 if not (bad_pos or bad_pnl or bad_cut) else 1


if __name__ == "__main__":
    sys.exit(main())
