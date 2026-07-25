"use client";

/**
 * Crypto Scalp Desk — dashboard for the scalp_prelive paper engine
 * (100 x 1m scalp strategies x 8 symbols = 800 live paper streams).
 *
 * Built on the shared desk primitives (components/desk/ui) and --desk-*
 * tokens. Reads only through the session-gated /api/scalp proxy. Paper
 * only: the PRE-REGISTERED gate is the sole route to a real-money
 * discussion.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  DeskBanner,
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

type Stats = {
  trades: number;
  wins: number;
  missed_fills: number;
  open_positions: number;
  pending_orders: number;
  net_pnl_usd_at_100_notional: number;
  trades_per_symbol: Record<string, number>;
};
type Health = { ok: boolean; uptime_min: number; bars_processed: number; strategies: number; streams: number };
type LbRow = {
  strategy: string; symbol: string; n: number; wr_pct: number; pf: number;
  net_usd: number; max_dd_pct: number; missed: number; gate_pass: boolean;
};
type Trade = {
  time: string; symbol: string; strategy: string; dir: string; entry: number; exit: number;
  reason: string; ret_net: number; pnl_usd: number; profile: string; hold_min: number;
};

const SYMBOLS = ["ALL", "BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "AVAXUSDT"];
const GATE_RULES = ["≥ 200 live trades", "PF ≥ 1.2", "max DD ≤ 25%", "both halves net-positive"];
const MIN_N_OPTS = [0, 5, 20, 50, 100, 200];

type SortKey = "n" | "wr_pct" | "pf" | "net_usd" | "max_dd_pct" | "missed";
type Side = "ALL" | "LONG" | "SHORT";

function fmtUSD(v: number): string {
  return `${v < 0 ? "-" : "+"}$${Math.abs(v).toFixed(2)}`;
}
function pnlToneClass(v: number): string {
  return v > 0 ? "desk-pnl-positive" : v < 0 ? "desk-pnl-negative" : "desk-pnl-neutral";
}
function fmtPrice(v: number): string {
  if (v >= 1000) return v.toLocaleString("en-US", { maximumFractionDigits: 1 });
  if (v >= 1) return v.toFixed(3);
  return v.toFixed(5);
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

function segButtonStyle(active: boolean, tone: "primary" | "success" | "error" = "primary") {
  const bg = active
    ? tone === "success" ? "var(--desk-success-container)" : tone === "error" ? "var(--desk-error-container)" : "var(--desk-primary-container)"
    : "transparent";
  const color = active
    ? tone === "success" ? "var(--desk-success)" : tone === "error" ? "var(--desk-error)" : "var(--desk-on-primary-container)"
    : "var(--desk-on-surface-variant)";
  return { background: bg, color, borderRadius: "var(--desk-radius-chip)", padding: "6px 12px", fontSize: "0.75rem", fontWeight: 700, border: "1px solid var(--desk-outline)", cursor: "pointer" } as const;
}

export default function ScalpDeskPage() {
  const [stats, setStats] = useState<Stats | null>(null);
  const [health, setHealth] = useState<Health | null>(null);
  const [rows, setRows] = useState<LbRow[]>([]);
  const [trades, setTrades] = useState<Trade[]>([]);
  const [symbol, setSymbol] = useState<string>("ALL");
  const [query, setQuery] = useState<string>("");
  const [side, setSide] = useState<Side>("ALL");
  const [minN, setMinN] = useState<number>(0);
  const [gateOnly, setGateOnly] = useState<boolean>(false);
  const [profitOnly, setProfitOnly] = useState<boolean>(false);
  const [sortKey, setSortKey] = useState<SortKey>("net_usd");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [error, setError] = useState<string>("");
  const [updatedAt, setUpdatedAt] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);

  const refresh = useCallback(async () => {
    try {
      const [h, s, lb, tr] = await Promise.all([
        fetch("/api/scalp/scalp/health", { cache: "no-store" }),
        fetch("/api/scalp/scalp/stats", { cache: "no-store" }),
        fetch("/api/scalp/scalp/leaderboard", { cache: "no-store" }),
        fetch("/api/scalp/scalp/trades?n=50", { cache: "no-store" }),
      ]);
      if (!h.ok || !s.ok || !lb.ok || !tr.ok) {
        const bad = [h, s, lb, tr].find((r) => !r.ok);
        setError(`desk unreachable (HTTP ${bad?.status})`);
        return;
      }
      setHealth(await h.json());
      setStats(await s.json());
      setRows(((await lb.json()) as { rows: LbRow[] }).rows ?? []);
      setTrades((await tr.json()) as Trade[]);
      setError("");
      setUpdatedAt(new Date().toLocaleTimeString());
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
      rows
        .filter((r) => symbol === "ALL" || r.symbol === symbol)
        .filter((r) => r.n >= minN)
        .filter((r) => !gateOnly || r.gate_pass)
        .filter((r) => !profitOnly || r.net_usd > 0)
        .filter((r) =>
          side === "ALL" ? true : side === "LONG" ? r.strategy.endsWith("_Long") : r.strategy.endsWith("_Short"),
        )
        .filter((r) => q === "" || r.strategy.toLowerCase().includes(q) || r.symbol.toLowerCase().includes(q))
        .slice()
        .sort((a, b) => {
          const d = a[sortKey] - b[sortKey];
          return sortDir === "desc" ? -d : d;
        }),
    [rows, symbol, minN, gateOnly, profitOnly, side, q, sortKey, sortDir],
  );
  const filtersActive =
    symbol !== "ALL" || q !== "" || side !== "ALL" || minN !== 0 || gateOnly || profitOnly;
  const resetFilters = () => {
    setSymbol("ALL");
    setQuery("");
    setSide("ALL");
    setMinN(0);
    setGateOnly(false);
    setProfitOnly(false);
  };
  const toggleSort = (k: SortKey) => {
    if (sortKey === k) setSortDir((d) => (d === "desc" ? "asc" : "desc"));
    else {
      setSortKey(k);
      setSortDir("desc");
    }
  };
  const gatePassers = rows.filter((r) => r.gate_pass);
  const winRate = stats && stats.trades > 0 ? (100 * stats.wins) / stats.trades : null;
  const fillRate =
    stats && stats.trades + stats.missed_fills > 0
      ? (100 * stats.trades) / (stats.trades + stats.missed_fills)
      : null;
  const engineStatus: DeskEngineStatus = !health ? "syncing" : health.ok ? "live" : "degraded";

  const leaderboardColumns: DeskColumn<LbRow>[] = [
    { id: "strategy", header: "Strategy", cell: (r) => <span className="desk-body-md" style={{ fontWeight: 600 }}>{r.strategy}</span> },
    { id: "symbol", header: "Symbol", cell: (r) => r.symbol.replace("USDT", "") },
    { id: "n", align: "right", header: <SortableHeader label="Trades" k="n" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />, cell: (r) => r.n },
    { id: "wr", align: "right", header: <SortableHeader label="WR %" k="wr_pct" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />, cell: (r) => r.wr_pct.toFixed(1) },
    { id: "pf", align: "right", header: <SortableHeader label="PF" k="pf" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />, cell: (r) => r.pf.toFixed(2) },
    {
      id: "net", align: "right",
      header: <SortableHeader label="Net $" k="net_usd" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />,
      cell: (r) => <span className={pnlToneClass(r.net_usd)} style={{ fontWeight: 600 }}>{fmtUSD(r.net_usd)}</span>,
    },
    { id: "dd", align: "right", header: <SortableHeader label="Max DD %" k="max_dd_pct" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />, cell: (r) => r.max_dd_pct.toFixed(1) },
    { id: "missed", align: "right", header: <SortableHeader label="Missed" k="missed" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />, cell: (r) => r.missed },
    {
      id: "gate", align: "right", header: "Gate",
      cell: (r) => (r.gate_pass ? <DeskChip tone="success" style={{ fontWeight: 700 }}>PASS</DeskChip> : <DeskChip tone="default">—</DeskChip>),
    },
  ];

  const tradeColumns: DeskColumn<Trade>[] = [
    { id: "time", header: "Time (UTC)", cell: (t) => new Date(t.time).toISOString().slice(5, 16).replace("T", " ") },
    { id: "symbol", header: "Symbol", cell: (t) => t.symbol.replace("USDT", "") },
    { id: "strategy", header: "Strategy", cell: (t) => <span className="desk-body-md" style={{ fontWeight: 600 }}>{t.strategy}</span> },
    {
      id: "dir", header: "Dir",
      cell: (t) => <span style={{ fontSize: "0.6875rem", fontWeight: 700, color: t.dir === "LONG" ? "var(--desk-success)" : "var(--desk-error)" }}>{t.dir}</span>,
    },
    { id: "entry", align: "right", header: "Entry", cell: (t) => fmtPrice(t.entry) },
    { id: "exit", align: "right", header: "Exit", cell: (t) => fmtPrice(t.exit) },
    {
      id: "reason", header: "Reason",
      cell: (t) => <DeskChip tone={t.reason === "TP" ? "success" : t.reason === "SL" ? "error" : "default"}>{t.reason}</DeskChip>,
    },
    { id: "hold", align: "right", header: "Hold", cell: (t) => `${t.hold_min}m` },
    { id: "pnl", align: "right", header: "P&L", cell: (t) => <span className={pnlToneClass(t.pnl_usd)} style={{ fontWeight: 600 }}>{fmtUSD(t.pnl_usd)}</span> },
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
            <span className="desk-body-md" style={{ fontWeight: 500 }}>Scalp Desk</span>
          </div>
          <div style={{ marginTop: 8, display: "flex", flexWrap: "wrap", alignItems: "center", gap: 12 }}>
            <h1 className="desk-display-lg" style={{ fontSize: "2rem" }}>Scalp Desk</h1>
            <StatusBadge status={engineStatus} />
          </div>
          <p className="desk-body-md" style={{ marginTop: 6, maxWidth: 720, color: "var(--desk-on-surface-variant)" }}>
            100 one-minute strategies across 8 major cryptos — 800 live paper streams from your scalp engine on AWS,
            with a maker-fill model identical to the backtest. Real money only via the pre-registered gate.
          </p>
        </div>

        {error && <DeskBanner variant="warning">{error} — retrying every 30s</DeskBanner>}

        {/* Stat cards */}
        <div className="desk-metrics-row">
          <DeskMetricTile
            label="Net P&L"
            value={stats ? fmtUSD(stats.net_pnl_usd_at_100_notional) : "—"}
            valueClassName={stats ? pnlToneClass(stats.net_pnl_usd_at_100_notional) : undefined}
            sub="$100 notional per trade"
            highlight
          />
          <DeskMetricTile
            label="Closed Trades"
            value={stats ? String(stats.trades) : "—"}
            sub={winRate === null ? "win rate —" : `win rate ${winRate.toFixed(1)}%`}
            subColor="profit"
          />
          <DeskMetricTile
            label="Open / Pending"
            value={stats ? `${stats.open_positions} / ${stats.pending_orders}` : "—"}
            sub="positions / resting post-only orders"
          />
        </div>

        {/* Desk analytics */}
        <DeskCard>
          <DeskSectionHeader
            title="Desk Analytics"
            actions={<span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>{updatedAt ? `computed ${updatedAt}` : "—"}</span>}
          />
          <div className="desk-metrics-row">
            <DeskMetricTile compact label="Engine" value={health ? (health.ok ? "UP" : "DOWN") : "—"} valueClassName={health ? (health.ok ? "desk-pnl-positive" : "desk-pnl-negative") : undefined} />
            <DeskMetricTile compact label="Uptime" value={health ? `${Math.floor(health.uptime_min / 60)}h ${health.uptime_min % 60}m` : "—"} />
            <DeskMetricTile compact label="Streams" value={health?.streams != null ? String(health.streams) : "—"} />
            <DeskMetricTile compact label="Bars Processed" value={health?.bars_processed != null ? health.bars_processed.toLocaleString() : "—"} />
            <DeskMetricTile compact label="Maker Fill Rate" value={fillRate === null ? "—" : `${fillRate.toFixed(0)}%`} sub="filled vs missed post-only entries" />
            <DeskMetricTile compact label="Gate Passers" value={rows.length ? String(gatePassers.length) : "—"} valueClassName={gatePassers.length > 0 ? "desk-pnl-positive" : undefined} />
          </div>
        </DeskCard>

        {/* Promotion gate */}
        <DeskCard>
          <DeskSectionHeader
            title="Promotion Gate"
            actions={
              <DeskChip tone={gatePassers.length > 0 ? "success" : "default"} style={{ fontWeight: 700 }}>
                {rows.length ? `${gatePassers.length} of ${rows.length} streams pass` : "—"}
              </DeskChip>
            }
          />
          <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
            {GATE_RULES.map((g) => (
              <DeskChip key={g} tone="primary">{g}</DeskChip>
            ))}
          </div>
          <p className="desk-body-md" style={{ marginTop: 14, maxWidth: 720, color: "var(--desk-on-surface-variant)" }}>
            Pre-registered before the first live trade: all 100 strategies failed offline qualification (0/400), so
            with 800 streams a few days of trading is expected to produce lucky leaders by variance alone. Leaderboard
            position alone never justifies real money — only gate survivors earn a go-live discussion.
          </p>
        </DeskCard>

        {/* Leaderboard */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Strategy Leaderboard"
            actions={<span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>{filtered.length} of {rows.length} streams</span>}
          />

          <div className="desk-toolbar" style={{ marginBottom: 12 }}>
            <div className="desk-toolbar__actions">
              <DeskSearchField value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Search strategy…" />

              <select
                value={symbol}
                onChange={(e) => setSymbol(e.target.value)}
                style={{
                  minHeight: 44, borderRadius: "var(--desk-radius-input)", border: "1px solid var(--desk-outline)",
                  background: "var(--desk-surface)", color: "var(--desk-on-surface)", fontSize: "0.8125rem", padding: "0 12px",
                }}
              >
                {SYMBOLS.map((s) => (
                  <option key={s} value={s}>{s === "ALL" ? "All symbols" : s.replace("USDT", "")}</option>
                ))}
              </select>

              <div style={{ display: "inline-flex", gap: 4 }}>
                {(["ALL", "LONG", "SHORT"] as Side[]).map((s) => (
                  <button
                    key={s}
                    type="button"
                    onClick={() => setSide(s)}
                    style={segButtonStyle(side === s, s === "LONG" ? "success" : s === "SHORT" ? "error" : "primary")}
                  >
                    {s === "ALL" ? "Both" : s === "LONG" ? "Long" : "Short"}
                  </button>
                ))}
              </div>

              <select
                value={minN}
                onChange={(e) => setMinN(Number(e.target.value))}
                style={{
                  minHeight: 44, borderRadius: "var(--desk-radius-input)", border: "1px solid var(--desk-outline)",
                  background: "var(--desk-surface)", color: "var(--desk-on-surface)", fontSize: "0.8125rem", padding: "0 12px",
                }}
              >
                {MIN_N_OPTS.map((n) => (
                  <option key={n} value={n}>{n === 0 ? "Any trades" : `≥ ${n} trades`}</option>
                ))}
              </select>

              <button type="button" onClick={() => setGateOnly((v) => !v)} style={segButtonStyle(gateOnly, "success")}>
                Gate pass only
              </button>
              <button type="button" onClick={() => setProfitOnly((v) => !v)} style={segButtonStyle(profitOnly, "success")}>
                Profitable only
              </button>
            </div>
            {filtersActive && (
              <button type="button" onClick={resetFilters} className="desk-label-md" style={{ color: "var(--desk-primary)", cursor: "pointer", fontWeight: 700, background: "none", border: "none" }}>
                Reset
              </button>
            )}
          </div>

          <DeskDataTable
            columns={leaderboardColumns}
            rows={filtered.slice(0, 150)}
            getRowKey={(r) => `${r.strategy}|${r.symbol}`}
            minWidth={860}
            empty={
              <DeskEmptyState
                title="No streams match"
                subtitle={rows.length === 0 ? "No closed trades yet — the desk trades 24/7; check back soon." : "No streams match these filters."}
              />
            }
          />
        </DeskCard>

        {/* Recent trades */}
        <DeskCard padding="md">
          <DeskSectionHeader title="Recent Trades" actions={<span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>last {trades.length}</span>} />
          <DeskDataTable
            columns={tradeColumns}
            rows={[...trades].reverse()}
            getRowKey={(t, i) => `${t.time}-${t.strategy}-${i}`}
            minWidth={860}
            empty={<DeskEmptyState title="No closed trades" subtitle="No closed trades yet — the desk trades 24/7; check back soon." />}
          />
        </DeskCard>

        <p className="desk-label-md" style={{ textAlign: "center", fontWeight: 400, paddingBottom: 8 }}>
          Paper trading only · $100 notional per trade · maker post-only fill model with missed fills counted · state
          persists on the engine host
        </p>
      </main>
    </div>
  );
}
