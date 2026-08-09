"""Regenerate the Live Demo Engine page from the live one.

The demo page is a clone, and a clone maintained by hand drifts: it had already
lost a type definition that its own code referenced. Regenerating it from a
script means a fix to the live page reaches the demo page too, instead of the
two slowly becoming different desks with the same layout.

Only three things differ, and each is a deliberate safety property rather than
a cosmetic change:

  the endpoints    - a separate engine process holding demo credentials
  the badge        - purple in every state, never the live green/red
  the description  - says plainly that this is not real money
"""
import io
import re

SRC = "client/src/app/live-engine/page.tsx"
DST = "client/src/app/live-demo-engine/page.tsx"

s = io.open(SRC, encoding="utf-8").read()

# --- endpoints -------------------------------------------------------------
# Quote-agnostic: the arm/disarm path is a TEMPLATE LITERAL,
# `/api/live-engine/${action}`, and a quote-specific replace left it pointing at
# the real engine. A demo page carrying that one string would arm real money
# from a page badged "NOT REAL MONEY".
s = s.replace("/api/scalp/scalp/", "/api/scalp-demo/scalp/")
s = s.replace("/api/live-engine/", "/api/live-demo-engine/")

# Assert on the result rather than trusting the replaces. This check already
# caught the template literal once.
for leaked in ("/api/live-engine/", "/api/scalp/scalp/"):
    if leaked in s:
        raise SystemExit(
            "a live endpoint survived the rewrite (%s) — refusing to emit a demo page "
            "that can reach the real wallet" % leaked)

# --- header ----------------------------------------------------------------
# Everything up to the end of the first doc comment, keeping the directive.
head_end = s.index(" */") + 3
body = s[head_end:]
header = '''"use client";

/**
 * Live DEMO Engine — demo.delta.exchange, NOT real money.
 *
 * GENERATED from the live page. Do not hand-edit: run the clone script, so a
 * fix to the live desk reaches this one instead of the two drifting apart.
 *
 * Two things are deliberately not shared with the live page:
 *
 *   The venue. This reads a separate engine process on port 8095 holding demo
 *   credentials. A single process with a mode flag would be one misread
 *   environment variable away from routing a demo order to the real wallet.
 *
 *   The roster. The demo venue lists none of the symbols the live engine
 *   trades, so the streams are re-pointed onto demo majors. Strategy logic is
 *   identical; only the symbol differs. An exact roster would have placed zero
 *   orders forever.
 *
 * Results here rehearse the PLUMBING — arming, sizing, bracket placement, fee
 * accounting, reconciliation — not the edge. Signals are driven by Binance bars
 * while fills come from Delta demo, so a profitable record here is not evidence
 * that the same streams profit live.
 */'''
s = header + body

# --- badge -----------------------------------------------------------------
old_badge = '''                // Green while the Delta Engine is on, red when it is off.
                background: armed ? "var(--desk-success)" : "var(--desk-error)",
              }}
            >
              REAL MONEY · ${CEILING}
            </span>'''
new_badge = '''                // Purple in every state, never the live page's green/red. The
                // badge answers "which wallet is this?" before it answers "is
                // it armed?" — a demo page that can flash the same green as the
                // real-money one is a page you can misread at a glance.
                background: "#7c5cff",
              }}
            >
              DEMO — NOT REAL MONEY
            </span>'''
if old_badge not in s:
    raise SystemExit("badge block not found — the live page changed shape, update this script")
s = s.replace(old_badge, new_badge, 1)

# --- heading + description --------------------------------------------------
s = s.replace(
    '<span className="desk-body-md" style={{ fontWeight: 500 }}>Live Engine</span>',
    '<span className="desk-body-md" style={{ fontWeight: 500 }}>Live Demo Engine</span>', 1)
s = s.replace(
    '<h1 className="desk-display-lg" style={{ fontSize: "2rem" }}>Live Engine</h1>',
    '<h1 className="desk-display-lg" style={{ fontSize: "2rem" }}>Live Demo Engine</h1>', 1)

m = re.search(r"            Real-money option <strong>buying</strong>.*?</p>", s, re.S)
if not m:
    raise SystemExit("description block not found — update this script")
s = s[:m.start()] + '''            Paper-equivalent trading on <strong>demo.delta.exchange</strong> — the demo wallet, never the real one.
            Same engine, same sizing, same taker fees and bracket placement as the Live Engine, so what is rehearsed
            here is what runs there. The demo venue lists none of the symbols the live engine trades, so the streams
            are re-pointed onto demo majors — identical strategy logic, different symbols. That makes this a rehearsal
            of the plumbing, not of the edge: signals come from Binance bars while fills come from Delta demo, so a
            profitable record here is not evidence the same streams profit live.
          </p>''' + s[m.end():]

io.open(DST, "w", encoding="utf-8", newline="").write(s)
print("regenerated %s from %s (%d bytes)" % (DST, SRC, len(s)))
