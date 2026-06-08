"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import {
  createChart,
  ColorType,
  LineStyle,
  LineSeries,
  AreaSeries,
  HistogramSeries,
  type Time,
  type LineData,
  type AreaData,
  type HistogramData,
} from "lightweight-charts";
import { TerminalPanel } from "@/components/terminal";
import { DeskEmptyState } from "@/components/desk/ui";
import { coerceEpochMs, toUtcChartTime } from "@/lib/chartTime";
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

function uniqueLineData(points: readonly ResearchChartPoint[]): LineData<Time>[] {
  const byTime = new Map<number, number>();
  for (const point of points) {
    if (!Number.isFinite(point.value)) continue;
    const time = toUtcChartTime(point.timestamp);
    if (time == null) continue;
    byTime.set(time as number, point.value);
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
    color: point.value >= 0 ? "rgba(38, 166, 154, 0.85)" : "rgba(239, 83, 80, 0.85)",
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
  height = 220,
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
        background: { type: ColorType.Solid, color: "#0d1117" },
        textColor: "#8b949e",
        fontFamily: "JetBrains Mono, monospace",
        fontSize: 10,
      },
      grid: {
        vertLines: { color: "rgba(48, 54, 61, 0.3)", style: LineStyle.Solid },
        horzLines: { color: "rgba(48, 54, 61, 0.3)", style: LineStyle.Solid },
      },
      crosshair: {
        vertLine: { color: "#4d7cfe", width: 1, style: LineStyle.Dashed, labelBackgroundColor: "#1f2937" },
        horzLine: { color: "#4d7cfe", width: 1, style: LineStyle.Dashed, labelBackgroundColor: "#1f2937" },
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
      autoSize: true,
      height,
    });

    if (kind === "histogram") {
      const series = chart.addSeries(HistogramSeries, { priceFormat: { type: "price", precision: 2, minMove: 0.01 } });
      series.setData(histogramData(points));
    } else if (kind === "area") {
      const series = chart.addSeries(AreaSeries, {
        lineColor: color,
        topColor: `${color}33`,
        bottomColor: `${color}05`,
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
    <TerminalPanel title={title} subtitle={subtitle} padding="none">
      {points.length < 2 ? (
        <DeskEmptyState title="Waiting for data" subtitle="Updates as mock history accumulates." />
      ) : (
        <div ref={containerRef} style={{ width: "100%", height, minHeight: height }} />
      )}
    </TerminalPanel>
  );
}

function ResearchMultiLineChart({
  title,
  subtitle,
  series,
  height = 280,
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
        background: { type: ColorType.Solid, color: "#0d1117" },
        textColor: "#8b949e",
        fontFamily: "JetBrains Mono, monospace",
        fontSize: 10,
      },
      grid: {
        vertLines: { color: "rgba(48, 54, 61, 0.3)" },
        horzLines: { color: "rgba(48, 54, 61, 0.3)" },
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
      autoSize: true,
      height,
    });

    for (const item of series) {
      const line = chart.addSeries(LineSeries, {
        color: item.color,
        lineWidth: 2,
        title: item.name.length > 20 ? item.name.slice(0, 20) : item.name,
      });
      line.setData(uniqueLineData(item.points));
    }

    chart.timeScale().fitContent();
    return () => chart.remove();
  }, [height, series]);

  return (
    <TerminalPanel title={title} subtitle={subtitle} padding="none">
      {series.length === 0 || series.every((item) => item.points.length < 2) ? (
        <DeskEmptyState title="Waiting for data" subtitle="Comparison charts populate after trades close." />
      ) : (
        <>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap", padding: "8px 12px", borderBottom: "1px solid var(--border)" }}>
            {series.map((item) => (
              <span key={item.name} style={{ display: "inline-flex", alignItems: "center", gap: 4, fontSize: 10, color: "var(--text-secondary)" }}>
                <span style={{ width: 8, height: 8, borderRadius: 999, background: item.color }} />
                {item.name}
              </span>
            ))}
          </div>
          <div ref={containerRef} style={{ width: "100%", height, minHeight: height }} />
        </>
      )}
    </TerminalPanel>
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
            timestamp: coerceEpochMs(point.timestamp),
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
    name: `#${item.id}`,
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
      <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 12, marginBottom: 12 }}>
        <div style={{ gridColumn: "span 1" }}>
          <ResearchSingleChart
            title="Equity Curve"
            subtitle="Live mock account equity"
            kind="area"
            points={equityCurve}
            color="#4d7cfe"
          />
        </div>
        <div style={{ gridColumn: "span 1" }}>
          <ResearchSingleChart
            title="Drawdown"
            subtitle="Peak-to-trough %"
            kind="line"
            points={drawdownCurve}
            color="#ef5350"
          />
        </div>
        <div style={{ gridColumn: "span 1" }}>
          <ResearchSingleChart
            title="Daily PnL"
            subtitle="Closed-trade net PnL"
            kind="histogram"
            points={dailyPnl}
            color="#26a69a"
          />
        </div>
        <div style={{ gridColumn: "span 1" }}>
          <ResearchSingleChart
            title="Cumulative PnL"
            subtitle="Realized net total"
            kind="line"
            points={cumulativeNetPnl}
            color="#26a69a"
          />
        </div>
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
        <ResearchMultiLineChart
          title="Strategy Comparison"
          subtitle="Top strategy curves"
          series={strategySeries}
        />
        <ResearchMultiLineChart
          title="Family Comparison"
          subtitle="PnL by strategy family"
          series={familySeries}
        />
      </div>
    </>
  );
}
