/**
 * Fetch Delta 1m candles → fixture file.
 *
 * Usage (single call, backward-compatible):
 *   npm run replay:fetch
 *   npm run replay:fetch -- --symbol=BTCUSD --bars=500
 *
 * Usage (multi-day paged fetch):
 *   npm run replay:fetch -- --days=30
 *   npm run replay:fetch -- --days=90 --symbol=BTCUSD
 *   npm run replay:fetch -- --days=30 --out=fixtures/replay/btcusd_1m_30d.json
 *
 * When --days is specified the script pages backwards in 500-bar chunks until
 * the full time window is covered, then writes btcusd_1m_{N}d.json.
 * Without --days the legacy single-call path writes btcusd_1m_live.json.
 */
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import {
  fetchDeltaFutures1mCandles,
  fetchDeltaFutures1mCandlesPaged,
  countCandleGaps,
} from "../src/lib/futuresKlinesFetch";
import {
  REPLAY_FIXTURE_DIR,
  replayFixturePath,
  replayFixturePathForDays,
  computeCoverageDays,
} from "../src/lib/futuresReplayFixtures";
import { loadEnvLocal, parseArg } from "./replayCliShared";

loadEnvLocal();

const symbol = parseArg("symbol", "BTCUSD");
const daysArg = parseArg("days", "");
const barsArg = parseArg("bars", "");
const outArg = parseArg("out", "");

const hasDays = daysArg !== "";
const days = hasDays ? Math.min(90, Math.max(1, Number(daysArg))) : 0;
const targetBars = hasDays
  ? days * 1440
  : Math.max(50, Number(barsArg || "500"));

// Determine output path
let outPath: string;
if (outArg) {
  outPath = path.isAbsolute(outArg)
    ? outArg
    : path.join(process.cwd(), outArg);
} else if (hasDays) {
  outPath = replayFixturePathForDays(days);
} else {
  outPath = replayFixturePath("live");
}

console.log(
  `[replay:fetch] symbol=${symbol}  ${hasDays ? `days=${days}  ` : ""}bars=${targetBars}  out=${outPath}`,
);
if (hasDays) {
  console.log(`[replay:fetch] Paging backwards in 500-bar chunks — estimated ~${Math.ceil(targetBars / 500)} requests`);
}

async function main() {
  let result;

  if (hasDays) {
    let lastPct = -1;
    result = await fetchDeltaFutures1mCandlesPaged(symbol, targetBars, {
      rateLimitMs: 150,
      onProgress: ({ fetched, target }) => {
        const pct = Math.floor((fetched / target) * 100);
        if (pct >= lastPct + 5) {
          lastPct = pct;
          process.stdout.write(`\r[replay:fetch] ${fetched.toLocaleString()}/${target.toLocaleString()} candles (${pct}%)  `);
        }
      },
    });
    process.stdout.write("\n");
  } else {
    result = await fetchDeltaFutures1mCandles(symbol, targetBars);
  }

  mkdirSync(REPLAY_FIXTURE_DIR, { recursive: true });

  const candles = result.candles;
  const gapCount = countCandleGaps(candles, 120_000);
  const coverageDays = computeCoverageDays(candles.length);
  const firstTs = candles.length > 0 ? new Date(candles[0]!.time).toISOString() : "—";
  const lastTs = candles.length > 0 ? new Date(candles[candles.length - 1]!.time).toISOString() : "—";

  const payload = {
    symbol: result.symbol,
    barMs: 60_000,
    fetchedAt: result.fetchedAt,
    source: result.source,
    fundingRate: result.fundingRate,
    candles,
  };

  writeFileSync(outPath, JSON.stringify(payload, null, 0));

  console.log(`[replay:fetch] ─────────────────────────────────`);
  if (hasDays) {
    console.log(`[replay:fetch] Requested days  : ${days}`);
    console.log(`[replay:fetch] Expected candles: ${targetBars.toLocaleString()}`);
  }
  console.log(`[replay:fetch] Candles fetched : ${candles.length.toLocaleString()}`);
  console.log(`[replay:fetch] Coverage        : ${coverageDays.toFixed(2)} days`);
  if (hasDays) {
    const pct = ((candles.length / targetBars) * 100).toFixed(1);
    const ok = candles.length >= targetBars * 0.8;
    console.log(`[replay:fetch] Coverage %      : ${pct}%  ${ok ? "✓ sufficient" : "⚠ below 80% — consider re-running"}`);
  }
  console.log(`[replay:fetch] First candle    : ${firstTs}`);
  console.log(`[replay:fetch] Last candle     : ${lastTs}`);
  console.log(`[replay:fetch] Gaps >2min      : ${gapCount}`);
  console.log(`[replay:fetch] Output          : ${outPath}`);
  console.log(`[replay:fetch] ─────────────────────────────────`);

  if (hasDays && candles.length < targetBars * 0.8) {
    console.warn(
      `[replay:fetch] WARNING: only ${coverageDays.toFixed(1)}d coverage fetched (need ${days}d).` +
      ` Exchange may have limited history. Re-run to append more data, or use a smaller --days value.`,
    );
  }
}

main().catch((e) => {
  console.error(e instanceof Error ? e.message : e);
  process.exit(1);
});
