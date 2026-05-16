import { NextResponse } from "next/server";
import { fetchAutoDisabledStrategyIds } from "@/lib/futuresReplayAutoDisable";
import {
  DESK_KILL_MAX_EXPECTANCY_USD_DEFAULT,
  DESK_KILL_MAX_SUM_NET_USD_DEFAULT,
  DESK_KILL_MIN_TRADES_DEFAULT,
} from "@/lib/paperTradesAnalytics";
import { makeSyntheticBtcusd1mCandles } from "@/lib/futuresReplayFixtures";
import { loadReplayFixture, type ReplayFixtureKind } from "@/lib/futuresReplayFixtures";
import { runPaperDeskReplay, type PaperReplayConfig } from "@/lib/futuresReplayEngine";
import { isPaperReplayApiAllowed } from "@/lib/futuresReplayUi";
import { createServiceSupabase } from "@/lib/supabase/server";

export const dynamic = "force-dynamic";

const BTC_FUTURE_TRADING_STRATEGY_IDS = [
  91, 92, 95, 96, 111, 112, 117, 118, 123, 124, 125, 126, 131, 132, 133, 134, 139, 140, 151, 152,
];

function loadFixtureCandles(bars: number, kind: ReplayFixtureKind) {
  try {
    return loadReplayFixture(kind, { maxBars: bars });
  } catch {
    if (kind === "live") {
      return { candles: makeSyntheticBtcusd1mCandles(bars), fundingRate: 0 };
    }
    try {
      return loadReplayFixture("sample", { maxBars: bars });
    } catch {
      return { candles: makeSyntheticBtcusd1mCandles(bars), fundingRate: 0 };
    }
  }
}

export async function GET(req: Request) {
  if (!isPaperReplayApiAllowed()) {
    return NextResponse.json(
      { ok: false, error: "paper-replay requires development or NEXT_PUBLIC_DESK_REPLAY_UI=1" },
      { status: 403 },
    );
  }

  const url = new URL(req.url);
  const symbol = url.searchParams.get("symbol")?.toUpperCase() ?? "BTCUSD";
  const bars = Math.min(2000, Math.max(50, Number(url.searchParams.get("bars") ?? 500) || 500));
  const slippageRaw = url.searchParams.get("slippageBps");
  const slippageBps =
    slippageRaw === null || slippageRaw === ""
      ? 5
      : Math.min(50, Math.max(0, Number(slippageRaw) || 0));
  const fixtureKind: ReplayFixtureKind =
    url.searchParams.get("fixture") === "live" ? "live" : "sample";
  const accountKey = url.searchParams.get("account_key")?.trim() || "btc_future_trading_20";

  let disabledStrategyIds: number[] = [];
  if (url.searchParams.get("autoDisable") === "1") {
    const supabase = createServiceSupabase();
    if (supabase) {
      const ids = await fetchAutoDisabledStrategyIds(supabase, accountKey, {
        windowDays: 14,
        minTrades: DESK_KILL_MIN_TRADES_DEFAULT,
        maxExpectancyUsd: DESK_KILL_MAX_EXPECTANCY_USD_DEFAULT,
        maxSumNetUsd: DESK_KILL_MAX_SUM_NET_USD_DEFAULT,
      });
      if (ids) disabledStrategyIds = ids;
    }
  }

  const loaded = loadFixtureCandles(bars, fixtureKind);
  const candles = loaded.candles;
  const config: PaperReplayConfig = {
    initialBalance: 1000,
    leverage: 25,
    slippageBps,
    strategyIds: BTC_FUTURE_TRADING_STRATEGY_IDS,
    maxPositions: 12,
    barMs: 60_000,
    symbol,
    signalThreshold: 26,
    volSizedNotional: url.searchParams.get("volSized") === "1",
    riskPctOfEquity: 0.01,
    minExpectedMoveSafetyK: 1,
    maxSameDirFracOfEquity: 0.35,
    allowFakeDiversity: false,
    minTpSlRatio: 2,
    fundingRate: loaded.fundingRate ?? 0,
    drawdownLock: url.searchParams.get("drawdownLock") === "1",
    disabledStrategyIds,
  };

  const result = runPaperDeskReplay(candles, config);

  return NextResponse.json({
    ok: true,
    symbol,
    bars: candles.length,
    config,
    trades: result.trades,
    stats: result.stats,
    finalBalance: result.finalBalance,
  });
}
