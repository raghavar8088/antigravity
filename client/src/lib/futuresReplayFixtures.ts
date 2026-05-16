/** Synthetic + file fixtures for offline replay. */

import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

export type ReplayCandle = {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
};

export type ReplayFixtureKind = "sample" | "live";

export type ReplayFixtureFile = {
  symbol: string;
  barMs?: number;
  fetchedAt?: string;
  source?: string;
  fundingRate?: number;
  candles: ReplayCandle[];
};

export const REPLAY_FIXTURE_DIR = join(process.cwd(), "fixtures", "replay");

export function replayFixturePath(kind: ReplayFixtureKind): string {
  return kind === "live"
    ? join(REPLAY_FIXTURE_DIR, "btcusd_1m_live.json")
    : join(REPLAY_FIXTURE_DIR, "btcusd_1m_sample.json");
}

/** Parse fixture JSON from API fetch or hand-authored file. */
export function parseReplayFixtureFile(raw: unknown): ReplayFixtureFile {
  if (!raw || typeof raw !== "object") {
    throw new Error("Fixture root must be an object");
  }
  const o = raw as Record<string, unknown>;
  const symbol = typeof o.symbol === "string" ? o.symbol : "BTCUSD";
  const candlesRaw = o.candles;
  if (!Array.isArray(candlesRaw)) {
    throw new Error("Fixture must include candles[]");
  }
  const candles: ReplayCandle[] = candlesRaw.map((row, i) => {
    if (!row || typeof row !== "object") {
      throw new Error(`candles[${i}] invalid`);
    }
    const c = row as Record<string, unknown>;
    const time = Number(c.time);
    const close = Number(c.close);
    if (!Number.isFinite(time) || !Number.isFinite(close)) {
      throw new Error(`candles[${i}] missing time/close`);
    }
    return {
      time,
      open: Number(c.open) || close,
      high: Number(c.high) || close,
      low: Number(c.low) || close,
      close,
      volume: Number(c.volume) || 0,
    };
  });
  return {
    symbol,
    barMs: typeof o.barMs === "number" ? o.barMs : 60_000,
    fetchedAt: typeof o.fetchedAt === "string" ? o.fetchedAt : undefined,
    source: typeof o.source === "string" ? o.source : undefined,
    fundingRate: typeof o.fundingRate === "number" ? o.fundingRate : undefined,
    candles,
  };
}

export function loadReplayFixtureFromFile(path: string): ReplayFixtureFile {
  const text = readFileSync(path, "utf8");
  return parseReplayFixtureFile(JSON.parse(text) as unknown);
}

export function loadReplayFixture(
  kind: ReplayFixtureKind,
  opts?: { maxBars?: number },
): ReplayFixtureFile {
  const path = replayFixturePath(kind);
  if (!existsSync(path)) {
    throw new Error(`Fixture not found: ${path} (run npm run replay:fetch for live)`);
  }
  const parsed = loadReplayFixtureFromFile(path);
  if (opts?.maxBars && parsed.candles.length > opts.maxBars) {
    return { ...parsed, candles: parsed.candles.slice(-opts.maxBars) };
  }
  return parsed;
}

/** UTC calendar day bounds [startMs, endMs). */
export function utcDayBoundsMs(dateUtc: string): { startMs: number; endMs: number } {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(dateUtc.trim());
  if (!m) throw new Error(`Invalid date (use YYYY-MM-DD): ${dateUtc}`);
  const y = Number(m[1]);
  const mo = Number(m[2]) - 1;
  const d = Number(m[3]);
  const startMs = Date.UTC(y, mo, d, 0, 0, 0, 0);
  const endMs = Date.UTC(y, mo, d + 1, 0, 0, 0, 0);
  return { startMs, endMs };
}

export function filterCandlesByTimeRange(
  candles: ReplayCandle[],
  startMs: number,
  endMs: number,
): ReplayCandle[] {
  return candles.filter((c) => c.time >= startMs && c.time < endMs);
}

/**
 * Deterministic BTCUSD-like 1m series (sin/cos mix) for golden replay tests.
 */
export function makeSyntheticBtcusd1mCandles(
  count: number,
  opts?: { startMs?: number; barMs?: number; basePrice?: number },
): ReplayCandle[] {
  const startMs = opts?.startMs ?? 1_700_000_000_000;
  const barMs = opts?.barMs ?? 60_000;
  const base = opts?.basePrice ?? 100_000;
  const out: ReplayCandle[] = [];
  let prevClose = base;

  for (let i = 0; i < count; i++) {
    const t = startMs + i * barMs;
    const wave =
      Math.sin(i / 15) * 2_500 +
      Math.cos(i / 7) * 1_200 +
      Math.sin(i / 3.7) * 400;
    const close = Math.max(1_000, base + wave);
    const open = i === 0 ? close : prevClose;
    const hi = Math.max(open, close) * (1 + 0.0008 + (i % 5) * 0.0001);
    const lo = Math.min(open, close) * (1 - 0.0008 - (i % 7) * 0.00005);
    const volume = 10 + (i % 20) * 0.5;
    out.push({
      time: t,
      open,
      high: hi,
      low: lo,
      close,
      volume,
    });
    prevClose = close;
  }
  return out;
}
