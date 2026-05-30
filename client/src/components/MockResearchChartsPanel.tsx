"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import {
  AreaSeries,
  HistogramSeries,
  LineSeries,
  LineStyle,
  createChart,
  type AreaData,
  type HistogramData,
  type LineData,
  type Time,
} from "lightweight-charts";
import { DeskCard, DeskEmptyState, DeskSectionHeader } from "@/components/desk/ui";
import type { MarketRegime } from "@/lib/marketRegimeClassifier";
import type { MockAccountState, MockTrade } from "@/lib/mockTradingEngine";
import {
  computeCumulativeNetPnlPoints,
  computeDailyPnlPoints,
  computeFamilyComparisonSeries,
  computeStrategyComparisonSeries,
  createEquitySnapshot,
  type EquityAnalyticsSnapshot,
  type ResearchChartPoint,
} from "@/lib/mockResearchAnalytics";

type SeriesKind = "line" | "area" | "histogram";

type MultiSeries = {
  name: string;
  color: string;
  points: ResearchChartPoint[];
};

interface MockResearchChartsPanelProps {
  trades: readonly MockTrade[];
  account: MockAccountState;
  regime: MarketRegime | null;
}

const PALETTE = ["#60a5fa", "#34d399", "#f59e0b", "#f472b6", "#a78bfa", "#22d3ee", "#fb7185", "#84cc16"];

function toChartTime(timestamp: number): Time {
  return Math.floor(timestamp / 1000) as Time;
}

function uniqueLineData(points: readonly ResearchChartPoint[]): LineData<Time>[] {
  const byTime = new Map<number, number>();
  for (const point of points) {
    if (!Number.isFinite(point.value)) continue;
    byTime.set(Math.floor(point.timestamp / 1000), point.value);
  }
  return [...byTime.entries()]
    .sort((a, b) => a[0] - b[0])
    .map(([time, value]) => ({ time: time as Time, value }));
}

function uniqueAreaData(points: readonly ResearchChartPoint[]): AreaData<Time>[] {
  return uniqueLineData(points).map((point) => ({ ...point }));
}

function histogramData(points: readonly ResearchChartPoint[]): HistogramData<Time>[] {
  return uniqueLineData(points).map((point) => ({
    time: point.time,
    value: point.value,
    color: point.value >= 0 ? "rgba(52, 211, 153, 0.85)" : "rgba(248, 113, 113, 0.85)",
  }));
}

function mergeEquitySnapshots(
  historical: readonly EquityAnalyticsSnapshot[],
  current: EquityAnalyticsSnapshot,
): EquityAnalyticsSnapshot[] {
  const byTime = new Map<number, EquityAnalyticsSnapshot>();
  for (const point of historical) byTime.set(point.timestamp, point);
  byTime.set(current.timestamp, current);
  return [...byTime.values()].sort((a, b) => a.timestamp - b.timestamp).slice(-1_500);
}

function ResearchSingleChart({
  title,
  subtitle,
  kind,
  points,
  color,
  height = 280,
}: {
  title: string;
  subtitle: string;
  kind: SeriesKind;
  points: ResearchChartPoint[];
  color: string;
  height?: number;
}) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!container || points.length === 0) return;

    const chart = createChart(container, {
      layout: {
        background: { color: "#0d1117" },
        textColor: "#c9d1d9",
        fontFamily: "Inter, system-ui, sans-serif",
        fontSize: 11,
      },
      grid: {
        vertLines: { color: "rgba(148, 163, 184, 0.10)", style: LineStyle.Solid },
        horzLines: { color: "rgba(148, 163, 184, 0.10)", style: LineStyle.Solid },
      },
      crosshair: {
        vertLine: { color: "#64748b", width: 1, style: LineStyle.Dashed, labelBackgroundColor: "#1f2937" },
        horzLine: { color: "#64748b", width: 1, style: LineStyle.Dashed, labelBackgroundColor: "#1f2937" },
      },
      rightPriceScale: {
        borderColor: "rgba(148, 163, 184, 0.25)",
        scaleMargins: { top: 0.1, bottom: 0.12 },
      },
      timeScale: {
        borderColor: "rgba(148, 163, 184, 0.25)",
        timeVisible: true,
        secondsVisible: false,
      },
      handleScroll: true,
      handleScale: true,
      autoSize: true,
      height,
    });

    if (kind === "histogram") {
      const series = chart.addSeries(HistogramSeries, { priceFormat: { type: "price", precision: 2, minMove: 0.01 } });
      series.setData(histogramData(points));
    } else if (kind === "area") {
      const series = chart.addSeries(AreaSeries, {
        lineColor: color,
        topColor: "rgba(96, 165, 250, 0.35)",
        bottomColor: "rgba(96, 165, 250, 0.03)",
        lineWidth: 2,
      });
      series.setData(uniqueAreaData(points));
    } else {
      const series = chart.addSeries(LineSeries, { color, lineWidth: 2 });
      series.setData(uniqueLineData(points));
    }

    chart.timeScale().fitContent();
    return () => chart.remove();
  }, [color, height, kind, points]);

  return (
    <DeskCard padding="md">
      <DeskSectionHeader title={title} subtitle={subtitle} />
      {points.length < 2 ? (
        <DeskEmptyState title="Waiting for data" subtitle="This chart updates as mock equity/trade history accumulates." />
      ) : (
        <div ref={containerRef} style={{ width: "100%", height, minHeight: height, borderRadius: 10, overflow: "hidden" }} />
      )}
    </DeskCard>
  );
}

function ResearchMultiLineChart({
  title,
  subtitle,
  series,
  height = 320,
}: {
  title: string;
  subtitle: string;
  series: MultiSeries[];
  height?: number;
}) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!container || series.length === 0) return;

    const chart = createChart(container, {
      layout: {
        background: { color: "#0d1117" },
        textColor: "#c9d1d9",
        fontFamily: "Inter, system-ui, sans-serif",
        fontSize: 11,
      },
      grid: {
        vertLines: { color: "rgba(148, 163, 184, 0.10)" },
        horzLines: { color: "rgba(148, 163, 184, 0.10)" },
      },
      rightPriceScale: {
        borderColor: "rgba(148, 163, 184, 0.25)",
        scaleMargins: { top: 0.1, bottom: 0.12 },
      },
      timeScale: {
        borderColor: "rgba(148, 163, 184, 0.25)",
        timeVisible: true,
        secondsVisible: false,
      },
      handleScroll: true,
      handleScale: true,
      autoSize: true,
      height,
    });

    for (const item of series) {
      const line = chart.addSeries(LineSeries, {
        color: item.color,
        lineWidth: 2,
        title: item.name.length > 26 ? item.name.slice(0, 26) : item.name,
      });
      line.setData(uniqueLineData(item.points));
    }

    chart.timeScale().fitContent();
    return () => chart.remove();
  }, [height, series]);

  return (
    <DeskCard padding="md">
      <DeskSectionHeader title={title} subtitle={subtitle} />
      {series.length === 0 || series.every((item) => item.points.length < 2) ? (
        <DeskEmptyState title="Waiting for data" subtitle="Comparison charts populate after multiple strategy/family trades close." />
      ) : (
        <>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 10 }}>
            {series.map((item) => (
              <span key={item.name} style={{ display: "inline-flex", alignItems: "center", gap: 6, fontSize: 12 }}>
                <span style={{ width: 10, height: 10, borderRadius: 999, background: item.color }} />
                {item.name}
              </span>
            ))}
          </div>
          <div ref={containerRef} style={{ width: "100%", height, minHeight: height, borderRadius: 10, overflow: "hidden" }} />
        </>
      )}
    </DeskCard>
  );
}

export function MockResearchChartsPanel({ trades, account, regime }: MockResearchChartsPanelProps) {
  const [historicalEquity, setHistoricalEquity] = useState<EquityAnalyticsSnapshot[]>([]);
  const currentSnapshot = useMemo(
    () => createEquitySnapshot({ account, trades, regime }),
    [account, regime, trades],
  );

  useEffect(() => {
    let cancelled = false;
    async function loadEquityHistory() {
      try {
        const res = await fetch("/api/mock-trading/equity?limit=1500", { cache: "no-store" });
        if (!res.ok) return;
        const json = (await res.json()) as {
          points?: Array<{
            timestamp: number;
            equity: number;
            realized_pnl: number;
            unrealized_pnl: number;
            drawdown_pct: number;
            daily_pnl?: number;
            regime?: MarketRegime;
          }>;
        };
        if (cancelled || !json.points) return;
        setHistoricalEquity(
          json.points.map((point) => ({
            timestamp: point.timestamp,
            equity: point.equity,
            realizedPnl: point.realized_pnl,
            unrealizedPnl: point.unrealized_pnl,
            cumulativeNetPnl: point.realized_pnl + point.unrealized_pnl,
            drawdownPct: point.drawdown_pct,
            dailyPnl: point.daily_pnl ?? 0,
            regime: point.regime ?? null,
          })),
        );
      } catch {
        // Historical chart hydration is best-effort; live local points still render.
      }
    }
    void loadEquityHistory();
    const id = setInterval(() => void loadEquityHistory(), 30_000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

  const equitySnapshots = useMemo(
    () => mergeEquitySnapshots(historicalEquity, currentSnapshot),
    [currentSnapshot, historicalEquity],
  );
  const equityCurve = equitySnapshots.map((point) => ({ timestamp: point.timestamp, value: point.equity }));
  const drawdownCurve = equitySnapshots.map((point) => ({ timestamp: point.timestamp, value: -Math.abs(point.drawdownPct) }));
  const dailyPnl = computeDailyPnlPoints(trades);
  const cumulativeNetPnl = computeCumulativeNetPnlPoints(trades);
  const strategySeries = computeStrategyComparisonSeries(trades, 5).map((item, index) => ({
    name: `#${item.id} ${item.name}`,
    color: PALETTE[index % PALETTE.length],
    points: item.points,
  }));
  const familySeries = computeFamilyComparisonSeries(trades, 6).map((item, index) => ({
    name: item.name,
    color: PALETTE[index % PALETTE.length],
    points: item.points,
  }));

  return (
    <>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(360px, 1fr))", gap: 12 }}>
        <ResearchSingleChart
          title="Equity Curve"
          subtitle="Live mock account equity, hydrated from persisted snapshots when MongoDB is available."
          kind="area"
          points={equityCurve}
          color="#60a5fa"
        />
        <ResearchSingleChart
          title="Drawdown Curve"
          subtitle="Peak-to-trough drawdown shown as a negative percentage curve."
          kind="line"
          points={drawdownCurve}
          color="#f87171"
        />
        <ResearchSingleChart
          title="Daily PnL"
          subtitle="Closed-trade net PnL by UTC day after simulated fees and slippage."
          kind="histogram"
          points={dailyPnl}
          color="#34d399"
        />
        <ResearchSingleChart
          title="Cumulative Net PnL"
          subtitle="Closed-trade cumulative realized PnL after realistic mock execution costs."
          kind="line"
          points={cumulativeNetPnl}
          color="#34d399"
        />
      </div>
      <ResearchMultiLineChart
        title="Strategy Comparison"
        subtitle="Top absolute-PnL strategy curves, useful for identifying leaders, laggards, and correlated behavior."
        series={strategySeries}
      />
      <ResearchMultiLineChart
        title="Family Comparison"
        subtitle="Cumulative net PnL by strategy family for regime/family research."
        series={familySeries}
      />
    </>
  );
}
