"""Regenerate the High Volume Crypto Trading page from the Crypto Scalp Desk.

The high-volume desk is the scalp desk restricted to the majors: same binary,
same strategies, same fee model, different symbol universe. Its page should
therefore BE the scalp desk's page, not a hand-copy of it — a copy drifts, and
two desks that render differently cannot be compared by eye, which is the only
reason to run both.

So this generates it, the same way scripts/clone_live_demo_page.py generates the
demo page. The scalp desk source is READ ONLY here; nothing in this script
writes to it.

Only four things differ, and each is a fact about the desk rather than a
cosmetic change:

  the endpoints  - a separate engine process on :8096 with its own universe
  the heading    - so the two pages are never confused at a glance
  the blurb      - says which symbols it runs and why the list is 14, not 24
  the admin path - reset/clear act on THIS desk's statistics, not the scalp's

Run:  python scripts/clone_high_volume_page.py
"""
import io
import re

SRC = "client/src/app/scalp-desk/page.tsx"
DST = "client/src/app/high-volume-crypto/page.tsx"

s = io.open(SRC, encoding="utf-8").read()

# --- endpoints -------------------------------------------------------------
# Every read and mutation must land on the high-volume proxy. A single surviving
# "/api/scalp/" would point this page at the 220-symbol desk and silently show
# the wrong desk's numbers under a heading that says "high volume".
s = s.replace("/api/scalp/scalp/", "/api/scalp-highvol/scalp/")

# Assert on the result rather than trusting the replace. The demo-page clone
# script learned this the hard way: a template literal survived a
# quote-specific replace and left a demo page pointed at the real wallet.
leaked = re.findall(r"/api/scalp/(?!-)", s)
if leaked:
    raise SystemExit(
        "a scalp-desk endpoint survived the rewrite (%d occurrence(s)) — refusing to "
        "emit a page that reads the wrong desk" % len(leaked))

# --- header ----------------------------------------------------------------
head_end = s.index(" */") + 3
body = s[head_end:]
header = '''"use client";

/**
 * High Volume Crypto Trading — the scalp engine on the majors only.
 *
 * GENERATED from the Crypto Scalp Desk page. Do not hand-edit: run
 * scripts/clone_high_volume_page.py, so a fix to the scalp desk reaches this
 * one instead of the two drifting apart.
 *
 * Same binary, same strategies, same maker-fill model, same pre-registered
 * gate. One thing differs: the symbol universe. The scalp desk discovers every
 * Delta perpetual above a turnover floor and runs ~220 of them; this process is
 * launched with an explicit list of the highest-volume currencies and runs only
 * those.
 *
 * That is a separate PROCESS, not a filter on the other desk. Concurrency
 * limits, the pending-order queue and the paper books are all per-process, so
 * filtering 220 symbols down to 14 in the browser would still show streams that
 * had been competing against 200-odd others for the same fill slots. What these
 * strategies do when only liquid majors are available is a different question,
 * and only a process that only holds liquid majors can answer it.
 *
 * The scalp desk is the control arm and is left exactly as it is. The
 * comparison between the two is the point, and it stops meaning anything the
 * moment either side is tuned to make it look better.
 */'''
s = header + body

# --- breadcrumb + heading ---------------------------------------------------
for old, new in (
    ('<span className="desk-body-md" style={{ fontWeight: 500 }}>Scalp Desk</span>',
     '<span className="desk-body-md" style={{ fontWeight: 500 }}>High Volume Crypto Trading</span>'),
    ('<h1 className="desk-display-lg" style={{ fontSize: "2rem" }}>Scalp Desk</h1>',
     '<h1 className="desk-display-lg" style={{ fontSize: "2rem" }}>High Volume Crypto Trading</h1>'),
):
    if old not in s:
        raise SystemExit("heading block not found — the scalp page changed shape, update this script")
    s = s.replace(old, new, 1)

# --- description ------------------------------------------------------------
m = re.search(r"            100 one-minute strategies across 8 major cryptos.*?</p>", s, re.S)
if not m:
    raise SystemExit("description block not found — update this script")
s = s[:m.start()] + '''            The same scalp strategies as the Crypto Scalp Desk, on the highest-volume currencies only —
            BTC, ETH, SOL, XRP, BNB, DOGE, ADA, AVAX, LINK, TRX, UNI, LTC, BCH and HYPE. Fourteen, not
            twenty-four: five of the top twenty-four by global volume are stablecoins, which are the quote
            side of a trade rather than something to trade, and five more are not listed as perpetuals on
            Delta. A separate engine process, so these streams compete only with each other for fill slots.
            Paper only — real money goes through the pre-registered gate, same as the scalp desk.
          </p>''' + s[m.end():]

io.open(DST, "w", encoding="utf-8", newline="").write(s)
print("regenerated %s from %s (%d bytes)" % (DST, SRC, len(s)))
