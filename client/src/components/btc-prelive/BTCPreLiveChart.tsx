"use client";

/**
 * BTC Pre-Live Engine — TradingView-style candle chart (Phase 1).
 *
 * Real Delta Exchange BTCUSD candles via /api/btc-prelive/candles:
 *  - timeframe switcher (5m/15m/1h/4h/1d)
 *  - lazy-loads up to ONE YEAR of history as the user pans left
 *  - live tail: 3s ticker poll updates the forming candle
 *
 * Modeled on components/BTCLiveChart.tsx (house lightweight-charts v5 style);
 * fully additive — nothing else imports this yet.
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { createChart, CandlestickSeries, HistogramSeries, LineStyle } from "lightweight-charts";
import type { CandlestickData, HistogramData, IChartApi, ISeriesApi, Time } from "lightweight-charts";

type Candle = { time: number; open: number; high: number; low: number; close: number; volume: number };

type CandlesResponse = {
  ok: boolean;
  candles?: Candle[];
  reachedHistoryCap?: boolean;
  lastPrice?: number;
  markPrice?: number;
  changePct24h?: number;
  fundingRate?: number;
  error?: string;
};

const TIMEFRAMES = ["5m", "15m", "1h", "4h", "1d"] as const;
export type Timeframe = (typeof TIMEFRAMES)[number];

const TF_MINUTES: Record<Timeframe, number> = { "5m": 5, "15m": 15, "1h": 60, "4h": 240, "1d": 1440 };

interface TickerStats {
  lastPrice: number;
  changePct24h: number;
  fundingRate: number;
}

interface Props {
  onTicker?: (stats: TickerStats) => void;
}

export default function BTCPreLiveChart({ onTicker }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const candleSeriesRef = useRef<ISeriesApi<"Candlestick"> | null>(null);
  const volumeSeriesRef = useRef<ISeriesApi<"Histogram"> | null>(null);

  const candlesRef = useRef<Candle[]>([]);
  const loadingOlderRef = useRef(false);
  const capReachedRef = useRef(false);
  const tfRef = useRef<Timeframe>("1h");

  const [timeframe, setTimeframe] = useState<Timeframe>("1h");
  const [status, setStatus] = useState<"loading" | "live" | "error">("loading");
  const [oldestLoaded, setOldestLoaded] = useState<number | null>(null);

  const applyData = useCallback(() => {
    const cs = candleSeriesRef.current;
    const vs = volumeSeriesRef.current;
    if (!cs || !vs) return;
    const candles = candlesRef.current;
    cs.setData(
      candles.map((c): CandlestickData<Time> => ({
        time: c.time as Time, open: c.open, high: c.high, low: c.low, close: c.close,
      })),
    );
    vs.setData(
      candles.map((c): HistogramData<Time> => ({
        time: c.time as Time,
        value: c.volume,
        color: c.close >= c.open ? "rgba(16,185,129,0.35)" : "rgba(239,68,68,0.35)",
      })),
    );
    setOldestLoaded(candles.length ? candles[0].time : null);
  }, []);

  const fetchPage = useCallback(
    async (tf: Timeframe, opts: { end?: number; ticker?: boolean } = {}): Promise<CandlesResponse | null> => {
      const qs = new URLSearchParams({ resolution: tf });
      if (opts.end) qs.set("end", String(opts.end));
      if (opts.ticker) qs.set("ticker", "1");
      try {
        const res = await fetch(`/api/btc-prelive/candles?${qs.toString()}`, { cache: "no-store" });
        if (!res.ok) return null;
        return (await res.json()) as CandlesResponse;
      } catch {
        return null;
      }
    },
    [],
  );

  /** Merge a batch of (older or newer) candles into candlesRef, dedup by time. */
  const mergeCandles = useCallback((incoming: Candle[]) => {
    if (!incoming.length) return;
    const byTime = new Map<number, Candle>();
    for (const c of candlesRef.current) byTime.set(c.time, c);
    for (const c of incoming) byTime.set(c.time, c);
    candlesRef.current = [...byTime.values()].sort((a, b) => a.time - b.time);
  }, []);

  /** Initial load (or timeframe switch): most recent page + reset pagination. */
  const loadInitial = useCallback(
    async (tf: Timeframe) => {
      setStatus("loading");
      candlesRef.current = [];
      capReachedRef.current = false;
      const data = await fetchPage(tf, { ticker: true });
      if (tfRef.current !== tf) return; // user switched again mid-flight
      if (!data?.ok || !data.candles?.length) {
        setStatus("error");
        return;
      }
      candlesRef.current = data.candles;
      capReachedRef.current = Boolean(data.reachedHistoryCap);
      applyData();
      chartRef.current?.timeScale().scrollToRealTime();
      setStatus("live");
      if (onTicker && data.lastPrice) {
        onTicker({
          lastPrice: data.lastPrice,
          changePct24h: data.changePct24h ?? 0,
          fundingRate: data.fundingRate ?? 0,
        });
      }
    },
    [applyData, fetchPage, onTicker],
  );

  /** Lazy-load an older page when the user pans near the left edge. */
  const loadOlder = useCallback(async () => {
    if (loadingOlderRef.current || capReachedRef.current) return;
    const earliest = candlesRef.current[0];
    if (!earliest) return;
    loadingOlderRef.current = true;
    const tf = tfRef.current;
    try {
      const data = await fetchPage(tf, { end: earliest.time });
      if (tfRef.current !== tf) return;
      if (data?.ok && data.candles?.length) {
        // Everything at/after `earliest.time` is already loaded.
        mergeCandles(data.candles.filter((c) => c.time < earliest.time));
        capReachedRef.current = Boolean(data.reachedHistoryCap);
        applyData();
      } else if (data?.ok) {
        capReachedRef.current = true;
      }
    } finally {
      loadingOlderRef.current = false;
    }
  }, [applyData, fetchPage, mergeCandles]);

  // Chart init (once)
  useEffect(() => {
    if (!containerRef.current) return;

    const chart = createChart(containerRef.current, {
      layout: {
        background: { color: "#ffffff" },
        textColor: "#374151",
        fontFamily: "Inter, system-ui, sans-serif",
        fontSize: 11,
      },
      grid: {
        vertLines: { color: "#f3f4f6", style: LineStyle.Solid },
        horzLines: { color: "#f3f4f6", style: LineStyle.Solid },
      },
      crosshair: {
        vertLine: { color: "#9ca3af", width: 1, style: LineStyle.Dashed, labelBackgroundColor: "#374151" },
        horzLine: { color: "#9ca3af", width: 1, style: LineStyle.Dashed, labelBackgroundColor: "#374151" },
      },
      rightPriceScale: { borderColor: "#e5e7eb", scaleMargins: { top: 0.08, bottom: 0.22 } },
      timeScale: { borderColor: "#e5e7eb", timeVisible: true, secondsVisible: false },
      handleScroll: true,
      handleScale: true,
      autoSize: true,
    });

    const candleSeries = chart.addSeries(CandlestickSeries, {
      upColor: "#10b981",
      downColor: "#ef4444",
      borderUpColor: "#059669",
      borderDownColor: "#dc2626",
      wickUpColor: "#6ee7b7",
      wickDownColor: "#fca5a5",
    });

    const volumeSeries = chart.addSeries(HistogramSeries, {
      priceFormat: { type: "volume" },
      priceScaleId: "vol",
    });
    chart.priceScale("vol").applyOptions({ scaleMargins: { top: 0.85, bottom: 0 } });

    // Pan-left pagination: when fewer than 30 bars remain off-screen to the
    // left, pull the next older page (until the 1-year cap).
    const onRangeChange = () => {
      const range = chart.timeScale().getVisibleLogicalRange();
      if (range && range.from < 30) void loadOlder();
    };
    chart.timeScale().subscribeVisibleLogicalRangeChange(onRangeChange);

    chartRef.current = chart;
    candleSeriesRef.current = candleSeries;
    volumeSeriesRef.current = volumeSeries;

    return () => {
      chart.timeScale().unsubscribeVisibleLogicalRangeChange(onRangeChange);
      chart.remove();
      chartRef.current = null;
      candleSeriesRef.current = null;
      volumeSeriesRef.current = null;
    };
  }, [loadOlder]);

  // Initial load + reload on timeframe change
  useEffect(() => {
    tfRef.current = timeframe;
    void loadInitial(timeframe);
  }, [timeframe, loadInitial]);

  // Live tail: 3s ticker poll updates/rolls the last candle
  useEffect(() => {
    const id = setInterval(async () => {
      const cs = candleSeriesRef.current;
      const tf = tfRef.current;
      if (!cs || document.hidden) return;
      const bucketSec = TF_MINUTES[tf] * 60;
      const nowSec = Math.floor(Date.now() / 1000);
      const data = await fetchPage(tf, { end: nowSec + bucketSec, ticker: true });
      if (!data?.ok || tfRef.current !== tf) return;

      const tail = (data.candles ?? []).slice(-3);
      if (tail.length) {
        mergeCandles(tail);
        const merged = candlesRef.current;
        const last = merged[merged.length - 1];
        const prev = merged[merged.length - 2];
        // series.update() only accepts same-or-newer bars — update the last two.
        if (prev) cs.update({ time: prev.time as Time, open: prev.open, high: prev.high, low: prev.low, close: prev.close });
        if (last) cs.update({ time: last.time as Time, open: last.open, high: last.high, low: last.low, close: last.close });
      }
      if (onTicker && data.lastPrice) {
        onTicker({
          lastPrice: data.lastPrice,
          changePct24h: data.changePct24h ?? 0,
          fundingRate: data.fundingRate ?? 0,
        });
      }
    }, 3000);
    return () => clearInterval(id);
  }, [fetchPage, mergeCandles, onTicker]);

  const oldestLabel = oldestLoaded
    ? new Date(oldestLoaded * 1000).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" })
    : "—";

  return (
    <div className="glass-panel overflow-hidden" style={{ borderRadius: 16 }}>
      <div className="flex flex-wrap items-center justify-between gap-2 px-5 pt-4 pb-2">
        <div className="flex items-center gap-2">
          <div
            className={`h-2 w-2 rounded-full ${
              status === "live" ? "bg-emerald-500 animate-pulse" : status === "loading" ? "bg-amber-400" : "bg-red-500"
            }`}
          />
          <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
            BTC / USD · Delta Exchange · {timeframe}
          </span>
        </div>

        <div className="flex items-center gap-3">
          <div className="flex overflow-hidden rounded-lg border border-zinc-200">
            {TIMEFRAMES.map((tf) => (
              <button
                key={tf}
                onClick={() => setTimeframe(tf)}
                className={`px-2.5 py-1 text-[11px] font-semibold transition-colors ${
                  tf === timeframe ? "bg-zinc-900 text-white" : "bg-white text-zinc-600 hover:bg-zinc-100"
                }`}
              >
                {tf}
              </button>
            ))}
          </div>
          <span className="text-[10px] text-zinc-400">
            history from {oldestLabel} · pan left for up to 1y · live 3s
          </span>
        </div>
      </div>

      <div ref={containerRef} style={{ height: 440, width: "100%" }} />
    </div>
  );
}
