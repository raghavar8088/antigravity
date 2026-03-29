"use client";

type PricePoint = {
  time: number;
  price: number;
};

type EquityPoint = {
  time: number;
  equity: number;
};

type StrategyBar = {
  name: string;
  pnl: number;
};

type PerformanceChartsProps = {
  priceSeries: PricePoint[];
  equitySeries: EquityPoint[];
  strategyBars: StrategyBar[];
};

type ChartPoint = {
  x: number;
  y: number;
};

const CHART_WIDTH = 720;
const CHART_HEIGHT = 220;
const CHART_PADDING = 18;

function toChartPoints(values: number[]): ChartPoint[] {
  if (values.length === 0) {
    return [];
  }

  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const usableWidth = CHART_WIDTH - CHART_PADDING * 2;
  const usableHeight = CHART_HEIGHT - CHART_PADDING * 2;

  return values.map((value, index) => {
    const x = CHART_PADDING + (index / Math.max(values.length - 1, 1)) * usableWidth;
    const y = CHART_HEIGHT - CHART_PADDING - ((value - min) / span) * usableHeight;
    return { x, y };
  });
}

function buildLinePath(points: ChartPoint[]): string {
  if (points.length === 0) {
    return "";
  }
  return points
    .map((point, index) => `${index === 0 ? "M" : "L"} ${point.x.toFixed(2)} ${point.y.toFixed(2)}`)
    .join(" ");
}

function buildAreaPath(points: ChartPoint[]): string {
  if (points.length === 0) {
    return "";
  }
  const first = points[0];
  const last = points[points.length - 1];
  return `${buildLinePath(points)} L ${last.x.toFixed(2)} ${(CHART_HEIGHT - CHART_PADDING).toFixed(2)} L ${first.x.toFixed(2)} ${(CHART_HEIGHT - CHART_PADDING).toFixed(2)} Z`;
}

function formatTimeLabel(timeMs: number): string {
  return new Date(timeMs).toLocaleTimeString([], { hour12: false, minute: "2-digit", second: "2-digit" });
}

function renderLineChart(
  id: string,
  values: number[],
  colorClass: string,
  gradientFrom: string,
  gradientTo: string
) {
  const points = toChartPoints(values);
  if (points.length < 2) {
    return (
      <div className="h-[220px] flex items-center justify-center text-sm text-gray-500">
        Waiting for live data...
      </div>
    );
  }

  return (
    <svg viewBox={`0 0 ${CHART_WIDTH} ${CHART_HEIGHT}`} className="w-full h-[220px]">
      <defs>
        <linearGradient id={`${id}-area`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={gradientFrom} stopOpacity="0.42" />
          <stop offset="100%" stopColor={gradientTo} stopOpacity="0.04" />
        </linearGradient>
      </defs>

      <line x1={CHART_PADDING} y1={CHART_HEIGHT - CHART_PADDING} x2={CHART_WIDTH - CHART_PADDING} y2={CHART_HEIGHT - CHART_PADDING} stroke="rgba(148,163,184,0.22)" />
      <path d={buildAreaPath(points)} fill={`url(#${id}-area)`} />
      <path d={buildLinePath(points)} fill="none" className={colorClass} strokeWidth={2.5} strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export default function PerformanceCharts({ priceSeries, equitySeries, strategyBars }: PerformanceChartsProps) {
  const visibleStrategies = [...strategyBars]
    .sort((left, right) => Math.abs(right.pnl) - Math.abs(left.pnl))
    .slice(0, 8);
  const maxAbsPnl = Math.max(...visibleStrategies.map((strategy) => Math.abs(strategy.pnl)), 1);

  return (
    <section className="grid grid-cols-1 xl:grid-cols-3 gap-6">
      <div className="glass-panel p-5 xl:col-span-2">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-bold uppercase tracking-[0.14em] text-cyan-300">BTC Price Trend</h3>
          {priceSeries.length > 0 && (
            <p className="text-xs text-gray-400 font-mono">
              {formatTimeLabel(priceSeries[0].time)} - {formatTimeLabel(priceSeries[priceSeries.length - 1].time)}
            </p>
          )}
        </div>
        {renderLineChart("price", priceSeries.map((point) => point.price), "stroke-cyan-400", "#22d3ee", "#0f172a")}
      </div>

      <div className="glass-panel p-5">
        <h3 className="text-sm font-bold uppercase tracking-[0.14em] text-emerald-300 mb-3">Strategy PnL Rank</h3>
        {visibleStrategies.length === 0 ? (
          <div className="h-[220px] flex items-center justify-center text-sm text-gray-500">
            Waiting for strategy stats...
          </div>
        ) : (
          <div className="space-y-2 h-[220px] overflow-hidden">
            {visibleStrategies.map((strategy) => {
              const width = `${Math.max((Math.abs(strategy.pnl) / maxAbsPnl) * 100, 8)}%`;
              const positive = strategy.pnl >= 0;
              return (
                <div key={strategy.name} className="space-y-1">
                  <div className="flex items-center justify-between text-xs font-mono">
                    <span className="text-gray-300 truncate mr-2">{strategy.name}</span>
                    <span className={positive ? "text-green-400" : "text-red-400"}>
                      {positive ? "+" : ""}${strategy.pnl.toFixed(2)}
                    </span>
                  </div>
                  <div className="h-2 rounded-full bg-slate-800/80 overflow-hidden">
                    <div
                      className={`h-full rounded-full ${positive ? "bg-green-400/85" : "bg-red-400/85"}`}
                      style={{ width }}
                    />
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      <div className="glass-panel p-5 xl:col-span-3">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-bold uppercase tracking-[0.14em] text-blue-300">Equity Curve</h3>
          {equitySeries.length > 0 && (
            <p className="text-xs text-gray-400 font-mono">
              {formatTimeLabel(equitySeries[0].time)} - {formatTimeLabel(equitySeries[equitySeries.length - 1].time)}
            </p>
          )}
        </div>
        {renderLineChart("equity", equitySeries.map((point) => point.equity), "stroke-blue-400", "#60a5fa", "#0f172a")}
      </div>
    </section>
  );
}
