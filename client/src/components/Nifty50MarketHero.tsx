"use client";

import { useId } from "react";
import type { NiftyMarketState } from "@/hooks/useNiftyMarket";

type Nifty50MarketHeroProps = {
  market: NiftyMarketState;
  subtitle?: string;
  vix?: number;
  currentTime?: number;
};

type ChartPoint = {
  x: number;
  y: number;
};

const CHART_WIDTH = 960;
const CHART_HEIGHT = 230;
const CHART_PADDING_X = 20;
const CHART_PADDING_Y = 18;

function formatIndex(value: number) {
  return value.toLocaleString("en-IN", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
}

function formatPercent(value: number) {
  const sign = value >= 0 ? "+" : "-";
  return `${sign}${Math.abs(value).toFixed(2)}%`;
}

function formatChange(value: number) {
  const sign = value >= 0 ? "+" : "-";
  return `${sign}${formatIndex(Math.abs(value))}`;
}

function formatClock(timestamp: number) {
  if (!timestamp) {
    return "--:--";
  }

  return new Date(timestamp).toLocaleTimeString("en-IN", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

function toPoints(values: number[]): ChartPoint[] {
  if (values.length === 0) {
    return [];
  }

  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const usableWidth = CHART_WIDTH - CHART_PADDING_X * 2;
  const usableHeight = CHART_HEIGHT - CHART_PADDING_Y * 2;

  return values.map((value, index) => ({
    x: CHART_PADDING_X + (index / Math.max(values.length - 1, 1)) * usableWidth,
    y: CHART_HEIGHT - CHART_PADDING_Y - ((value - min) / span) * usableHeight,
  }));
}

function buildLine(points: ChartPoint[]) {
  if (points.length === 0) {
    return "";
  }

  return points
    .map((point, index) => `${index === 0 ? "M" : "L"} ${point.x.toFixed(2)} ${point.y.toFixed(2)}`)
    .join(" ");
}

function buildArea(points: ChartPoint[]) {
  if (points.length === 0) {
    return "";
  }

  const first = points[0];
  const last = points[points.length - 1];
  return `${buildLine(points)} L ${last.x.toFixed(2)} ${(CHART_HEIGHT - CHART_PADDING_Y).toFixed(2)} L ${first.x.toFixed(2)} ${(CHART_HEIGHT - CHART_PADDING_Y).toFixed(2)} Z`;
}

export default function Nifty50MarketHero({
  market,
  subtitle = "Official NSE spot feed for the live NIFTY 50 index.",
  vix,
  currentTime = 0,
}: Nifty50MarketHeroProps) {
  const gradientId = useId().replace(/:/g, "");
  const visibleSeries = market.series.slice(-72);
  const chartValues = visibleSeries.map((point) => point.price);
  const points = toPoints(chartValues);
  const positive = market.percentChange >= 0;
  const lineColor = positive ? "#188038" : "#d93025";
  const fillColor = positive ? "#34a853" : "#ea4335";
  const windowStart = visibleSeries[0]?.time ?? 0;
  const windowEnd = visibleSeries[visibleSeries.length - 1]?.time ?? 0;
  const hasLivePrice = market.price > 0;
  const isStale = currentTime > 0 && market.updatedAt > 0 && currentTime - market.updatedAt > 90_000;

  return (
    <section className="glass-panel relative overflow-hidden px-6 py-7 md:px-7">
      <div className={`absolute -right-12 -top-12 h-44 w-44 rounded-full blur-3xl pointer-events-none ${positive ? "bg-emerald-500/10" : "bg-rose-500/10"}`} />

      <div className="flex flex-col gap-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="px-1">
            <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">
              NIFTY 50 Price
            </div>
            <div className="mt-4 flex flex-wrap items-end gap-4">
              <div className={`text-[clamp(2.55rem,5vw,3.35rem)] font-semibold leading-none tracking-tight ${positive ? "text-emerald-600" : "text-rose-600"}`}>
                {hasLivePrice ? formatIndex(market.price) : "Waiting"}
              </div>
              <div className={`pb-1 text-xl font-semibold leading-none ${positive ? "text-emerald-600" : "text-rose-600"}`}>
                {hasLivePrice ? formatPercent(market.percentChange) : "--"}
              </div>
            </div>
            <div className="mt-2 px-0.5 text-sm" style={{ color: "var(--text-secondary)" }}>
              {hasLivePrice
                ? `Day change ${formatChange(market.change)} | Updated ${formatClock(market.updatedAt)}`
                : subtitle}
            </div>
            {hasLivePrice && (market.open > 0 || market.high > 0) && (
              <div className="mt-2 px-0.5 flex flex-wrap gap-4 text-xs font-mono" style={{ color: "var(--text-secondary)" }}>
                {market.open > 0 && <span>O <span className="text-zinc-700 font-semibold">{formatIndex(market.open)}</span></span>}
                {market.high > 0 && <span>H <span className="text-emerald-600 font-semibold">{formatIndex(market.high)}</span></span>}
                {market.low > 0 && <span>L <span className="text-rose-600 font-semibold">{formatIndex(market.low)}</span></span>}
                {market.close > 0 && <span>Prev Close <span className="text-zinc-700 font-semibold">{formatIndex(market.close)}</span></span>}
              </div>
            )}
          </div>

          <div className="flex flex-wrap gap-2 px-1">
            <span className="pill-green">LIVE</span>
            <span className="rounded-full border border-orange-200 bg-orange-50 px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em] text-orange-700">
              Angel One
            </span>
            <span className="rounded-full border border-zinc-200 bg-white px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em] text-zinc-600">
              NSE · NIFTY 50
            </span>
            <span className={`rounded-full border px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em] ${hasLivePrice && !isStale ? "border-emerald-200 bg-emerald-50 text-emerald-700" : "border-amber-200 bg-amber-50 text-amber-700"}`}>
              {hasLivePrice ? (isStale ? "Feed Stale" : "Feed Healthy") : "Feed Waiting"}
            </span>
            {vix != null && vix > 0 && (
              <span className={`rounded-full border px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em] ${
                vix > 20 ? "border-rose-200 bg-rose-50 text-rose-700" :
                vix > 15 ? "border-amber-200 bg-amber-50 text-amber-700" :
                "border-emerald-200 bg-emerald-50 text-emerald-700"
              }`}>
                VIX {vix.toFixed(2)}
              </span>
            )}
          </div>
        </div>

        <div
          className="overflow-hidden rounded-[24px] border px-4 py-4"
          style={{ borderColor: "var(--border)", background: "linear-gradient(180deg, rgba(255,255,255,0.96), rgba(248,249,250,0.92))" }}
        >
          {points.length < 2 ? (
            <div className="flex h-[230px] items-center justify-center text-sm" style={{ color: "var(--text-secondary)" }}>
              {hasLivePrice
                ? "Live index connected, but there are not enough chart points yet to render replay-quality movement."
                : "Waiting for live NIFTY 50 chart points..."}
            </div>
          ) : (
            <svg viewBox={`0 0 ${CHART_WIDTH} ${CHART_HEIGHT}`} className="h-[230px] w-full">
              <defs>
                <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={fillColor} stopOpacity="0.28" />
                  <stop offset="100%" stopColor={fillColor} stopOpacity="0.03" />
                </linearGradient>
              </defs>

              <line
                x1={CHART_PADDING_X}
                y1={CHART_HEIGHT - CHART_PADDING_Y}
                x2={CHART_WIDTH - CHART_PADDING_X}
                y2={CHART_HEIGHT - CHART_PADDING_Y}
                stroke="rgba(148,163,184,0.28)"
              />
              <path d={buildArea(points)} fill={`url(#${gradientId})`} />
              <path
                d={buildLine(points)}
                fill="none"
                stroke={lineColor}
                strokeWidth={3}
                strokeLinecap="round"
                strokeLinejoin="round"
              />
              {points.length > 0 ? (
                <circle
                  cx={points[points.length - 1].x}
                  cy={points[points.length - 1].y}
                  r="5"
                  fill={lineColor}
                />
              ) : null}
            </svg>
          )}
        </div>

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <div className="metric-card min-h-[88px]">
            <div className="metric-label">Chart Window</div>
            <div className="metric-value text-zinc-900">
              {windowStart && windowEnd ? `${formatClock(windowStart)} - ${formatClock(windowEnd)}` : "--"}
            </div>
          </div>
          <div className="metric-card min-h-[88px]">
            <div className="metric-label">Series Points</div>
            <div className="metric-value text-zinc-900">{visibleSeries.length}</div>
          </div>
          <div className="metric-card min-h-[88px]">
            <div className="metric-label">Last Update</div>
            <div className="metric-value text-zinc-900">{market.updatedAt ? formatClock(market.updatedAt) : "--"}</div>
          </div>
        </div>
      </div>
    </section>
  );
}
