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
import { fmtIST, fmtISTClock } from "@/lib/istTime";

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
  live_gross_usd: number;
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

type SortKey =
  | "n"
  | "wr_pct"
  | "pf"
  | "net_usd"
  | "max_dd_pct"
  | "missed"
  | "live_net_usd"
  | "live_fee_drag_pct"
  | "live_gross_usd"
  | "live_fees_usd"
  | "live_roi_pct"
  // Computed, not a field on the row — see leaderboardMetric.
  | "qual";
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
  /** On the desk's $100-account basis — the same as its closed P&L. */
  pnlAt1000: number;
};

/**
 * How close a stream is to the pre-registered go-live gate, as a percentage.
 *
 * The gate is >=200 trades AND PF >= 1.2 AND maxDD <= 25% AND net positive.
 * Progress is the mean of the four, each capped at 100% so a stream cannot
 * offset a missing sample with a flattering profit factor — exactly the trade
 * one lucky fill would otherwise make on its behalf.
 *
 * The trade-count term dominates on purpose. Across 27,784 streams a few days
 * produces lucky leaders by variance alone, and sample size is the only one of
 * the four that variance cannot fake.
 *
 * MODULE level, not inside the component. As a const inside it, the filter
 * useMemo above called it before its initialiser had run — a temporal dead
 * zone crash that TypeScript cannot see, because a closure referencing a
 * later const is legal until it executes. The page compiled, built, and threw
 * on load.
 */
function qualPct(r: LbRow): number {
    if (!r.n) return 0;
    const trades = Math.min(r.n / 200, 1);
    const pf = Math.min((r.pf || 0) / 1.2, 1);
    const dd = r.max_dd_pct <= 25 ? 1 : Math.max(0, 25 / r.max_dd_pct);
    const net = (r.live_net_usd ?? 0) > 0 ? 1 : 0;
    return ((trades + pf + dd + net) / 4) * 100;
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
  // Column-level filters. Separate from the chips above because these are
  // thresholds rather than toggles, and the number that matters differs by
  // question: fee drag for "can it pay for itself", qualified % for "is there
  // enough evidence", net for "did it earn anything on live terms".
  const [minWR, setMinWR] = useState<number>(0);
  const [minNet, setMinNet] = useState<number>(0);
  const [maxDrag, setMaxDrag] = useState<number>(0);
  const [minQual, setMinQual] = useState<number>(0);
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
  // The exact (strategy, symbol) STREAMS that route live, as "strategy|SYMBOL".
  //
  // Filtering the board on strategy NAME alone showed the cross product — 3
  // names across 92 symbols read as 276 "live-routed streams" when 6 route. A
  // board that overstates live exposure by 46x is worse than no board.
  const [liveStreams, setLiveStreams] = useState<string[]>([]);

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
        const body = (await po.json()) as {
          rows?: OpenPos[];
          live_strategies?: string[];
          live_streams?: string[];
        };
        setPositions(body.rows ?? []);
        setLiveRoster(body.live_strategies ?? []);
        setLiveStreams(body.live_streams ?? []);
      } else {
        setPositions([]);
      }
      setError("");
      setUpdatedAt(fmtISTClock(Date.now()));
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
        .filter((r) => r.wr_pct >= minWR)
        .filter((r) => (r.live_net_usd ?? 0) >= minNet)
        // 0 means "no cap" — a drag filter that excluded everything by default
        // would hide the whole board on first load.
        .filter((r) => maxDrag <= 0 || (r.live_fee_drag_pct ?? 0) <= maxDrag)
        .filter((r) => qualPct(r) >= minQual)
        .filter((r) =>
          side === "ALL" ? true : side === "LONG" ? r.strategy.endsWith("_Long") : r.strategy.endsWith("_Short"),
        )
        .filter((r) => q === "" || r.strategy.toLowerCase().includes(q) || r.symbol.toLowerCase().includes(q))
        .slice()
        .sort((a, b) => {
          // Via a metric function rather than a[sortKey], because two sortable
          // columns are DERIVED: Qualified % is computed from several fields,
          // and Capital is 100 + net. Indexing the row directly would have made
          // those columns silently unsortable — the arrows would move and the
          // order would not.
          const d = leaderboardMetric(a, sortKey) - leaderboardMetric(b, sortKey);
          return sortDir === "desc" ? -d : d;
        }),
    [rows, symbol, minN, gateOnly, profitOnly, side, q, minWR, minNet, maxDrag, minQual, sortKey, sortDir],
  );
  const filtersActive =
    symbol !== "ALL" || q !== "" || side !== "ALL" || minN !== 0 || gateOnly || profitOnly ||
    minWR !== 0 || minNet !== 0 || maxDrag !== 0 || minQual !== 0;
  const resetFilters = () => {
    setSymbol("ALL");
    setQuery("");
    setSide("ALL");
    setMinN(0);
    setGateOnly(false);
    setProfitOnly(false);
    setMinWR(0);
    setMinNet(0);
    setMaxDrag(0);
    setMinQual(0);
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
  const liveStreamSet = useMemo(() => new Set(liveStreams), [liveStreams]);



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
      // On the same $100-account basis as the desk's closed trades, so an open
      // and a closed position read on one scale.
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

  /** The number a given column sorts on, including the derived ones. */
  const leaderboardMetric = (r: LbRow, k: SortKey): number => {
    if (k === "qual") return qualPct(r);
    const v = r[k];
    return typeof v === "number" ? v : 0;
  };

  const leaderboardColumns: DeskColumn<LbRow>[] = [
    {
      id: "sl",
      header: "#",
      align: "right",
      // Position in the CURRENT ordering, taken from the render index rather
      // than stored on the row: sorting or filtering renumbers it, which is
      // what a serial number on a sortable table should do. A number that
      // survived a re-sort would be a rank pretending to be an identity.
      cell: (_r, i) => (
        <span style={{ color: "var(--desk-on-surface-variant)" }}>{i + 1}</span>
      ),
    },
    { id: "strategy", header: "Strategy", cell: (r) => <span className="desk-body-md" style={{ fontWeight: 600 }}>{r.strategy}</span> },
    { id: "symbol", header: "Symbol", cell: (r) => r.symbol.replace("USDT", "") },
    {
      id: "route",
      header: "Route",
      // Whether this exact (strategy, symbol) STREAM is on the live engine's
      // roster — not whether the strategy name appears somewhere on it.
      //
      // The distinction matters: ANTI_Recurrence_Quantification_Signal is live
      // on eight symbols and not on twenty others, so matching by name alone
      // would mark every row of it LIVE and tell an operator that real money
      // follows a stream it has never touched.
      cell: (r) =>
        liveStreamSet.has(`${r.strategy}|${r.symbol}`) ? (
          <DeskChip tone="warning" title="this exact stream is on the live engine's roster">
            LIVE
          </DeskChip>
        ) : (
          <span style={{ color: "var(--desk-on-surface-variant)" }}>NO</span>
        ),
    },
    {
      id: "capital",
      align: "right",
      header: <SortableHeader label="Capital" k="live_net_usd" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />,
      // What $100 on this stream alone would be worth, on live terms.
      //
      // The Net $ column further right is the same $100 basis but with MAKER fees —
      // the paper desk's own basis. It is not what this stream would have made
      // with money, and the two differ by more than a factor of ten. Showing
      // only the flattering one on the board people actually read is how a
      // strategy gets promoted on a number that was never about money.
      cell: (r) => {
        const cap = 100 + (r.live_net_usd ?? 0);
        return (
          <span className={pnlToneClass(r.live_net_usd ?? 0)} style={{ fontWeight: 700 }}>
            {r.n > 0 ? `$${cap.toFixed(2)}` : "—"}
          </span>
        );
      },
    },
    {
      id: "qual",
      align: "right",
      header: <SortableHeader label="Qualified %" k="qual" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />,
      // Progress toward the pre-registered gate, not a verdict. Dominated by
      // the trade count, which is the only gate term variance cannot fake.
      cell: (r) => {
        const q = qualPct(r);
        return (
          <span className={q >= 100 ? "desk-pnl-positive" : undefined}
            style={{ fontWeight: q >= 100 ? 700 : 400, opacity: q < 40 ? 0.7 : 1 }}>
            {`${q.toFixed(0)}%`}
          </span>
        );
      },
    },
    { id: "n", align: "right", header: <SortableHeader label="Trades" k="n" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />, cell: (r) => r.n },
    {
      id: "gross",
      align: "right",
      header: <SortableHeader label="Gross" k="live_gross_usd" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />,
      cell: (r) => <span className={pnlToneClass(r.live_gross_usd ?? 0)}>{fmtUSD(r.live_gross_usd ?? 0)}</span>,
    },
    {
      id: "fees",
      align: "right",
      header: <SortableHeader label="− Taker Fees" k="live_fees_usd" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />,
      cell: (r) => <span className="desk-pnl-negative">{fmtUSD(-(r.live_fees_usd ?? 0))}</span>,
    },
    {
      id: "livenet",
      align: "right",
      // Ordered Gross → − Fees → = Net so the row reads as the equation it is.
      header: <SortableHeader label="= Net on $100" k="live_net_usd" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />,
      cell: (r) => (
        <span className={pnlToneClass(r.live_net_usd ?? 0)} style={{ fontWeight: 700 }}>
          {fmtUSD(r.live_net_usd ?? 0)}
        </span>
      ),
    },
    {
      id: "roi",
      align: "right",
      header: <SortableHeader label="ROI %" k="live_roi_pct" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />,
      cell: (r) => (
        <span className={pnlToneClass(r.live_roi_pct ?? 0)}>
          {`${(r.live_roi_pct ?? 0) >= 0 ? "+" : ""}${(r.live_roi_pct ?? 0).toFixed(1)}%`}
        </span>
      ),
    },
    {
      id: "drag",
      align: "right",
      header: <SortableHeader label="Fee Drag" k="live_fee_drag_pct" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />,
      // Above 100% means the stream earns less than it costs to trade.
      cell: (r) => (
        <span className={(r.live_fee_drag_pct ?? 0) >= 100 ? "desk-pnl-negative" : undefined}>
          {`${(r.live_fee_drag_pct ?? 0).toFixed(0)}%`}
        </span>
      ),
    },
    {
      id: "verdict",
      header: "Qualified",
      // A question, not a verdict, and silent below 30 trades. A green chip at
      // n=1 is how a leaderboard starts lying — and across 27,784 streams the
      // right tail is long enough to produce hundreds of them.
      cell: (r) =>
        r.n < 30 ? (
          <DeskChip tone="default">too few</DeskChip>
        ) : (r.live_net_usd ?? 0) > 0 ? (
          <DeskChip tone="success" style={{ fontWeight: 700 }}>YES</DeskChip>
        ) : (
          <DeskChip tone="danger">NO</DeskChip>
        ),
    },
    { id: "wr", align: "right", header: <SortableHeader label="WR %" k="wr_pct" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />, cell: (r) => r.wr_pct.toFixed(1) },
    { id: "pf", align: "right", header: <SortableHeader label="PF" k="pf" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />, cell: (r) => r.pf.toFixed(2) },
    {
      id: "net", align: "right",
      header: <SortableHeader label="Net $ (paper)" k="net_usd" sortKey={sortKey} sortDir={sortDir} onSort={toggleSort} />,
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
    { id: "time", header: "Time (IST)", cell: (t) => fmtIST(t.time) },
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
            sub="each strategy on its own $100 account"
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

            {/* Column thresholds.
                Separate from the toggles above because these are numbers, and
                the one that matters depends on the question being asked: fee
                drag for "can it pay for itself", qualified % for "is there
                enough evidence yet", net for "did it earn on live terms". */}
            <div style={{ display: "flex", flexWrap: "wrap", gap: 10, alignItems: "center", marginTop: 10 }}>
              {[
                { label: "min WR %", value: minWR, set: setMinWR, opts: [0, 50, 60, 70, 80, 90] },
                { label: "min Net $100", value: minNet, set: setMinNet, opts: [0, 1, 2, 5, 10] },
                { label: "max Fee Drag %", value: maxDrag, set: setMaxDrag, opts: [0, 25, 50, 75, 100] },
                { label: "min Qualified %", value: minQual, set: setMinQual, opts: [0, 50, 75, 90, 100] },
              ].map((f) => (
                <label key={f.label} className="desk-label-md" style={{ display: "flex", alignItems: "center", gap: 6 }}>
                  <span style={{ color: "var(--desk-on-surface-variant)" }}>{f.label}</span>
                  <select
                    value={f.value}
                    onChange={(e) => f.set(Number(e.target.value))}
                    style={{
                      padding: "4px 8px",
                      borderRadius: 6,
                      border: "1px solid var(--desk-outline)",
                      background: f.value !== 0 ? "var(--desk-primary-container)" : "transparent",
                      color: "inherit",
                      fontWeight: f.value !== 0 ? 700 : 400,
                    }}
                  >
                    {f.opts.map((o) => (
                      <option key={o} value={o}>
                        {o === 0 ? "any" : o}
                      </option>
                    ))}
                  </select>
                </label>
              ))}
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
            minWidth={1560}
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
          Paper trading only · $100 account per strategy (3x notional) · SL floor -$10, TP floor $20/$30/$50 by profile
          (scalp/revert/runner, ~1:2 to 1:5 reward-to-risk) · maker post-only fill model with missed fills counted ·
          state persists on the engine host
        </p>
      </main>
    </div>
  );
}
