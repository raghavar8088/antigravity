"use client";

/**
 * Live DEMO Engine — demo.delta.exchange, NOT real money.
 *
 * A clone of the Live Engine page pointed at the demo venue. Same layout, same
 * columns, same controls, so that what is rehearsed here is what runs there.
 *
 * Two things are deliberately NOT shared with the live page:
 *
 *   The venue. This reads a separate engine process on port 8095 holding demo
 *   credentials. A single process with a mode flag would be one misread
 *   environment variable away from routing a demo order to the real wallet.
 *
 *   The roster. The demo venue lists 16 perpetuals and NONE of the ten the
 *   live engine trades, so the streams are re-pointed onto demo majors
 *   (BTC/ETH/SOL/XRP/DOGE/ADA/ONDO/1000SHIB). Strategy logic and pairing
 *   structure are identical; only the symbol differs. An exact roster would
 *   have placed zero orders forever.
 *
 * Results here therefore rehearse the PLUMBING - arming, sizing, bracket
 * placement, fee accounting, reconciliation - not the edge. Demo fills come
 * from a different book with different liquidity, so a profitable demo record
 * is not evidence that the same streams profit live.
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
  exitReason?: string;
  status: string;
};

type PerpStats = {
  armed: boolean;
  equityUsd: number;
  riskPerTradeUsd: number;
  strategies: string[];
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
  trades: number;
  wins: number;
  winRatePct: number;
  grossUsd: number;
  feesUsd: number;
  netUsd: number;
  /** Fees as a share of gross profit — the figure that decided the options desk. */
  feeDragPct: number;
  allowed: boolean;
  live: boolean;
  reason: string;
};

function fmtUSD(v: number | undefined): string {
  if (v === undefined || Number.isNaN(v)) return "—";
  const abs = Math.abs(v);
  // Option premiums here are cents-scale; 2dp would round a real move to $0.00.
  const dp = abs > 0 && abs < 1 ? 4 : 2;
  return `${v < 0 ? "-" : ""}$${abs.toFixed(dp)}`;
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


  const refresh = useCallback(async () => {
    try {
      const [st, ac, po, cp, dp, or, ro, ps, pt, rc, vn, au] = await Promise.all([
        fetch("/api/live-demo-engine/state", { cache: "no-store" }),
        fetch("/api/live-demo-engine/account", { cache: "no-store" }),
        fetch("/api/live-demo-engine/positions", { cache: "no-store" }),
        fetch("/api/live-demo-engine/closed-positions", { cache: "no-store" }),
        fetch("/api/live-demo-engine/daily-pnl", { cache: "no-store" }),
        fetch("/api/live-demo-engine/orders", { cache: "no-store" }),
        fetch("/api/live-demo-engine/roster", { cache: "no-store" }),
        // The scalp desk's real-money perpetual arm. Same Delta wallet,
        // different desk — a board showing only options would report half
        // the live exposure while looking complete.
        fetch("/api/scalp-demo/scalp/live/stats", { cache: "no-store" }),
        fetch("/api/scalp-demo/scalp/live/trades", { cache: "no-store" }),
        fetch("/api/live-demo-engine/reconciliation", { cache: "no-store" }),
        fetch("/api/live-demo-engine/venue", { cache: "no-store" }),
        fetch("/api/live-demo-engine/audit", { cache: "no-store" }),
      ]);
      if (!st.ok) {
        setError(`control plane unreachable (HTTP ${st.status})`);
        return;
      }
      setState(await st.json());
      if (ac.ok) setAccount(await ac.json());
      if (po.ok) setPositions((await po.json()) as Position[]);
      if (cp.ok) setClosed((await cp.json()) as ClosedPosition[]);
      if (dp.ok) setDaily((await dp.json()) as DailyPnl[]);
      if (or.ok) setOrders((await or.json()) as Order[]);
      if (ro.ok) setRoster((await ro.json()) as Eligibility[]);
      if (ps.ok) {
        const body = (await ps.json()) as { enabled?: boolean; stats?: PerpStats };
        setPerp(body.enabled ? (body.stats ?? null) : null);
      }
      if (pt.ok) setPerpTrades(((await pt.json()) as PerpTrade[]) ?? []);
      if (rc.ok) setRecon(await rc.json());
      // Venue truth is additive: if Delta is unreachable the rest of the page
      // must keep working, and the section says so rather than showing empty
      // tables that read as "nothing happened".
      if (vn.ok) setVenue((await vn.json()) as VenuePayload);
      if (au.ok) setAudit(((await au.json()) as { entries: AuditEntry[] }).entries ?? []);
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
      strategy: p.strategy,
    }));
    for (const t of perp?.openPositions ?? []) {
      out.push({
        desk: "scalp",
        symbol: t.symbol,
        side: t.side,
        size: t.contracts,
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
          p.ifTargetUsd ? <span className="desk-pnl-positive">{fmtUSD(p.ifTargetUsd)}</span> : "—",
      },
      {
        id: "ifstop",
        align: "right",
        header: "If SL",
        cell: (p) => (p.ifStopUsd ? <span className="desk-pnl-negative">{fmtUSD(p.ifStopUsd)}</span> : "—"),
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
        cell: (p) => <span className={pnlTone(p.unrealizedPnl)}>{fmtUSD(p.unrealizedPnl)}</span>,
      },
    ],
    [],
  );

  const dailyColumns: DeskColumn<DailyPnl>[] = useMemo(
    () => [
      { id: "date", header: "Date (IST)", cell: (d) => fmtISTDayLabel(d.date) },
      { id: "cap", align: "right", header: "Capital used", cell: (d) => fmtUSD(d.capitalUsd) },
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
              {fmtUSD(d.feesUsd)}
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
            title={d.grossPnlUsd !== undefined ? `Gross ${fmtUSD(d.grossPnlUsd)} before fees` : undefined}
          >
            {fmtUSD(d.pnlUsd)}
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
                ? `Gross ${fmtUSD(c.grossPnl)} − fees ${fmtUSD(c.feesUsd)}`
                : undefined
            }
          >
            {fmtUSD(c.realizedPnl)}
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
      { id: "prem", align: "right", header: "Premium", cell: (o) => fmtUSD(o.premiumUsd) },
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
      const blankPerp = (name: string): LeaderRow => ({
        desk: "scalp",
        strategy: name,
        trades: 0,
        wins: 0,
        winRatePct: 0,
        grossUsd: 0,
        feesUsd: 0,
        netUsd: 0,
        feeDragPct: 0,
        allowed: true,
        // The scalp desk's promotion gate passes none of these; the bridge
        // trades them on owner instruction, not on a gate verdict.
        live: false,
        reason: "scalp perpetual — owner-selected, gate not passed",
      });

      for (const name of perp.strategies ?? []) perpRows.set(name, blankPerp(name));
      for (const t of perpTrades) {
        if (t.status !== "CLOSED") continue;
        const row = perpRows.get(t.strategy) ?? blankPerp(t.strategy);
        row.trades += 1;
        const net = t.realisedPnl ?? 0;
        if (net > 0) row.wins += 1;
        // The perpetual bridge books P&L net; it does not split out fees, so
        // gross is reported as net rather than invented.
        row.netUsd += net;
        row.grossUsd += net;
        perpRows.set(t.strategy, row);
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

    // Traded strategies first, worst-to-best by net so losses are not buried
    // below a scroll; untraded rows last.
    rows.sort((a, b) => {
      if ((a.trades > 0) !== (b.trades > 0)) return a.trades > 0 ? -1 : 1;
      return a.netUsd - b.netUsd;
    });
    return rows;
  }, [perp, perpTrades]);

  const leaderColumns: DeskColumn<LeaderRow>[] = useMemo(
    () => [
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
      { id: "trades", header: "Fills", align: "right", cell: (r) => r.trades || "—" },
      {
        id: "wr",
        header: "WR %",
        align: "right",
        cell: (r) => (r.trades > 0 ? r.winRatePct.toFixed(1) : "—"),
      },
      {
        id: "gross",
        header: "Gross $",
        align: "right",
        cell: (r) => (r.trades > 0 ? <span className={pnlTone(r.grossUsd)}>{fmtUSD(r.grossUsd)}</span> : "—"),
      },
      {
        id: "fees",
        header: "Fees $",
        align: "right",
        cell: (r) => (r.trades > 0 ? fmtUSD(r.feesUsd) : "—"),
      },
      {
        id: "net",
        header: "Net $",
        align: "right",
        cell: (r) => (r.trades > 0 ? <span className={pnlTone(r.netUsd)}>{fmtUSD(r.netUsd)}</span> : "—"),
      },
      {
        id: "drag",
        header: "Fee drag",
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
    [],
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
            <span className="desk-body-md" style={{ fontWeight: 500 }}>Live Demo Engine</span>
          </div>
          <div style={{ marginTop: 8, display: "flex", flexWrap: "wrap", alignItems: "center", gap: 12 }}>
            <h1 className="desk-display-lg" style={{ fontSize: "2rem" }}>Live Demo Engine</h1>
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
                // Purple in every state, never the live page's green/red. The
                // badge answers "which wallet is this?" before it answers
                // "is it armed?" — the armed state is shown by the arm control
                // itself, and a demo page that can flash the same green as the
                // real-money one is a page you can misread at a glance.
                background: "#7c5cff",
              }}
            >
              DEMO — NOT REAL MONEY
            </span>
          </div>
          <p className="desk-body-md" style={{ marginTop: 6, maxWidth: 760, color: "var(--desk-on-surface-variant)" }}>
            Paper-equivalent trading on <strong>demo.delta.exchange</strong> — the demo wallet, never the real one.
            Same engine, same sizing, same taker fees and bracket placement as the Live Engine, so what is rehearsed
            here is what runs there. The demo venue lists 16 perpetuals and <strong>none</strong> of the ten the live
            engine trades, so the 31 streams are re-pointed onto demo majors (BTC, ETH, SOL, XRP, DOGE, ADA, ONDO,
            1000SHIB) — identical strategy logic, different symbols. That makes this a rehearsal of the plumbing, not
            of the edge: demo fills come from a different book, so a profitable record here is not evidence the same
            streams profit live.
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
            subtitle={account ? `${account.source} · ${ageLabel(account.asOf)}${account.stale ? " · STALE" : ""}` : "—"}
          />
          <div className="desk-metrics-row">
            <DeskMetricTile label="Real Equity" value={account ? fmtUSD(account.equityUsd) : "—"} sub={account?.stale ? "stale — see source" : "delta wallet"} highlight />
            <DeskMetricTile label={`Tradable (≤ $${CEILING})`} value={account ? fmtUSD(account.tradableUsd) : "—"} sub="ceiling enforced server-side" />
            <DeskMetricTile label="Available" value={account ? fmtUSD(account.availableUsd) : "—"} sub="equity − margin" />
            <DeskMetricTile label="Margin Used" value={account ? fmtUSD(account.marginUsedUsd) : "—"} sub="delta positions" />
            <DeskMetricTile label="Open Risk" value={account ? fmtUSD(account.openRiskUsd) : "—"} sub="premium at risk (long)" />
            <DeskMetricTile label="Realized Today" value={account ? fmtUSD(account.realizedTodayUsd) : "—"} valueClassName={account ? pnlTone(account.realizedTodayUsd) : undefined} sub="to daily breaker" />
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
                ? `${leaderRows.filter((r) => r.trades > 0).length} of ${leaderRows.length} perpetual strategies have filled · realized ${fmtUSD(
                    leaderRows.reduce((s, r) => s + r.netUsd, 0),
                  )}`
                : "every strategy enabled on the perpetual desk, ranked once it has real fills"
            }
          />
          <DeskDataTable
            columns={leaderColumns}
            rows={leaderRows}
            getRowKey={(r) => r.strategy}
            stickyHeader
            empty={
              <span style={{ color: "var(--desk-on-surface-variant)" }}>
                No strategies enabled yet.
              </span>
            }
          />
          <p style={{ marginTop: 12, fontSize: 12, color: "var(--desk-on-surface-variant)" }}>
            Built from CLOSED live positions — real fills, real fees. This is not the paper desks&rsquo;
            leaderboard: those rank strategies on model premiums, and a strategy topping one has repeatedly
            not been the same strategy that earns here. Ranked worst net first so losses are not buried
            below a scroll. &ldquo;Gate&rdquo; is the pre-registered go-live bar, which permission to trade
            does not imply.
          </p>
        </DeskCard>

        {/* Closed positions — what SL/TP/expiry actually took off, and its result */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Closed Positions"
            subtitle={
              allClosed.length
                ? `${allClosed.length} closed across both desks · realized ${fmtUSD(
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
            empty={<span style={{ color: "var(--desk-on-surface-variant)" }}>No closed positions yet.</span>}
          />
        </DeskCard>

        {/* Daily P&L — realised results per IST day */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Daily P&L"
            subtitle={
              daily.length
                ? `${daily.length} day(s) · total realized ${fmtUSD(daily.reduce((s2, d) => s2 + (d.pnlUsd || 0), 0))}`
                : "realised results per IST day, from closed positions"
            }
          />
          <DeskDataTable
            columns={dailyColumns}
            rows={daily}
            getRowKey={(d) => d.date}
            stickyHeader
            empty={<span style={{ color: "var(--desk-on-surface-variant)" }}>No closed trades yet.</span>}
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
            empty={<span style={{ color: "var(--desk-on-surface-variant)" }}>No live orders yet.</span>}
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
            empty={<span style={{ color: "var(--desk-on-surface-variant)" }}>No strategies.</span>}
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
            subtitle="Straight from the exchange. Not filtered by strategy or desk; where this disagrees with the sections above, this is correct."
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
                value={fmtUSD(b.balance)}
                sub={`available ${fmtUSD(b.availableBalance)}`}
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
                    <span className={pnlTone(r.unrealizedPnl ?? 0)}>{fmtUSD(r.unrealizedPnl)}</span>
                  ),
                },
                { id: "margin", align: "right", header: "Margin", cell: (r: VenuePosition) => fmtUSD(r.margin) },
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
                  cell: (r: VenueHistoricalOrder) => fmtUSD(r.paidCommission),
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
                  cell: (r: VenueFill) => <span className="desk-pnl-negative">{fmtUSD(r.commission)}</span>,
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
                  cell: (r: VenueLedger) => <span className={pnlTone(r.amount)}>{fmtUSD(r.amount)}</span>,
                },
                { id: "balance", align: "right", header: "Balance After", cell: (r: VenueLedger) => fmtUSD(r.balance) },
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
            empty={<span style={{ color: "var(--desk-on-surface-variant)" }}>No audit entries yet.</span>}
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
