import type { Signal } from "@/internal/strategy/evaluator";
import type { SignalQualityScore } from "@/internal/trading/signal_scoring";

export interface SignalCandidate {
  signal: Signal;
  quality: SignalQualityScore;
  duplicateCount: number;
  conflictCount: number;
}

export interface SignalAggregatorV2Options {
  maxPerSymbol?: number;
  minConflictScoreGap?: number;
}

export interface AggregationResult {
  candidates: SignalCandidate[];
  deduplicated: number;
  conflictsResolved: number;
  compressed: number;
}

const DEFAULT_MAX_PER_SYMBOL = 3;
const DEFAULT_CONFLICT_GAP = 8;

function duplicateKey(signal: Signal): string {
  return [
    signal.Symbol,
    signal.Direction,
    signal.StrategyID,
    Math.round(signal.Entry * 100) / 100,
    Math.round(signal.StopLoss * 100) / 100,
    Math.round(signal.TakeProfit * 100) / 100,
  ].join("|");
}

function symbolSideKey(signal: Signal): string {
  return `${signal.Symbol}|${signal.Direction}`;
}

function symbolKey(signal: Signal): string {
  return signal.Symbol;
}

export class SignalAggregatorV2 {
  private readonly maxPerSymbol: number;
  private readonly minConflictScoreGap: number;

  constructor(opts: SignalAggregatorV2Options = {}) {
    this.maxPerSymbol = Math.max(1, Math.floor(opts.maxPerSymbol ?? DEFAULT_MAX_PER_SYMBOL));
    this.minConflictScoreGap = Math.max(0, opts.minConflictScoreGap ?? DEFAULT_CONFLICT_GAP);
  }

  aggregate(scoredSignals: readonly SignalQualityScore[]): AggregationResult {
    const byDuplicate = new Map<string, SignalCandidate>();
    let deduplicated = 0;

    for (const quality of scoredSignals) {
      const key = duplicateKey(quality.signal);
      const existing = byDuplicate.get(key);
      if (!existing) {
        byDuplicate.set(key, { signal: quality.signal, quality, duplicateCount: 1, conflictCount: 0 });
        continue;
      }
      deduplicated += 1;
      existing.duplicateCount += 1;
      if (quality.SignalScore > existing.quality.SignalScore) {
        existing.signal = quality.signal;
        existing.quality = quality;
      }
    }

    const sideWinners = new Map<string, SignalCandidate>();
    for (const candidate of byDuplicate.values()) {
      const key = symbolSideKey(candidate.signal);
      const existing = sideWinners.get(key);
      if (!existing || candidate.quality.SignalScore > existing.quality.SignalScore) {
        sideWinners.set(key, candidate);
      }
    }

    const bySymbol = new Map<string, SignalCandidate[]>();
    for (const candidate of sideWinners.values()) {
      const bucket = bySymbol.get(symbolKey(candidate.signal)) ?? [];
      bucket.push(candidate);
      bySymbol.set(symbolKey(candidate.signal), bucket);
    }

    const candidates: SignalCandidate[] = [];
    let conflictsResolved = 0;
    let compressed = 0;

    for (const bucket of bySymbol.values()) {
      bucket.sort((a, b) => b.quality.SignalScore - a.quality.SignalScore);
      const buy = bucket.find((c) => c.signal.Direction === "BUY");
      const sell = bucket.find((c) => c.signal.Direction === "SELL");

      let filtered = bucket;
      if (buy && sell) {
        const gap = Math.abs(buy.quality.SignalScore - sell.quality.SignalScore);
        if (gap < this.minConflictScoreGap) {
          conflictsResolved += 2;
          continue;
        }
        const winner = buy.quality.SignalScore >= sell.quality.SignalScore ? buy : sell;
        const loser = winner === buy ? sell : buy;
        winner.conflictCount += 1;
        loser.conflictCount += 1;
        filtered = [winner, ...bucket.filter((c) => c !== buy && c !== sell)];
        conflictsResolved += 1;
      }

      const selected = filtered.slice(0, this.maxPerSymbol);
      compressed += Math.max(0, filtered.length - selected.length);
      candidates.push(...selected);
    }

    candidates.sort((a, b) => b.quality.SignalScore - a.quality.SignalScore);
    return { candidates, deduplicated, conflictsResolved, compressed };
  }
}
