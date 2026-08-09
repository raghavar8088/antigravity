"use client";

/**
 * Crypto Options Selling Desk — dashboard for the options_selling paper
 * engine (50 BTC option-selling strategies: scalp / intraday / swing).
 *
 * Built on the shared desk primitives (components/desk/ui) and --desk-*
 * tokens so it inherits the light/dark theme toggle and matches every other
 * institutional dashboard in the app. Reads only through the session-gated
 * /api/options-selling proxy. Paper only — short options against the
 * engine's synthetic BTC chain, no real money.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  DeskBanner,
  DeskButton,
  DeskCard,
  DeskChip,
  DeskDataTable,
  DeskEmptyState,
  DeskLinearProgress,
  DeskMetricTile,
  DeskSearchField,
  DeskSectionHeader,
  StatusBadge,
  type DeskColumn,
  type DeskEngineStatus,
} from "@/components/desk/ui";
import { DeskAdminControls } from "@/components/desk/DeskAdminControls";
import { fmtIST, fmtISTClock } from "@/lib/istTime";

type StrategyStatus = {
  strategyId: number;
  name: string;
  category: string;
  optionType: string;
  rosterState: "ACTIVE" | "WATCHLIST" | "DISABLED";
  status: string;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  shadowTrades: number;
  shadowWins: number;
  shadowLosses: number;
  shadowPnl: number;
  shadowWinRate: number;
  score: number;
  regime: string;
  regimeFit: number;
  allocationUsd: number;
  sizeMultiplier: number;
  disableReason?: string;
  hasPosition: boolean;
  hasShadowPosition: boolean;
};

type OptionTrade = {
  id: string;
  strategyId: number;
  strategyName: string;
  optionType: string;
  strike: number;
  expiryMins: number;
  entryPremium: number;
  exitPremium: number;
  quantity: number;
  costBasis: number;
  netPnl: number;
  returnPct: number;
  entryBtcPrice: number;
  exitBtcPrice: number;
  entryTime: string;
  exitTime: string;
  exitReason: string;
};

type OptionPosition = {
  id: string;
  strategyId: number;
  strategyName: string;
  optionType: string;
  strike: number;
  expiryTime: string;
  entryPremium: number;
  currentPremium: number;
  quantity: number;
  costBasis: number;
  marginBlocked: number;
  entryBtcPrice: number;
  entryTime: string;
  unrealizedPnl: number;
  peakGainPct: number;
  iv: number;
  delta: number;
};

type AggregateStats = {
  balance: number;
  equity: number;
  totalTrades: number;
  openPositions: number;
  totalWins: number;
  totalLosses: number;
  winRate: number;
  totalPnl: number;
  totalPremiumSpent: number;
  unrealizedPnl: number;
};

const CATEGORIES = ["ALL", "Momentum", "Mean Reversion", "Breakout", "Capitulation", "Hybrid"];
const ROSTER_STATES = ["ALL", "ACTIVE", "WATCHLIST", "DISABLED"] as const;

type SortKey = "totalTrades" | "winRate" | "totalPnl" | "score" | "allocationUsd";

function fmtUSD(v: number): string {
  return `${v < 0 ? "-" : "+"}$${Math.abs(v).toFixed(2)}`;
}
function pnlToneClass(v: number): string {
  return v > 0 ? "desk-pnl-positive" : v < 0 ? "desk-pnl-negative" : "desk-pnl-neutral";
}
function fmtPrice(v: number): string {
  return v.toLocaleString("en-US", { maximumFractionDigits: 0 });
}
function holdingBucket(name: string): string {
  if (name.startsWith("Scalp_")) return "Scalp";
  if (name.startsWith("Intraday_")) return "Intraday";
  if (name.startsWith("Swing_")) return "Swing";
  return "Other";
}
function rosterTone(state: string): "success" | "warning" | "default" {
  return state === "ACTIVE" ? "success" : state === "WATCHLIST" ? "warning" : "default";
}

function SortableHeader({
  label, k, sortKey, sortDir, onSort,
}: {
  label: string; k: SortKey; sortKey: SortKey; sortDir: "asc" | "desc"; onSort: (k: SortKey) => void;
}) {
  const active = sortKey === k;
  return (
    <button
      type="button"
      onClick={() => onSort(k)}
      className="ml-auto inline-flex items-center gap-1 transition-colors"
      style={{ color: active ? "var(--desk-primary)" : "inherit" }}
    >
      {label}
      <span style={{ fontSize: 8, lineHeight: 1 }}>{active ? (sortDir === "desc" ? "▼" : "▲") : "▲▼"}</span>
    </button>
  );
}

export default function OptionsSellingDeskPage() {
  const [stats, setStats] = useState<AggregateStats | null>(null);
  const [strategies, setStrategies] = useState<StrategyStatus[]>([]);
  const [positions, setPositions] = useState<OptionPosition[]>([]);
  const [trades, setTrades] = useState<OptionTrade[]>([]);
  const [category, setCategory] = useState<string>("ALL");
  const [rosterFilter, setRosterFilter] = useState<(typeof ROSTER_STATES)[number]>("ALL");
  const [query, setQuery] = useState<string>("");
  const [sortKey, setSortKey] = useState<SortKey>("score");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [error, setError] = useState<string>("");
  const [updatedAt, setUpdatedAt] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);
  const [showAllTrades, setShowAllTrades] = useState<boolean>(false);

  const refresh = useCallback(async () => {
    try {
      const [s, st, p, tr] = await Promise.all([
        fetch("/api/options-selling/stats", { cache: "no-store" }),
        fetch("/api/options-selling/strategies", { cache: "no-store" }),
        fetch("/api/options-selling/positions", { cache: "no-store" }),
        fetch("/api/options-selling/trades", { cache: "no-store" }),
      ]);
      if (!s.ok || !st.ok || !p.ok || !tr.ok) {
        const bad = [s, st, p, tr].find((r) => !r.ok);
        setError(`desk unreachable (HTTP ${bad?.status})`);
        return;
      }
      setStats(await s.json());
      setStrategies((await st.json()) as StrategyStatus[]);
      setPositions((await p.json()) as OptionPosition[]);
      setTrades((await tr.json()) as OptionTrade[]);
      setError("");
      setUpdatedAt(fmtISTClock(Date.now()));
    } catch {
      setError("desk unreachable");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
    const t = setInterval(() => void refresh(), 30_000);
    return () => clearInterval(t);
  }, [refresh]);

  const q = query.trim().toLowerCase();
  const filtered = useMemo(
    () =>
      strategies
        .filter((r) => category === "ALL" || r.category === category)
        .filter((r) => rosterFilter === "ALL" || r.rosterState === rosterFilter)
        .filter((r) => q === "" || r.name.toLowerCase().includes(q))
        .slice()
        .sort((a, b) => {
          const d = a[sortKey] - b[sortKey];
          return sortDir === "desc" ? -d : d;
        }),
    [strategies, category, rosterFilter, q, sortKey, sortDir],
  );
  const filtersActive = category !== "ALL" || rosterFilter !== "ALL" || q !== "";
  const resetFilters = () => {
    setCategory("ALL");
    setRosterFilter("ALL");
    setQuery("");
  };
  const toggleSort = (k: SortKey) => {
    if (sortKey === k) setSortDir((d) => (d === "desc" ? "asc" : "desc"));
    else {
      setSortKey(k);
      setSortDir("desc");
    }
  };

  const activeCount = strategies.filter((r) => r.rosterState === "ACTIVE").length;
  const categoryCounts = CATEGORIES.slice(1).map((c) => ({
    category: c,
    total: strategies.filter((r) => r.category === c).length,
    active: strategies.filter((r) => r.category === c && r.rosterState === "ACTIVE").length,
  }));
  const engineUp = strategies.length > 0 && !error;
  const engineStatus: DeskEngineStatus = !strategies.length ? "syncing" : engineUp ? "live" : "degraded";

  const strategyColumns: DeskColumn<StrategyStatus>[] = [
    { id: "name", header: "Strategy", cell: (r) => <span className="desk-body-md" style={{ fontWeight: 600 }}>{r.name}</span> },
    { id: "bucket", header: "Bucket", cell: (r) => holdingBucket(r.name) },
    { id: "category", header: "Category", cell: (r) => r.category },
    {
      id: "type",
      header: "Type",
      cell: (r) => (
        <span style={{ fontSize: "0.6875rem", fontWeight: 700, color: r.optionType === "PUT" ? "var(--desk-success)" : "var(--desk-error)" }}>
          {r.optionType}
        </span>
      ),
    },
    {
      id: "trades", align: "right",
      header: <SortableHeader label="Trades" k="totalTrades" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />,
      cell: (r) => r.totalTrades,
    },
    {
      id: "wr", align: "right",
      header: <SortableHeader label="WR %" k="winRate" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />,
      cell: (r) => r.winRate.toFixed(1),
    },
    {
      id: "net", align: "right",
      header: <SortableHeader label="Net $" k="totalPnl" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />,
      cell: (r) => <span className={pnlToneClass(r.totalPnl)} style={{ fontWeight: 600 }}>{fmtUSD(r.totalPnl)}</span>,
    },
    {
      id: "score", align: "right",
      header: <SortableHeader label="Score" k="score" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />,
      cell: (r) => r.score.toFixed(1),
    },
    {
      id: "alloc", align: "right",
      header: <SortableHeader label="Alloc $" k="allocationUsd" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />,
      cell: (r) => r.allocationUsd.toFixed(0),
    },
    { id: "roster", align: "right", header: "Roster", cell: (r) => <DeskChip tone={rosterTone(r.rosterState)} style={{ fontWeight: 700 }}>{r.rosterState}</DeskChip> },
  ];

  const positionColumns: DeskColumn<OptionPosition>[] = [
    { id: "strategy", header: "Strategy", cell: (p) => <span className="desk-body-md" style={{ fontWeight: 600 }}>{p.strategyName}</span> },
    {
      id: "type", header: "Type",
      cell: (p) => <span style={{ fontSize: "0.6875rem", fontWeight: 700, color: p.optionType === "PUT" ? "var(--desk-success)" : "var(--desk-error)" }}>{p.optionType}</span>,
    },
    { id: "strike", align: "right", header: "Strike", cell: (p) => fmtPrice(p.strike) },
    { id: "entry", align: "right", header: "Entry Premium", cell: (p) => p.entryPremium.toFixed(2) },
    { id: "current", align: "right", header: "Current Premium", cell: (p) => p.currentPremium.toFixed(2) },
    { id: "margin", align: "right", header: "Margin", cell: (p) => fmtUSD(p.marginBlocked) },
    { id: "unrealized", align: "right", header: "Unrealized", cell: (p) => <span className={pnlToneClass(p.unrealizedPnl)} style={{ fontWeight: 600 }}>{fmtUSD(p.unrealizedPnl)}</span> },
  ];

  const tradeColumns: DeskColumn<OptionTrade>[] = [
    { id: "time", header: "Exit Time (IST)", cell: (t) => fmtIST(t.exitTime) },
    { id: "strategy", header: "Strategy", cell: (t) => <span className="desk-body-md" style={{ fontWeight: 600 }}>{t.strategyName}</span> },
    {
      id: "type", header: "Type",
      cell: (t) => <span style={{ fontSize: "0.6875rem", fontWeight: 700, color: t.optionType === "PUT" ? "var(--desk-success)" : "var(--desk-error)" }}>{t.optionType}</span>,
    },
    { id: "strike", align: "right", header: "Strike", cell: (t) => fmtPrice(t.strike) },
    { id: "entry", align: "right", header: "Entry", cell: (t) => t.entryPremium.toFixed(2) },
    { id: "exit", align: "right", header: "Exit", cell: (t) => t.exitPremium.toFixed(2) },
    {
      id: "reason", header: "Reason",
      cell: (t) => <DeskChip tone={t.exitReason === "TP" ? "success" : t.exitReason === "SL" ? "error" : "default"}>{t.exitReason}</DeskChip>,
    },
    { id: "ret", align: "right", header: "Ret %", cell: (t) => t.returnPct.toFixed(1) },
    { id: "pnl", align: "right", header: "P&L", cell: (t) => <span className={pnlToneClass(t.netPnl)} style={{ fontWeight: 600 }}>{fmtUSD(t.netPnl)}</span> },
  ];

  return (
    <div style={{ minHeight: "100%", background: "var(--desk-surface-dim)" }}>
      <DeskLinearProgress visible={loading} />
      <main className="desk-page">
        {/* Breadcrumb + heading */}
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 6, fontSize: "0.8125rem" }}>
            <Link href="/terminal" className="desk-label-md" style={{ fontWeight: 400, textDecoration: "none" }}>Home</Link>
            <span style={{ color: "var(--desk-outline)" }}>›</span>
            <span className="desk-body-md" style={{ fontWeight: 500 }}>Options Selling Desk</span>
          </div>
          <div style={{ marginTop: 8, display: "flex", flexWrap: "wrap", alignItems: "center", gap: 12 }}>
            <h1 className="desk-display-lg" style={{ fontSize: "2rem" }}>Crypto Options Selling</h1>
            <StatusBadge status={engineStatus} />
          </div>
          <p className="desk-body-md" style={{ marginTop: 6, maxWidth: 720, color: "var(--desk-on-surface-variant)" }}>
            50 BTC premium-selling strategies spanning scalp, intraday and swing holding periods — short puts and calls
            against synthetic BTC option premiums, sized by a tail-risk-aware roster that rotates only the top 13 into
            live paper capital at a time. Paper trading only, no real money.
          </p>
        </div>

        {error && (
          <DeskBanner variant="warning">{error} — retrying every 30s</DeskBanner>
        )}

        {/* Stat cards */}
        <div className="desk-metrics-row">
          <DeskMetricTile
            label="Net P&L"
            value={stats ? fmtUSD(stats.totalPnl) : "—"}
            valueClassName={stats ? pnlToneClass(stats.totalPnl) : undefined}
            sub={stats ? `realized + unrealized ${fmtUSD(stats.unrealizedPnl)}` : "—"}
            highlight
          />
          <DeskMetricTile
            label="Closed Trades"
            value={stats ? String(stats.totalTrades) : "—"}
            sub={stats ? `win rate ${stats.winRate.toFixed(1)}%` : "win rate —"}
            subColor="profit"
          />
          <DeskMetricTile
            label="Open Positions"
            value={stats ? String(stats.openPositions) : "—"}
            sub="short option positions live"
          />
          <DeskMetricTile
            label="Active Roster"
            value={strategies.length ? `${activeCount} / 13` : "—"}
            sub={`of ${strategies.length} strategies, capped 4 per category`}
          />
        </div>

        {/* Category mix */}
        <DeskCard>
          <DeskSectionHeader
            title="Category Mix"
            actions={<span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>{updatedAt ? `computed ${updatedAt}` : "—"}</span>}
          />
          <div className="desk-metrics-row">
            {categoryCounts.map((c) => (
              <DeskMetricTile key={c.category} compact label={c.category} value={c.total} sub={`${c.active} active`} />
            ))}
          </div>
        </DeskCard>

        {/* Strategy leaderboard */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Strategy Leaderboard"
            actions={<span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>{filtered.length} of {strategies.length} strategies</span>}
          />

          <div className="desk-toolbar" style={{ marginBottom: 12 }}>
            <div className="desk-toolbar__actions">
              <DeskSearchField
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Search strategy…"
              />
              <select
                value={category}
                onChange={(e) => setCategory(e.target.value)}
                className="desk-select"
                style={{
                  minHeight: 44, borderRadius: "var(--desk-radius-input)", border: "1px solid var(--desk-outline)",
                  background: "var(--desk-surface)", color: "var(--desk-on-surface)", fontSize: "0.8125rem", padding: "0 12px",
                }}
              >
                {CATEGORIES.map((c) => (
                  <option key={c} value={c}>{c === "ALL" ? "All categories" : c}</option>
                ))}
              </select>
              <div style={{ display: "inline-flex", gap: 4 }}>
                {ROSTER_STATES.map((s) => (
                  <button
                    key={s}
                    type="button"
                    onClick={() => setRosterFilter(s)}
                    className="desk-chip"
                    style={{
                      borderRadius: "var(--desk-radius-chip)",
                      padding: "6px 12px",
                      fontSize: "0.75rem",
                      fontWeight: 700,
                      border: "1px solid var(--desk-outline)",
                      cursor: "pointer",
                      background: rosterFilter === s ? "var(--desk-primary-container)" : "transparent",
                      color: rosterFilter === s ? "var(--desk-on-primary-container)" : "var(--desk-on-surface-variant)",
                    }}
                  >
                    {s === "ALL" ? "All" : s.charAt(0) + s.slice(1).toLowerCase()}
                  </button>
                ))}
              </div>
            </div>
            {filtersActive && (
              <button type="button" onClick={resetFilters} className="desk-label-md" style={{ color: "var(--desk-primary)", cursor: "pointer", fontWeight: 700, background: "none", border: "none" }}>
                Reset
              </button>
            )}
          </div>

          <DeskDataTable
            columns={strategyColumns}
            rows={filtered}
            getRowKey={(r) => String(r.strategyId)}
            minWidth={860}
            empty={
              <DeskEmptyState
                title="No strategies match"
                subtitle={strategies.length === 0 ? "No strategy data yet — the engine trades 24/7; check back soon." : "No strategies match these filters."}
              />
            }
          />
        </DeskCard>

        {/* Open positions */}
        <DeskCard padding="md">
          <DeskSectionHeader title="Open Positions" actions={<span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>{positions.length} live</span>} />
          <DeskDataTable
            columns={positionColumns}
            rows={positions}
            getRowKey={(p) => p.id}
            minWidth={720}
            empty={<DeskEmptyState title="No open positions" subtitle="No open positions right now." />}
          />
        </DeskCard>

        {/* Recent trades */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Trade History"
            actions={
              <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                  {showAllTrades
                    ? `all ${trades.length}`
                    : `last ${Math.min(150, trades.length)} of ${trades.length}`}
                </span>
                {trades.length > 150 && (
                  <DeskButton variant="text" onClick={() => setShowAllTrades((v) => !v)}>
                    {showAllTrades ? "Show less" : "View all trade history"}
                  </DeskButton>
                )}
              </div>
            }
          />
          <DeskDataTable
            columns={tradeColumns}
            rows={showAllTrades ? trades : trades.slice(0, 150)}
            getRowKey={(t) => t.id}
            minWidth={860}
            empty={<DeskEmptyState title="No closed trades" subtitle="No closed trades yet — the desk trades 24/7; check back soon." />}
          />
        </DeskCard>

        <DeskAdminControls
          resetPath="/api/options-selling/reset"
          clearPath="/api/options-selling/clear-history"
          capitalLabel="Starting balance (USD)"
          onDone={() => void refresh()}
        />

        <p className="desk-label-md" style={{ textAlign: "center", fontWeight: 400, paddingBottom: 8 }}>
          Paper trading only · short premium against synthetic BTC option chain · 13 of 50 strategies rotated live at a
          time, max 4 per category · state persists on the engine host
        </p>
      </main>
    </div>
  );
}
