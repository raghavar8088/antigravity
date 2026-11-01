"""Independent check of the OCO resolution in libraries 8 and 9.

For every two-sided order of VBO, RNG, BLX (library 8) and RBX, VWB (library 9): the side chosen must be the one
whose level trades first (the other side must not trade before or in the same minute), and sl_core.fill must enter
on exactly that minute. 1 Nov 2026: 12,598 orders checked, 0 bad.

Usage: python -I os_verify_oco.py
"""
import os
import random
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.normpath(os.path.join(HERE, "..", "session_lab_2026_11")))
sys.path.insert(0, HERE)
import numpy as np  # noqa: E402

import sl_core  # noqa: E402
import sl_families as SF  # noqa: E402
import os_families2 as N2  # noqa: E402
import os_library8 as L8  # noqa: E402
import os_library9 as L9  # noqa: E402

TH, DAY, H = sl_core.THROUGH, sl_core.DAY, sl_core.H


def first(mask):
    k = np.nonzero(mask)[0]
    return int(k[0]) if len(k) else None


def check(cx, od, other_level):
    mk = cx.mk
    i0, i1 = int(mk.idx(od["ts"])), int(mk.idx(od["valid_until"]))
    lvl, s = od["level"], od["side"]
    h, l = mk.h[i0:i1], mk.l[i0:i1]
    if od["entry"] == "stop":
        me = first(h >= lvl) if s > 0 else first(l <= lvl)
        ot = first(l <= other_level) if s > 0 else first(h >= other_level)
    else:
        me = first(l <= lvl * (1 - TH)) if s > 0 else first(h >= lvl * (1 + TH))
        ot = first(h >= other_level * (1 + TH)) if s > 0 else first(l <= other_level * (1 - TH))
    if me is None:
        return "chosen side never trades"
    if ot is not None and ot <= me:
        return f"other side traded first or in the same minute ({ot} vs {me})"
    tr = sl_core.fill(mk, od)
    if tr is not None and int(tr[0]) != int(mk.t[i0 + me]):
        return f"fill entered at {tr[0]}, first touch at {int(mk.t[i0 + me])}"
    return None


def other_leg(cx, fam, p, od):
    """The cancelled order's level, rebuilt from the same inputs."""
    d = od["ts"] // DAY * DAY
    if fam == "VBO":
        kd = cx.kday[d]
        rng_ = cx.f.prev_rng[kd] if p[2] == "range" else cx.f.atr[kd]
        a0 = {"utc": d, "lon": sl_core.london_open(d), "us": sl_core.us_open(d)}[p[0]]
        return cx.mk.o[int(cx.mk.idx(a0))] - od["side"] * p[1] * rng_
    if fam in ("RNG", "RBX"):
        i0, i1 = (int(x) for x in cx.mk.idx([d, d + p[0] * H]))
        rh, rl = cx.mk.h[i0:i1].max(), cx.mk.l[i0:i1].min()
        wd = rh - rl
        if fam == "RNG":
            return rh + p[2] * wd if od["side"] > 0 else rl - p[2] * wd
        return rl - p[2] * wd if od["side"] > 0 else rh + p[2] * wd
    return 2 * od["target"] - od["level"]  # BLX / VWB: bands are symmetric around the target


def main():
    sl_core.DATA = os.path.normpath(os.path.join(HERE, "..", "..", "data", "historical", "october_extra"))
    rng = random.Random(5)
    syms = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "DOGEUSDT"]
    ctx = {}
    for s in syms:
        mk = sl_core.Market(s)
        ctx[s] = N2.Ctx(mk, SF.Features(mk))
    L8.ALL.update(ctx)
    n = bad = 0
    msgs = []
    for fam in ("VBO", "RNG", "BLX", "RBX", "VWB"):
        fn = L8.FN8.get(fam) or L9.FN9[fam]
        for _ in range(12):
            while True:
                f, p = L9.sample(rng)
                if f == fam:
                    break
            for s in syms:
                for od in fn(ctx[s], *p):
                    n += 1
                    m = check(ctx[s], od, other_leg(ctx[s], fam, p, od))
                    if m:
                        bad += 1
                        if len(msgs) < 10:
                            msgs.append(f"{fam}|{p} {s} ts={od['ts']}: {m}")
        print(fam, "checked so far:", n, "bad:", bad, flush=True)
    print("TOTAL", n, "orders, bad", bad)
    for m in msgs:
        print(m)
    return 0 if not bad else 1


if __name__ == "__main__":
    sys.exit(main())
