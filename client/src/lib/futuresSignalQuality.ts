/**
 * futuresSignalQuality.ts
 * Pure functions. No side effects. Fully testable.
 *
 * Scores each entry signal on multiple quality dimensions
 * before it reaches the entry gate. Acts as a pre-filter
 * that enriches the signal with a quality score breakdown.
 *
 * Higher quality = more likely to reach TP before SL.
 */

export interface SignalQualityInput {
  signalScore: number;
  atrPct: number;
  spreadPct: number;
  volumeRatio: number;
  regime: "chop" | "trendLow" | "trendHigh" | string;
  regimeFitsStrategy: boolean;
  ema20AboveEma50: boolean;
  priceAboveEma20: boolean;
  side: "LONG" | "SHORT";
  openPositionCount: number;
  sameSideCount: number;
  hoursIntoSession: number;
  strategyWinRate: number;
  strategyTrades: number;
  cooldownRemainMs: number;
}

export interface SignalQualityScore {
  total: number;
  pass: boolean;
  minPassScore: number;
  momentumScore: number;
  regimeScore: number;
  sessionScore: number;
  strategyScore: number;
  signalStrengthScore: number;
  isHighQuality: boolean;
  isMarginal: boolean;
  isLowQuality: boolean;
  deductions: string[];
  bonuses: string[];
}

const MIN_PASS_SCORE = 50;
const HIGH_QUALITY = 75;

export function scoreSignalQuality(input: SignalQualityInput): SignalQualityScore {
  const deductions: string[] = [];
  const bonuses: string[] = [];

  let momentumScore = 0;

  if (input.atrPct >= 0.001) {
    momentumScore += 8;
    bonuses.push("ATR strong");
  } else if (input.atrPct >= 0.0006) {
    momentumScore += 4;
  } else {
    deductions.push("ATR too low");
  }

  if (input.volumeRatio >= 1.5) {
    momentumScore += 7;
    bonuses.push("Volume spike");
  } else if (input.volumeRatio >= 1.1) {
    momentumScore += 4;
  } else if (input.volumeRatio < 0.8) {
    deductions.push("Low volume");
  } else {
    momentumScore += 2;
  }

  if (input.spreadPct <= 0.0002) {
    momentumScore += 5;
    bonuses.push("Tight spread");
  } else if (input.spreadPct <= 0.0005) {
    momentumScore += 3;
  } else {
    deductions.push("Wide spread");
  }

  momentumScore = Math.min(momentumScore, 20);

  let regimeScore = 0;

  if (input.regimeFitsStrategy) {
    regimeScore += 10;
    bonuses.push("Regime aligned");
  } else {
    deductions.push("Regime mismatch");
  }

  const isBullish = input.ema20AboveEma50 && input.priceAboveEma20;
  const isBearish = !input.ema20AboveEma50 && !input.priceAboveEma20;

  if (input.side === "LONG" && isBullish) {
    regimeScore += 7;
    bonuses.push("Trend aligned LONG");
  }
  if (input.side === "SHORT" && isBearish) {
    regimeScore += 7;
    bonuses.push("Trend aligned SHORT");
  }
  if (input.side === "LONG" && isBearish) {
    deductions.push("Counter-trend LONG");
  }
  if (input.side === "SHORT" && isBullish) {
    deductions.push("Counter-trend SHORT");
  }

  if (input.regime === "chop") {
    regimeScore = Math.max(0, regimeScore - 5);
    deductions.push("Chop regime -5");
  }

  regimeScore = Math.min(regimeScore, 20);

  let sessionScore = 20;

  if (input.openPositionCount >= 8) {
    sessionScore -= 8;
    deductions.push("Many open positions");
  } else if (input.openPositionCount >= 4) {
    sessionScore -= 4;
  }

  if (input.sameSideCount >= 2) {
    sessionScore -= 6;
    deductions.push("Same-side concentration");
  } else if (input.sameSideCount >= 1) {
    sessionScore -= 2;
  }

  if (input.cooldownRemainMs > 0) {
    sessionScore -= 10;
    deductions.push(`Cooldown ${Math.ceil(input.cooldownRemainMs / 60_000)}m remaining`);
  }

  if (input.hoursIntoSession >= 4 && input.hoursIntoSession <= 20) {
    sessionScore = Math.min(sessionScore + 2, 20);
    bonuses.push("Active session hours");
  }

  sessionScore = Math.max(0, Math.min(sessionScore, 20));

  let strategyScore = 0;

  if (input.strategyTrades < 5) {
    strategyScore = 10;
  } else {
    if (input.strategyWinRate >= 0.6) {
      strategyScore += 12;
      bonuses.push("High WR strategy");
    } else if (input.strategyWinRate >= 0.4) {
      strategyScore += 8;
    } else if (input.strategyWinRate >= 0.3) {
      strategyScore += 4;
    } else {
      deductions.push("Low WR strategy");
    }

    if (input.strategyTrades >= 50) strategyScore += 5;
    else if (input.strategyTrades >= 20) strategyScore += 3;
    else strategyScore += 1;

    if (input.strategyWinRate >= 0.5 && input.strategyTrades >= 20) {
      strategyScore += 3;
      bonuses.push("Proven edge");
    }
  }

  strategyScore = Math.min(strategyScore, 20);

  const normalised = Math.min(input.signalScore / 50, 1);
  let signalStrengthScore = Math.round(normalised * 20);

  if (input.signalScore >= 40) {
    bonuses.push("Very strong signal");
  }

  signalStrengthScore = Math.min(signalStrengthScore, 20);

  const total = Math.min(
    momentumScore + regimeScore + sessionScore + strategyScore + signalStrengthScore,
    100,
  );

  const pass = total >= MIN_PASS_SCORE;
  const isHighQuality = total >= HIGH_QUALITY;
  const isMarginal = total >= MIN_PASS_SCORE && total < HIGH_QUALITY;
  const isLowQuality = total < MIN_PASS_SCORE;

  return {
    total,
    pass,
    minPassScore: MIN_PASS_SCORE,
    momentumScore,
    regimeScore,
    sessionScore,
    strategyScore,
    signalStrengthScore,
    isHighQuality,
    isMarginal,
    isLowQuality,
    deductions,
    bonuses,
  };
}

export function rankSignalsByQuality(
  inputs: Array<SignalQualityInput & { strategyId: number }>,
): Array<{ strategyId: number; quality: SignalQualityScore }> {
  return inputs
    .map((inp) => ({
      strategyId: inp.strategyId,
      quality: scoreSignalQuality(inp),
    }))
    .sort((a, b) => b.quality.total - a.quality.total);
}
