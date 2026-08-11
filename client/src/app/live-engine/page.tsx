"use client";

/**
 * Live Engine — REAL-MONEY option-buying control plane.
 *
 * Reads and mutates only through the session-gated /api/live-engine proxy. This
 * module trades real capital on Delta BTC options (long premium only), capped at
 * a $100 server-enforced ceiling. It ships DISARMED; the Delta Engine toggle
 * turns live trading on immediately. Built on the shared desk primitives so it matches
 * the Options Buying desk, but carries persistent, unmissable REAL MONEY
 * differentiation so it can never be mistaken for a paper desk.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  DeskBanner,
  DeskButton,
  DeskCard,
  DeskChip,
  DeskDataTable,
  DeskLinearProgress,
  DeskMetricTile,
  DeskSectionHeader,
  DeskSwitch,
  type DeskColumn,
} from "@/components/desk/ui";
import { fmtIST, fmtISTSeconds, fmtISTDayLabel } from "@/lib/istTime";

const ARM_PHRASE = "ARM LIVE $100";
const CEILING = 100;

type LiveState = {
  state: "ARMED" | "DISARMED";
  armed: boolean;
  armedBy?: string;
  armedAt?: string;
  lastDisarmReason?: string;
  lastDisarmAt?: string;
  consecutiveRejects: number;
  maxConsecutiveRejects: number;
  ceilingUsd: number;
  configured: boolean;
  killSwitchActive: boolean;
  killSwitchReason?: string;
  killSwitchControllable?: boolean;
  /** Master switch for real-money OPTION orders. Configuration-only. */
  optionsTradingEnabled?: boolean;
};

type Account = {
  equityUsd: number;
  tradableUsd: number;
  ceilingUsd: number;
  availableUsd: number;
  marginUsedUsd: number;
  openRiskUsd: number;
  realizedTodayUsd: number;
  /** Wallet balance ROI is measured from; 0 until a baseline is captured. */
  inceptionEquityUsd?: number;
  inceptionAt?: string;
  roiUsd?: number;
  roiPct?: number;
  distanceToBreakerPct: number;
  source: string;
  asOf: string;
  stale: boolean;
};

type Position = {
  symbol: string;
  side: string;
  size: number;
  entryPrice: number;
  markPrice: number;
  unrealizedPnl: number;
  marginUsd: number;
  takeProfitPrice: number;
  stopLossPrice: number;
  takeProfitUsd: number;
  stopLossUsd: number;
  liquidationPrice: string;
  strategy: string;
};

type ClosedPosition = {
  id: string;
  strategy: string;
  optionType: string;
  symbol: string;
  contracts: number;
  entryPrice: number;
  exitPrice: number;
  /** NET of both Delta fee legs — what the account actually kept. */
  realizedPnl: number;
  /** Pre-fee result, shown alongside so the cost of trading is visible. */
  grossPnl?: number;
  feesUsd?: number;
  exitReason: string;
  openedAt: string;
  closedAt?: string;
};

type DailyPnl = {
  date: string;
  capitalUsd: number;
  /** NET of fees. grossPnlUsd is the pre-fee figure this used to report. */
  pnlUsd: number;
  grossPnlUsd?: number;
  feesUsd?: number;
  /** Fees as a share of the premium deployed that day. */
  feeDragPct?: number;
  roiPct: number;
  trades: number;
  wins: number;
  winRatePct: number;
};

type Order = {
  id: string;
  strategy: string;
  optionType: string;
  strike: number;
  symbol: string;
  contracts: number;
  side: string;
  premiumUsd: number;
  fillPrice: number;
  status: string;
  deltaOrderId: string;
  openedAt: string;
  rejectReason?: string;
};

type Gate = { name: string; pass: boolean; requirement: string; actual: string };
type Eligibility = { strategy: string; live: boolean; reason: string; gates: Gate[]; allowed: boolean };

type Recon = {
  matched: boolean;
  enginePositions: number;
  deltaPositions: number;
  mismatches?: string[];
  asOf: string;
  error?: string;
};

type AuditEntry = { at: string; actor: string; action: string; reason?: string; detail?: string };

/**
 * One row of the live strategy leaderboard.
 *
 * Built from CLOSED live positions — real fills, real fees — not from the paper
 * desks. The paper leaderboards rank a strategy on model premiums; this ranks it
 * on what the account actually kept, which is the only number that justifies
 * leaving it enabled.
 */
/**
 * A live position on the shared Delta wallet, from either desk.
 *
 * Live Positions previously listed only this engine's option positions. That was
 * correct for the engine and WRONG for the page: the scalp bridge's perpetuals
 * are real money on the same wallet, and the page reported "0 open" while Delta
 * showed two. Hiding another desk's position is a worse failure than
 * misattributing it — the first is invisible, the second at least prompts a
 * question.
 */
type UnifiedPosition = {
  desk: "options" | "scalp";
  symbol: string;
  side: string;
  size: number;
  entryPrice: number;
  markPrice: number;
  unrealizedPnl: number;
  marginUsd: number;
  stopPrice?: number;
  targetPrice?: number;
  /** What this position pays at target / costs at stop, NET of the round trip. */
  ifTargetUsd?: number;
  ifStopUsd?: number;
  /** Position size in USD — what this position has deployed. */
  notionalUsd?: number;
  strategy: string;
};

/** One real perpetual trade from the scalp desk's live arm. */
type PerpTrade = {
  strategy: string;
  symbol: string;
  side: string;
  contracts: number;
  entryPrice: number;
  markPrice?: number;
  unrealizedPnl?: number;
  stopPrice?: number;
  targetPrice?: number;
  ifTargetUsd?: number;
  ifStopUsd?: number;
  exitPrice?: number;
  openedAt?: string;
  closedAt?: string;
  realisedPnl?: number;
  /** Round-trip taker fees, booked by the bridge. */
  feesUsd?: number;
  /** Position size in USD — the denominator that makes edge comparable. */
  notionalUsd?: number;
  /** Realised risk / planned risk on a stop-out. 1.00 = closed on the stop. */
  stopOvershoot?: number;
  exitReason?: string;
  status: string;
};

type PerpStats = {
  armed: boolean;
  equityUsd: number;
  riskPerTradeUsd: number;
  strategies: string[];
  /** Switched off by the owner — engine truth, not browser state. */
  disabledStrategies?: string[];
  /** The roster at its real granularity: (strategy, symbol) streams. */
  liveStreams?: {
    strategy: string;
    symbol: string;
    enabled: boolean;
    gridRefusals?: number;
    lastStopTicks?: number;
  }[];
  openPositions: PerpTrade[];
  submitted: number;
  rejected: number;
  closed: number;
  realisedPnlUsd: number;
};

type LeaderRow = {
  /** Which live desk this strategy trades on. Both spend the same wallet. */
  desk: "options" | "scalp";
  strategy: string;
  /** The instrument this stream trades. One strategy runs several. */
  symbol: string;
  trades: number;
  wins: number;
  winRatePct: number;
  grossUsd: number;
  feesUsd: number;
  netUsd: number;
  /** Fees as a share of gross profit — the figure that decided the options desk. */
  feeDragPct: number;
  /** Mean stop overshoot across this strategy's stop-outs, and its sample size. */
  stopOvershoot?: number;
  stopOuts: number;
  /** Whether this strategy may open new live positions. */
  enabled: boolean;
  /** Signals refused pre-trade because the symbol's tick grid is too coarse. */
  gridRefusals: number;
  lastStopTicks: number;
  /** Total notional traded, summed across fills. */
  notionalUsd: number;
  /** Sum of each fill's own ROI %, for the simple (equal-weighted) average. */
  roiPctSum: number;
  /** Fills that had a usable notional, so the average divides by the right n. */
  roiSamples: number;
  allowed: boolean;
  live: boolean;
  reason: string;
};

/**
 * USD → INR rate, or null until it loads.
 *
 * Module-level so all 31 money call sites convert without threading a prop
 * through every column definition. Set once per refresh from /api/fx/usd-inr,
 * alongside the state updates that trigger the re-render.
 */
let usdInrRate: number | null = null;

function setUsdInrRate(rate: number | null): void {
  usdInrRate = rate && Number.isFinite(rate) && rate > 0 ? rate : null;
}

/**
 * Format a money amount in rupees.
 *
 * Falls back to DOLLARS with a $ sign when the rate has not loaded, rather than
 * printing a dollar figure under a ₹ sign. A wrong currency symbol on a real
 * P&L is a 95x misstatement that looks completely ordinary — the symbol always
 * tells the truth about which currency the number actually is.
 */
/**
 * Fills a stream needs before its record is ranked on merit rather than filed
 * as unproven.
 *
 * Ten is not enough to conclude anything — the promotion gate asks for 200 —
 * but it is enough to separate "this has a record" from "this fired once and
 * won". The board's job here is ordering, not certification, and the Gate
 * column still says what the evidence is worth.
 */
/** Columns the Strategy Leaderboard can be sorted by. */
type LeaderSortKey =
  | "avgRoi"
  | "profitPct"
  | "trades"
  | "winRatePct"
  | "feesUsd"
  | "netUsd"
  | "stopOvershoot"
  | "feeDragPct";

/**
 * Sortable column header, same behaviour and glyphs as the Options Selling
 * desk so the two boards are operated the same way.
 */
function LeaderSortHeader({
  label, k, sortKey, sortDir, onSort,
}: {
  label: string;
  k: LeaderSortKey;
  sortKey: LeaderSortKey;
  sortDir: "asc" | "desc";
  onSort: (k: LeaderSortKey) => void;
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

/**
 * Delta's BTC option contract size. One contract is 0.001 BTC, so a premium
 * quoted per BTC must be scaled by it — the same multiplier whose absence once
 * reported a $0.05 result as $50.
 */
const OPTION_CONTRACT_SIZE_BTC = 0.001;

const LEADER_MIN_SAMPLE = 10;

/** Rows shown before the board is expanded. */
const LEADER_PREVIEW_ROWS = 25;

/**
 * Capital a stream actually deploys: the average size of one of its positions.
 *
 * A stream holds one position at a time — the per-symbol cap enforces it — so
 * the money committed to it is one position's notional, not the sum of every
 * position it has ever opened. Summing would treat a stream that recycled $2
 * eleven times as though it had been given $22.
 */
function capitalDeployed(r: { notionalUsd: number; trades: number }): number {
  return r.trades > 0 ? r.notionalUsd / r.trades : 0;
}

/**
 * Total net profit as a percentage of the capital the stream deploys.
 *
 * The "+104%" / "-10%" reading: this stream has roughly doubled the money it
 * commits, or lost a tenth of it. Return on the capital at work, which is the
 * question "is this strategy profitable" actually asks.
 *
 * Deliberately NOT net over the desk's whole balance. All 173 streams share one
 * $10, so dividing by it measures how big a slice of the desk a stream happens
 * to represent rather than how well it uses what it is given — and it would
 * rank a stream that fires constantly above a better one that fires rarely.
 */
function profitPctOfCapital(r: { netUsd: number; notionalUsd: number; trades: number }): number {
  const cap = capitalDeployed(r);
  return cap > 0 ? (r.netUsd / cap) * 100 : 0;
}

/**
 * The simple average of each fill's own ROI — the equal-weighted mean.
 *
 * Five trades at +5% and five at -2% average to +1.5%, regardless of how large
 * any one of them was. That is a different number from net over total notional,
 * which is weighted by position size and can be carried by a single big trade.
 *
 * This is the "does a typical trade from this stream make money" figure, and it
 * is the one that says whether the strategy itself is profitable rather than
 * whether it happened to size well.
 */
function avgRoiPerTrade(r: { roiPctSum: number; roiSamples: number }): number {
  return r.roiSamples > 0 ? r.roiPctSum / r.roiSamples : 0;
}

/**
 * Net as a share of everything traded — profit per unit of turnover, and the
 * only figure comparable against the fee.
 *
 * Kept because it answers a different question from return on capital. A stream
 * that recycles its capital often can show a large return while earning less on
 * each trade than the trade costs; this is the number that catches that.
 */
function edgePct(r: { netUsd: number; notionalUsd: number }): number {
  return r.notionalUsd > 0 ? (r.netUsd / r.notionalUsd) * 100 : 0;
}

/** Delta's round-trip taker cost, for comparison against edge. */
const ROUND_TRIP_FEE_PCT = 0.118;

function fmtMoney(v: number | undefined): string {
  if (v === undefined || Number.isNaN(v)) return "—";
  if (usdInrRate === null) {
    const abs = Math.abs(v);
    const dp = abs > 0 && abs < 1 ? 4 : 2;
    return `${v < 0 ? "-" : ""}$${abs.toFixed(dp)}`;
  }
  const inr = v * usdInrRate;
  const abs = Math.abs(inr);
  // Fixed-size mode trades one contract, so a stop-out is a fraction of a
  // rupee. 2dp would round every result on the desk to ₹0.00 and report a
  // desk that is trading as a desk that is doing nothing.
  const dp = abs > 0 && abs < 1 ? 4 : 2;
  return `${inr < 0 ? "-" : ""}₹${abs.toLocaleString("en-IN", {
    minimumFractionDigits: dp,
    maximumFractionDigits: dp,
  })}`;
}
/**
 * Render a price at a sensible precision.
 *
 * These come off Delta as float64 and print as 0.17512000000000003 — 17
 * significant figures of representation noise on an instrument whose tick is
 * 0.0001. Cheap perpetuals need more decimals than BTC, so the precision scales
 * with magnitude rather than being fixed.
 */
function fmtPrice(v: number | undefined): string {
  if (v === undefined || Number.isNaN(v) || v === 0) return "—";
  const abs = Math.abs(v);
  const dp = abs >= 1000 ? 1 : abs >= 1 ? 3 : 5;
  return v.toFixed(dp);
}

function pnlTone(v: number): string {
  return v > 0 ? "desk-pnl-positive" : v < 0 ? "desk-pnl-negative" : "desk-pnl-neutral";
}

/**
 * Payload readers that do not lie about shape.
 *
 * A control plane may answer a panel it has no source for with
 * `{items: [], notApplicable: true, reason}` instead of a list. This page used
 * to cast every response straight to its type — `(await cp.json()) as
 * ClosedPosition[]` — and a cast is a compile-time claim about a runtime
 * payload, not a check. When the demo control plane started answering that way,
 * `closed.length` read undefined and the table's `.slice()` threw, blanking the
 * whole page. It survives here because these readers verify before they hand
 * anything to a setter.
 *
 * Kept on the LIVE page rather than patched into the generated demo one:
 * scripts/clone_live_demo_page.py rebuilds the demo page from this file, so a
 * fix applied there is deleted by the next regeneration. This is the only place
 * it stays fixed.
 */
type NotApplicable = { notApplicable?: boolean; reason?: string };

function naReasonOf(payload: unknown): string | null {
  if (payload && typeof payload === "object" && !Array.isArray(payload)) {
    const p = payload as NotApplicable;
    if (p.notApplicable) return p.reason ?? "not available on this venue";
  }
  return null;
}

/** An array, or an empty one — never an object wearing an array's type. */
function asArray<T>(payload: unknown): T[] {
  return Array.isArray(payload) ? (payload as T[]) : [];
}

/** An object, or null when the payload is a not-applicable marker. */
function asRecord<T>(payload: unknown): T | null {
  if (!payload || typeof payload !== "object" || Array.isArray(payload)) return null;
  if (naReasonOf(payload)) return null;
  return payload as T;
}
function ageLabel(iso?: string): string {
  if (!iso) return "no data";
  const ms = Date.now() - new Date(iso).getTime();
  if (Number.isNaN(ms)) return "no data";
  const s = Math.max(0, Math.round(ms / 1000));
  if (s < 60) return `${s}s ago`;
  return `${Math.round(s / 60)}m ago`;
}

/**
 * The account exactly as DELTA reports it.
 *
 * Every other section of this page is the engine's own read model. That is the
 * arrangement the 2026-08-01 audit found unfalsifiable: the bridge reported
 * +$0.9424 for a day the venue recorded as -$3.5405, and stats, trades and the
 * leaderboard all agreed with each other because all three were computed from
 * the same wrong numbers.
 *
 * Nothing below is derived, filtered by strategy, or attributed to a desk.
 * Where it disagrees with the rest of the page, this is right.
 */
type VenueBalance = { asset: string; balance: number; availableBalance: number };
type VenuePosition = {
  symbol: string;
  size: number;
  entryPrice: number;
  margin: number;
  liquidationPrice?: string;
  unrealizedPnl?: number;
};
type VenueOpenOrder = {
  orderId: string;
  symbol: string;
  side: string;
  size: number;
  price: number;
  state: string;
  createdAt: string;
};
type VenueHistoricalOrder = {
  id: number;
  symbol: string;
  side: string;
  size: number;
  unfilledSize: number;
  avgFillPrice: number;
  orderType: string;
  state: string;
  reduceOnly: boolean;
  cancelReason?: string;
  paidCommission: number;
  createdAt: string;
};
type VenueFill = {
  /** A UUID on this endpoint, not a number. */
  id: string;
  orderId: string;
  symbol: string;
  side: string;
  size: number;
  price: number;
  role: string;
  commission: number;
  createdAt: string;
};
type VenueLedger = {
  id: number;
  type: string;
  amount: number;
  balance: number;
  asset: string;
  productName?: string;
  createdAt: string;
};
type VenuePayload = {
  asOf: string;
  balances?: VenueBalance[];
  positions?: VenuePosition[];
  openOrders?: VenueOpenOrder[];
  orderHistory?: VenueHistoricalOrder[];
  fills?: VenueFill[];
  ledger?: VenueLedger[];
  /** Per-section failures. An empty table and a broken table look identical. */
  errors?: Record<string, string>;
};

function DeskEmptyStateInline({ text }: { text: string }) {
  return (
    <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "10px 2px" }}>
      {text}
    </p>
  );
}

/**
 * The Live Engine Paper Desk.
 *
 * The promoted strategies on $100 of paper money each, against real Delta
 * prices, with Delta's real taker fee on both legs. Only the money is
 * simulated.
 *
 * It is the control the page was missing. The scalp leaderboard said 79.7% wins
 * and +$37 gross where the same streams returned 33.3% and -$13.91 with money,
 * and nothing could explain the gap — the scalp desk runs 66,000 streams on
 * different levels and a different fee model, so it was answering a different
 * question. This desk holds every variable equal to the live bridge except
 * execution, so a disagreement between the two means slippage and latency and
 * nothing else.
 */
type PaperAccount = {
  strategy: string;
  /** This strategy's contribution to the SHARED balance, not its own account. */
  shareOfEquityPct: number;
  trades: number;
  wins: number;
  grossUsd: number;
  feesUsd: number;
  netUsd: number;
};
type PaperOpen = {
  strategy: string;
  symbol: string;
  dir: string;
  entry: number;
  stop: number;
  target: number;
  contracts: number;
  openedAt: string;
};
type PaperTrade = {
  strategy: string;
  symbol: string;
  dir: string;
  entry: number;
  exit: number;
  reason: string;
  grossUsd: number;
  feesUsd: number;
  netUsd: number;
  closedAt: string;
  holdMin: number;
};
type PaperDesk = {
  startingEquityUsd: number;
  /** ONE balance for the whole desk. */
  equityUsd: number;
  netUsd: number;
  roiPct: number;
  openNotionalUsd: number;
  maxNotionalUsd: number;
  maxConcurrent: number;
  maxLeverage: number;
  /** Leverage SET ON THE PRODUCT at Delta — decides where liquidation sits, not size. */
  productLeverage: number;
  liquidationDistPct: number;
  feeRatePerSide: number;
  accounts?: PaperAccount[];
  openPositions?: PaperOpen[];
  recentTrades?: PaperTrade[];
  uptimeMin: number;
};

export default function LiveEnginePage() {
  const [state, setState] = useState<LiveState | null>(null);
  const [fx, setFx] = useState<{ rate: number; asOf: string | null } | null>(null);
  const [showAllStrategies, setShowAllStrategies] = useState(false);
  const [leaderSort, setLeaderSort] = useState<LeaderSortKey>("avgRoi");
  const [leaderSortDir, setLeaderSortDir] = useState<"asc" | "desc">("desc");
  const toggleLeaderSort = useCallback((k: LeaderSortKey) => {
    setLeaderSort((prev) => {
      if (prev === k) {
        setLeaderSortDir((d) => (d === "desc" ? "asc" : "desc"));
        return prev;
      }
      // A new column starts on its most useful end — descending — rather than
      // inheriting the previous column's direction, which silently shows the
      // WORST rows first on a board people read top-down.
      setLeaderSortDir("desc");
      return k;
    });
  }, []);
  /**
   * Why a panel is empty, keyed by panel, rendered in that panel's own empty
   * state. "No rows" and "this venue has no such desk" look identical on screen
   * and mean opposite things — and a silent zero read as a result is the
   * recurring failure on these desks, not an acceptable default.
   */
  const [na, setNa] = useState<Record<string, string>>({});
  const [account, setAccount] = useState<Account | null>(null);
  const [positions, setPositions] = useState<Position[]>([]);
  const [closed, setClosed] = useState<ClosedPosition[]>([]);
  const [daily, setDaily] = useState<DailyPnl[]>([]);
  const [orders, setOrders] = useState<Order[]>([]);
  const [roster, setRoster] = useState<Eligibility[]>([]);
  const [perp, setPerp] = useState<PerpStats | null>(null);
  const [perpTrades, setPerpTrades] = useState<PerpTrade[]>([]);
  const [recon, setRecon] = useState<Recon | null>(null);
  const [audit, setAudit] = useState<AuditEntry[]>([]);
  const [error, setError] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);
  const [busy, setBusy] = useState<boolean>(false);
  const [actionMsg, setActionMsg] = useState<string>("");
  const [showAllOrders, setShowAllOrders] = useState<boolean>(false);
  const [showAllClosed, setShowAllClosed] = useState<boolean>(false);
  const [confirmClear, setConfirmClear] = useState<boolean>(false);


  const refresh = useCallback(async () => {
    try {
      const [st, ac, po, cp, dp, or, ro, ps, pt, rc, vn, au] = await Promise.all([
        fetch("/api/live-engine/state", { cache: "no-store" }),
        fetch("/api/live-engine/account", { cache: "no-store" }),
        fetch("/api/live-engine/positions", { cache: "no-store" }),
        fetch("/api/live-engine/closed-positions", { cache: "no-store" }),
        fetch("/api/live-engine/daily-pnl", { cache: "no-store" }),
        fetch("/api/live-engine/orders", { cache: "no-store" }),
        fetch("/api/live-engine/roster", { cache: "no-store" }),
        // The scalp desk's real-money perpetual arm. Same Delta wallet,
        // different desk — a board showing only options would report half
        // the live exposure while looking complete.
        fetch("/api/scalp/scalp/live/stats", { cache: "no-store" }),
        fetch("/api/scalp/scalp/live/trades", { cache: "no-store" }),
        fetch("/api/live-engine/reconciliation", { cache: "no-store" }),
        fetch("/api/live-engine/venue", { cache: "no-store" }),
        fetch("/api/live-engine/audit", { cache: "no-store" }),
      ]);
      if (!st.ok) {
        setError(`control plane unreachable (HTTP ${st.status})`);
        return;
      }

      // The display rate. Deliberately not part of the Promise.all above: a
      // currency-conversion outage must never stop the desk's own numbers from
      // loading. On failure the rate stays null and every amount renders in
      // dollars with a $ sign, which is wrong-currency but never wrong-number.
      try {
        const fx = await fetch("/api/fx/usd-inr", { cache: "no-store" });
        const body = (await fx.json()) as { ok?: boolean; rate?: number; asOf?: string | null };
        if (fx.ok && body.ok && typeof body.rate === "number") {
          setUsdInrRate(body.rate);
          setFx({ rate: body.rate, asOf: body.asOf ?? null });
        } else {
          setUsdInrRate(null);
          setFx(null);
        }
      } catch {
        setUsdInrRate(null);
        setFx(null);
      }
      const notes: Record<string, string> = {};
      /** Read a payload once, and remember any "no source here" it carries. */
      const read = async (res: Response, key: string): Promise<unknown> => {
        if (!res.ok) return null;
        const body = await res.json().catch(() => null);
        const reason = naReasonOf(body);
        if (reason) {
          notes[key] = reason;
          return null;
        }
        return body;
      };

      setState(asRecord<LiveState>(await read(st, "state")));
      setAccount(asRecord<Account>(await read(ac, "account")));
      setPositions(asArray<Position>(await read(po, "positions")));
      setClosed(asArray<ClosedPosition>(await read(cp, "closed")));
      setDaily(asArray<DailyPnl>(await read(dp, "daily")));
      setOrders(asArray<Order>(await read(or, "orders")));
      setRoster(asArray<Eligibility>(await read(ro, "roster")));
      if (ps.ok) {
        const body = (await ps.json().catch(() => null)) as { enabled?: boolean; stats?: PerpStats } | null;
        setPerp(body?.enabled ? (body.stats ?? null) : null);
      }
      setPerpTrades(asArray<PerpTrade>(await read(pt, "perpTrades")));
      setRecon(asRecord<Recon>(await read(rc, "recon")));
      // Venue truth is additive: if Delta is unreachable the rest of the page
      // must keep working, and the section says so rather than showing empty
      // tables that read as "nothing happened".
      setVenue(asRecord<VenuePayload>(await read(vn, "venue")));
      setAudit(asArray<AuditEntry>(asRecord<{ entries: AuditEntry[] }>(await read(au, "audit"))?.entries));
      setNa(notes);
      setError("");
    } catch {
      setError("control plane unreachable");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
    const t = setInterval(() => void refresh(), 15_000);
    return () => clearInterval(t);
  }, [refresh]);

  /**
   * Wipe the live trade record on BOTH desks.
   *
   * Both, in one action, because the two panels this empties are already
   * merged: Closed Positions counts "across both desks" and the leaderboard
   * ranks the perpetual streams. Clearing one engine would leave the other's
   * rows on screen, which reads as the button having failed.
   *
   * Open positions survive on purpose — both engines drop only CLOSED and
   * FAILED rows. An open position is real money on Delta, and its row is the
   * only thing tying it to a stop, a target and the strategy that opened it.
   *
   * Each result is reported separately. A single "cleared" message covering a
   * pair of calls where one 500'd is how a UI teaches an operator to trust a
   * control that does not work.
   */
  const clearLiveData = useCallback(async () => {
    setBusy(true);
    setActionMsg("");
    try {
      const [opt, perpRes] = await Promise.all([
        fetch("/api/live-engine/clear-history", { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" }),
        fetch("/api/scalp/scalp/live/clear-history", { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" }),
      ]);
      const optBody = (await opt.json().catch(() => null)) as { cleared?: number } | null;
      const perpBody = (await perpRes.json().catch(() => null)) as { cleared?: number } | null;
      const parts: string[] = [];
      parts.push(opt.ok ? `options desk: ${optBody?.cleared ?? 0} cleared` : `options desk FAILED (HTTP ${opt.status})`);
      parts.push(perpRes.ok ? `perp desk: ${perpBody?.cleared ?? 0} cleared` : `perp desk FAILED (HTTP ${perpRes.status})`);
      setActionMsg(parts.join(" · ") + (opt.ok && perpRes.ok ? " — open positions untouched" : ""));
      setConfirmClear(false);
      await refresh();
    } catch {
      setActionMsg("clear failed — the control plane did not respond; nothing was cleared");
    } finally {
      setBusy(false);
    }
  }, [refresh]);

  /**
   * Switch one strategy on or off for the live desk.
   *
   * Posts to the perpetual engine and then refetches rather than flipping local
   * state optimistically. An optimistic toggle on a REAL-MONEY control shows
   * "off" the instant it is clicked whether or not the engine agreed, which is
   * the worst possible lie for this particular switch.
   */
  const toggleStrategy = useCallback(
    async (strategy: string, symbol: string, enabled: boolean) => {
      setBusy(true);
      setActionMsg("");
      try {
        const res = await fetch("/api/scalp/scalp/live/strategy", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ strategy, symbol, enabled }),
        });
        if (!res.ok) {
          setActionMsg(`${strategy} on ${symbol}: HTTP ${res.status} — switch NOT applied`);
        } else {
          setActionMsg(`${strategy} on ${symbol} switched ${enabled ? "ON" : "OFF"}`);
        }
      } catch (e) {
        setActionMsg(`${strategy} on ${symbol}: ${e instanceof Error ? e.message : String(e)} — switch NOT applied`);
      } finally {
        await refresh();
        setBusy(false);
      }
    },
    [refresh],
  );

  const mutate = useCallback(
    async (action: string, body: Record<string, unknown>) => {
      setBusy(true);
      setActionMsg("");
      try {
        const res = await fetch(`/api/live-engine/${action}`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        });
        const data = (await res.json()) as { ok?: boolean; error?: string; result?: unknown };
        if (!res.ok || data.ok === false) {
          setActionMsg(data.error ?? `HTTP ${res.status}`);
        } else {
          setActionMsg(`${action} ok`);
        }
        await refresh();
      } catch (e) {
        setActionMsg(e instanceof Error ? e.message : "request failed");
      } finally {
        setBusy(false);
      }
    },
    [refresh],
  );

  const [perpBusy, setPerpBusy] = useState<boolean>(false);
  const [venue, setVenue] = useState<VenuePayload | null>(null);
  const [venueTab, setVenueTab] = useState<string>("positions");

  /**
   * Arm or disarm the PERPETUAL desk.
   *
   * Separate from `mutate` because it is a separate engine on a separate
   * process: `mutate` drives the options bridge in cmd/antigravity, this drives
   * the perp bridge in cmd/scalp_prelive. They share one Delta wallet and
   * nothing else — one being armed says nothing about the other.
   */
  const perpMutate = useCallback(
    async (action: "arm" | "disarm") => {
      setPerpBusy(true);
      try {
        const res = await fetch(`/api/scalp/scalp/live/${action}`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          // The engine's field is `confirm`, not `confirmation` — a mismatch
          // here fails every arm with a 400 that looks like a permissions
          // problem. Checked against the handler rather than assumed.
          body: JSON.stringify(
            action === "arm"
              ? { confirm: "ARM LIVE TRADING", actor: "ui" }
              : { actor: "ui" },
          ),
        });
        if (!res.ok) {
          setError(`perp ${action} failed (HTTP ${res.status})`);
          return;
        }
        await refresh();
      } catch {
        setError(`perp ${action} failed`);
      } finally {
        setPerpBusy(false);
      }
    },
    [refresh],
  );

  const armed = state?.armed ?? false;

  /**
   * An empty table says why it is empty. Without this, "no source on this
   * venue" and "traded nothing" render as the same blank row.
   */
  const emptyNote = useCallback(
    (key: string, fallback: string) => (
      <span style={{ color: "var(--desk-on-surface-variant)" }}>{na[key] ?? fallback}</span>
    ),
    [na],
  );

  /** Every live position on the wallet, whichever desk opened it. */
  const allPositions: UnifiedPosition[] = useMemo(() => {
    const out: UnifiedPosition[] = positions.map((p) => ({
      desk: "options" as const,
      symbol: p.symbol,
      side: p.side,
      size: p.size,
      entryPrice: p.entryPrice,
      markPrice: p.markPrice,
      unrealizedPnl: p.unrealizedPnl,
      marginUsd: p.marginUsd,
      stopPrice: p.stopLossPrice,
      targetPrice: p.takeProfitPrice,
      // Options: premium at mark. Long premium is what the position is worth,
      // and it is the figure the desk already treats as capital at risk.
      notionalUsd: p.markPrice * p.size * OPTION_CONTRACT_SIZE_BTC,
      strategy: p.strategy,
    }));
    for (const t of perp?.openPositions ?? []) {
      out.push({
        desk: "scalp",
        symbol: t.symbol,
        side: t.side,
        size: t.contracts,
        notionalUsd: t.notionalUsd,
        entryPrice: t.entryPrice,
        // The venue's own mark, from the SAME custody read that decides this
        // position's exits — so the screen and the risk engine cannot disagree
        // about where the position stands.
        markPrice: t.markPrice ?? t.entryPrice,
        unrealizedPnl: t.unrealizedPnl ?? 0,
        marginUsd: 0,
        stopPrice: t.stopPrice,
        targetPrice: t.targetPrice,
        ifTargetUsd: t.ifTargetUsd,
        ifStopUsd: t.ifStopUsd,
        strategy: t.strategy,
      });
    }
    return out;
  }, [positions, perp]);

  const unifiedPositionColumns: DeskColumn<UnifiedPosition>[] = useMemo(
    () => [
      { id: "sym", header: "Symbol", cell: (p) => p.symbol },
      { id: "strat", header: "Strategy", cell: (p) => p.strategy || "—" },
      {
        id: "side",
        header: "Side",
        cell: (p) => (
          <DeskChip tone={p.side.toUpperCase() === "BUY" || p.side.toUpperCase() === "LONG" ? "success" : "default"}>
            {p.side}
          </DeskChip>
        ),
      },
      { id: "size", align: "right", header: "Size", cell: (p) => p.size },
      { id: "mark", align: "right", header: "Mark", cell: (p) => fmtPrice(p.markPrice) },
      { id: "entry", align: "right", header: "Entry", cell: (p) => fmtPrice(p.entryPrice) },
      {
        id: "stop",
        align: "right",
        header: "Stop",
        cell: (p) => (p.stopPrice ? fmtPrice(p.stopPrice) : "—"),
      },
      {
        id: "target",
        align: "right",
        header: "Target",
        cell: (p) => (p.targetPrice ? fmtPrice(p.targetPrice) : "—"),
      },
      {
        id: "iftarget",
        align: "right",
        header: "If TP",
        // NET of the round-trip fee, like every other figure here. The fee is
        // charged whichever way the trade goes, so it shrinks this AND deepens
        // "If SL" — it does not cancel between them.
        cell: (p) =>
          p.ifTargetUsd ? <span className="desk-pnl-positive">{fmtMoney(p.ifTargetUsd)}</span> : "—",
      },
      {
        id: "ifstop",
        align: "right",
        header: "If SL",
        cell: (p) => (p.ifStopUsd ? <span className="desk-pnl-negative">{fmtMoney(p.ifStopUsd)}</span> : "—"),
      },
      {
        id: "rr",
        align: "right",
        header: "R:R",
        // The ratio AFTER fees, which is the one that decides the trade. A 1:3
        // position on paper is nearer 1:2.5 once the round trip is paid.
        cell: (p) =>
          p.ifTargetUsd && p.ifStopUsd ? `1:${(p.ifTargetUsd / Math.abs(p.ifStopUsd)).toFixed(2)}` : "—",
      },
      {
        id: "upnl",
        align: "right",
        header: "Unrealized",
        cell: (p) => <span className={pnlTone(p.unrealizedPnl)}>{fmtMoney(p.unrealizedPnl)}</span>,
      },
    ],
    [],
  );

  const dailyColumns: DeskColumn<DailyPnl>[] = useMemo(
    () => [
      { id: "date", header: "Date (IST)", cell: (d) => fmtISTDayLabel(d.date) },
      { id: "cap", align: "right", header: "Capital used", cell: (d) => fmtMoney(d.capitalUsd) },
      {
        id: "roi", align: "right", header: "ROI",
        cell: (d) => <span className={pnlTone(d.roiPct)} style={{ fontWeight: 600 }}>{d.roiPct.toFixed(1)}%</span>,
      },
      { id: "trades", align: "right", header: "Trades", cell: (d) => d.trades },
      {
        id: "fees",
        align: "right",
        header: "Fees",
        cell: (d) =>
          d.feesUsd === undefined ? "—" : (
            <span
              title={
                d.feeDragPct !== undefined
                  ? `${d.feeDragPct.toFixed(1)}% of the premium deployed that day`
                  : undefined
              }
            >
              {fmtMoney(d.feesUsd)}
              {d.feeDragPct !== undefined && (
                <span style={{ opacity: 0.6 }}> ({d.feeDragPct.toFixed(0)}%)</span>
              )}
            </span>
          ),
      },
      {
        id: "pnl", align: "right", header: "P&L (net)",
        cell: (d) => (
          <span
            className={pnlTone(d.pnlUsd)}
            style={{ fontWeight: 600 }}
            title={d.grossPnlUsd !== undefined ? `Gross ${fmtMoney(d.grossPnlUsd)} before fees` : undefined}
          >
            {fmtMoney(d.pnlUsd)}
          </span>
        ),
      },
      {
        id: "win", align: "right", header: "Win %",
        cell: (d) => `${d.winRatePct.toFixed(0)}% (${d.wins}/${d.trades})`,
      },
    ],
    [],
  );

  /**
   * Closed trades from BOTH live desks, newest first.
   *
   * This listed only the options engine's closes, so the perpetual desk's real
   * fills — the ones actually being traded now — were absent from the page that
   * exists to show them. The options rows are days old and the perp rows are
   * minutes old, so ordering by close time puts the current desk on top without
   * hiding the older record.
   */
  const allClosed: ClosedPosition[] = useMemo(() => {
    const rows: ClosedPosition[] = [...closed];
    for (const t of perpTrades) {
      if (t.status !== "CLOSED") continue;
      rows.push({
        id: `perp-${t.strategy}-${t.symbol}-${t.closedAt ?? ""}`,
        strategy: t.strategy,
        // A perpetual has no option type; the column renders "—" rather than
        // borrowing CALL/PUT, which would misdescribe the instrument.
        optionType: "",
        symbol: t.symbol,
        contracts: t.contracts,
        entryPrice: t.entryPrice,
        exitPrice: t.exitPrice ?? 0,
        realizedPnl: t.realisedPnl ?? 0,
        // The perp bridge books net and does not split fees out, so gross is
        // reported as net rather than fabricating a split.
        grossPnl: t.realisedPnl ?? 0,
        feesUsd: undefined,
        exitReason: t.exitReason ?? "",
        openedAt: t.openedAt ?? "",
        closedAt: t.closedAt,
      } as ClosedPosition);
    }
    rows.sort((a, b) => {
      const at = a.closedAt ? Date.parse(a.closedAt) : 0;
      const bt = b.closedAt ? Date.parse(b.closedAt) : 0;
      return bt - at;
    });
    return rows;
  }, [closed, perpTrades]);

  const closedColumns: DeskColumn<ClosedPosition>[] = useMemo(
    () => [
      {
        id: "closedAt",
        header: "Closed (IST)",
        cell: (c) => fmtIST(c.closedAt),
      },
      { id: "sym", header: "Symbol", cell: (c) => c.symbol || "—" },
      { id: "strat", header: "Strategy", cell: (c) => c.strategy || "—" },
      { id: "ct", align: "right", header: "Contracts", cell: (c) => c.contracts },
      { id: "entry", align: "right", header: "Entry", cell: (c) => (c.entryPrice ? c.entryPrice.toFixed(2) : "—") },
      { id: "exit", align: "right", header: "Exit", cell: (c) => (c.exitPrice ? c.exitPrice.toFixed(2) : "—") },
      {
        id: "why",
        header: "Exit reason",
        cell: (c) => {
          const r = c.exitReason || "";
          // Colour by the real outcome, not the label: a strategy-driven "SL" can
          // close a real leg that is in profit, and showing that red would lie.
          const tone = c.realizedPnl > 0 ? "success" : c.realizedPnl < 0 ? "error" : "default";
          return r ? (
            <DeskChip tone={tone} title={r.startsWith("strategy_") ? "Strategy exit (decided on the paper chain)" : "Real risk exit (measured on the live position)"}>
              {r}
            </DeskChip>
          ) : "—";
        },
      },
      {
        id: "pnl",
        align: "right",
        header: "Realized P&L (net)",
        cell: (c) => (
          <span
            className={pnlTone(c.realizedPnl)}
            style={{ fontWeight: 600 }}
            title={
              c.grossPnl !== undefined && c.feesUsd !== undefined
                ? `Gross ${fmtMoney(c.grossPnl)} − fees ${fmtMoney(c.feesUsd)}`
                : undefined
            }
          >
            {fmtMoney(c.realizedPnl)}
          </span>
        ),
      },
    ],
    [],
  );

  const orderColumns: DeskColumn<Order>[] = useMemo(
    () => [
      { id: "strat", header: "Strategy", cell: (o) => o.strategy || "—" },
      { id: "type", header: "Type", cell: (o) => <DeskChip tone={o.optionType === "CALL" ? "success" : "primary"}>{o.optionType}</DeskChip> },
      { id: "sym", header: "Symbol", cell: (o) => o.symbol || "—" },
      { id: "ct", align: "right", header: "Contracts", cell: (o) => o.contracts },
      { id: "prem", align: "right", header: "Premium", cell: (o) => fmtMoney(o.premiumUsd) },
      {
        id: "status",
        header: "Status",
        cell: (o) => (
          <DeskChip tone={o.status === "OPEN" ? "success" : o.status === "FAILED" ? "error" : "default"}>{o.status}</DeskChip>
        ),
      },
      { id: "reject", header: "Reject reason", cell: (o) => <span style={{ color: "var(--desk-error)" }}>{o.rejectReason ?? ""}</span> },
    ],
    [],
  );

  /**
   * Aggregate closed live trades per strategy.
   *
   * A strategy with no closed trade yet still gets a row, because "enabled and
   * has never filled" is a materially different state from "enabled and losing",
   * and omitting it makes the first look like the second never happened.
   */
  const leaderRows: LeaderRow[] = useMemo(() => {
    const byStrategy = new Map<string, LeaderRow>();

    // Option strategies are deliberately NOT on this board.
    //
    // Their results have not been hidden — the options engine's fills remain in
    // Closed Positions, and any open option still appears in Live Positions with
    // an OPTION desk tag. This board is the perpetual strategy board, which is
    // what it is being read for.
    //
    // That distinction matters: removing a desk from the ONLY place it appeared
    // would recreate the blind spot that let two funded perpetuals sit
    // unaccounted for earlier today.

    // ── the scalp desk's perpetual arm ────────────────────────────────────
    //
    // A different desk, the same Delta wallet. Showing only the options engine
    // here would report half the account's real exposure while looking
    // complete — which is precisely the confusion that made a scalp perp
    // appear as an unexplained "Delta reports 1 position" mismatch.
    if (perp) {
      const perpRows = new Map<string, LeaderRow>();
      const blankPerp = (name: string, symbol: string): LeaderRow => ({
        desk: "scalp",
        strategy: name,
        symbol,
        trades: 0,
        wins: 0,
        winRatePct: 0,
        grossUsd: 0,
        feesUsd: 0,
        netUsd: 0,
        feeDragPct: 0,
        stopOuts: 0,
        // Rendered from the engine's disabled set, so the switch shows what the
        // engine will actually do rather than what this tab last clicked.
        enabled: !(perp.disabledStrategies ?? []).includes(`${name}|${symbol.toUpperCase()}`),
        gridRefusals: 0,
        lastStopTicks: 0,
        notionalUsd: 0,
        roiPctSum: 0,
        roiSamples: 0,
        allowed: true,
        // The scalp desk's promotion gate passes none of these; the bridge
        // trades them on owner instruction, not on a gate verdict.
        live: false,
        reason: "scalp perpetual — owner-selected, gate not passed",
      });

      // Seeded from the STREAM roster, not the strategy list. A strategy that
      // runs on three symbols is three positions with three records; merging
      // them into one row hides which instrument a result came from, which is
      // what decides whether the result means anything.
      const streamKey = (strategy: string, symbol: string) => `${strategy}|${(symbol || "").toUpperCase()}`;
      for (const st of perp.liveStreams ?? []) {
        const row = blankPerp(st.strategy, st.symbol);
        row.gridRefusals = st.gridRefusals ?? 0;
        row.lastStopTicks = st.lastStopTicks ?? 0;
        perpRows.set(streamKey(st.strategy, st.symbol), row);
      }
      for (const t of perpTrades) {
        if (t.status !== "CLOSED") continue;
        const k = streamKey(t.strategy, t.symbol);
        const row = perpRows.get(k) ?? blankPerp(t.strategy, t.symbol);
        row.trades += 1;
        const net = t.realisedPnl ?? 0;
        if (net > 0) row.wins += 1;

        // The bridge DOES split out fees — it has since brackets were added,
        // and this board went on reporting $0.00 against trades that each paid
        // ~$0.35. On a planned risk of ~$1.92 a trade, that is a fifth of the
        // intended loss displayed as nothing, which is how a desk looks
        // survivable when it is not.
        //
        // realisedPnl is already NET, so gross is recovered by adding the fee
        // back rather than by inventing a pre-fee figure.
        const fees = t.feesUsd ?? 0;
        row.feesUsd += fees;
        row.notionalUsd += t.notionalUsd ?? 0;
        // Each fill's OWN return, accumulated for a simple average.
        //
        // Deliberately not net/totalNotional, which weights by position size
        // and would let one large trade decide the figure. The question here is
        // "what does a typical trade return", so every trade counts once.
        const tradeNotional = t.notionalUsd ?? 0;
        if (tradeNotional > 0) {
          row.roiPctSum += ((t.realisedPnl ?? 0) / tradeNotional) * 100;
          row.roiSamples += 1;
        }
        // Accumulated as a sum here and divided at the end — averaging an
        // average as trades arrive would weight the earliest stop-out most.
        if (t.stopOvershoot && t.stopOvershoot > 0) {
          row.stopOvershoot = (row.stopOvershoot ?? 0) + t.stopOvershoot;
          row.stopOuts += 1;
        }
        row.netUsd += net;
        row.grossUsd += net + fees;
        perpRows.set(k, row);
      }
      // Fee drag: what share of gross the venue took. Computed against the
      // MAGNITUDE of gross so a losing strategy reports a meaningful ratio —
      // signed division would flip it negative and read as a rebate.
      //
      // Left undefined at zero trades rather than shown as 0%, because "0% fee
      // drag" claims the venue is free.
      for (const v of perpRows.values()) {
        v.winRatePct = v.trades > 0 ? (v.wins / v.trades) * 100 : 0;
        v.feeDragPct = Math.abs(v.grossUsd) > 1e-9 ? (v.feesUsd / Math.abs(v.grossUsd)) * 100 : 0;
        // Undefined rather than 0 when nothing has stopped out: "0.00x" would
        // read as a perfect stop record on a strategy that has never had one.
        v.stopOvershoot = v.stopOuts > 0 ? (v.stopOvershoot ?? 0) / v.stopOuts : undefined;
      }
      // Positions still open are not counted — this board is realised results.
      for (const [k, v] of perpRows) byStrategy.set("scalp:" + k, v);
    }

    const rows = Array.from(byStrategy.values()).map((r) => ({
      ...r,
      winRatePct: r.trades > 0 ? (r.wins / r.trades) * 100 : 0,
      // Against GROSS PROFIT, not against turnover: a desk can look cheap on
      // turnover while fees eat most of what it made.
      feeDragPct: r.grossUsd > 0 ? (r.feesUsd / r.grossUsd) * 100 : 0,
    }));

    // Best performer first.
    //
    // Ranked on EDGE PER TRADE, not on total net. Total net rewards whichever
    // stream happened to fill most often, and in fixed-size mode it also
    // rewards whichever symbol is expensive — one LABUSD contract is 48x one
    // SOLVUSD contract, so a Net $ ranking would sort by coin price.
    //
    // Sample size decides the TIER, not the sort within it. A single lucky fill
    // can show a huge per-trade edge, and no amount of arithmetic weighting
    // makes one observation comparable to thirty — so streams that have not
    // reached a usable sample rank below those that have, however good they
    // look. That is the difference between "best" and "luckiest", and this desk
    // has been fooled by the second often enough.
    // Unfilled rows always sink, whichever column is sorted. They have no value
    // for any metric, and letting them float on a 0 would put 114 empty rows
    // above every real result the moment someone sorts ascending.
    const metric = (r: LeaderRow): number => {
      switch (leaderSort) {
        case "profitPct": return profitPctOfCapital(r);
        case "trades": return r.trades;
        case "winRatePct": return r.winRatePct;
        case "feesUsd": return r.feesUsd;
        case "netUsd": return r.netUsd;
        case "stopOvershoot": return r.stopOvershoot ?? 0;
        case "feeDragPct": return r.feeDragPct;
        default: return avgRoiPerTrade(r);
      }
    };
    rows.sort((a, b) => {
      const aEmpty = a.trades === 0;
      const bEmpty = b.trades === 0;
      if (aEmpty !== bEmpty) return aEmpty ? 1 : -1;
      if (aEmpty) return a.strategy.localeCompare(b.strategy);
      const d = metric(a) - metric(b);
      return leaderSortDir === "desc" ? -d : d;
    });
    return rows;
  }, [perp, perpTrades, leaderSort, leaderSortDir]);

  /**
   * Desk-level record across every closed live trade.
   *
   * UNRECONCILED exits are counted separately and excluded from the win rate.
   * Those are positions that vanished from the venue with no matching fill, so
   * the engine booked their whole notional as the result — a placeholder, not a
   * measured outcome. Three of them currently carry 84% of the desk's realised
   * loss, and letting them score as wins or losses would put a number on the
   * board that no trade produced.
   */
  const deskRecord = useMemo(() => {
    const closed = perpTrades.filter((t) => t.status === "CLOSED");
    const scored = closed.filter((t) => t.exitReason !== "UNRECONCILED");
    const wins = scored.filter((t) => (t.realisedPnl ?? 0) > 0).length;
    return {
      closed: closed.length,
      scored: scored.length,
      wins,
      unreconciled: closed.length - scored.length,
      winRatePct: scored.length > 0 ? (wins / scored.length) * 100 : 0,
    };
  }, [perpTrades]);

  /** Rank in the current ordering, keyed by stream. */
  const leaderRank = useMemo(() => {
    const m = new Map<string, number>();
    let n = 0;
    for (const r of leaderRows) {
      if (r.trades === 0) continue; // unranked: nothing to place
      m.set(`${r.strategy}|${r.symbol}`, ++n);
    }
    return m;
  }, [leaderRows]);

  const leaderColumns: DeskColumn<LeaderRow>[] = useMemo(
    () => [
      {
        id: "rank",
        header: "#",
        align: "right",
        // Position in the CURRENT ordering, so it renumbers when a column is
        // sorted rather than pretending to be a fixed identity.
        cell: (r) => (
          <span style={{ color: "var(--desk-on-surface-variant)" }}>
            {r.trades === 0 ? "—" : (leaderRank.get(`${r.strategy}|${r.symbol}`) ?? "—")}
          </span>
        ),
      },
      {
        id: "desk",
        header: "Desk",
        cell: (r) => (
          <DeskChip tone={r.desk === "scalp" ? "warning" : "default"}>
            {r.desk === "scalp" ? "PERP" : "OPTION"}
          </DeskChip>
        ),
      },
      { id: "strat", header: "Strategy", cell: (r) => r.strategy },
      {
        id: "roi",
        header: <LeaderSortHeader label="Profit %" k="profitPct" sortKey={leaderSort} sortDir={leaderSortDir} onSort={toggleLeaderSort} />,
        align: "right",
        // Net profit over the capital this stream deploys — one position's
        // notional, since it holds one at a time. +104% means it has roughly
        // doubled the money it commits; -10% means it lost a tenth of it.
        //
        // The tooltip carries profit per unit TRADED against the fee, because
        // the two disagree in the case that matters: a stream recycling its
        // capital often can show a large return while losing money on every
        // individual trade.
        cell: (r) => {
          if (r.trades === 0 || r.notionalUsd <= 0) {
            return <span style={{ opacity: 0.5 }} title="no fills yet — nothing to measure">—</span>;
          }
          const pct = profitPctOfCapital(r);
          const edge = edgePct(r);
          const beatsFees = edge > ROUND_TRIP_FEE_PCT;
          return (
            <span
              className={pnlTone(pct)}
              style={{ fontWeight: r.trades >= LEADER_MIN_SAMPLE ? 700 : 400 }}
              title={
                `${fmtMoney(r.netUsd)} net on ${fmtMoney(capitalDeployed(r))} of deployed capital, ` +
                `recycled across ${r.trades} fill(s) (${fmtMoney(r.notionalUsd)} traded in total). ` +
                `Per unit traded it earns ${edge.toFixed(3)}%, which ` +
                (beatsFees
                  ? `clears the ${ROUND_TRIP_FEE_PCT}% round-trip fee.`
                  : `does NOT clear the ${ROUND_TRIP_FEE_PCT}% round-trip fee — it costs more to trade than it earns.`) +
                (r.trades < LEADER_MIN_SAMPLE
                  ? ` Only ${r.trades} fill(s), so it ranks below streams with ${LEADER_MIN_SAMPLE}+.`
                  : "")
              }
            >
              {`${pct >= 0 ? "+" : ""}${pct.toFixed(1)}%`}
              {!beatsFees && (
                <span className="desk-pnl-negative" title="earns less per trade than the fee costs"> ⚠</span>
              )}
            </span>
          );
        },
      },
      {
        id: "avgroi",
        header: <LeaderSortHeader label="Avg ROI / trade" k="avgRoi" sortKey={leaderSort} sortDir={leaderSortDir} onSort={toggleLeaderSort} />,
        align: "right",
        // The simple mean of each fill's own return: 5 trades at +5% and 5 at
        // -2% give +1.5%. Every trade counts once, so the figure describes a
        // TYPICAL trade rather than the biggest one.
        //
        // The board is sorted on this, plainly descending, at the owner's
        // instruction. A stream with one fill can therefore lead — the "?"
        // marks it, because one trade is not evidence of anything.
        cell: (r) => {
          if (r.roiSamples === 0) {
            return <span style={{ opacity: 0.5 }} title="no fills yet — nothing to average">—</span>;
          }
          const avg = avgRoiPerTrade(r);
          return (
            <span
              className={pnlTone(avg)}
              style={{ fontWeight: r.trades >= LEADER_MIN_SAMPLE ? 700 : 400 }}
              title={
                `Simple average across ${r.roiSamples} fill(s): each trade's own net over its own notional, ` +
                `summed and divided by ${r.roiSamples}. Not weighted by position size, so one large trade cannot carry it.` +
                (r.trades < LEADER_MIN_SAMPLE
                  ? ` Only ${r.trades} fill(s) — treat this as unproven.`
                  : "")
              }
            >
              {`${avg >= 0 ? "+" : ""}${avg.toFixed(2)}%`}
            </span>
          );
        },
      },
      {
        id: "trades",
        header: <LeaderSortHeader label="Fills" k="trades" sortKey={leaderSort} sortDir={leaderSortDir} onSort={toggleLeaderSort} />,
        align: "right",
        cell: (r) => r.trades || "—",
      },
      {
        id: "wr",
        header: <LeaderSortHeader label="WR %" k="winRatePct" sortKey={leaderSort} sortDir={leaderSortDir} onSort={toggleLeaderSort} />,
        align: "right",
        cell: (r) => (r.trades > 0 ? r.winRatePct.toFixed(1) : "—"),
      },
      {
        id: "fees",
        header: <LeaderSortHeader label="Fees $" k="feesUsd" sortKey={leaderSort} sortDir={leaderSortDir} onSort={toggleLeaderSort} />,
        align: "right",
        cell: (r) => (r.trades > 0 ? fmtMoney(r.feesUsd) : "—"),
      },
      {
        id: "net",
        header: <LeaderSortHeader label="Net $" k="netUsd" sortKey={leaderSort} sortDir={leaderSortDir} onSort={toggleLeaderSort} />,
        align: "right",
        cell: (r) => (r.trades > 0 ? <span className={pnlTone(r.netUsd)}>{fmtMoney(r.netUsd)}</span> : "—"),
      },
      {
        id: "symbol",
        header: "Symbol",
        // The desk trades streams, so the instrument belongs next to the name.
        // Without it, one strategy's three symbols read as three identical rows
        // and there is no way to tell which instrument produced a result.
        cell: (r) => (
          <span style={{ fontFamily: "var(--desk-font-mono, monospace)", fontSize: "0.85em" }}>
            {r.symbol || "—"}
          </span>
        ),
      },
      {
        id: "switch",
        header: "Live",
        // The same DeskSwitch used for the desk arm, not a coloured pill.
        //
        // The pill was clickable but read as a status label, so the control
        // looked like output. On a real-money desk the difference between
        // "this is how it is" and "click to change it" has to be obvious at a
        // glance, and matching the arm toggle means the affordance is already
        // familiar.
        //
        // Governs ENTRY only. A strategy switched off opens nothing from the
        // next signal; whatever it already holds keeps its stop and target and
        // exits normally. Flattening is close-all, deliberately louder.
        cell: (r) => (
          <DeskSwitch
            id={`strategy-switch-${r.strategy}-${r.symbol}`}
            checked={r.enabled}
            disabled={busy}
            ariaLabel={`${r.strategy} on ${r.symbol} live trading`}
            // Short label so a 31-row table stays readable; the strategy name
            // is already the row, and ariaLabel carries the full context for
            // screen readers.
            label={r.enabled ? "on" : "off"}
            onColor="var(--desk-success)"
            offColor="var(--desk-error)"
            onChange={(next) => void toggleStrategy(r.strategy, r.symbol, next)}
          />
        ),
      },
      {
        id: "grid",
        header: "Grid",
        // A stream can be switched ON and still be structurally unable to
        // trade, because its symbol's tick grid is too coarse for the stop the
        // strategy wants. Shown next to the switch, because that is exactly
        // where "on" would otherwise be a lie: 19 of 31 streams currently sit
        // in this state, and without this they read as strategies that simply
        // are not signalling.
        cell: (r) =>
          r.gridRefusals > 0 ? (
            <span
              className="desk-pnl-negative"
              title={`Refused ${r.gridRefusals} signal(s) before entry: the stop was ${r.lastStopTicks.toFixed(1)} ticks wide, under the ${20} needed for it to survive this price grid. Switched ON, but it cannot open a position on this symbol.`}
              style={{ fontWeight: 700 }}
            >
              blocked
              <span style={{ opacity: 0.7, fontWeight: 400 }}>
                {` ${r.lastStopTicks.toFixed(1)}t ×${r.gridRefusals}`}
              </span>
            </span>
          ) : (
            <span style={{ opacity: 0.5 }}>—</span>
          ),
      },
      {
        id: "overshoot",
        header: <LeaderSortHeader label="Stop overshoot" k="stopOvershoot" sortKey={leaderSort} sortDir={leaderSortDir} onSort={toggleLeaderSort} />,
        align: "right",
        // Realised risk over planned risk on stop-outs. 1.00x is a stop that
        // closed where it was placed.
        //
        // On the board rather than in a detail view because stops have failed
        // here five distinct ways and each fix looked complete when shipped.
        // The sample size is shown next to it: one clean stop-out cannot
        // distinguish a fix from a quiet market.
        cell: (r) =>
          r.stopOvershoot === undefined ? (
            <span title="no stop-outs yet — nothing to measure">—</span>
          ) : (
            <span
              className={r.stopOvershoot > 1.25 ? "desk-pnl-negative" : undefined}
              title={`mean across ${r.stopOuts} stop-out(s); 1.00x closed on the stop, above 1.25x exceeds the slippage cap`}
            >
              {r.stopOvershoot.toFixed(2)}×
              <span style={{ opacity: 0.6 }}> (n={r.stopOuts})</span>
            </span>
          ),
      },
      {
        id: "drag",
        header: <LeaderSortHeader label="Fee drag" k="feeDragPct" sortKey={leaderSort} sortDir={leaderSortDir} onSort={toggleLeaderSort} />,
        align: "right",
        // Share of GROSS PROFIT eaten by fees. This is the number that showed the
        // options desk was unviable on cheap contracts, so it is on the board
        // rather than buried in a detail view.
        cell: (r) =>
          r.trades > 0 && r.grossUsd > 0 ? (
            <span className={r.feeDragPct > 30 ? "desk-pnl-negative" : undefined} title="fees as a share of gross profit">
              {r.feeDragPct.toFixed(0)}%
            </span>
          ) : (
            "—"
          ),
      },
      {
        id: "gate",
        header: "Gate",
        cell: (r) => (
          <DeskChip tone={r.live ? "success" : "default"} title={r.reason}>
            {r.live ? "PASSED" : "not passed"}
          </DeskChip>
        ),
      },
    ],
    // These columns close over live state. The dependency list was empty, which
    // memoised the columns against the FIRST render — when perp is still null.
    // Profit % would then have divided by an equity of 0 and rendered +0.00%
    // for every row forever, which reads as "nothing has made any money"
    // rather than "the denominator never arrived".
    [perp, busy, toggleStrategy, leaderRank, leaderSort, leaderSortDir, toggleLeaderSort],
  );

  const rosterColumns: DeskColumn<Eligibility>[] = useMemo(
    () => [
      { id: "strat", header: "Strategy", cell: (e) => e.strategy },
      {
        id: "enabled",
        header: "Live-enabled",
        cell: (e) => (
          <DeskButton
            data-testid={`toggle-${e.strategy}`}
            variant={e.allowed ? "tonal" : "outlined"}
            disabled={busy}
            onClick={() => void mutate("strategy", { strategy: e.strategy, enabled: !e.allowed })}
          >
            {e.allowed ? "ENABLED" : "disabled"}
          </DeskButton>
        ),
      },
      { id: "live", header: "Gate", cell: (e) => <DeskChip tone={e.live ? "success" : "default"}>{e.live ? "LIVE" : "NOT LIVE"}</DeskChip> },
      { id: "reason", header: "Reason", cell: (e) => <span style={{ color: "var(--desk-on-surface-variant)" }}>{e.reason}</span> },
      {
        id: "gates",
        header: "Gates",
        cell: (e) => (
          <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
            {e.gates.map((g) => (
              <DeskChip key={g.name} tone={g.pass ? "success" : "error"} title={`${g.requirement} — have ${g.actual}`}>
                {g.name}
              </DeskChip>
            ))}
          </div>
        ),
      },
    ],
    [busy, mutate],
  );

  const auditColumns: DeskColumn<AuditEntry>[] = useMemo(
    () => [
      { id: "at", header: "Time (IST)", cell: (a) => fmtISTSeconds(a.at) },
      { id: "actor", header: "Actor", cell: (a) => a.actor },
      { id: "action", header: "Action", cell: (a) => <DeskChip tone={a.action.includes("DISARM") ? "warning" : a.action === "ARM" ? "error" : "default"}>{a.action}</DeskChip> },
      { id: "reason", header: "Reason", cell: (a) => a.reason ?? "" },
      { id: "detail", header: "Detail", cell: (a) => <span style={{ color: "var(--desk-on-surface-variant)" }}>{a.detail ?? ""}</span> },
    ],
    [],
  );

  const reconMismatch = recon ? !recon.matched : false;

  return (
    <div
      style={{ minHeight: "100%", background: "var(--desk-surface-dim)" }}
      data-testid="live-engine-root"
      data-armed={armed ? "true" : "false"}
    >
      <DeskLinearProgress visible={loading} />
      <main className="desk-page">
        {/* Header + persistent REAL MONEY differentiation */}
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 6, fontSize: "0.8125rem" }}>
            <Link href="/terminal" className="desk-label-md" style={{ fontWeight: 400, textDecoration: "none" }}>Home</Link>
            <span style={{ color: "var(--desk-outline)" }}>›</span>
            <span className="desk-body-md" style={{ fontWeight: 500 }}>Live Engine</span>
          </div>
          <div style={{ marginTop: 8, display: "flex", flexWrap: "wrap", alignItems: "center", gap: 12 }}>
            <h1 className="desk-display-lg" style={{ fontSize: "2rem" }}>Live Engine</h1>
            <span
              data-testid="real-money-badge"
              className="desk-mono"
              style={{
                fontWeight: 700,
                fontSize: "0.75rem",
                letterSpacing: "0.06em",
                padding: "4px 10px",
                borderRadius: 6,
                color: "#fff",
                // Green while the Delta Engine is on, red when it is off.
                background: armed ? "var(--desk-success)" : "var(--desk-error)",
              }}
            >
              REAL MONEY · ${CEILING}
            </span>
          </div>
          {/*
            The conversion, stated. Every money figure on this page is a USD
            amount multiplied by this rate, and a converted number is only as
            true as the rate behind it — so the rate and its age are shown
            rather than assumed. When the provider is unreachable the amounts
            stay in dollars and say so, instead of being restated 95x under a
            rupee sign.
          */}
          <p className="desk-label-md" style={{ marginTop: 6, color: "var(--desk-on-surface-variant)" }}>
            {fx ? (
              <>
                Amounts in <strong>INR</strong> at ₹{fx.rate.toFixed(4)}/$
                {fx.asOf ? ` · rate as of ${fx.asOf}` : ""} · instrument prices stay in USD, as Delta quotes them
              </>
            ) : (
              <span className="desk-pnl-negative">
                INR rate unavailable — amounts below are shown in USD
              </span>
            )}
          </p>
          <p className="desk-body-md" style={{ marginTop: 6, maxWidth: 760, color: "var(--desk-on-surface-variant)" }}>
            Real-money option <strong>buying</strong> on Delta BTC options (long premium only), capped at a
            ${CEILING} server-enforced ceiling. Starts off; turning the Delta Engine on places real orders immediately.
            Naked selling is excluded by decision. 10× is inert on long options — buying pays the premium in full, with
            no borrow and no liquidation price.
          </p>
        </div>

        {error && <DeskBanner variant="warning">{error} — retrying every 15s</DeskBanner>}
        {actionMsg && <DeskBanner variant={actionMsg.endsWith("ok") ? "success" : "error"}>{actionMsg}</DeskBanner>}

        {/* Engine on/off state — visible without scrolling. Green when on. */}
        <DeskBanner
          variant={armed ? "success" : "info"}
          title={armed ? "● DELTA ENGINE ON — LIVE ORDERS ENABLED" : "○ DELTA ENGINE OFF — no live orders"}
        >
          <span data-testid="armed-state">
            {armed
              ? `On since ${ageLabel(state?.armedAt)}. Live orders can be placed against real capital.`
              : `Off${state?.lastDisarmReason ? ` · last reason: ${state.lastDisarmReason}` : ""}. No live orders will be placed.`}
            {state?.killSwitchActive ? " · KILL SWITCH ON" : ""}
            {state && !state.configured ? " · broker not configured" : ""}
          </span>
        </DeskBanner>

        {/* SECTION 1a — Perpetual desk control.

            This desk is armed independently of the options engine above. The
            page previously showed its positions and P&L with no way to turn it
            on, so the options toggle read as if it governed everything. */}
        <DeskCard>
          <DeskSectionHeader
            title="Scalp Perpetual Desk"
            subtitle="Separate engine, separate arm — the Delta Engine toggle above does NOT control it."
            actions={
              <DeskChip tone={perp?.armed ? "success" : "default"} style={{ fontWeight: 700 }}>
                {perp?.armed ? "ARMED" : "DISARMED"}
              </DeskChip>
            }
          />
          <div style={{ display: "flex", flexWrap: "wrap", gap: 24, alignItems: "center", padding: "0 4px 8px" }}>
            <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
              <DeskSwitch
                id="perp-arm-toggle"
                checked={perp?.armed ?? false}
                disabled={perpBusy || !perp}
                ariaLabel="Scalp perpetual desk"
                label={perp?.armed ? "Perp desk armed" : "Perp desk disarmed"}
                onColor="var(--desk-success)"
                offColor="var(--desk-error)"
                onChange={(next) => void perpMutate(next ? "arm" : "disarm")}
              />
              <span className="desk-label-md" style={{ color: "var(--desk-on-surface-variant)" }}>
                {perp?.armed
                  ? `${perp.strategies.length} scalp strategies can place real orders`
                  : "off — scalp strategies fill on paper only"}
              </span>
            </div>
            <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
              <span className="desk-label-md" style={{ color: "var(--desk-on-surface-variant)" }}>
                Equity / risk per trade
              </span>
              <span className="desk-mono desk-body-md" style={{ fontWeight: 600 }}>
                {perp ? `$${perp.equityUsd.toFixed(0)} · $${perp.riskPerTradeUsd.toFixed(2)}` : "—"}
              </span>
            </div>
          </div>
          <p className="desk-body-md" style={{ marginTop: 6, maxWidth: 780, color: "var(--desk-on-surface-variant)" }}>
            Arming does not back-fill positions already open on paper — a live order tracks a fill at the moment it
            happens, so only NEW signals are routed. Max 3 concurrent perps regardless of how many strategies signal.
            <strong> This arm does not survive a restart:</strong> it is held in memory so a crash loop can never
            re-arm itself unattended. Disarming stops new orders; open positions keep their stop, target and time stop.
          </p>
        </DeskCard>

        {/* SECTION 1 — Arm / Disarm / Close All */}
        <DeskCard>
          <DeskSectionHeader
            title="Control"
            subtitle="OPTIONS engine only. Places real orders immediately; auto-disarm is one-way. The scalp perpetual desk is armed separately above."
            actions={
              <DeskChip tone={state?.optionsTradingEnabled ? "success" : "default"} style={{ fontWeight: 700 }}>
                {state?.optionsTradingEnabled ? "OPTION TRADING ON" : "OPTION TRADING OFF"}
              </DeskChip>
            }
          />
          <div style={{ display: "flex", flexWrap: "wrap", gap: 24, alignItems: "center", padding: "0 4px 8px" }}>
            {/* Delta Engine — green on, red off. Toggling on goes live at once. */}
            <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
              <DeskSwitch
                id="arm-toggle"
                checked={armed}
                disabled={busy}
                ariaLabel="Delta Engine"
                label={armed ? "Delta Engine on" : "Delta Engine off"}
                onColor="var(--desk-success)"
                offColor="var(--desk-error)"
                onChange={(next) => {
                  if (next) {
                    void mutate("arm", { confirmation: ARM_PHRASE });
                  } else {
                    void mutate("disarm", { reason: "Delta Engine turned off from UI" });
                  }
                }}
              />
              <span className="desk-label-md" style={{ color: "var(--desk-on-surface-variant)" }}>
                {armed
                  ? `live orders enabled · ${ageLabel(state?.armedAt)}`
                  : "off — no live orders will be placed"}
              </span>
              {/* An armed engine with the master switch off places nothing. Saying
                  so here is the whole fix: the previous page showed only "live
                  orders enabled", which was true of the engine and false of the
                  desk. */}
              {armed && state?.optionsTradingEnabled === false && (
                <span className="desk-label-md" style={{ color: "var(--desk-warning, var(--desk-on-surface-variant))", fontWeight: 600 }}>
                  option trading is OFF by configuration — no option order will be placed
                </span>
              )}
            </div>

            {/* Kill switch — red on (halted), green off (trading allowed). */}
            <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
              <DeskSwitch
                id="kill-switch-toggle"
                checked={state?.killSwitchActive ?? false}
                disabled={busy || state?.killSwitchControllable === false}
                ariaLabel="Kill switch"
                label={state?.killSwitchActive ? "Kill switch on — trading halted" : "Kill switch off — trading allowed"}
                onColor="var(--desk-error)"
                offColor="var(--desk-success)"
                onChange={(next) => void mutate("kill-switch", { active: next })}
              />
              <span className="desk-label-md" style={{ color: state?.killSwitchActive ? "var(--desk-error)" : "var(--desk-on-surface-variant)" }}>
                {state?.killSwitchActive
                  ? (state?.killSwitchReason || "halted — blocks new orders and turns the engine off")
                  : "toggle on to halt all new orders immediately"}
              </span>
            </div>

            <DeskButton data-testid="close-all" variant="outlined" disabled={busy} onClick={() => void mutate("close-all", {})}>
              Panic — CLOSE ALL
            </DeskButton>
            <div style={{ marginLeft: "auto", alignSelf: "center" }} className="desk-label-md">
              Consecutive rejects: {state?.consecutiveRejects ?? 0} / {state?.maxConsecutiveRejects ?? 3} → auto-disarm
            </div>
          </div>
        </DeskCard>

        {/* SECTION 2 — Live account strip */}
        <DeskCard>
          <DeskSectionHeader
            title="Live Account"
            subtitle={
              account
                ? `${account.source} · ${ageLabel(account.asOf)}${account.stale ? " · STALE" : ""}`
                : na.account ?? "—"
            }
          />
          <div className="desk-metrics-row">
            <DeskMetricTile
              label="Real Equity"
              value={account ? fmtMoney(account.equityUsd) : "—"}
              sub={account?.stale ? "stale — see source" : "delta wallet"}
              highlight
            />
            {/*
              ROI beside the capital it is measured on. Against the FIRST wallet
              balance recorded, not the configured desk equity: the desk risks
              $10 of a wallet holding $61, and dividing by the risk budget would
              report a return on money that was never the denominator.

              Shown as "—" until a baseline exists rather than as 0%, because
              "0% return" is a claim about performance and "no baseline yet" is
              a claim about the record.
            */}
            <DeskMetricTile
              label="ROI since inception"
              value={
                account?.inceptionEquityUsd && account.inceptionEquityUsd > 0
                  ? `${(account.roiPct ?? 0) >= 0 ? "+" : ""}${(account.roiPct ?? 0).toFixed(2)}%`
                  : "—"
              }
              valueClassName={account?.inceptionEquityUsd ? pnlTone(account.roiPct ?? 0) : undefined}
              sub={
                account?.inceptionEquityUsd && account.inceptionEquityUsd > 0
                  ? `${fmtMoney(account.roiUsd)} on ${fmtMoney(account.inceptionEquityUsd)} opening`
                  : "no baseline captured yet"
              }
            />
            <DeskMetricTile label={`Tradable (≤ $${CEILING})`} value={account ? fmtMoney(account.tradableUsd) : "—"} sub="ceiling enforced server-side" />
            <DeskMetricTile label="Available" value={account ? fmtMoney(account.availableUsd) : "—"} sub="equity − margin" />
            <DeskMetricTile label="Margin Used" value={account ? fmtMoney(account.marginUsedUsd) : "—"} sub="delta positions" />
            <DeskMetricTile label="Open Risk" value={account ? fmtMoney(account.openRiskUsd) : "—"} sub="premium at risk (long)" />
            <DeskMetricTile
              label="Win rate"
              value={deskRecord.scored > 0 ? `${deskRecord.winRatePct.toFixed(1)}%` : "—"}
              valueClassName={
                deskRecord.scored > 0
                  ? // 25% is breakeven at the desk's 1:3 geometry — below it the
                    // desk loses money however green individual rows look.
                    pnlTone(deskRecord.winRatePct - 25)
                  : undefined
              }
              sub={
                deskRecord.scored > 0
                  ? `${deskRecord.wins} of ${deskRecord.scored} closed${
                      deskRecord.unreconciled > 0 ? ` · ${deskRecord.unreconciled} unreconciled excluded` : ""
                    }`
                  : "no closed trades yet"
              }
            />
            <DeskMetricTile label="Realized Today" value={account ? fmtMoney(account.realizedTodayUsd) : "—"} valueClassName={account ? pnlTone(account.realizedTodayUsd) : undefined} sub="to daily breaker" />
          </div>
        </DeskCard>

        {/* SECTION 6 (surfaced high) — Reconciliation, shown loudly on mismatch */}
        {reconMismatch ? (
          <DeskBanner variant="error" title="⚠ RECONCILIATION MISMATCH — engine state vs Delta truth">
            <span data-testid="recon-status">
              engine {recon?.enginePositions} open · Delta {recon?.deltaPositions} positions.
              {(recon?.mismatches ?? []).map((m) => ` ${m}.`)}
              {recon?.error ? ` error: ${recon.error}` : ""} Surfaced only — this does not stop the engine; the adoption sweep normally reconciles it.
            </span>
          </DeskBanner>
        ) : (
          <DeskCard>
            <DeskSectionHeader title="Reconciliation (options engine)" subtitle={recon ? `${ageLabel(recon.asOf)}` : "—"} />
            <div className="desk-metrics-row">
              <DeskMetricTile compact label="Engine open" value={recon?.enginePositions ?? "—"} />
              <DeskMetricTile compact label="Delta positions" value={recon?.deltaPositions ?? "—"} />
              <DeskMetricTile compact label="Status" value={<span data-testid="recon-status">{recon ? (recon.matched ? "MATCHED" : "MISMATCH") : "—"}</span>} subColor={recon?.matched ? "profit" : "loss"} sub={recon?.error ?? ""} />
            </div>
          </DeskCard>
        )}

        {/* SECTION 3 — Live positions */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Live Positions"
            subtitle={
              allPositions.length
                ? `${allPositions.length} open on the Delta wallet · ${positions.length} option, ${allPositions.length - positions.length} perpetual`
                : "0 open"
            }
            actions={
              allPositions.length ? (
                // Same summary the Paper Desk carries: how many, how much is
                // committed, and what it is worth right now. Deployed is the
                // sum of position notionals, NOT the margin Delta froze — the
                // margin is a tenth of it at 10x, and reporting that as
                // "deployed" would understate the exposure by an order of
                // magnitude on a page whose whole job is showing real risk.
                <span className="desk-label-md" style={{ color: "var(--desk-on-surface-variant)" }}>
                  {`${allPositions.length} open · ${fmtMoney(
                    allPositions.reduce((sum, p) => sum + (p.notionalUsd ?? 0), 0),
                  )} deployed`}{" "}
                  <span
                    className={pnlTone(allPositions.reduce((sum, p) => sum + p.unrealizedPnl, 0))}
                    style={{ fontWeight: 700 }}
                  >
                    {fmtMoney(allPositions.reduce((sum, p) => sum + p.unrealizedPnl, 0))}
                  </span>
                </span>
              ) : undefined
            }
          />
          <DeskDataTable
            columns={unifiedPositionColumns}
            rows={allPositions}
            getRowKey={(p, i) => `${p.desk}-${p.symbol}-${i}`}
            stickyHeader
            empty={<span style={{ color: "var(--desk-on-surface-variant)" }}>No live positions.</span>}
          />
        </DeskCard>

        {/* Strategy leaderboard — REAL fills only, ranked worst net first */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Strategy Leaderboard"
            subtitle={
              leaderRows.some((r) => r.trades > 0)
                ? `best first by average ROI per trade · ${leaderRows.filter((r) => r.trades > 0).length} of ${leaderRows.length} filled · realized ${fmtMoney(
                    leaderRows.reduce((s, r) => s + r.netUsd, 0),
                  )}`
                : "best first by average net per fill · streams under 10 fills rank below proven ones, however good they look"
            }
            actions={
              /* Two-step, and the second step states the count and the
                 consequence. A one-click control here would let a misclick
                 destroy the fill record that every promotion decision reads —
                 and unlike a bad trade, there is nothing to undo it with. */
              confirmClear ? (
                <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
                  <span className="desk-body-md" style={{ color: "var(--desk-error)", fontWeight: 600 }}>
                    Delete {allClosed.length} closed {allClosed.length === 1 ? "row" : "rows"} on both desks? Open
                    positions are kept. This cannot be undone.
                  </span>
                  <DeskButton variant="text" onClick={() => setConfirmClear(false)} disabled={busy}>
                    Cancel
                  </DeskButton>
                  <DeskButton variant="danger-tonal" onClick={() => void clearLiveData()} disabled={busy}>
                    {busy ? "Clearing…" : "Yes, clear it"}
                  </DeskButton>
                </div>
              ) : (
                <DeskButton variant="outlined" onClick={() => setConfirmClear(true)} disabled={busy}>
                  Clear live data
                </DeskButton>
              )
            }
          />
          <DeskDataTable
            columns={leaderColumns}
            // Top 25 by default. 173 rows is a scroll nobody reads to the end
            // of, and the rows that matter are the ranked ones at the top —
            // 115 of them have never filled and carry no information at all.
            rows={showAllStrategies ? leaderRows : leaderRows.slice(0, LEADER_PREVIEW_ROWS)}
            // Keyed by STREAM, not by strategy.
            //
            // Rows became per (strategy, symbol) and this key did not follow,
            // so 143 rows shared about 90 keys — ANTI_M1_Break_D30_T20_Long
            // alone appears on three symbols. React reconciles by key, so the
            // duplicates made it reuse DOM nodes and render stale rows in stale
            // order: the array was sorted correctly and the table showed the
            // previous ordering, which looked exactly like a sort that had not
            // been applied.
            getRowKey={(r) => `${r.strategy}|${r.symbol}`}
            stickyHeader
            empty={
              <span style={{ color: "var(--desk-on-surface-variant)" }}>
                No strategies enabled yet.
              </span>
            }
          />
          {leaderRows.length > LEADER_PREVIEW_ROWS && (
            <div style={{ marginTop: 12, display: "flex", alignItems: "center", gap: 12 }}>
              <DeskButton variant="outlined" onClick={() => setShowAllStrategies((v) => !v)}>
                {showAllStrategies
                  ? `Show top ${LEADER_PREVIEW_ROWS} only`
                  : `View all ${leaderRows.length} strategies`}
              </DeskButton>
              {/*
                What is hidden, stated. A truncated table that does not say it
                is truncated reads as the whole roster, and this one hides 148
                rows — including every stream that has never filled.
              */}
              <span className="desk-label-md" style={{ color: "var(--desk-on-surface-variant)" }}>
                {showAllStrategies
                  ? `showing all ${leaderRows.length}`
                  : `showing the top ${LEADER_PREVIEW_ROWS} of ${leaderRows.length} · ${
                      leaderRows.length - LEADER_PREVIEW_ROWS
                    } hidden, ${leaderRows.filter((r) => r.trades === 0).length} of which have never filled`}
              </span>
            </div>
          )}
          <p style={{ marginTop: 12, fontSize: 12, color: "var(--desk-on-surface-variant)" }}>
            Built from CLOSED live positions — real fills, real fees. This is not the paper desks&rsquo;
            leaderboard: those rank strategies on model premiums, and a strategy topping one has repeatedly
            not been the same strategy that earns here. Ranked best first by net per fill, with streams under 10 fills below proven
            ones however good they look. &ldquo;Gate&rdquo; is the pre-registered go-live bar, which permission to trade
            does not imply.
          </p>
        </DeskCard>

        {/* Closed positions — what SL/TP/expiry actually took off, and its result */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Closed Positions"
            subtitle={
              allClosed.length
                ? `${allClosed.length} closed across both desks · realized ${fmtMoney(
                    allClosed.reduce((s, c) => s + (c.realizedPnl || 0), 0),
                  )}`
                : "closed by take-profit, stop-loss, time stop or expiry"
            }
            actions={
              allClosed.length > 100 ? (
                <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                  <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                    {showAllClosed ? `all ${closed.length}` : `last 100 of ${closed.length}`}
                  </span>
                  <DeskButton variant="text" onClick={() => setShowAllClosed((v) => !v)}>
                    {showAllClosed ? "Show less" : "View all"}
                  </DeskButton>
                </div>
              ) : undefined
            }
          />
          <DeskDataTable
            columns={closedColumns}
            rows={showAllClosed ? allClosed : allClosed.slice(0, 100)}
            getRowKey={(c) => c.id}
            stickyHeader
            empty={emptyNote("closed", "No closed positions yet.")}
          />
        </DeskCard>

        {/* Daily P&L — realised results per IST day */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Daily P&L"
            subtitle={
              daily.length
                ? `${daily.length} day(s) · total realized ${fmtMoney(daily.reduce((s2, d) => s2 + (d.pnlUsd || 0), 0))}`
                : "realised results per IST day, from closed positions"
            }
          />
          <DeskDataTable
            columns={dailyColumns}
            rows={daily}
            getRowKey={(d) => d.date}
            stickyHeader
            empty={emptyNote("daily", "No closed trades yet.")}
          />
        </DeskCard>

        {/* SECTION 4 — Orders / fills */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Order History"
            subtitle={showAllOrders ? `all ${orders.length}` : `showing ${Math.min(100, orders.length)} of ${orders.length}`}
            actions={
              orders.length > 100 ? (
                <DeskButton variant="text" onClick={() => setShowAllOrders((v) => !v)}>
                  {showAllOrders ? "Show less" : "View all order history"}
                </DeskButton>
              ) : undefined
            }
          />
          <DeskDataTable
            columns={orderColumns}
            rows={showAllOrders ? orders : orders.slice(0, 100)}
            getRowKey={(o) => o.id}
            stickyHeader
            empty={emptyNote("orders", "No live orders yet.")}
          />
        </DeskCard>

        {/* SECTION 5 — Live roster with gate status */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Live Roster"
            subtitle="Long-premium only · toggle Live-enabled to add/remove a strategy from live capital (reversible). Only ENABLED strategies place real orders."
          />
          <DeskDataTable
            columns={rosterColumns}
            rows={roster}
            getRowKey={(e) => e.strategy}
            stickyHeader
            empty={emptyNote("roster", "No strategies.")}
          />
        </DeskCard>

        {/* Cost transparency — the live-edge question, impossible to overlook */}
        <DeskCard>
          <DeskSectionHeader title="Cost — round-trip fee as % of premium" subtitle="Delta charges 0.03% of notional per side, capped at 10% of premium. This ratio does not improve with account size." />
          <div className="desk-metrics-row">
            <DeskMetricTile compact label="$1.78 premium" value="≈ 2%" sub="round-trip" subColor="profit" />
            <DeskMetricTile compact label="$0.45 premium" value="≈ 9%" sub="round-trip" subColor="neutral" />
            <DeskMetricTile compact label="$0.30 premium" value="≈ 13%" sub="round-trip" subColor="loss" />
            <DeskMetricTile compact label="$0.19 premium" value="≈ 20%" sub="cap binds" subColor="loss" />
          </div>
          <p className="desk-body-md" style={{ padding: "0 4px", color: "var(--desk-on-surface-variant)" }}>
            Per-contract cost against the strategy&apos;s real selected premium is populated from live Delta quotes at the
            testnet stage — the fee % of premium, not the account size, is the live-edge question.
          </p>
        </DeskCard>

        {/* The Live Engine Paper Desk card lived here.

            Moved to its own page at /live-engine-paper when it grew to four
            independent books. It stayed here reading the OLD single-book API
            shape, so when that endpoint began returning {accounts: [...]} the
            card dereferenced undefined and took the whole page down — a
            duplicate surface that nobody was maintaining, breaking the one
            that shows real money. */}

        {/* SECTION 6b — DELTA EXCHANGE, verbatim.

            Six private endpoints, unfiltered. This is the control the audit
            found missing: every other surface here is the engine checking its
            own arithmetic. */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Delta Exchange — Venue Truth"
            subtitle={
              na.venue ??
              "Straight from the exchange. Not filtered by strategy or desk; where this disagrees with the sections above, this is correct."
            }
            actions={
              <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                {venue ? ageLabel(venue.asOf) : "—"}
              </span>
            }
          />

          {venue?.errors && (
            <div
              className="desk-body-md"
              style={{
                margin: "0 0 12px",
                padding: "8px 12px",
                borderRadius: 6,
                border: "1px solid var(--desk-error)",
                color: "var(--desk-error)",
              }}
            >
              {Object.entries(venue.errors).map(([k, v]) => (
                <div key={k}>
                  <strong>{k}</strong>: {v}
                </div>
              ))}
            </div>
          )}

          {/* Balances read as tiles rather than a table — there are one or two
              assets and they are the headline number, not a list. */}
          <div className="desk-metrics-row" style={{ marginBottom: 16 }}>
            {(venue?.balances ?? []).map((b) => (
              <DeskMetricTile
                key={b.asset}
                compact
                label={`${b.asset} balance`}
                value={fmtMoney(b.balance)}
                sub={`available ${fmtMoney(b.availableBalance)}`}
              />
            ))}
            {!venue?.balances?.length && (
              <DeskMetricTile compact label="Wallet" value="—" sub="no balance returned" />
            )}
          </div>

          <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginBottom: 12 }}>
            {[
              ["positions", `Positions (${venue?.positions?.length ?? 0})`],
              ["openOrders", `Open Orders (${venue?.openOrders?.length ?? 0})`],
              ["history", `Order History (${venue?.orderHistory?.length ?? 0})`],
              ["fills", `Fills (${venue?.fills?.length ?? 0})`],
              ["ledger", `Ledger (${venue?.ledger?.length ?? 0})`],
            ].map(([id, label]) => (
              <button
                key={id}
                type="button"
                onClick={() => setVenueTab(id)}
                className="desk-label-md"
                style={{
                  cursor: "pointer",
                  padding: "5px 12px",
                  borderRadius: 6,
                  border: "1px solid var(--desk-outline)",
                  background: venueTab === id ? "var(--desk-primary)" : "transparent",
                  color: venueTab === id ? "var(--desk-on-primary)" : "var(--desk-on-surface-variant)",
                  fontWeight: 600,
                }}
              >
                {label}
              </button>
            ))}
          </div>

          {venueTab === "positions" && (
            <DeskDataTable
              columns={[
                { id: "symbol", header: "Symbol", cell: (r: VenuePosition) => r.symbol },
                { id: "size", align: "right", header: "Size", cell: (r: VenuePosition) => r.size },
                { id: "entry", align: "right", header: "Entry", cell: (r: VenuePosition) => fmtPrice(r.entryPrice) },
                {
                  id: "upnl",
                  align: "right",
                  header: "Unrealized",
                  cell: (r: VenuePosition) => (
                    <span className={pnlTone(r.unrealizedPnl ?? 0)}>{fmtMoney(r.unrealizedPnl)}</span>
                  ),
                },
                { id: "margin", align: "right", header: "Margin", cell: (r: VenuePosition) => fmtMoney(r.margin) },
                {
                  id: "liq",
                  align: "right",
                  header: "Liquidation",
                  cell: (r: VenuePosition) => r.liquidationPrice || "—",
                },
              ]}
              rows={venue?.positions ?? []}
              getRowKey={(r: VenuePosition, i: number) => `${r.symbol}-${i}`}
              minWidth={760}
              empty={<DeskEmptyStateInline text="Delta reports no open positions." />}
            />
          )}

          {venueTab === "openOrders" && (
            <DeskDataTable
              columns={[
                { id: "id", header: "Order", cell: (r: VenueOpenOrder) => r.orderId },
                { id: "symbol", header: "Symbol", cell: (r: VenueOpenOrder) => r.symbol },
                { id: "side", header: "Side", cell: (r: VenueOpenOrder) => r.side?.toUpperCase() },
                { id: "size", align: "right", header: "Size", cell: (r: VenueOpenOrder) => r.size },
                { id: "price", align: "right", header: "Price", cell: (r: VenueOpenOrder) => fmtPrice(r.price) },
                { id: "state", header: "State", cell: (r: VenueOpenOrder) => r.state },
                { id: "at", header: "Placed", cell: (r: VenueOpenOrder) => r.createdAt?.slice(0, 19).replace("T", " ") },
              ]}
              rows={venue?.openOrders ?? []}
              getRowKey={(r: VenueOpenOrder) => r.orderId}
              minWidth={820}
              empty={<DeskEmptyStateInline text="No resting orders on the venue." />}
            />
          )}

          {venueTab === "history" && (
            <DeskDataTable
              columns={[
                { id: "at", header: "Time", cell: (r: VenueHistoricalOrder) => r.createdAt?.slice(0, 19).replace("T", " ") },
                { id: "symbol", header: "Symbol", cell: (r: VenueHistoricalOrder) => r.symbol },
                { id: "side", header: "Side", cell: (r: VenueHistoricalOrder) => r.side?.toUpperCase() },
                { id: "size", align: "right", header: "Size", cell: (r: VenueHistoricalOrder) => r.size },
                {
                  id: "fill",
                  align: "right",
                  header: "Avg Fill",
                  cell: (r: VenueHistoricalOrder) => fmtPrice(r.avgFillPrice),
                },
                { id: "type", header: "Type", cell: (r: VenueHistoricalOrder) => r.orderType },
                {
                  id: "state",
                  header: "State",
                  cell: (r: VenueHistoricalOrder) => (
                    <DeskChip tone={r.state === "closed" ? "success" : r.state === "cancelled" ? "danger" : "default"}>
                      {r.state}
                    </DeskChip>
                  ),
                },
                {
                  id: "fee",
                  align: "right",
                  header: "Fee",
                  cell: (r: VenueHistoricalOrder) => fmtMoney(r.paidCommission),
                },
                {
                  // The venue's own reason, which the engine's log cannot know.
                  id: "why",
                  header: "Reason",
                  cell: (r: VenueHistoricalOrder) => r.cancelReason || (r.reduceOnly ? "reduce-only" : "—"),
                },
              ]}
              rows={venue?.orderHistory ?? []}
              getRowKey={(r: VenueHistoricalOrder) => String(r.id)}
              minWidth={980}
              empty={<DeskEmptyStateInline text="No order history returned." />}
            />
          )}

          {venueTab === "fills" && (
            <DeskDataTable
              columns={[
                { id: "at", header: "Time", cell: (r: VenueFill) => r.createdAt?.slice(0, 19).replace("T", " ") },
                { id: "symbol", header: "Symbol", cell: (r: VenueFill) => r.symbol },
                { id: "side", header: "Side", cell: (r: VenueFill) => r.side?.toUpperCase() },
                { id: "size", align: "right", header: "Size", cell: (r: VenueFill) => r.size },
                { id: "price", align: "right", header: "Fill Price", cell: (r: VenueFill) => fmtPrice(r.price) },
                {
                  id: "role",
                  header: "Role",
                  // Taker vs maker is the difference between paying 0.059% and
                  // earning a rebate. The bridge places taker orders only, so
                  // anything else here is a finding.
                  cell: (r: VenueFill) => <DeskChip tone={r.role === "taker" ? "default" : "success"}>{r.role}</DeskChip>,
                },
                {
                  id: "fee",
                  align: "right",
                  header: "Commission",
                  cell: (r: VenueFill) => <span className="desk-pnl-negative">{fmtMoney(r.commission)}</span>,
                },
              ]}
              rows={venue?.fills ?? []}
              getRowKey={(r: VenueFill) => r.id}
              minWidth={860}
              empty={<DeskEmptyStateInline text="No fills returned." />}
            />
          )}

          {venueTab === "ledger" && (
            <DeskDataTable
              columns={[
                { id: "at", header: "Time", cell: (r: VenueLedger) => r.createdAt?.slice(0, 19).replace("T", " ") },
                {
                  id: "type",
                  header: "Type",
                  // FUNDING is the reason this tab exists. Perp funding is
                  // charged every 8h on any position held across the window and
                  // appears in NO other endpoint — not fills, not P&L.
                  cell: (r: VenueLedger) => (
                    <DeskChip tone={r.type?.includes("funding") ? "primary" : "default"}>{r.type}</DeskChip>
                  ),
                },
                { id: "product", header: "Product", cell: (r: VenueLedger) => r.productName || "—" },
                {
                  id: "amount",
                  align: "right",
                  header: "Amount",
                  cell: (r: VenueLedger) => <span className={pnlTone(r.amount)}>{fmtMoney(r.amount)}</span>,
                },
                { id: "balance", align: "right", header: "Balance After", cell: (r: VenueLedger) => fmtMoney(r.balance) },
                { id: "asset", header: "Asset", cell: (r: VenueLedger) => r.asset },
              ]}
              rows={venue?.ledger ?? []}
              getRowKey={(r: VenueLedger) => String(r.id)}
              minWidth={820}
              empty={<DeskEmptyStateInline text="No wallet transactions returned." />}
            />
          )}
        </DeskCard>

        {/* SECTION 7 — Audit log */}
        <DeskCard padding="md">
          <DeskSectionHeader title="Audit Log" subtitle="Every arm, disarm, auto-disarm, close-all and roster change — actor + timestamp" />
          <DeskDataTable
            columns={auditColumns}
            rows={[...audit].reverse().slice(0, 100)}
            getRowKey={(a, i) => `${a.at}-${i}`}
            stickyHeader
            empty={emptyNote("audit", "No audit entries yet.")}
          />
        </DeskCard>

        <p className="desk-label-md" style={{ textAlign: "center", padding: "8px 0 24px", color: "var(--desk-on-surface-variant)" }}>
          Real money · $100 ceiling enforced in the engine · long premium only · every order passes the risk gate, OMS,
          and kill switch · auto-disarm is one-way
        </p>
      </main>

    </div>
  );
}
