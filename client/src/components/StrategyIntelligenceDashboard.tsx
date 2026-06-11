"use client";

import { useCallback, useEffect, useState } from "react";

type AllocationTier = "A" | "B" | "C" | "D" | "F";
type StrategyStatus = "HEALTHY" | "WARNING" | "CRITICAL" | "INSUFFICIENT_DATA";

type StrategyRow = {
  strategy_id: string;
  status: StrategyStatus;
  enabled: boolean;
  total_pnl: number;
  expectancy: number;
  profit_factor: number;
  win_rate: number;
  max_drawdown: number;
  sample_size: number;
  avg_win: number;
  avg_loss: number;
  health_reasons: string[];
  evidence_score: number;
  allocation_tier: AllocationTier;
  last_computed: string;
};

type Summary = {
  healthy: number;
  warning: number;
  critical: number;
  insufficient_data: number;
  total: number;
};

type PortfolioStats = {
  total_realized_pnl: number;
  total_trades: number;
  win_rate: number;
  profit_factor: number;
};

type IntelData = {
  total_strategies: number;
  summary: Summary;
  portfolio_stats: PortfolioStats;
  strategies: StrategyRow[];
  server_time: string;
};

type View = "top20" | "top50" | "bottom20" | "retirement" | "all";

const VIEW_LABELS: Record<View, string> = {
  top20: "Top 20",
  top50: "Top 50",
  bottom20: "Bottom 20",
  retirement: "Retirement Candidates",
  all: "All Strategies",
};

const STATUS_COLOR: Record<StrategyStatus, string> = {
  HEALTHY: "#22c55e",
  WARNING: "#f59e0b",
  CRITICAL: "#ef4444",
  INSUFFICIENT_DATA: "#6b7280",
};

const TIER_COLOR: Record<AllocationTier, string> = {
  A: "#22c55e",
  B: "#3b82f6",
  C: "#f59e0b",
  D: "#f97316",
  F: "#ef4444",
};

function fmt(n: number, decimals = 2) {
  return Number.isFinite(n) ? n.toFixed(decimals) : "—";
}

function fmtPnl(n: number) {
  const s = `$${Math.abs(n).toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
  return n >= 0 ? `+${s}` : `-${s}`;
}

export default function StrategyIntelligenceDashboard() {
  const [view, setView] = useState<View>("top50");
  const [data, setData] = useState<IntelData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [sortKey, setSortKey] = useState<keyof StrategyRow>("expectancy");
  const [sortAsc, setSortAsc] = useState(false);
  const [search, setSearch] = useState("");

  const load = useCallback(
    async (v: View) => {
      setLoading(true);
      setError(null);
      try {
        const res = await fetch(`/api/strategy-intelligence?view=${v}&limit=600`);
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const json = await res.json();
        if (!json.ok) throw new Error(json.error ?? "API error");
        setData(json);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load strategy data");
      } finally {
        setLoading(false);
      }
    },
    [],
  );

  useEffect(() => {
    load(view);
    const id = setInterval(() => load(view), 30_000);
    return () => clearInterval(id);
  }, [view, load]);

  const handleSort = (key: keyof StrategyRow) => {
    if (sortKey === key) setSortAsc((p) => !p);
    else { setSortKey(key); setSortAsc(false); }
  };

  const rows = (data?.strategies ?? [])
    .filter((r) =>
      !search || r.strategy_id.toLowerCase().includes(search.toLowerCase()),
    )
    .sort((a, b) => {
      const av = a[sortKey];
      const bv = b[sortKey];
      if (typeof av === "number" && typeof bv === "number") {
        return sortAsc ? av - bv : bv - av;
      }
      return sortAsc
        ? String(av).localeCompare(String(bv))
        : String(bv).localeCompare(String(av));
    });

  const s = data?.summary;
  const ps = data?.portfolio_stats;

  return (
    <div style={{ fontFamily: "monospace", fontSize: 13, color: "#e2e8f0", background: "#0f172a", minHeight: "100vh", padding: 24 }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 20 }}>
        <h1 style={{ fontSize: 20, fontWeight: 700, color: "#f8fafc", margin: 0 }}>
          STRATEGY INTELLIGENCE COMMAND CENTER
        </h1>
        {data && (
          <span style={{ color: "#64748b", fontSize: 11 }}>
            Updated {new Date(data.server_time).toLocaleTimeString()} · {data.total_strategies} strategies
          </span>
        )}
      </div>

      {/* Summary ribbon */}
      {s && ps && (
        <div style={{ display: "grid", gridTemplateColumns: "repeat(8, 1fr)", gap: 8, marginBottom: 20 }}>
          {[
            { label: "HEALTHY", value: s.healthy, color: "#22c55e" },
            { label: "WARNING", value: s.warning, color: "#f59e0b" },
            { label: "CRITICAL", value: s.critical, color: "#ef4444" },
            { label: "INSUFFICIENT", value: s.insufficient_data, color: "#6b7280" },
            { label: "TOTAL PnL", value: fmtPnl(ps.total_realized_pnl), color: ps.total_realized_pnl >= 0 ? "#22c55e" : "#ef4444" },
            { label: "TOTAL TRADES", value: ps.total_trades.toLocaleString(), color: "#94a3b8" },
            { label: "WIN RATE", value: `${(ps.win_rate * 100).toFixed(1)}%`, color: "#94a3b8" },
            { label: "PROFIT FACTOR", value: fmt(ps.profit_factor), color: ps.profit_factor >= 1.25 ? "#22c55e" : ps.profit_factor >= 1.0 ? "#f59e0b" : "#ef4444" },
          ].map((tile) => (
            <div key={tile.label} style={{ background: "#1e293b", borderRadius: 6, padding: "10px 12px", border: "1px solid #334155" }}>
              <div style={{ color: "#64748b", fontSize: 10, marginBottom: 4 }}>{tile.label}</div>
              <div style={{ color: tile.color, fontSize: 16, fontWeight: 700 }}>{tile.value}</div>
            </div>
          ))}
        </div>
      )}

      {/* View selector + search */}
      <div style={{ display: "flex", gap: 8, marginBottom: 16, flexWrap: "wrap", alignItems: "center" }}>
        {(Object.keys(VIEW_LABELS) as View[]).map((v) => (
          <button
            key={v}
            onClick={() => setView(v)}
            style={{
              padding: "6px 14px",
              borderRadius: 4,
              border: "1px solid",
              borderColor: view === v ? "#3b82f6" : "#334155",
              background: view === v ? "#1d4ed8" : "#1e293b",
              color: view === v ? "#fff" : "#94a3b8",
              cursor: "pointer",
              fontSize: 12,
              fontFamily: "monospace",
            }}
          >
            {VIEW_LABELS[v]}
          </button>
        ))}
        <input
          type="text"
          placeholder="Search strategy..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          style={{
            marginLeft: "auto",
            padding: "6px 12px",
            background: "#1e293b",
            border: "1px solid #334155",
            borderRadius: 4,
            color: "#e2e8f0",
            fontSize: 12,
            fontFamily: "monospace",
            width: 220,
          }}
        />
      </div>

      {loading && (
        <div style={{ color: "#64748b", padding: 40, textAlign: "center" }}>
          LOADING STRATEGY INTELLIGENCE DATA...
        </div>
      )}

      {error && (
        <div style={{ color: "#ef4444", padding: 20, background: "#1e293b", borderRadius: 6, border: "1px solid #ef4444" }}>
          BACKEND AUTHORITY UNAVAILABLE: {error}
        </div>
      )}

      {!loading && !error && rows.length === 0 && (
        <div style={{ color: "#64748b", padding: 40, textAlign: "center" }}>
          NO DATA AVAILABLE — Backend has not generated strategy metrics yet.
        </div>
      )}

      {!loading && !error && rows.length > 0 && (
        <div style={{ overflowX: "auto" }}>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
            <thead>
              <tr style={{ background: "#1e293b", borderBottom: "1px solid #334155" }}>
                {(
                  [
                    ["strategy_id", "STRATEGY"],
                    ["status", "STATUS"],
                    ["allocation_tier", "TIER"],
                    ["enabled", "ENABLED"],
                    ["total_pnl", "TOTAL PnL"],
                    ["expectancy", "EXPECTANCY"],
                    ["profit_factor", "PF"],
                    ["win_rate", "WIN%"],
                    ["max_drawdown", "MAX DD"],
                    ["sample_size", "TRADES"],
                    ["evidence_score", "EVIDENCE"],
                  ] as [keyof StrategyRow, string][]
                ).map(([key, label]) => (
                  <th
                    key={key}
                    onClick={() => handleSort(key)}
                    style={{
                      padding: "8px 10px",
                      textAlign: "left",
                      color: sortKey === key ? "#3b82f6" : "#64748b",
                      cursor: "pointer",
                      userSelect: "none",
                      whiteSpace: "nowrap",
                      fontFamily: "monospace",
                    }}
                  >
                    {label} {sortKey === key ? (sortAsc ? "▲" : "▼") : ""}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((r, i) => (
                <tr
                  key={r.strategy_id}
                  style={{
                    background: i % 2 === 0 ? "#0f172a" : "#111827",
                    borderBottom: "1px solid #1e293b",
                  }}
                >
                  <td style={{ padding: "7px 10px", color: "#cbd5e1", maxWidth: 280, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={r.strategy_id}>
                    {r.strategy_id}
                  </td>
                  <td style={{ padding: "7px 10px" }}>
                    <span style={{ color: STATUS_COLOR[r.status], fontWeight: 600 }}>{r.status}</span>
                  </td>
                  <td style={{ padding: "7px 10px" }}>
                    <span style={{
                      background: TIER_COLOR[r.allocation_tier] + "22",
                      color: TIER_COLOR[r.allocation_tier],
                      padding: "2px 8px",
                      borderRadius: 3,
                      fontWeight: 700,
                    }}>
                      {r.allocation_tier}
                    </span>
                  </td>
                  <td style={{ padding: "7px 10px", color: r.enabled ? "#22c55e" : "#6b7280" }}>
                    {r.enabled ? "YES" : "NO"}
                  </td>
                  <td style={{ padding: "7px 10px", color: r.total_pnl >= 0 ? "#22c55e" : "#ef4444", fontWeight: 600 }}>
                    {fmtPnl(r.total_pnl)}
                  </td>
                  <td style={{ padding: "7px 10px", color: r.expectancy >= 0 ? "#22c55e" : "#ef4444" }}>
                    ${fmt(r.expectancy)}
                  </td>
                  <td style={{ padding: "7px 10px", color: r.profit_factor >= 1.25 ? "#22c55e" : r.profit_factor >= 1.0 ? "#f59e0b" : "#ef4444" }}>
                    {fmt(r.profit_factor)}
                  </td>
                  <td style={{ padding: "7px 10px" }}>{fmt(r.win_rate * 100, 1)}%</td>
                  <td style={{ padding: "7px 10px", color: "#f59e0b" }}>
                    {fmt(r.max_drawdown * 100, 1)}%
                  </td>
                  <td style={{ padding: "7px 10px", color: r.sample_size >= 30 ? "#94a3b8" : "#6b7280" }}>
                    {r.sample_size}
                  </td>
                  <td style={{ padding: "7px 10px" }}>
                    <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                      <div style={{
                        width: 60, height: 6, background: "#334155", borderRadius: 3, overflow: "hidden",
                      }}>
                        <div style={{
                          width: `${r.evidence_score}%`,
                          height: "100%",
                          background: r.evidence_score >= 70 ? "#22c55e" : r.evidence_score >= 40 ? "#f59e0b" : "#ef4444",
                          borderRadius: 3,
                        }} />
                      </div>
                      <span style={{ color: "#94a3b8" }}>{r.evidence_score}</span>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
