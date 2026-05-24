/**
 * futuresSoakTracker.ts
 * Daily paper-desk soak snapshots (localStorage, dev/paper only).
 */

import type { DeskRollingPnLScorecard } from "./futuresDeskPnLTracker";

export type SoakDayGrade = "GREEN" | "YELLOW" | "RED";

export interface SoakDaySnapshot {
  dateUtc: string;
  closes: number;
  expectancy: number;
  feePctOfAbsGross: number;
  winRate: number;
  profitModeSkipCount: number;
  grade: SoakDayGrade;
}

const MAX_SOAK_DAYS = 14;

export function utcDateString(nowMs = Date.now()): string {
  return new Date(nowMs).toISOString().slice(0, 10);
}

function gradeFromScorecard(scorecard: DeskRollingPnLScorecard): SoakDayGrade {
  if (scorecard.paperReadyHint === "ON_TRACK") return "GREEN";
  if (
    scorecard.last50.tradeCount >= 10 &&
    scorecard.passesFeeTarget50 &&
    scorecard.passesExpectancyTarget50
  ) {
    return "GREEN";
  }
  if (scorecard.paperReadyHint === "REVIEW" || scorecard.last20.tradeCount >= 10) {
    return "YELLOW";
  }
  return "RED";
}

export function snapshotFromScorecard(
  scorecard: DeskRollingPnLScorecard,
  skipCount: number,
  dateUtc = utcDateString(scorecard.computedAt),
): SoakDaySnapshot {
  const slice = scorecard.last50.tradeCount >= 10 ? scorecard.last50 : scorecard.last20;
  return {
    dateUtc,
    closes: scorecard.closes48h > 0 ? Math.min(scorecard.closes48h, slice.tradeCount) : slice.tradeCount,
    expectancy: slice.expectancy,
    feePctOfAbsGross: slice.feePctOfAbsGross,
    winRate: slice.winRate,
    profitModeSkipCount: skipCount,
    grade: gradeFromScorecard(scorecard),
  };
}

/** Upsert today's UTC row; keeps last MAX_SOAK_DAYS days sorted oldest-first. */
export function appendSoakSnapshot(
  existing: SoakDaySnapshot[],
  scorecard: DeskRollingPnLScorecard,
  skipCount: number,
): SoakDaySnapshot[] {
  const snap = snapshotFromScorecard(scorecard, skipCount);
  const without = existing.filter((s) => s.dateUtc !== snap.dateUtc);
  const next = [...without, snap].sort((a, b) => a.dateUtc.localeCompare(b.dateUtc));
  return next.slice(-MAX_SOAK_DAYS);
}

export function loadSoakHistory(key: string): SoakDaySnapshot[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as SoakDaySnapshot[];
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

export function saveSoakHistory(key: string, snaps: SoakDaySnapshot[]): void {
  if (typeof window === "undefined") return;
  try {
    localStorage.setItem(key, JSON.stringify(snaps.slice(-MAX_SOAK_DAYS)));
  } catch {
    /* quota / private mode */
  }
}

export function soakTrendSummary(snaps: SoakDaySnapshot[]): {
  daysTracked: number;
  greenDays: number;
  avgExpectancy7d: number;
  improving: boolean;
} {
  const last7 = snaps.slice(-7);
  const daysTracked = last7.length;
  const greenDays = last7.filter((s) => s.grade === "GREEN").length;
  const avgExpectancy7d =
    daysTracked > 0 ? last7.reduce((s, d) => s + d.expectancy, 0) / daysTracked : 0;

  let improving = false;
  if (last7.length >= 6) {
    const recent = last7.slice(-3);
    const prior = last7.slice(-6, -3);
    const recentAvg = recent.reduce((s, d) => s + d.expectancy, 0) / recent.length;
    const priorAvg = prior.reduce((s, d) => s + d.expectancy, 0) / prior.length;
    improving = recentAvg > priorAvg;
  }

  return { daysTracked, greenDays, avgExpectancy7d, improving };
}
