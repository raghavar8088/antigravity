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

type Stats = {
  trades: number;
  wins: number;
  missed_fills: number;
  open_positions: number;
  pending_orders: number;
  net_pnl_usd_at_1000_notional: number;
  trades_per_symbol: Record<string, number>;
};
type Health = { ok: boolean; uptime_min: number; bars_processed: number; strategies: number; streams: number };
type LbRow = {
  strategy: string; symbol: string; n: number; wr_pct: number; pf: number;
  net_usd: number; max_dd_pct: number; missed: number; gate_pass: boolean;
  /** The same record on the LIVE desk's terms: $100 account, taker fees. */
  live_net_usd: number;
  live_roi_pct: number;
  live_fees_usd: number;
  live_fee_drag_pct: number;
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

/**
 * One open PAPER position on the desk.
 *
 * `live` marks the streams that are also on the real-money allow-list — the
 * only ones whose paper position predicts a live order. The desk runs 2,416
 * streams and holds a few hundred positions at once, so showing all of them
 * buries the handful that carry money.
 */
type OpenPos = {
  strategy: string;
  symbol: string;
  dir: string;
  entry: number;
  sl: number;
  tp: number;
  profile: string;
  openedAt: string;
  heldMin: number;
  live: boolean;
  /** Last closed 1m bar. 0 when the symbol has no bar yet. */
  mark: number;
  pnlPct: number;
  /** Stated at $1,000 notional — the same basis as the desk's closed P&L. */
  pnlAt1000: number;
};

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
  const [showAllTrades, setShowAllTrades] = useState<boolean>(false);
  const [positions, setPositions] = useState<OpenPos[]>([]);
  // Default to the live-enabled streams only: this section exists to answer
  // "what is exposed right now", and the other ~330 paper positions are noise
  // against that question. Toggleable rather than hard-filtered — the data is
  // still there for anyone who wants it.
  const [liveOnly, setLiveOnly] = useState<boolean>(true);
  // The live allow-list itself, so the leaderboard covers strategies that are
  // currently FLAT — which is most of them, most of the time.
  const [liveRoster, setLiveRoster] = useState<string[]>([]);

  const refresh = useCallback(async () => {
    try {
      const [h, s, lb, tr, po] = await Promise.all([
        fetch("/api/scalp/scalp/health", { cache: "no-store" }),
        fetch("/api/scalp/scalp/stats", { cache: "no-store" }),
        fetch("/api/scalp/scalp/leaderboard", { cache: "no-store" }),
        fetch(`/api/scalp/scalp/trades?n=${showAllTrades ? 5000 : 50}`, { cache: "no-store" }),
        fetch("/api/scalp/scalp/positions", { cache: "no-store" }),
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
      // Positions are additive: an older engine without this endpoint should
      // leave the rest of the page working rather than blanking it.
      if (po.ok) {
        const body = (await po.json()) as { rows?: OpenPos[]; live_strategies?: string[] };
        setPositions(body.rows ?? []);
        setLiveRoster(body.live_strategies ?? []);
      } else {
        setPositions([]);
      }
      setError("");
      setUpdatedAt(new Date().toLocaleTimeString());
    } catch {
      setError("desk unreachable");
    } finally {
      setLoading(false);
    }
  }, [showAllTrades]);

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

  const livePositions = useMemo(() => positions.filter((p) => p.live), [positions]);
  const shownPositions = liveOnly ? livePositions : positions;

  const openPnl = useMemo(
    () => shownPositions.reduce((a, p) => a + (p.mark > 0 ? p.pnlAt1000 : 0), 0),
    [shownPositions],
  );

  /**
   * Leaderboard restricted to the streams that can reach the venue.
   *
   * The full leaderboard ranks 2,416 streams, and with that many a few days of
   * trading produces lucky leaders by variance alone — which is why the gate
   * exists. This table answers a narrower and more useful question: how are the
   * strategies that are ACTUALLY routing real orders performing on paper?
   *
   * Derived from the same rows, so it cannot disagree with the table below it.
   */
  const liveStrategyNames = useMemo(() => new Set(liveRoster), [liveRoster]);
  const liveRows = useMemo(
    () =>
      rows
        .filter((r) => liveStrategyNames.has(r.strategy))
        // Ranked by the $100 taker result, not the paper one. Sorting by paper
        // net would put the same strategies on top that the paper board already
        // flatters — the habit this table exists to break.
        .sort((a, b) => (b.live_net_usd ?? 0) - (a.live_net_usd ?? 0)),
    [rows, liveStrategyNames],
  );

  /**
   * Columns for the live-routed board.
   *
   * Deliberately NOT the paper leaderboard's columns. That board reports $1,000
   * notional with maker fees; this reports the $100 account the desk actually
   * runs, with taker fees both legs. Both are shown — the gap between them is
   * what a promotion decision turns on.
   */
  /**
   * ONE $100 portfolio spread across the live-routed streams.
   *
   * The leaderboard column gives every stream its own $100 account, which is
   * the right way to COMPARE strategies but is not a portfolio: 136 streams on
   * $100 each is $13,600 of capital. Summing that column would report the
   * return on thirteen thousand dollars as if it were the return on one
   * hundred, and it would be the most flattering number on the page.
   *
   * An equal-weight portfolio gives each stream $100/N. P&L scales linearly
   * with size, so each stream contributes its own figure divided by N — which
   * makes the portfolio return the MEAN of the per-strategy returns, not their
   * sum. That single division is the whole difference between a real number and
   * a fantasy.
   */
  const portfolio = useMemo(() => {
    const traded = liveRows.filter((r) => r.n > 0);
    const n = traded.length;
    if (n === 0) {
      return { n: 0, net: 0, fees: 0, roi: 0, value: 100, perStrategy: 0, trades: 0, winners: 0 };
    }
    const net = traded.reduce((a, r) => a + (r.live_net_usd ?? 0), 0) / n;
    const fees = traded.reduce((a, r) => a + (r.live_fees_usd ?? 0), 0) / n;
    return {
      n,
      net,
      fees,
      roi: net,
      value: 100 + net,
      perStrategy: 100 / n,
      trades: traded.reduce((a, r) => a + r.n, 0),
      winners: traded.filter((r) => (r.live_net_usd ?? 0) > 0).length,
    };
  }, [liveRows]);

  /**
   * How close a stream is to the pre-registered go-live gate, as a percentage.
   *
   * The gate is >=200 trades AND PF >= 1.2 AND maxDD <= 25% AND net positive.
   * Progress is the mean of the four, each capped at 100% so a strategy cannot
   * offset a missing sample with a flattering profit factor — which is exactly
   * the trade a single lucky trade would otherwise make for it.
   *
   * The trade-count term dominates on purpose. With 2,416 streams running, a
   * few days produces lucky leaders by variance alone, and sample size is the
   * only one of the four that variance cannot fake.
   *
   * The gate's fourth real condition - net positive in BOTH calendar halves of
   * the live window - is not computable from a leaderboard row, so this is
   * progress toward the gate, not a verdict on it. gate_pass from the engine
   * remains the verdict.
   */
  const qualPct = (r: LbRow): number => {
    if (!r.n) return 0;
    const trades = Math.min(r.n / 200, 1);
    const pf = Math.min((r.pf || 0) / 1.2, 1);
    const dd = r.max_dd_pct <= 25 ? 1 : Math.max(0, 25 / r.max_dd_pct);
    const net = (r.live_net_usd ?? 0) > 0 ? 1 : 0;
    return ((trades + pf + dd + net) / 4) * 100;
  };

  const liveLbColumns: DeskColumn<LbRow>[] = [
    { id: "strategy", header: "Strategy", cell: (r) => <span className="desk-body-md" style={{ fontWeight: 600 }}>{r.strategy}</span> },
    { id: "symbol", header: "Symbol", cell: (r) => r.symbol.replace("USDT", "") },
    {
      id: "capital",
      align: "right",
      header: "Capital",
      // What $100 given to THIS strategy alone is worth now, on live terms.
      // Stated as a balance rather than a P&L because a balance is the thing an
      // operator actually checks, and $95.40 is harder to misread than -$4.60.
      cell: (r) => {
        const cap = 100 + (r.live_net_usd ?? 0);
        return (
          <span className={pnlToneClass(r.live_net_usd ?? 0)} style={{ fontWeight: 700 }}>
            {`$${cap.toFixed(2)}`}
          </span>
        );
      },
    },
    {
      id: "qual",
      align: "right",
      header: "Qualified %",
      // Progress toward the go-live gate, not a verdict. Dominated by the trade
      // count, which is the only gate term variance cannot fake.
      cell: (r) => {
        const q = qualPct(r);
        return (
          <span
            className={q >= 100 ? "desk-pnl-positive" : undefined}
            style={{ fontWeight: q >= 100 ? 700 : 400, opacity: q < 40 ? 0.7 : 1 }}
          >
            {`${q.toFixed(0)}%`}
          </span>
        );
      },
    },
    { id: "n", align: "right", header: "Trades", cell: (r) => r.n },
    { id: "wr", align: "right", header: "WR %", cell: (r) => r.wr_pct.toFixed(1) },
    {
      id: "livenet",
      align: "right",
      header: "Net on $100",
      cell: (r) => (
        <span className={pnlToneClass(r.live_net_usd ?? 0)} style={{ fontWeight: 700 }}>
          {fmtUSD(r.live_net_usd ?? 0)}
        </span>
      ),
    },
    {
      id: "roi",
      align: "right",
      header: "ROI %",
      cell: (r) => (
        <span className={pnlToneClass(r.live_roi_pct ?? 0)}>
          {`${(r.live_roi_pct ?? 0) >= 0 ? "+" : ""}${(r.live_roi_pct ?? 0).toFixed(1)}%`}
        </span>
      ),
    },
    {
      id: "fees",
      align: "right",
      header: "Taker Fees",
      cell: (r) => <span className="desk-pnl-negative">{fmtUSD(-(r.live_fees_usd ?? 0))}</span>,
    },
    {
      id: "drag",
      align: "right",
      header: "Fee Drag",
      // Above 100% means it earns less than it costs to trade.
      cell: (r) => (
        <span className={(r.live_fee_drag_pct ?? 0) >= 100 ? "desk-pnl-negative" : undefined}>
          {`${(r.live_fee_drag_pct ?? 0).toFixed(0)}%`}
        </span>
      ),
    },
    {
      id: "paper",
      align: "right",
      header: "Paper $1k",
      cell: (r) => (
        <span className={pnlToneClass(r.net_usd)} style={{ opacity: 0.6 }}>{fmtUSD(r.net_usd)}</span>
      ),
    },
    {
      id: "verdict",
      header: "Qualified",
      cell: (r) =>
        (r.live_net_usd ?? 0) > 0 ? (
          <DeskChip tone="success" style={{ fontWeight: 700 }}>YES</DeskChip>
        ) : (
          <DeskChip tone="danger">NO</DeskChip>
        ),
    },
  ];

  const positionColumns: DeskColumn<OpenPos>[] = [
    {
      id: "strategy",
      header: "Strategy",
      cell: (p) => (
        <span className="desk-body-md" style={{ fontWeight: 600 }}>
          {p.strategy}
        </span>
      ),
    },
    { id: "symbol", header: "Symbol", cell: (p) => p.symbol.replace("USDT", "") },
    {
      id: "dir",
      header: "Side",
      cell: (p) => (
        <DeskChip tone={p.dir?.toUpperCase() === "LONG" ? "success" : "danger"}>{(p.dir || "?").toUpperCase()}</DeskChip>
      ),
    },
    { id: "entry", align: "right", header: "Entry", cell: (p) => fmtPrice(p.entry) },
    { id: "sl", align: "right", header: "Stop", cell: (p) => fmtPrice(p.sl) },
    { id: "tp", align: "right", header: "Target", cell: (p) => fmtPrice(p.tp) },
    {
      id: "mark",
      align: "right",
      header: "Mark",
      cell: (p) => (p.mark > 0 ? fmtPrice(p.mark) : "—"),
    },
    {
      id: "upnl",
      align: "right",
      header: "Unreal. P&L",
      // At $1,000 notional, matching the desk's closed-trade basis so an open
      // and a closed position can be read on one scale.
      cell: (p) =>
        p.mark > 0 ? (
          <span className={pnlToneClass(p.pnlAt1000)} style={{ fontWeight: 600 }}>
            {fmtUSD(p.pnlAt1000)}
          </span>
        ) : (
          "—"
        ),
    },
    {
      id: "upnlpct",
      align: "right",
      header: "%",
      cell: (p) =>
        p.mark > 0 ? (
          <span className={pnlToneClass(p.pnlPct)}>{`${p.pnlPct >= 0 ? "+" : ""}${p.pnlPct.toFixed(2)}%`}</span>
        ) : (
          "—"
        ),
    },
    { id: "profile", header: "Profile", cell: (p) => p.profile || "—" },
    {
      id: "held",
      align: "right",
      header: "Held",
      // Held time matters more than it looks: 91% of this desk's exits are the
      // time stop, so age is the best single predictor of how a position ends.
      cell: (p) => `${p.heldMin}m`,
    },
    {
      id: "live",
      header: "Route",
      cell: (p) =>
        p.live ? (
          <DeskChip tone="primary" style={{ fontWeight: 700 }}>
            LIVE
          </DeskChip>
        ) : (
          <span style={{ color: "var(--desk-on-surface-variant)" }}>paper</span>
        ),
    },
  ];

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
            value={stats ? fmtUSD(stats.net_pnl_usd_at_1000_notional) : "—"}
            valueClassName={stats ? pnlToneClass(stats.net_pnl_usd_at_1000_notional) : undefined}
            sub="$1,000 notional per trade"
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

        {/* Open positions — live-routed streams first */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Open Positions"
            actions={
              <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                  {livePositions.length} live-routed · {positions.length} total
                </span>
                <span
                  className={`desk-mono desk-label-md ${pnlToneClass(openPnl)}`}
                  style={{ fontWeight: 700 }}
                >
                  open {fmtUSD(openPnl)}
                </span>
                <button
                  type="button"
                  onClick={() => setLiveOnly((v) => !v)}
                  className="desk-label-md"
                  style={{
                    cursor: "pointer",
                    padding: "4px 10px",
                    borderRadius: 6,
                    border: "1px solid var(--desk-outline)",
                    background: liveOnly ? "var(--desk-primary)" : "transparent",
                    color: liveOnly ? "var(--desk-on-primary)" : "var(--desk-on-surface-variant)",
                    fontWeight: 600,
                  }}
                >
                  {liveOnly ? "Live strategies only" : "All streams"}
                </button>
              </div>
            }
          />
          {shownPositions.length === 0 ? (
            <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "8px 0 0" }}>
              {liveOnly
                ? "None of the live-routed strategies hold a position right now."
                : "No open positions."}
            </p>
          ) : (
            <DeskDataTable
              columns={positionColumns}
              rows={shownPositions}
              getRowKey={(p) => `${p.strategy}|${p.symbol}`}
              minWidth={1120}
            />
          )}
          <p className="desk-body-md" style={{ marginTop: 12, maxWidth: 760, color: "var(--desk-on-surface-variant)" }}>
            These are PAPER positions. A row marked LIVE means that stream is also on the real-money allow-list, so a
            fill here places a real order too — it does not mean this particular position is funded. Live exposure is on
            the Live Engine page.
          </p>
        </DeskCard>

        {/* Live-routed leaderboard — the streams that spend real money */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Live Strategy Leaderboard"
            subtitle="Each strategy on its OWN $100 account at 3x notional, with Delta taker fees both legs — the terms the live desk actually trades on."
            actions={
              <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                {liveRows.length} live-routed streams
              </span>
            }
          />
          {/* One $100 portfolio, equal-weight across the traded streams. */}
          <div className="desk-metrics-row" style={{ marginBottom: 16 }}>
            <DeskMetricTile
              label="Portfolio Value"
              value={`$${portfolio.value.toFixed(2)}`}
              valueClassName={pnlToneClass(portfolio.net)}
              sub={`started at $100.00`}
              highlight
            />
            <DeskMetricTile
              compact
              label="Net P&L"
              value={fmtUSD(portfolio.net)}
              valueClassName={pnlToneClass(portfolio.net)}
              sub={`${portfolio.roi >= 0 ? "+" : ""}${portfolio.roi.toFixed(2)}% return`}
            />
            <DeskMetricTile
              compact
              label="Allocation"
              value={portfolio.n ? `$${portfolio.perStrategy.toFixed(2)}` : "—"}
              sub={`each, across ${portfolio.n} streams`}
            />
            <DeskMetricTile
              compact
              label="Taker Fees Paid"
              value={fmtUSD(-portfolio.fees)}
              valueClassName="desk-pnl-negative"
              sub={`over ${portfolio.trades} trades`}
            />
            <DeskMetricTile
              compact
              label="Streams Positive"
              value={portfolio.n ? `${portfolio.winners} / ${portfolio.n}` : "—"}
              valueClassName={portfolio.winners > portfolio.n / 2 ? "desk-pnl-positive" : undefined}
              sub="net > 0 on live terms"
            />
          </div>

          {liveRows.length === 0 ? (
            <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "8px 0 0" }}>
              No closed trades yet on the live-routed streams.
            </p>
          ) : (
            <DeskDataTable
              columns={liveLbColumns}
              rows={liveRows}
              getRowKey={(r) => `live-${r.strategy}|${r.symbol}`}
              minWidth={860}
            />
          )}
          <p className="desk-body-md" style={{ marginTop: 12, maxWidth: 780, color: "var(--desk-on-surface-variant)" }}>
            The tiles above are ONE $100 portfolio split equally across the traded streams — each gets
            ${portfolio.perStrategy.toFixed(2)}, so the portfolio return is the AVERAGE of the per-strategy returns,
            not their sum. Summing the column below would be the return on ${(portfolio.n * 100).toLocaleString()} of
            capital reported as if it were $100.
            <br />
            <br />
            <strong>Capital</strong> is what $100 given to that strategy <em>alone</em> would be worth now — not a
            slice of the portfolio above. <strong>Qualified %</strong> is progress toward the pre-registered gate
            (≥200 trades, PF ≥ 1.2, max DD ≤ 25%, net positive), averaged across the four and capped so a lucky profit
            factor cannot substitute for a missing sample. It is progress, not a verdict: the gate&apos;s
            both-halves-positive condition is not computable from a single row.
            <br />
            <br />
            <strong>Net on $100</strong> restates each strategy&apos;s record on live terms: a $100 account, 3x
            notional, Delta&apos;s taker fee of 0.059% per side. <strong>Paper $1k</strong> is the old headline —
            $1,000 notional with maker fees — shown alongside because the gap between the two is the whole point. A
            taker round trip costs 0.118% of notional, which is larger than the average move most of these strategies
            target, so a high paper rank does not survive the restatement. Qualified = positive on the $100 column.
            That, not paper rank, is what earns a place in the Live Engine.
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
            rows={showAllTrades ? filtered : filtered.slice(0, 150)}
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
          <DeskSectionHeader
            title="Trade History"
            actions={
              <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                  {showAllTrades
                    ? `all ${filtered.length}`
                    : `last ${Math.min(150, filtered.length)} of ${filtered.length}`}
                </span>
                {filtered.length > 150 && (
                  <DeskButton variant="text" onClick={() => setShowAllTrades((v) => !v)}>
                    {showAllTrades ? "Show less" : "View all trade history"}
                  </DeskButton>
                )}
              </div>
            }
          />
          <DeskDataTable
            columns={tradeColumns}
            rows={[...trades].reverse()}
            getRowKey={(t, i) => `${t.time}-${t.strategy}-${i}`}
            minWidth={860}
            empty={<DeskEmptyState title="No closed trades" subtitle="No closed trades yet — the desk trades 24/7; check back soon." />}
          />
        </DeskCard>

        <DeskAdminControls
          resetPath="/api/scalp/scalp/reset"
          clearPath="/api/scalp/scalp/clear-trades"
          capitalLabel="Notional per trade (USD)"
          capitalPlaceholder="1000"
          onDone={() => void refresh()}
        />

        <p className="desk-label-md" style={{ textAlign: "center", fontWeight: 400, paddingBottom: 8 }}>
          Paper trading only · $1,000 notional per trade · SL floor -$10, TP floor $20/$30/$50 by profile
          (scalp/revert/runner, ~1:2 to 1:5 reward-to-risk) · maker post-only fill model with missed fills counted ·
          state persists on the engine host
        </p>
      </main>
    </div>
  );
}
