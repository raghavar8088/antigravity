/**
 * futuresMTFConfluence.ts
 * Pure functions. No side effects. Fully testable.
 *
 * Takes OHLCV snapshots from multiple timeframes and
 * returns a confluence score and directional bias.
 */

import type { FuturesSignalInputs } from "./futuresSignals";

export interface TFSnapshot {
  tf: "1m" | "5m" | "15m" | "1h" | "4h" | "1d";
  close: number;
  ema20: number;
  ema50: number;
  rsi: number;
  atr: number;
  volumeRatio: number;
  isAvailable: boolean;
}

export type Bias = "BULLISH" | "BEARISH" | "NEUTRAL";

export interface TFBiasResult {
  tf: string;
  bias: Bias;
  strength: number;
  reasons: string[];
}

export interface MTFConfluenceResult {
  overallBias: Bias;
  confluenceScore: number;
  agrees: boolean;
  tfResults: TFBiasResult[];
  alignedCount: number;
  totalAvailable: number;
  highTFBias: Bias;
  lowTFBias: Bias;
  isConfluent: boolean;
  reasons: string[];
}

const TF_WEIGHTS: Record<string, number> = {
  "1d": 4,
  "4h": 3,
  "1h": 2,
  "15m": 1.5,
  "5m": 1,
  "1m": 0.5,
};

const MTF_TF_ORDER: TFSnapshot["tf"][] = ["1m", "5m", "15m", "1h", "4h", "1d"];

export function signalInputsToTFSnapshot(
  tf: TFSnapshot["tf"],
  input: FuturesSignalInputs | null | undefined,
): TFSnapshot {
  if (!input) {
    return {
      tf,
      close: 0,
      ema20: 0,
      ema50: 0,
      rsi: 50,
      atr: 0,
      volumeRatio: 1,
      isAvailable: false,
    };
  }
  return {
    tf,
    close: input.markPrice || input.price,
    ema20: input.fast,
    ema50: input.slow,
    rsi: input.rsi14,
    atr: input.atr14,
    volumeRatio: input.volRatio,
    isAvailable: true,
  };
}

export function buildTFSnapshotsFromInputMap(
  inputByTf: ReadonlyMap<string, FuturesSignalInputs>,
): TFSnapshot[] {
  return MTF_TF_ORDER.map((tf) =>
    signalInputsToTFSnapshot(tf, inputByTf.get(tf) ?? null),
  );
}

function scoreTFBias(snap: TFSnapshot): TFBiasResult {
  if (!snap.isAvailable) {
    return { tf: snap.tf, bias: "NEUTRAL", strength: 0, reasons: ["Data unavailable"] };
  }

  let bullSignals = 0;
  let bearSignals = 0;
  const reasons: string[] = [];

  if (snap.ema20 > snap.ema50) {
    bullSignals++;
    reasons.push("EMA20 > EMA50");
  } else {
    bearSignals++;
    reasons.push("EMA20 < EMA50");
  }

  if (snap.close > snap.ema20) {
    bullSignals++;
    reasons.push("Price above EMA20");
  } else {
    bearSignals++;
    reasons.push("Price below EMA20");
  }

  if (snap.rsi > 55) {
    bullSignals++;
    reasons.push(`RSI ${snap.rsi.toFixed(0)} bullish`);
  } else if (snap.rsi < 45) {
    bearSignals++;
    reasons.push(`RSI ${snap.rsi.toFixed(0)} bearish`);
  }

  const strength = Math.max(bullSignals, bearSignals);
  const bias: Bias =
    bullSignals > bearSignals ? "BULLISH" : bearSignals > bullSignals ? "BEARISH" : "NEUTRAL";

  return { tf: snap.tf, bias, strength, reasons };
}

export function computeMTFConfluence(
  snapshots: TFSnapshot[],
  signalSide: "LONG" | "SHORT",
): MTFConfluenceResult {
  const available = snapshots.filter((s) => s.isAvailable);
  const reasons: string[] = [];

  if (available.length === 0) {
    return {
      overallBias: "NEUTRAL",
      confluenceScore: 0,
      agrees: false,
      tfResults: [],
      alignedCount: 0,
      totalAvailable: 0,
      highTFBias: "NEUTRAL",
      lowTFBias: "NEUTRAL",
      isConfluent: false,
      reasons: ["No TF data available"],
    };
  }

  const tfResults = snapshots.map(scoreTFBias);

  let bullWeight = 0;
  let bearWeight = 0;

  for (const result of tfResults) {
    const w = TF_WEIGHTS[result.tf] ?? 1;
    if (result.bias === "BULLISH") bullWeight += w * result.strength;
    if (result.bias === "BEARISH") bearWeight += w * result.strength;
  }

  const totalWeight = bullWeight + bearWeight;
  const overallBias: Bias =
    bullWeight > bearWeight ? "BULLISH" : bearWeight > bullWeight ? "BEARISH" : "NEUTRAL";

  const dominantWeight = Math.max(bullWeight, bearWeight);
  const confluenceScore =
    totalWeight > 0 ? Math.round((dominantWeight / totalWeight) * 100) : 0;

  const highTFs = tfResults.filter((r) => ["4h", "1d"].includes(r.tf));
  const highBull = highTFs.filter((r) => r.bias === "BULLISH").length;
  const highBear = highTFs.filter((r) => r.bias === "BEARISH").length;
  const highTFBias: Bias =
    highBull > highBear ? "BULLISH" : highBear > highBull ? "BEARISH" : "NEUTRAL";

  const lowTFs = tfResults.filter((r) => ["1m", "5m"].includes(r.tf));
  const lowBull = lowTFs.filter((r) => r.bias === "BULLISH").length;
  const lowBear = lowTFs.filter((r) => r.bias === "BEARISH").length;
  const lowTFBias: Bias =
    lowBull > lowBear ? "BULLISH" : lowBear > lowBull ? "BEARISH" : "NEUTRAL";

  const signalBias: Bias = signalSide === "LONG" ? "BULLISH" : "BEARISH";
  const agrees = overallBias === signalBias || overallBias === "NEUTRAL";

  const alignedCount = tfResults.filter((r) => r.bias === overallBias).length;

  if (!agrees) reasons.push(`MTF bias ${overallBias} conflicts with ${signalSide}`);
  if (highTFBias !== overallBias && highTFBias !== "NEUTRAL") {
    reasons.push(`High TF (4h/1d) bias ${highTFBias} diverges`);
  }
  if (confluenceScore >= 70) reasons.push("Strong TF confluence");
  if (confluenceScore < 50) reasons.push("Weak TF confluence");

  return {
    overallBias,
    confluenceScore,
    agrees,
    tfResults,
    alignedCount,
    totalAvailable: available.length,
    highTFBias,
    lowTFBias,
    isConfluent: confluenceScore >= 60 && agrees,
    reasons,
  };
}

export function mtfSkipReason(
  result: MTFConfluenceResult,
  minScore = 55,
): string | null {
  if (!result.agrees) {
    return `MTF_BIAS_CONFLICT:${result.overallBias}`;
  }
  if (result.confluenceScore < minScore) {
    return `MTF_WEAK_CONFLUENCE:${result.confluenceScore}`;
  }
  return null;
}
