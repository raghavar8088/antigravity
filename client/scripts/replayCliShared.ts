import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import {
  loadReplayFixture,
  makeSyntheticBtcusd1mCandles,
  replayFixturePath,
  type ReplayCandle,
  type ReplayFixtureKind,
} from "../src/lib/futuresReplayFixtures";
import type { PaperReplayConfig } from "../src/lib/futuresReplayEngine";
import { BTC_FUTURE_TRADING_STRATEGY_IDS } from "../src/lib/btcFutureTradingRoster";

export { BTC_FUTURE_TRADING_STRATEGY_IDS };

export function loadEnvLocal(): void {
  const path = join(process.cwd(), ".env.local");
  if (!existsSync(path)) return;
  for (const line of readFileSync(path, "utf8").split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;
    const eq = trimmed.indexOf("=");
    if (eq <= 0) continue;
    const key = trimmed.slice(0, eq).trim();
    let val = trimmed.slice(eq + 1).trim();
    if (
      (val.startsWith('"') && val.endsWith('"')) ||
      (val.startsWith("'") && val.endsWith("'"))
    ) {
      val = val.slice(1, -1);
    }
    if (!(key in process.env)) process.env[key] = val;
  }
}

export function parseArg(name: string, fallback: string): string {
  const hit = process.argv.find((a) => a.startsWith(`--${name}=`));
  return hit ? (hit.split("=")[1] ?? fallback) : fallback;
}

export function parseFlag(name: string): boolean {
  return process.argv.includes(`--${name}=1`) || process.argv.includes(`--${name}`);
}

export function loadReplayCandles(opts: {
  fixture: ReplayFixtureKind;
  bars: number;
}): { candles: ReplayCandle[]; fundingRate: number; fixturePath: string } {
  const fixturePath = replayFixturePath(opts.fixture);
  if (opts.fixture === "live") {
    const file = loadReplayFixture("live", { maxBars: opts.bars });
    return {
      candles: file.candles,
      fundingRate: file.fundingRate ?? 0,
      fixturePath,
    };
  }
  try {
    const file = loadReplayFixture("sample", { maxBars: opts.bars });
    return { candles: file.candles, fundingRate: file.fundingRate ?? 0, fixturePath };
  } catch {
    const candles = makeSyntheticBtcusd1mCandles(Math.max(opts.bars, 500));
    return { candles: candles.slice(0, opts.bars), fundingRate: 0, fixturePath };
  }
}

export function buildReplayConfigFromCli(): PaperReplayConfig {
  const slippageBps = Number(parseArg("slippageBps", "5"));
  const disableCsv = parseArg("disableIds", "");
  const disabledFromCsv = disableCsv
    ? disableCsv
        .split(",")
        .map((s) => Number(s.trim()))
        .filter((n) => Number.isFinite(n) && n > 0)
    : undefined;

  return {
    initialBalance: Number(parseArg("balance", "1000")),
    leverage: 25,
    slippageBps,
    strategyIds: BTC_FUTURE_TRADING_STRATEGY_IDS,
    maxPositions: 12,
    barMs: 60_000,
    symbol: parseArg("symbol", "BTCUSD"),
    signalThreshold: Number(parseArg("signalThreshold", "26")),
    volSizedNotional: parseFlag("volSized"),
    riskPctOfEquity: Number(parseArg("riskPct", "0.01")),
    minExpectedMoveSafetyK: Number(parseArg("minMoveK", "1")),
    maxSameDirFracOfEquity: Number(parseArg("sameDirFrac", "0.35")),
    allowFakeDiversity: parseFlag("fakeDiv"),
    minTpSlRatio: 2,
    fundingRate: Number(parseArg("fundingRate", "0")),
    drawdownLock: parseFlag("drawdownLock"),
    disabledStrategyIds: disabledFromCsv,
    maxOpenPerCategory: (() => {
      const hit = process.argv.find((a) => a.startsWith("--maxOpenPerCategory="));
      if (!hit) return undefined;
      const n = Number(hit.split("=")[1]);
      if (!Number.isFinite(n)) return undefined;
      return Math.min(12, Math.max(1, Math.floor(n)));
    })(),
    sessionStartUtcHour: (() => {
      const hit = process.argv.find((a) => a.startsWith("--sessionStart="));
      if (!hit) return undefined;
      const n = Number(hit.split("=")[1]);
      if (!Number.isFinite(n)) return undefined;
      return Math.min(23, Math.max(0, Math.floor(n)));
    })(),
    sessionEndUtcHour: (() => {
      const hit = process.argv.find((a) => a.startsWith("--sessionEnd="));
      if (!hit) return undefined;
      const n = Number(hit.split("=")[1]);
      if (!Number.isFinite(n)) return undefined;
      return Math.min(24, Math.max(0, Math.floor(n)));
    })(),
  };
}
