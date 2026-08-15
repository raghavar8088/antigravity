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
# No DELTA_API_KEY / DELTA_API_SECRET. The process has no order-routing path,
# and the proxy carries no arm/disarm route, so there is nothing to arm.
```

## Run

```bash
docker run -d --name antigravity_scalp_highvol --restart unless-stopped \
  --env-file /home/ubuntu/.scalp_highvol_env \
  -p 8096:8094 \
  -v /home/ubuntu/antigravity/data/scalp_highvol:/app/data/scalp_prelive \
  antigravity-scalp:acct06 \
  -symbols BTCUSD,ETHUSD,SOLUSD,XRPUSD,BNBUSD,DOGEUSD,ADAUSD,AVAXUSD,LINKUSD,TRXUSD,UNIUSD,LTCUSD,BCHUSD,HYPEUSD
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

## Regenerating the page

`client/src/app/high-volume-crypto/page.tsx` is GENERATED. Do not hand-edit it:

```bash
python scripts/clone_high_volume_page.py
```

It rebuilds the page from `client/src/app/scalp-desk/page.tsx`, rewriting the
endpoints, heading and blurb. The script asserts that no `/api/scalp/` endpoint
survived the rewrite and refuses to emit a page that would read the wrong desk.
