# High Volume Crypto Trading — deploy

A third `scalp_prelive` container, port 8096. Same image, same binary, same
strategies as the Crypto Scalp Desk on 8094. The only difference is the symbol
universe: an explicit `-symbols` list instead of `-symbols auto`.

No Go code was written for this module. `scalp_prelive` already takes a symbol
CSV, so the desk is a launch argument, not a fork.

## The universe

Fourteen symbols, from the top 24 cryptocurrencies by 24h global volume:

```
BTCUSD ETHUSD SOLUSD XRPUSD BNBUSD DOGEUSD ADAUSD
AVAXUSD LINKUSD TRXUSD UNIUSD LTCUSD BCHUSD HYPEUSD
```

### Why fourteen and not twenty-four

Ten of the top 24 cannot be traded here, for two different reasons:

**Five are stablecoins** — USDT, USDC, USD1, USDS, USDG. They rank high because
they are the *quote* side of nearly every trade in the market; that volume is
settlement, not a directional opinion anyone expressed. Delta lists no perpetual
for them, and a "long USDT" position is not a trade.

**Five are not listed on Delta as perpetuals** — ACE, CAP, PLUME, DOS, JOHN.
These are also, without exception, the entries whose 24h volume exceeded their
own market cap: ACE turned over 15.4x its cap, JOHN 9.7x at market-cap rank
8,716, DOS 3.1x, CAP 2.6x, PLUME 2.2x. Bitcoin's ratio is 0.02x. A coin trading
its entire capitalisation several times a day is showing wash volume or a
single-venue incentive programme, not depth — so their absence costs the desk
nothing it would have wanted.

What remains is the set that is simultaneously high-volume, genuinely liquid and
actually reachable on the venue this engine executes against.

**The container runs twelve of them.** XRPUSD and LINKUSD are absent from the
deployed `-symbols` list and have been since the desk was launched, so the
engine reports `12 symbols x 1674 streams = 20088 total streams`. Fourteen is
what the analysis above selected; twelve is what is running. Adding the two back
is a restart with a longer list, and it resets nothing — but it does change the
denominator every figure on the board is read against, so it is a decision
rather than a correction.

## Why a separate process

Concurrency limits, the pending-order queue and the paper books are per-process.
Filtering the scalp desk's output down to these fourteen in the browser would
show streams that had spent the whole session competing against ~220 other
symbols for the same fill slots — the numbers would be the scalp desk's, wearing
a different heading.

The question this desk exists to answer is what these strategies do when only
liquid majors are available. Only a process that holds only liquid majors can
answer it.

The scalp desk on 8094 is therefore untouched and stays the control arm. Two
processes, identical code, different universes. That difference is the whole
experiment, and it stops meaning anything the moment either side is tuned to
make the comparison look better.

## Required env (`/home/ubuntu/.scalp_highvol_env`)

Copy `~/.scalp_env` and strip the trading credentials — this desk is paper-only
and must not hold keys:

```
SCALP_API_TOKEN=<must match SCALP_HIGHVOL_API_TOKEN on Vercel>
SCALP_LIVE_ENABLED=false
SCALP_HV_PAPER=true          # the nine-stream $500 Paper Trading book (see below)
# No DELTA_API_KEY / DELTA_API_SECRET. The process has no order-routing path,
# and the proxy carries no arm/disarm route, so there is nothing to arm.
```

## Run

```bash
docker run -d --name antigravity_scalp_highvol --restart unless-stopped \
  --env-file /home/ubuntu/.scalp_highvol_env \
  -p 8096:8094 \
  -v /home/ubuntu/antigravity/data/scalp_highvol:/app/data/scalp_prelive \
  antigravity-scalp:hvpaper1 \
  -symbols BTCUSD,ETHUSD,SOLUSD,BNBUSD,DOGEUSD,ADAUSD,AVAXUSD,TRXUSD,UNIUSD,LTCUSD,BCHUSD,HYPEUSD
```

The container port stays 8094 — that is baked into the image's ENTRYPOINT. Only
the host port differs.

## Firewall

**Port 8096 must be opened in the Lightsail firewall**, or Vercel cannot reach
it and the page shows a 502. `ufw` is inactive on this box; the AWS-level
firewall is what gates every external port.

The existing rule is a range, `8094 → 8095`. Widen it to `8094 → 8096` rather
than adding a second rule:

> Lightsail console → instance → Networking → IPv4 Firewall → edit the
> `8094-8095` Custom/TCP rule → change the end of the range to `8096` →
> Any IPv4 address.

Do not restrict the source IP: Vercel's functions egress from a rotating pool.

## Vercel env

```
SCALP_HIGHVOL_ENGINE_URL   = http://13.233.8.80:8096   (optional; this is the default)
SCALP_HIGHVOL_API_TOKEN    = the container's SCALP_API_TOKEN
```

The proxy falls back to `SCALP_API_TOKEN` then `BTC_PRE_LIVE_API_TOKEN` if the
dedicated one is unset, so the desk works as long as one of the three matches.

## The Paper Trading tab

A second tab on the module: **High Volume Crypto Paper Trading**, the owner's
nine-stream selection off this desk's own board, trading ONE $500 balance.

Why a book and not a filtered view of the leaderboard. A board row is earned
while competing with 20,087 other streams for three concurrency slots, so it
reports what a stream did with the fills it happened to win — a frequency
property before it is a quality one. The book gives nine streams a balance of
their own and starts their record at zero, which is a different measurement and
the only one a promotion could be argued from.

**Engine side.** `internal/delta/perp_paper_highvol.go` holds the roster, the
$500, and a small registry for NAMED books — rosters that are neither numbered,
gold, nor per-symbol. Symbol books refuse any stream off their one symbol and
the gold book is filtered to the gold instruments; this roster spans six
symbols, so it gets its own registration path rather than loosening a check that
is load-bearing for the other two.

Per-book capital is new with it: `livePaperStartingEquityFor` resolves
`SCALP_PAPER_EQUITY_<ID>`, then the size the book registered with, then the
process default. Every ratio the page shows — ROI, share of book, "started at" —
divides by the book's own number, so a $500 book cannot report a $100 book's
percentages. That failure would not have looked like one: a correct balance with
a five-times-inflated return beside it.

**Opt-in per process**, via `SCALP_HV_PAPER=true` in
`/home/ubuntu/.scalp_highvol_env`. Without it the book does not exist. Every
scalp desk shares this binary, so registering unconditionally would give the
scalp, demo and Top Crypto processes a ninth book watching streams their
operators never chose, trading inside their payloads under an id their pages do
not render.

The desk says what it found at boot:

```
[PAPER BOOK HVPAPER] registered: 9 streams, $500.00 starting equity
[HV PAPER] all 9 rostered streams are served by this process
```

That second line is the one to read after a deploy. The roster is written down
rather than derived, so a strategy that left the pack or a symbol below the
turnover floor leaves a row that can never fill — invisible as one silent zero
among 20,088 streams, a ninth of the desk on a nine-row book. When some cannot
be served, the desk names them instead.

**Sizing.** The book runs the process defaults: 3 concurrent positions at 3x
aggregate, so $500 a position and a $1,500 ceiling. Nine streams queue for three
slots, which means the three 10m rows take most of the fills and the 1d row few
— a property of the clock, not of the edge, and the main reason a thin sample
here will still mislead.

**It is not a promotion.** The nine had 18-39 closed trades each against a gate
that asks for 200, selected from the right tail of 20,088 in-sample streams on a
desk that was -$21,837 over 45,158 trades at the time. Membership grants nothing
at the venue: `PerpStreamPermitted` decides that, and it does not read this book.

## Rebuilding the engine image

The host has 1.9GB of RAM and runs the live Indian-market stack, so a Go build
inside it competes with production for memory — the failure mode is the backend
being OOM-killed, not a slow build. Cross-compile on the developer machine and
ship the binary:

```bash
# developer machine, from engine/
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -ldflags="-s -w" \
  -o /tmp/scalp_prelive ./cmd/scalp_prelive
scp -i <key> /tmp/scalp_prelive ubuntu@13.233.8.80:~/hvpaper-build/scalp_prelive

# on the host
cd ~/hvpaper-build && docker build -t antigravity-scalp:<tag> .
```

`~/hvpaper-build/Dockerfile` is runtime-only — the same alpine layers, user,
paths, port and entrypoint as `engine/Dockerfile.scalp`, with the binary copied
in rather than compiled.

Stopping the container saves the paper books (`shutdown: paper books saved`), so
replace it rather than leaving it stopped, and read the restore lines in the new
container's log before walking away.

## Regenerating the page

`client/src/app/high-volume-crypto/DeskView.tsx` is GENERATED — the DESK tab's
body, nothing else. `page.tsx` beside it is hand-written and owns the tab strip;
`PaperTradingTab.tsx` is the $500 book. Do not hand-edit the generated file:

```bash
python scripts/clone_high_volume_page.py
```

It rebuilds the view from `client/src/app/scalp-desk/page.tsx`, rewriting the
endpoints, heading, blurb and export name. The script asserts that no
`/api/scalp/` endpoint survived the rewrite and refuses to emit a page that
would read the wrong desk.

It writes `DeskView.tsx`, never `page.tsx`: generating over the page would
delete the paper tab on every run, and silently, since the desk board is what a
reviewer looks at to decide the clone worked.
