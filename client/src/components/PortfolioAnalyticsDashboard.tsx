"use client";

import { useEffect, useState } from "react";

type ExposureMetrics = {
  long_exposure_btc: number;
  short_exposure_btc: number;
  net_exposure_btc: number;
  gross_exposure_btc: number;
  exposure_usd: number;
};

type DrawdownMetrics = {
  peak_equity: number;
  current_drawdown: number;
  max_drawdown: number;
  rolling_drawdown_24h: number;
  daily_drawdown: number;
  weekly_drawdown: number;
};

type AccountingSnapshot = {
  account_key: string;
  computed_at: string;
  balance: number;
  equity: number;
  starting_balance: number;
  realized_pnl: number;
  unrealized_pnl: number;
  gross_pnl: number;
  net_pnl: number;
  entry_fees: number;
  exit_fees: number;
  total_fees: number;
  exposure: ExposureMetrics;
  drawdown: DrawdownMetrics;
  total_trades: number;
  winning_trades: number;
  losing_trades: number;
  win_rate: number;
  open_position_count: number;
  profit_factor: number | null;
  expectancy: number;
  average_win: number;
  average_loss: number;
  sharpe: number | null;
  sortino: number | null;
  calmar: number | null;
  recovery_factor: number | null;
};

type EquityPoint = { snapped_at: string; equity: number };

function pct(n: number) {
  return `${(n * 100).toFixed(2)}%`;
}

function usd(n: number) {
  const abs = Math.abs(n).toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  return n < 0 ? `-$${abs}` : `$${abs}`;
}

function safeNum(v: number | null | undefined, decimals = 2) {
  if (v === null || v === undefined || !Number.isFinite(v)) return "—";
  return v.toFixed(decimals);
}

type MetricTileProps = {
  label: string;
  value: string;
  color?: string;
  sub?: string;
};

function MetricTile({ label, value, color = "#e2e8f0", sub }: MetricTileProps) {
  return (
    <div style={{ background: "#1e293b", borderRadius: 6, padding: "12px 14px", border: "1px solid #334155" }}>
      <div style={{ color: "#64748b", fontSize: 10, marginBottom: 4 }}>{label}</div>
      <div style={{ color, fontSize: 18, fontWeight: 700 }}>{value}</div>
      {sub && <div style={{ color: "#475569", fontSize: 10, marginTop: 3 }}>{sub}</div>}
    </div>
  );
}

export default function PortfolioAnalyticsDashboard() {
  const [snapshot, setSnapshot] = useState<AccountingSnapshot | null>(null);
  const [curve, setCurve] = useState<EquityPoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      try {
        const [portRes, equityRes] = await Promise.all([
          fetch("/api/paper-desk/portfolio"),
          fetch("/api/paper-desk/equity?points=96&days=7"),
        ]);

        if (cancelled) return;

        if (!portRes.ok) throw new Error(`Portfolio API: HTTP ${portRes.status}`);
        const portData = await portRes.json();
        if (!portData.ok) throw new Error(portData.error ?? "Portfolio unavailable");
        setSnapshot(portData.snapshot);

        if (equityRes.ok) {
          const eqData = await equityRes.json();
          if (eqData.ok && Array.isArray(eqData.curve)) {
            setCurve(eqData.curve.slice(-96));
          }
        }
        setError(null);
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : "Portfolio data unavailable");
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    load();
    const id = setInterval(load, 10_000);
    return () => { cancelled = true; clearInterval(id); };
  }, []);

  if (loading) {
    return (
      <div style={outerStyle}>
        <div style={{ color: "#64748b", padding: 40, textAlign: "center", fontFamily: "monospace" }}>
          LOADING PORTFOLIO ANALYTICS...
        </div>
      </div>
    );
  }

  if (error || !snapshot) {
    return (
      <div style={outerStyle}>
        <div style={{ color: "#ef4444", padding: 20, background: "#1e293b", borderRadius: 6, border: "1px solid #ef4444", fontFamily: "monospace" }}>
          BACKEND AUTHORITY UNAVAILABLE: {error ?? "No portfolio data"}
        </div>
      </div>
    );
  }

  const s = snapshot;
  const dd = s.drawdown;
  const exp = s.exposure;
  const pnlColor = (n: number) => (n >= 0 ? "#22c55e" : "#ef4444");
  const ddColor = (n: number) => (n < -0.05 ? "#ef4444" : n < -0.02 ? "#f59e0b" : "#22c55e");

  // Mini equity sparkline
  const sparkPoints = curve.slice(-50);
  const sparkMin = Math.min(...sparkPoints.map((p) => p.equity), s.equity);
  const sparkMax = Math.max(...sparkPoints.map((p) => p.equity), s.equity);
  const sparkRange = sparkMax - sparkMin || 1;

  return (
    <div style={outerStyle}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 20 }}>
        <h2 style={{ fontSize: 18, fontWeight: 700, color: "#f8fafc", margin: 0, fontFamily: "monospace" }}>
          PORTFOLIO ANALYTICS
        </h2>
        <span style={{ color: "#475569", fontSize: 11, fontFamily: "monospace" }}>
          AUTHORITY: MONGODB · {new Date(s.computed_at).toLocaleTimeString()}
        </span>
      </div>

      {/* Primary metrics */}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 10, marginBottom: 16 }}>
        <MetricTile label="BALANCE" value={usd(s.balance)} sub={`Starting: ${usd(s.starting_balance)}`} />
        <MetricTile label="EQUITY" value={usd(s.equity)} color={pnlColor(s.equity - s.starting_balance)} sub={pct((s.equity - s.starting_balance) / s.starting_balance)} />
        <MetricTile label="REALIZED PnL" value={usd(s.realized_pnl)} color={pnlColor(s.realized_pnl)} />
        <MetricTile label="UNREALIZED PnL" value={usd(s.unrealized_pnl)} color={pnlColor(s.unrealized_pnl)} />
      </div>

      {/* Risk metrics */}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 10, marginBottom: 16 }}>
        <MetricTile label="CURRENT DRAWDOWN" value={pct(dd.current_drawdown)} color={ddColor(dd.current_drawdown)} />
        <MetricTile label="MAX DRAWDOWN" value={pct(dd.max_drawdown)} color={ddColor(dd.max_drawdown)} />
        <MetricTile label="DAILY DRAWDOWN" value={pct(dd.daily_drawdown)} color={ddColor(dd.daily_drawdown)} />
        <MetricTile label="WEEKLY DRAWDOWN" value={pct(dd.weekly_drawdown)} color={ddColor(dd.weekly_drawdown)} />
      </div>

      {/* Performance metrics */}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 10, marginBottom: 16 }}>
        <MetricTile label="PROFIT FACTOR" value={safeNum(s.profit_factor)} color={s.profit_factor && s.profit_factor >= 1.25 ? "#22c55e" : s.profit_factor && s.profit_factor >= 1.0 ? "#f59e0b" : "#ef4444"} />
        <MetricTile label="SHARPE RATIO" value={safeNum(s.sharpe)} color={s.sharpe && s.sharpe >= 1.5 ? "#22c55e" : s.sharpe && s.sharpe >= 0.5 ? "#f59e0b" : "#ef4444"} />
        <MetricTile label="SORTINO RATIO" value={safeNum(s.sortino)} />
        <MetricTile label="CALMAR RATIO" value={safeNum(s.calmar)} />
      </div>

      {/* Trade stats */}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 10, marginBottom: 16 }}>
        <MetricTile label="TOTAL TRADES" value={s.total_trades.toLocaleString()} />
        <MetricTile label="WIN RATE" value={pct(s.win_rate)} color={s.win_rate >= 0.5 ? "#22c55e" : s.win_rate >= 0.4 ? "#f59e0b" : "#ef4444"} sub={`${s.winning_trades}W / ${s.losing_trades}L`} />
        <MetricTile label="EXPECTANCY" value={usd(s.expectancy)} color={pnlColor(s.expectancy)} />
        <MetricTile label="TOTAL FEES" value={usd(s.total_fees)} color="#f59e0b" />
      </div>

      {/* Exposure */}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 10, marginBottom: 20 }}>
        <MetricTile label="EXPOSURE USD" value={usd(exp.exposure_usd)} />
        <MetricTile label="LONG EXPOSURE" value={`${exp.long_exposure_btc.toFixed(4)} BTC`} color="#22c55e" />
        <MetricTile label="SHORT EXPOSURE" value={`${exp.short_exposure_btc.toFixed(4)} BTC`} color="#ef4444" />
        <MetricTile label="NET EXPOSURE" value={`${exp.net_exposure_btc.toFixed(4)} BTC`} color={exp.net_exposure_btc >= 0 ? "#22c55e" : "#ef4444"} />
      </div>

      {/* Equity Sparkline */}
      {sparkPoints.length > 1 && (
        <div style={{ background: "#1e293b", borderRadius: 6, padding: 16, border: "1px solid #334155" }}>
          <div style={{ color: "#64748b", fontSize: 10, marginBottom: 8 }}>EQUITY CURVE (LAST {sparkPoints.length} SNAPSHOTS)</div>
          <svg width="100%" height="60" viewBox={`0 0 ${sparkPoints.length} 60`} preserveAspectRatio="none">
            <polyline
              points={sparkPoints
                .map((p, i) => `${i},${60 - ((p.equity - sparkMin) / sparkRange) * 56}`)
                .join(" ")}
              fill="none"
              stroke="#3b82f6"
              strokeWidth="1.5"
            />
          </svg>
          <div style={{ display: "flex", justifyContent: "space-between", color: "#475569", fontSize: 10, marginTop: 4 }}>
            <span>{new Date(sparkPoints[0].snapped_at).toLocaleDateString()}</span>
            <span>{new Date(sparkPoints[sparkPoints.length - 1].snapped_at).toLocaleDateString()}</span>
          </div>
        </div>
      )}
    </div>
  );
}

const outerStyle: React.CSSProperties = {
  fontFamily: "monospace",
  fontSize: 13,
  color: "#e2e8f0",
  background: "#0f172a",
  padding: 24,
};
