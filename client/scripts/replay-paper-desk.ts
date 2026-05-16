/**
 * Offline replay CLI — run from `client/`:
 *   npm run replay
 *   npm run replay -- --fixture=live --bars=500 --slippageBps=5 --drawdownLock=1
 */
import { createServiceSupabase } from "../src/lib/supabase/server";
import { fetchAutoDisabledStrategyIds, parseDisabledStrategyIdsCsv } from "../src/lib/futuresReplayAutoDisable";
import { runPaperDeskReplay } from "../src/lib/futuresReplayEngine";
import type { ReplayFixtureKind } from "../src/lib/futuresReplayFixtures";
import {
  buildReplayConfigFromCli,
  loadEnvLocal,
  loadReplayCandles,
  parseArg,
  parseFlag,
} from "./replayCliShared";

loadEnvLocal();

async function main() {
  const fixture = (parseArg("fixture", "sample") as ReplayFixtureKind) || "sample";
  const bars = Number(parseArg("bars", "500"));
  const { candles, fundingRate, fixturePath } = loadReplayCandles({ fixture, bars });

  const config = buildReplayConfigFromCli();
  config.fundingRate = Number.isFinite(config.fundingRate) && config.fundingRate !== 0
    ? config.fundingRate!
    : fundingRate;

  const extraDisabled = parseDisabledStrategyIdsCsv(parseArg("disableIds", ""));
  let autoDisabled: number[] = [];
  if (parseFlag("autoDisable")) {
    const supabase = createServiceSupabase();
    if (!supabase) {
      console.error("[replay] --autoDisable=1 but Supabase env missing (.env.local)");
    } else {
      const ids = await fetchAutoDisabledStrategyIds(supabase, parseArg("account_key", "btc_future_trading_20"));
      if (ids) autoDisabled = ids;
    }
  }
  config.disabledStrategyIds = [...new Set([...(config.disabledStrategyIds ?? []), ...extraDisabled, ...autoDisabled])];

  const result = runPaperDeskReplay(candles, config);
  console.log(
    JSON.stringify(
      {
        fixture,
        fixturePath,
        bars: candles.length,
        config,
        stats: result.stats,
        finalBalance: result.finalBalance,
        tradeCount: result.trades.length,
        firstTrade: result.trades[0] ?? null,
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
