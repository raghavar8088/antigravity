"use client";

import { useCallback, useEffect, useRef, useState } from "react";

// ── Types ────────────────────────────────────────────────────────────────────

interface EnginePosition {
  id: string;
  strategyName: string;
  side: "BUY" | "SELL";
  size: number;
  entryPrice: number;
  currentPrice?: number;
  unrealisedPnl?: number;
  takeProfitPct?: number;
  stopLossPct?: number;
  openedAt?: string;
}

interface EngineTrade {
  id: string;
  strategyName: string;
  side: string;
  entryPrice: number;
  exitPrice: number;
  size: number;
  netPnl?: number;
  grossPnl?: number;
  reason?: string;
  closedAt?: string;
}

/** Shape returned by Go engine /api/stats */
interface EngineStatsRaw {
  balance?: number;
  equity?: number;
  cashBalance?: number;
  dailyPnl?: number;
  openPositions?: number;
  aggregate?: {
    totalTrades?: number;
    winRate?: number;   // 0–100 percentage
    totalPnl?: number;
    maxDrawdown?: number;
    profitFactor?: number;
  };
}

interface EngineStats {
  totalTrades: number;
  openPositions: number;
  winRate: number;      // 0–1 fraction
  netPnlUSD: number;
  balanceUSD: number;
  equityUSD: number;
  maxDrawdownPct: number;
  dailyPnlUSD: number;
}

interface DeskStatus {
  health: "online" | "offline" | "degraded";
  lastFetchMs: number;
  positions: EnginePosition[];
  recentTrades: EngineTrade[];
  stats: EngineStats | null;   // normalised, never the raw EngineStatsRaw
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function fmtUsd(n: number): string {
  const abs = Math.abs(n);
  const sign = n < 0 ? "-" : n > 0 ? "+" : "";
  if (abs >= 1_000_000) return `${sign}$${(abs / 1_000_000).toFixed(2)}M`;
  if (abs >= 1_000) return `${sign}$${(abs / 1_000).toFixed(1)}K`;
  return `${sign}$${abs.toFixed(2)}`;
}

function fmtPrice(n: number): string {
  return `$${n.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

function pnlColor(n: number): string {
  if (n > 0) return "#22c55e";
  if (n < 0) return "#ef4444";
  return "#6b7280";
}

function age(iso?: string): string {
  if (!iso) return "—";
  const secs = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
  if (secs < 60) return `${secs}s ago`;
  if (secs < 3600) return `${Math.floor(secs / 60)}m ago`;
  return `${Math.floor(secs / 3600)}h ago`;
}

// ── Fetch helpers ─────────────────────────────────────────────────────────────

async function fetchJson<T>(url: string): Promise<T | null> {
  try {
    const r = await fetch(url, { cache: "no-store", signal: AbortSignal.timeout(8_000) });
    if (!r.ok) return null;
    return await r.json() as T;
  } catch {
    return null;
  }
}

// ── Hook ─────────────────────────────────────────────────────────────────────

function useDeskStatus(pollMs = 6_000) {
  const [status, setStatus] = useState<DeskStatus>({
    health: "offline",
    lastFetchMs: 0,
    positions: [],
    recentTrades: [],
    stats: null,
  });

  const fetchAll = useCallback(async () => {
    const [health, posData, tradeData, statsData] = await Promise.all([
      fetchJson<{ status?: string; ok?: boolean }>("/api/engine/health"),
      fetchJson<EnginePosition[] | { positions?: EnginePosition[] }>("/api/engine/api/positions"),
      fetchJson<EngineTrade[] | { trades?: EngineTrade[] }>("/api/engine/api/trades"),
      fetchJson<EngineStatsRaw>("/api/engine/api/stats"),
    ]);

    const positions: EnginePosition[] = Array.isArray(posData)
      ? posData as EnginePosition[]
      : (posData as { positions?: EnginePosition[] })?.positions ?? [];

    const trades: EngineTrade[] = Array.isArray(tradeData)
      ? tradeData as EngineTrade[]
      : (tradeData as { trades?: EngineTrade[] })?.trades ?? [];

    // Normalise the flat+nested stats format from the Go engine
    const raw = statsData;
    const agg = raw?.aggregate;
    const mappedStats: EngineStats | null = raw ? {
      totalTrades: agg?.totalTrades ?? 0,
      openPositions: raw.openPositions ?? positions.length,
      winRate: agg?.winRate != null ? agg.winRate / 100 : 0,  // engine sends 0-100
      netPnlUSD: agg?.totalPnl ?? 0,
      balanceUSD: raw.balance ?? raw.cashBalance ?? 0,
      equityUSD: raw.equity ?? raw.balance ?? 0,
      maxDrawdownPct: agg?.maxDrawdown ?? 0,
      dailyPnlUSD: raw.dailyPnl ?? 0,
    } : null;

    setStatus({
      health: health != null ? "online" : "offline",
      lastFetchMs: Date.now(),
      positions,
      recentTrades: trades.slice(0, 8),
      stats: mappedStats,
    });
  }, []);

  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  useEffect(() => {
    fetchAll();
    timerRef.current = setInterval(fetchAll, pollMs);
    return () => { if (timerRef.current) clearInterval(timerRef.current); };
  }, [fetchAll, pollMs]);

  return status;
}

// ── Sub-components ────────────────────────────────────────────────────────────

function StatusDot({ health }: { health: DeskStatus["health"] }) {
  const color = health === "online" ? "#22c55e" : health === "degraded" ? "#f59e0b" : "#6b7280";
  const label = health === "online" ? "ENGINE ONLINE" : health === "degraded" ? "DEGRADED" : "ENGINE OFFLINE";
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
      <span style={{
        width: 8, height: 8, borderRadius: "50%", background: color,
        boxShadow: health === "online" ? `0 0 6px ${color}` : "none",
        display: "inline-block",
        animation: health === "online" ? "pulse 2s infinite" : "none",
      }} />
      <span style={{ fontSize: 11, fontWeight: 600, color, letterSpacing: "0.05em" }}>{label}</span>
    </div>
  );
}

function KpiTile({ label, value, color }: { label: string; value: string; color?: string }) {
  return (
    <div style={{
      background: "rgba(255,255,255,0.03)", border: "1px solid rgba(255,255,255,0.08)",
      borderRadius: 8, padding: "10px 14px", minWidth: 110,
    }}>
      <div style={{ fontSize: 10, color: "#6b7280", fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.08em", marginBottom: 4 }}>{label}</div>
      <div style={{ fontSize: 18, fontWeight: 700, color: color ?? "#e5e7eb", lineHeight: 1.2 }}>{value}</div>
    </div>
  );
}

function PositionRow({ p }: { p: EnginePosition }) {
  const pnl = p.unrealisedPnl ?? 0;
  return (
    <div style={{
      display: "flex", alignItems: "center", justifyContent: "space-between",
      padding: "7px 12px", borderRadius: 6, border: "1px solid rgba(255,255,255,0.07)",
      background: "rgba(255,255,255,0.02)", fontSize: 12, gap: 8,
    }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8, flex: 1, minWidth: 0, overflow: "hidden" }}>
        <span style={{
          padding: "1px 6px", borderRadius: 4, fontSize: 10, fontWeight: 700,
          background: p.side === "BUY" ? "rgba(34,197,94,0.15)" : "rgba(239,68,68,0.15)",
          color: p.side === "BUY" ? "#22c55e" : "#ef4444",
          flexShrink: 0,
        }}>{p.side}</span>
        <span style={{ color: "#d1d5db", fontWeight: 500, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{p.strategyName}</span>
      </div>
      <span style={{ color: "#9ca3af", fontSize: 11 }}>{p.size.toFixed(4)} BTC @ {fmtPrice(p.entryPrice)}</span>
      <span style={{ color: pnlColor(pnl), fontWeight: 600, minWidth: 70, textAlign: "right" }}>
        {fmtUsd(pnl)}
      </span>
      <span style={{ color: "#6b7280", fontSize: 11, minWidth: 50, textAlign: "right" }}>{age(p.openedAt)}</span>
    </div>
  );
}

function TradeRow({ t }: { t: EngineTrade }) {
  const pnl = t.netPnl ?? t.grossPnl ?? 0;
  return (
    <div style={{
      display: "flex", alignItems: "center", justifyContent: "space-between",
      padding: "6px 12px", borderRadius: 6, border: "1px solid rgba(255,255,255,0.06)",
      background: pnl > 0 ? "rgba(34,197,94,0.04)" : pnl < 0 ? "rgba(239,68,68,0.04)" : "rgba(255,255,255,0.02)",
      fontSize: 12, gap: 8,
    }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8, flex: 1, minWidth: 0 }}>
        <span style={{
          padding: "1px 6px", borderRadius: 4, fontSize: 10, fontWeight: 700,
          background: pnl > 0 ? "rgba(34,197,94,0.15)" : "rgba(239,68,68,0.15)",
          color: pnl > 0 ? "#22c55e" : "#ef4444",
        }}>{t.reason ?? (pnl >= 0 ? "TP" : "SL")}</span>
        <span style={{ color: "#d1d5db", fontWeight: 500 }}>{t.strategyName}</span>
      </div>
      <span style={{ color: "#9ca3af", fontSize: 11 }}>{t.side} {t.size?.toFixed(4)} BTC</span>
      <span style={{ color: pnlColor(pnl), fontWeight: 700, minWidth: 72, textAlign: "right" }}>
        {fmtUsd(pnl)}
      </span>
      <span style={{ color: "#6b7280", fontSize: 11, minWidth: 50, textAlign: "right" }}>{age(t.closedAt)}</span>
    </div>
  );
}

// ── Main Component ────────────────────────────────────────────────────────────

export function LiveDeskStatus() {
  const status = useDeskStatus();
  const s = status.stats;

  const equityUsd = s?.equityUSD ?? s?.balanceUSD ?? 0;
  const netPnl = s?.netPnlUSD ?? 0;
  const winRate = s?.winRate ? `${(s.winRate * 100).toFixed(1)}%` : "—";
  const totalTrades = s?.totalTrades ?? 0;
  const openCount = status.positions.length;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", flexWrap: "wrap", gap: 8 }}>
        <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <h2 style={{ margin: 0, fontSize: 15, fontWeight: 700, color: "#f9fafb" }}>
            BTC Paper Desk — Live Status
          </h2>
          <StatusDot health={status.health} />
        </div>
        {status.lastFetchMs > 0 && (
          <span style={{ fontSize: 10, color: "#4b5563" }}>
            Updated {Math.round((Date.now() - status.lastFetchMs) / 1000)}s ago
          </span>
        )}
      </div>

      {/* KPI strip */}
      <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
        <KpiTile label="Portfolio Equity" value={equityUsd > 0 ? `$${(equityUsd / 1000).toFixed(1)}K` : "—"} />
        <KpiTile label="Net P&L" value={netPnl !== 0 ? fmtUsd(netPnl) : "—"} color={pnlColor(netPnl)} />
        <KpiTile label="Today P&L" value={s?.dailyPnlUSD != null && s.dailyPnlUSD !== 0 ? fmtUsd(s.dailyPnlUSD) : "—"} color={pnlColor(s?.dailyPnlUSD ?? 0)} />
        <KpiTile label="Win Rate" value={winRate} color={s?.winRate && s.winRate > 0.5 ? "#22c55e" : "#f59e0b"} />
        <KpiTile label="Total Trades" value={totalTrades > 0 ? String(totalTrades) : "0"} />
        <KpiTile label="Open Positions" value={String(openCount)} color={openCount > 0 ? "#60a5fa" : "#6b7280"} />
        <KpiTile label="Max Drawdown" value={s?.maxDrawdownPct ? `$${s.maxDrawdownPct.toFixed(0)}` : "—"} color={s?.maxDrawdownPct && s.maxDrawdownPct > 5000 ? "#ef4444" : "#6b7280"} />
      </div>

      {/* Open Positions */}
      <div>
        <div style={{ fontSize: 12, fontWeight: 600, color: "#9ca3af", marginBottom: 8, textTransform: "uppercase", letterSpacing: "0.07em" }}>
          Open Positions ({openCount})
        </div>
        {status.health === "offline" ? (
          <div style={{ padding: "24px 0", textAlign: "center", color: "#6b7280", fontSize: 13 }}>
            Engine offline — start the Go engine and set INTERNAL_API_URL
          </div>
        ) : openCount === 0 ? (
          <div style={{ padding: "16px 12px", textAlign: "center", color: "#6b7280", fontSize: 12,
            border: "1px dashed rgba(255,255,255,0.08)", borderRadius: 8 }}>
            No open positions. Engine is scanning for signals…
          </div>
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            {status.positions.map((p) => <PositionRow key={p.id} p={p} />)}
          </div>
        )}
      </div>

      {/* Trade History */}
      <div>
        <div style={{ fontSize: 12, fontWeight: 600, color: "#9ca3af", marginBottom: 8, textTransform: "uppercase", letterSpacing: "0.07em" }}>
          Trade History
        </div>
        {status.recentTrades.length === 0 ? (
          <div style={{ padding: "16px 12px", textAlign: "center", color: "#6b7280", fontSize: 12,
            border: "1px dashed rgba(255,255,255,0.08)", borderRadius: 8 }}>
            {status.health === "online" ? "No completed trades yet — waiting for first signal" : "—"}
          </div>
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 3 }}>
            {status.recentTrades.map((t) => <TradeRow key={t.id} t={t} />)}
          </div>
        )}
      </div>

      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.5; }
        }
      `}</style>
    </div>
  );
}
