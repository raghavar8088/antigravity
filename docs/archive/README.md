# 2026-07 options engine custody — archived 2026-08-09

The options engine's trade custody was purged on 2026-08-09. The Live Engine
page reads Closed Positions, Order History and Daily P&L from that one file, so
until it was cleared the page showed 11-day-old options activity while the live
engine had moved to the 31 perpetual streams of paper Account 03.

Nothing live depended on it: 0 records were OPEN, and the file had not been
written since 2026-07-29.

| file | what it is |
|---|---|
| `2026-07_options_engine_custody_raw.json` | the complete 127-record file, exactly as it stood |
| `2026-07_options_engine_closed_trades.csv` | the 24 real-money closes, readable |

## Why the CSV carries two P&L columns

`realizedPnlUsd_asStored` sums to **-$72.28**. `grossPnlUsd_derivedFromPrices`
sums to **-$1.35**, which is what the UI reported.

One 2026-07-27 record is responsible for the entire gap:

    P-BTC-63800-280726   1 contract   entry 100   exit 29   stored -71.00

Every other record equals `(exit - entry) x contracts x 0.001`, a contract
being 0.001 BTC. That one skipped the contract size, so its stored field is
1000x too large.

It is a fossil, not a live defect — it predates the fix that added the
multiplier, and all three P&L paths in `internal/delta/live_bridge.go` now
apply it. The UI was never wrong, because it derives from prices rather than
reading the stored field.

Both columns are kept rather than silently correcting one. An archive that
quietly rewrites history is worse than one that shows the discrepancy.
