import type { MarketCandle } from "@/hooks/useLiveBTCMarket";

export type MarketSignal = {
  side: "BUY" | "SELL" | "NEUTRAL";
  confidence: number;
  tag: string;
  scoreBuy: number;
  scoreSell: number;
  reasons: string[];
  strategies: string[];
};

export type MarketSentiment = {
  label: "STRONG BUY" | "BULLISH" | "NEUTRAL" | "BEARISH" | "STRONG SELL";
  colorClass: string;
};

type BollingerBand = {
  upper: number;
  middle: number;
  lower: number;
  width: number;
};

function isBull(candle: MarketCandle): boolean {
  return candle.close > candle.open;
}

function isBear(candle: MarketCandle): boolean {
  return candle.close < candle.open;
}

function bodySize(candle: MarketCandle): number {
  return Math.abs(candle.close - candle.open);
}

export function calcEMA(series: number[], period: number): number[] {
  if (series.length === 0) {
    return [];
  }

  const multiplier = 2 / (period + 1);
  const output: number[] = [series[0]];
  for (let index = 1; index < series.length; index += 1) {
    output.push(series[index] * multiplier + output[index - 1] * (1 - multiplier));
  }
  return output;
}

export function calcRSI(closes: number[], period = 14): number[] {
  if (closes.length < period + 1) {
    return closes.map(() => 50);
  }

  const output: number[] = [];
  let gains = 0;
  let losses = 0;

  for (let index = 1; index <= period; index += 1) {
    const delta = closes[index] - closes[index - 1];
    if (delta > 0) {
      gains += delta;
    } else {
      losses -= delta;
    }
  }

  gains /= period;
  losses /= period;

  for (let index = 0; index < period; index += 1) {
    output.push(50);
  }

  output.push(losses === 0 ? 100 : 100 - 100 / (1 + gains / losses));

  for (let index = period + 1; index < closes.length; index += 1) {
    const delta = closes[index] - closes[index - 1];
    gains = (gains * (period - 1) + (delta > 0 ? delta : 0)) / period;
    losses = (losses * (period - 1) + (delta < 0 ? -delta : 0)) / period;
    output.push(losses === 0 ? 100 : 100 - 100 / (1 + gains / losses));
  }

  return output;
}

export function calcMACDHistogram(closes: number[]): number[] {
  const fast = calcEMA(closes, 12);
  const slow = calcEMA(closes, 26);
  const macd = fast.map((value, index) => value - slow[index]);
  const signal = calcEMA(macd, 9);
  return macd.map((value, index) => value - signal[index]);
}

export function calcVWAP(candles: MarketCandle[]): number[] {
  let cumulativeVolume = 0;
  let cumulativeTurnover = 0;

  return candles.map((candle) => {
    cumulativeVolume += candle.volume;
    cumulativeTurnover += candle.close * candle.volume;
    return cumulativeVolume > 0 ? cumulativeTurnover / cumulativeVolume : candle.close;
  });
}

function calcBollingerBands(closes: number[], period = 20, multiplier = 2): BollingerBand[] {
  return closes.map((value, index) => {
    if (index < period - 1) {
      return { upper: value, middle: value, lower: value, width: 0 };
    }

    const slice = closes.slice(index - period + 1, index + 1);
    const average = slice.reduce((sum, entry) => sum + entry, 0) / period;
    const variance = slice.reduce((sum, entry) => sum + (entry - average) ** 2, 0) / period;
    const deviation = Math.sqrt(variance);
    const upper = average + deviation * multiplier;
    const lower = average - deviation * multiplier;

    return {
      upper,
      middle: average,
      lower,
      width: average > 0 ? (upper - lower) / average : 0,
    };
  });
}

export function calcMarketSentiment(prices: number[]): MarketSentiment {
  if (prices.length < 40) {
    return { label: "NEUTRAL", colorClass: "text-zinc-400" };
  }

  const rsi = calcRSI(prices, 14);
  const macd = calcMACDHistogram(prices);
  const ema9 = calcEMA(prices, 9);
  const ema21 = calcEMA(prices, 21);
  const fairValue = calcEMA(prices, 55);

  let bullishCloses = 0;
  for (let index = Math.max(1, prices.length - 5); index < prices.length; index += 1) {
    if (prices[index] > prices[index - 1]) {
      bullishCloses += 1;
    }
  }

  let score = 0;
  const lastIndex = prices.length - 1;
  if (rsi[lastIndex] > 60) {
    score += 2;
  } else if (rsi[lastIndex] < 40) {
    score -= 2;
  }

  if (macd[lastIndex] > 0) {
    score += 2;
  } else if (macd[lastIndex] < 0) {
    score -= 2;
  }

  if (ema9[lastIndex] > ema21[lastIndex]) {
    score += 1;
  } else {
    score -= 1;
  }

  if (bullishCloses >= 4) {
    score += 2;
  } else if (bullishCloses <= 1) {
    score -= 2;
  }

  if (prices[lastIndex] > fairValue[lastIndex]) {
    score += 1;
  } else {
    score -= 1;
  }

  if (score >= 4) {
    return { label: "STRONG BUY", colorClass: "text-emerald-300" };
  }
  if (score >= 2) {
    return { label: "BULLISH", colorClass: "text-green-300" };
  }
  if (score <= -4) {
    return { label: "STRONG SELL", colorClass: "text-rose-300" };
  }
  if (score <= -2) {
    return { label: "BEARISH", colorClass: "text-red-300" };
  }

  return { label: "NEUTRAL", colorClass: "text-zinc-400" };
}

export function detectMarketSignal(candles: MarketCandle[]): MarketSignal | null {
  if (candles.length < 40) {
    return null;
  }

  const closes = candles.map((candle) => candle.close);
  const volumes = candles.map((candle) => candle.volume);
  const lastIndex = closes.length - 1;
  const latest = candles[lastIndex];
  const previous = candles[lastIndex - 1];
  const recent = candles.slice(-3);
  const averageBody = candles.slice(-20).reduce((sum, candle) => sum + bodySize(candle), 0) / 20;
  const volumeNow = volumes[lastIndex] || 0;
  const volumeAverage = volumes.slice(-20).reduce((sum, value) => sum + value, 0) / 20;

  const ema9 = calcEMA(closes, 9);
  const ema21 = calcEMA(closes, 21);
  const ema55 = calcEMA(closes, 55);
  const rsi = calcRSI(closes, 14);
  const macdHistogram = calcMACDHistogram(closes);
  const vwap = calcVWAP(candles);
  const bollinger = calcBollingerBands(closes, 20, 2);

  let buy = 0;
  let sell = 0;
  const reasons: string[] = [];
  const strategies: string[] = [];

  if (ema9[lastIndex] > ema21[lastIndex]) {
    buy += 2;
    reasons.push("EMA fast trend up");
    strategies.push("EMA Cross");
  } else {
    sell += 2;
    reasons.push("EMA fast trend down");
    strategies.push("EMA Cross");
  }

  if (ema21[lastIndex] > ema55[lastIndex]) {
    buy += 1;
    reasons.push("Trend above fair value");
  } else {
    sell += 1;
    reasons.push("Trend below fair value");
  }

  if (macdHistogram[lastIndex] > 0 && macdHistogram[lastIndex] >= macdHistogram[lastIndex - 1]) {
    buy += 2;
    reasons.push("MACD momentum rising");
    strategies.push("MACD");
  } else if (macdHistogram[lastIndex] < 0 && macdHistogram[lastIndex] <= macdHistogram[lastIndex - 1]) {
    sell += 2;
    reasons.push("MACD momentum falling");
    strategies.push("MACD");
  }

  if (rsi[lastIndex] < 32) {
    buy += 3;
    reasons.push(`RSI oversold (${rsi[lastIndex].toFixed(0)})`);
    strategies.push("RSI Revert");
  } else if (rsi[lastIndex] > 68) {
    sell += 3;
    reasons.push(`RSI overbought (${rsi[lastIndex].toFixed(0)})`);
    strategies.push("RSI Revert");
  }

  if (recent.every((candle) => isBull(candle) && bodySize(candle) > averageBody * 0.25)) {
    buy += 2;
    reasons.push("Three-candle bullish momentum");
    strategies.push("Momentum");
  }

  if (recent.every((candle) => isBear(candle) && bodySize(candle) > averageBody * 0.25)) {
    sell += 2;
    reasons.push("Three-candle bearish momentum");
    strategies.push("Momentum");
  }

  if (closes[lastIndex] > vwap[lastIndex] * 1.0004) {
    buy += 1;
    reasons.push("Price above VWAP");
    strategies.push("VWAP");
  } else if (closes[lastIndex] < vwap[lastIndex] * 0.9996) {
    sell += 1;
    reasons.push("Price below VWAP");
    strategies.push("VWAP");
  }

  const band = bollinger[lastIndex];
  const bandRange = band.upper - band.lower;
  const bandPosition = bandRange > 0 ? (latest.close - band.lower) / bandRange : 0.5;
  if (bandPosition < 0.18) {
    buy += 2;
    reasons.push("Near lower Bollinger band");
    strategies.push("BB Bounce");
  } else if (bandPosition > 0.82) {
    sell += 2;
    reasons.push("Near upper Bollinger band");
    strategies.push("BB Bounce");
  }

  if (band.width > 0 && lastIndex >= 5) {
    const priorWidths = bollinger.slice(lastIndex - 5, lastIndex).map((entry) => entry.width);
    const averageWidth = priorWidths.reduce((sum, value) => sum + value, 0) / priorWidths.length;
    if (averageWidth > 0 && band.width > averageWidth * 1.35) {
      if (latest.close >= previous.close) {
        buy += 1;
        reasons.push("Volatility expansion upward");
      } else {
        sell += 1;
        reasons.push("Volatility expansion downward");
      }
      strategies.push("BB Squeeze");
    }
  }

  if (volumeAverage > 0 && volumeNow > volumeAverage * 1.35) {
    if (latest.close >= latest.open) {
      buy += 1;
      reasons.push(`Volume expansion (${(volumeNow / volumeAverage).toFixed(1)}x)`);
    } else {
      sell += 1;
      reasons.push(`Volume expansion (${(volumeNow / volumeAverage).toFixed(1)}x)`);
    }
    strategies.push("Volume");
  }

  const side = buy > sell ? "BUY" : sell > buy ? "SELL" : "NEUTRAL";
  const dominance = Math.max(buy, sell);
  const confidence = Math.min(95, Math.round(40 + dominance * 5 + Math.abs(buy - sell) * 2));

  const primaryStrategy = strategies[strategies.length - 1];
  const tagMap: Record<string, { BUY: string; SELL: string }> = {
    "EMA Cross": { BUY: "EMA Scalp Long", SELL: "EMA Scalp Short" },
    MACD: { BUY: "MACD Momentum Long", SELL: "MACD Momentum Short" },
    "RSI Revert": { BUY: "RSI Revert Long", SELL: "RSI Revert Short" },
    Momentum: { BUY: "Momentum Long", SELL: "Momentum Short" },
    VWAP: { BUY: "VWAP Hold Long", SELL: "VWAP Hold Short" },
    "BB Bounce": { BUY: "BB Bounce Long", SELL: "BB Bounce Short" },
    "BB Squeeze": { BUY: "BB Squeeze Long", SELL: "BB Squeeze Short" },
    Volume: { BUY: "Volume Break Long", SELL: "Volume Break Short" },
  };

  const tag = side === "NEUTRAL"
    ? "No Edge"
    : primaryStrategy && tagMap[primaryStrategy]
      ? tagMap[primaryStrategy][side]
      : side === "BUY"
        ? "Aggressive Long"
        : "Aggressive Short";

  return {
    side,
    confidence,
    tag,
    scoreBuy: buy,
    scoreSell: sell,
    reasons,
    strategies,
  };
}
