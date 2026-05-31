"use client";

import { useEffect, useRef, useMemo } from "react";
import {
  createChart,
  createSeriesMarkers,
  ColorType,
  LineStyle,
  CrosshairMode,
  CandlestickSeries,
  type IChartApi,
  type ISeriesApi,
  type ISeriesMarkersPluginApi,
  type SeriesMarker,
  type Time,
} from "lightweight-charts";
import type { OHLCVCandle } from "@/lib/mockResearchIndicators";
import type { MockTrade } from "@/lib/mockTradingEngine";

interface InstitutionalChartProps {
  candles: OHLCVCandle[];
  trades?: MockTrade[];
  height?: number;
  title?: string;
}

export function InstitutionalChart({
  candles,
  trades = [],
  height = 400,
  title = "BTCUSD · 1m",
}: InstitutionalChartProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const seriesRef = useRef<ISeriesApi<"Candlestick"> | null>(null);
  const markersRef = useRef<ISeriesMarkersPluginApi<Time> | null>(null);

  const chartData = useMemo(() => {
    return candles.map((c) => ({
      time: (c.time / 1000) as Time,
      open: c.open,
      high: c.high,
      low: c.low,
      close: c.close,
    }));
  }, [candles]);

  const markers = useMemo(() => {
    const items: SeriesMarker<Time>[] = [];
    for (const trade of trades) {
      if (trade.openedAt) {
        items.push({
          time: (Math.floor(trade.openedAt / 60000) * 60) as Time,
          position: trade.side === "BUY" ? "belowBar" : "aboveBar",
          color: trade.side === "BUY" ? "#26a69a" : "#ef5350",
          shape: trade.side === "BUY" ? "arrowUp" : "arrowDown",
          text: `Entry #${trade.strategyId}`,
        });
      }
      if (trade.closedAt) {
        items.push({
          time: (Math.floor(trade.closedAt / 60000) * 60) as Time,
          position: trade.realizedPnl > 0 ? "aboveBar" : "belowBar",
          color: "#8b949e",
          shape: "circle",
          text: `Exit ${trade.exitReason ?? ""}`,
        });
      }
    }
    return items.sort((a, b) => (a.time as number) - (b.time as number));
  }, [trades]);

  useEffect(() => {
    if (!containerRef.current) return;

    const chart = createChart(containerRef.current, {
      layout: {
        background: { type: ColorType.Solid, color: "#0d1117" },
        textColor: "#8b949e",
        fontFamily: "JetBrains Mono, monospace",
        fontSize: 11,
      },
      grid: {
        vertLines: { color: "rgba(48, 54, 61, 0.5)", style: LineStyle.Solid },
        horzLines: { color: "rgba(48, 54, 61, 0.5)", style: LineStyle.Solid },
      },
      crosshair: {
        mode: CrosshairMode.Normal,
        vertLine: {
          color: "#4d7cfe",
          width: 1,
          style: LineStyle.Solid,
          labelBackgroundColor: "#4d7cfe",
        },
        horzLine: {
          color: "#4d7cfe",
          width: 1,
          style: LineStyle.Solid,
          labelBackgroundColor: "#4d7cfe",
        },
      },
      rightPriceScale: {
        borderColor: "#30363d",
        scaleMargins: { top: 0.1, bottom: 0.1 },
      },
      timeScale: {
        borderColor: "#30363d",
        timeVisible: true,
        secondsVisible: false,
      },
      handleScroll: true,
      handleScale: true,
    });

    const candlestickSeries = chart.addSeries(CandlestickSeries, {
      upColor: "#26a69a",
      downColor: "#ef5350",
      borderVisible: false,
      wickUpColor: "#26a69a",
      wickDownColor: "#ef5350",
    });

    chartRef.current = chart;
    seriesRef.current = candlestickSeries;
    markersRef.current = createSeriesMarkers(candlestickSeries, []);

    const handleResize = () => {
      if (containerRef.current) {
        chart.applyOptions({ width: containerRef.current.clientWidth });
      }
    };

    window.addEventListener("resize", handleResize);

    return () => {
      window.removeEventListener("resize", handleResize);
      chart.remove();
    };
  }, []);

  useEffect(() => {
    if (seriesRef.current && chartData.length > 0) {
      seriesRef.current.setData(chartData);
      markersRef.current?.setMarkers(markers);
    }
  }, [chartData, markers]);

  return (
    <div style={{ position: "relative", width: "100%", height }}>
      <div
        style={{
          position: "absolute",
          top: 10,
          left: 12,
          zIndex: 10,
          display: "flex",
          flexDirection: "column",
          gap: 2,
          pointerEvents: "none",
        }}
      >
        <div style={{ fontSize: 13, fontWeight: 700, color: "var(--text-primary)" }}>{title}</div>
        <div style={{ fontSize: 10, color: "var(--text-muted)" }}>REAL-TIME SIMULATION</div>
      </div>
      <div ref={containerRef} style={{ width: "100%", height: "100%" }} />
    </div>
  );
}
