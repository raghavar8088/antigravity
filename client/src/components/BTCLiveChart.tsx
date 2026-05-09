"use client";

import { useEffect, useRef, useCallback } from "react";
import { createChart, CandlestickSeries, LineStyle } from "lightweight-charts";
import type { IChartApi, ISeriesApi, CandlestickData, Time, IPriceLine } from "lightweight-charts";
import type { BTCSpotPosition } from "@/hooks/useBTCSpotScalperEngine";

type Candle = { time: number; open: number; high: number; low: number; close: number; volume: number };

interface Props {
  positions: BTCSpotPosition[];
}

export default function BTCLiveChart({ positions }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const seriesRef = useRef<ISeriesApi<any> | null>(null);
  const priceLineRefs = useRef<IPriceLine[]>([]);
  const lastCandleTimeRef = useRef<number>(0);

  // Initialise chart once
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
      rightPriceScale: {
        borderColor: "#e5e7eb",
        scaleMargins: { top: 0.08, bottom: 0.08 },
      },
      timeScale: {
        borderColor: "#e5e7eb",
        timeVisible: true,
        secondsVisible: false,
        tickMarkFormatter: (time: number) => {
          const d = new Date(time * 1000);
          return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
        },
      },
      handleScroll: true,
      handleScale: true,
      autoSize: true,
    });

    const series = chart.addSeries(CandlestickSeries, {
      upColor: "#10b981",
      downColor: "#ef4444",
      borderUpColor: "#059669",
      borderDownColor: "#dc2626",
      wickUpColor: "#6ee7b7",
      wickDownColor: "#fca5a5",
    });

    chartRef.current = chart;
    seriesRef.current = series;

    return () => {
      chart.remove();
      chartRef.current = null;
      seriesRef.current = null;
    };
  }, []);

  // Fetch candles and update chart
  const fetchAndUpdate = useCallback(async () => {
    const series = seriesRef.current;
    if (!series) return;
    try {
      const res = await fetch("/api/btc/spot-klines", { cache: "no-store" });
      if (!res.ok) return;
      const data = await res.json() as {
        ok?: boolean;
        candles?: Candle[];
        livePrice?: number;
      };
      if (!data.ok || !data.candles?.length) return;

      const tvCandles: CandlestickData<Time>[] = data.candles.map((c) => ({
        time: c.time as Time,
        open: c.open,
        high: c.high,
        low: c.low,
        close: c.close,
      }));

      series.setData(tvCandles);

      // Append live tick as last bar update if we have a livePrice
      const lastCandle = tvCandles[tvCandles.length - 1];
      if (lastCandle && data.livePrice && data.livePrice > 0) {
        series.update({
          time: lastCandle.time,
          open: lastCandle.open,
          high: Math.max(lastCandle.high, data.livePrice),
          low: Math.min(lastCandle.low, data.livePrice),
          close: data.livePrice,
        });
      }

      lastCandleTimeRef.current = typeof lastCandle?.time === "number" ? (lastCandle.time as number) : 0;

      // Scroll to latest bar
      chartRef.current?.timeScale().scrollToRealTime();
    } catch {
      // silently ignore
    }
  }, []);

  useEffect(() => {
    void fetchAndUpdate();
    const id = setInterval(() => void fetchAndUpdate(), 2000);
    return () => clearInterval(id);
  }, [fetchAndUpdate]);

  // Draw position lines whenever positions change
  useEffect(() => {
    const series = seriesRef.current;
    if (!series) return;

    // Remove old lines
    for (const pl of priceLineRefs.current) {
      try { series.removePriceLine(pl); } catch { /* ignore */ }
    }
    priceLineRefs.current = [];

    for (const pos of positions) {
      const isLong = pos.side === "LONG";
      const labelSuffix = pos.strategyName.length > 18
        ? pos.strategyName.slice(0, 18) + "…"
        : pos.strategyName;

      // Entry line — blue solid
      priceLineRefs.current.push(
        series.createPriceLine({
          price: pos.entryPrice,
          color: isLong ? "#3b82f6" : "#8b5cf6",
          lineWidth: 1,
          lineStyle: LineStyle.Solid,
          axisLabelVisible: true,
          title: `${isLong ? "▲" : "▼"} ${labelSuffix} ENTRY`,
        })
      );

      // TP line — green dashed
      priceLineRefs.current.push(
        series.createPriceLine({
          price: pos.tpPrice,
          color: "#10b981",
          lineWidth: 1,
          lineStyle: LineStyle.Dashed,
          axisLabelVisible: true,
          title: `TP ${isLong ? "▲" : "▼"}`,
        })
      );

      // SL line — red dashed
      priceLineRefs.current.push(
        series.createPriceLine({
          price: pos.slPrice,
          color: "#ef4444",
          lineWidth: 1,
          lineStyle: LineStyle.Dashed,
          axisLabelVisible: true,
          title: `SL ${isLong ? "▲" : "▼"}`,
        })
      );
    }
  }, [positions]);

  // Handle resize
  useEffect(() => {
    const container = containerRef.current;
    if (!container || !chartRef.current) return;
    const ro = new ResizeObserver(() => {
      chartRef.current?.applyOptions({ width: container.clientWidth });
    });
    ro.observe(container);
    return () => ro.disconnect();
  }, []);

  return (
    <div className="glass-panel overflow-hidden" style={{ borderRadius: 16 }}>
      {/* Header */}
      <div className="flex items-center justify-between px-5 pt-4 pb-2">
        <div className="flex items-center gap-2">
          <div className="h-2 w-2 rounded-full bg-orange-500 animate-pulse" />
          <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
            BTC / USD · 1m Live Chart
          </span>
        </div>
        <div className="flex items-center gap-3">
          {positions.length > 0 && (
            <div className="flex items-center gap-2">
              <span className="inline-flex items-center gap-1 text-[10px] text-zinc-500">
                <span className="inline-block h-2 w-5 rounded border-b border-blue-500" style={{ borderStyle: "solid" }} />
                Entry
              </span>
              <span className="inline-flex items-center gap-1 text-[10px] text-zinc-500">
                <span className="inline-block h-2 w-5 rounded border-b border-emerald-500" style={{ borderStyle: "dashed" }} />
                TP
              </span>
              <span className="inline-flex items-center gap-1 text-[10px] text-zinc-500">
                <span className="inline-block h-2 w-5 rounded border-b border-red-500" style={{ borderStyle: "dashed" }} />
                SL
              </span>
              <span className="ml-1 rounded-full bg-orange-100 px-2 py-0.5 text-[10px] font-semibold text-orange-700">
                {positions.length} open
              </span>
            </div>
          )}
          <span className="text-[10px] text-zinc-400">Delta Exchange · auto-refresh 2s</span>
        </div>
      </div>

      {/* Chart */}
      <div ref={containerRef} style={{ height: 380, width: "100%" }} />
    </div>
  );
}
