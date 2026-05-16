/**
 * Compare Supabase live closes vs replay on live fixture for a UTC day.
 *   npm run replay:compare -- --account_key=btc_future_trading_20 --date=2026-05-16
 */
import { createServiceSupabase, isPaperTradesPersistenceConfigured } from "../src/lib/supabase/server";
import {
  filterCandlesByTimeRange,
  loadReplayFixture,
  utcDayBoundsMs,
} from "../src/lib/futuresReplayFixtures";
import { fetchAutoDisabledStrategyIds } from "../src/lib/futuresReplayAutoDisable";
import {
  formatReplayCompareTable,
  loadSupabaseTradesForUtcDay,
  statsFromClosedTrades,
} from "../src/lib/futuresReplayCompare";
import { runPaperDeskReplay } from "../src/lib/futuresReplayEngine";
import {
  BTC_FUTURE_TRADING_STRATEGY_IDS,
  buildReplayConfigFromCli,
  loadEnvLocal,
  parseArg,
  parseFlag,
} from "./replayCliShared";

loadEnvLocal();

async function main() {
  const accountKey = parseArg("account_key", "btc_future_trading_20");
  const date = parseArg("date", new Date().toISOString().slice(0, 10));

  if (!isPaperTradesPersistenceConfigured()) {
    console.log("Supabase not configured (NEXT_PUBLIC_SUPABASE_URL + SUPABASE_SERVICE_ROLE_KEY in .env.local).");
    console.log("Skipping live side; run replay only with: npm run replay -- --fixture=live");
    process.exit(0);
  }

  const supabase = createServiceSupabase();
  if (!supabase) {
    console.log("Supabase client unavailable.");
    process.exit(0);
  }

  const liveLoad = await loadSupabaseTradesForUtcDay(supabase, accountKey, date);
  if (!liveLoad.ok) {
    console.log(`Could not load live trades: ${liveLoad.reason}`);
    process.exit(0);
  }

  const liveStats = statsFromClosedTrades(liveLoad.trades);
  if (liveLoad.trades.length === 0) {
    console.log(`No closed trades in Supabase for ${accountKey} on UTC ${date}.`);
  }

  let fixture;
  try {
    fixture = loadReplayFixture("live");
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    console.log(`${msg}\nRun: npm run replay:fetch -- --bars=500`);
    process.exit(0);
  }

  const { startMs, endMs } = utcDayBoundsMs(date);
  let candles = filterCandlesByTimeRange(fixture.candles, startMs, endMs);
  if (candles.length < 18) {
    console.warn(
      `[compare] Only ${candles.length} live fixture bars in UTC ${date}; using full fixture (${fixture.candles.length} bars).`,
    );
    candles = fixture.candles;
  }

  const config = buildReplayConfigFromCli();
  config.fundingRate = fixture.fundingRate ?? 0;
  config.strategyIds = BTC_FUTURE_TRADING_STRATEGY_IDS;
  config.signalThreshold = 26;
  config.slippageBps = Number(parseArg("slippageBps", "5"));
  config.drawdownLock = parseFlag("drawdownLock");

  if (parseFlag("autoDisable")) {
    const ids = await fetchAutoDisabledStrategyIds(supabase, accountKey);
    if (ids?.length) config.disabledStrategyIds = [...new Set([...(config.disabledStrategyIds ?? []), ...ids])];
  }

  const replayResult = runPaperDeskReplay(candles, config);

  console.log(`\nCompare ${accountKey} UTC ${date}`);
  console.log(`Live fixture bars: ${candles.length} (funding ${config.fundingRate})`);
  console.log(formatReplayCompareTable(liveStats, replayResult.stats));
  console.log(
    JSON.stringify(
      {
        date,
        accountKey,
        live: liveStats,
        replay: replayResult.stats,
        note: "Replay is 1 bar = 1 step; live polls every ~4s. Expect trade-count drift.",
      },
      null,
      2,
    ),
  );
}

main().catch((e) => {
  console.error(e instanceof Error ? e.message : e);
  process.exit(1);
});
