"use client";

/**
 * High Volume Crypto Paper Trading — the owner's nine streams, on one $500 book.
 *
 * The desk tab next door ranks 20,088 streams. A leaderboard is a sort, not a
 * test, so the nine rows at the top of it earned their place while sharing three
 * concurrency slots with 20,079 others: most of their signals were refused, and
 * the trades they did take are the ones that arrived when a slot happened to be
 * free. That is a frequency property, not a quality one, and it is the reason
 * this tab exists — here the nine compete only with each other, out of a single
 * balance, and what comes back is a record of them rather than a slice of a
 * board.
 *
 * ONE BALANCE, NOT NINE. $500 total, shared. The sibling books learned this the
 * expensive way: per-strategy accounts multiplied the capital, so nine streams
 * meant $4,500 deployed while every row reported as though it owned the whole
 * book. A win here really does fund the next position elsewhere, exactly as one
 * wallet does.
 *
 * PAPER, STRUCTURALLY. Membership of this book grants nothing at the venue —
 * that is decided by the live roster alone, which does not read this book. The
 * prices, the taker fee on both legs, the margin rules and the size caps are the
 * real ones; only the money is simulated.
 *
 * The record starts at zero on the day the book is created. Nothing from the
 * leaderboard carries over, because those trades were taken under different
 * rules against different competition, and a history recorded under two rule
 * sets is worse than no history — it still looks complete.
 */

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  DeskBanner,
  DeskCard,
  DeskChip,
  DeskDataTable,
  DeskLinearProgress,
  DeskMetricTile,
  DeskSectionHeader,
  StatusBadge,
  type DeskColumn,
  type DeskEngineStatus,
} from "@/components/desk/ui";
import { fmtIST, fmtISTClock } from "@/lib/istTime";

/** The engine's id for this book. Must match delta.PaperAccountHighVol. */
const BOOK_ID = "HVPAPER";

type Stream = {
  strategy: string;
  symbol: string;
  live: boolean;
  trades: number;
  wins: number;
  grossUsd: number;
  feesUsd: number;
  netUsd: number;
  shareOfEquityPct: number;
};
type OpenPos = {
  strategy: string;
  symbol: string;
  dir: string;
  entry: number;
  stop: number;
  target: number;
  contracts: number;
  openedAt: string;
  mark: number;
  unrealisedUsd: number;
  unrealisedPct: number;
};
type ClosedTrade = {
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
type Book = {
  account: string;
  startingEquityUsd: number;
  equityUsd: number;
  netUsd: number;
  roiPct: number;
  openNotionalUsd: number;
  openUnrealisedUsd: number;
  maxNotionalUsd: number;
  maxConcurrent: number;
  maxLeverage: number;
  positionUsd: number;
  productLeverage: number;
  liquidationDistPct: number;
  feeRatePerSide: number;
  accounts?: Stream[];
  openPositions?: OpenPos[];
  recentTrades?: ClosedTrade[];
  uptimeMin: number;
};

/**
 * The selection, with the record each row carried when it was chosen.
 *
 * Held on the page as well as in the engine because the two answer different
 * questions. The engine's roster decides what trades; this is the evidence the
 * choice was made on, and it has to stay visible beside the live figures — a
 * stream at +$24.56 on the board and -$8 in the book is the interesting case,
 * and it is unreadable if the first number is gone by the time the second
 * arrives.
 */
const SELECTED_AT: Record<string, { rank: number; n: number; wr: number; net: number }> = {
  "MTF_1h_LevelRetest_Long|ADAUSD": { rank: 2, n: 19, wr: 77, net: 24.56 },
  "MTF_1d_OpeningRangeBreak_Long|DOGEUSD": { rank: 3, n: 35, wr: 79, net: 23.29 },
  "MTF_4h_LevelRetest_Long|HYPEUSD": { rank: 4, n: 18, wr: 77, net: 22.79 },
  "MTF_4h_LinRegBreak_Long|LTCUSD": { rank: 11, n: 37, wr: 80, net: 15.17 },
  "MTF_10m_DirectionalTriangle_Long|UNIUSD": { rank: 15, n: 18, wr: 77, net: 14.04 },
  "MTF_10m_OpeningRangeBreak_Long|UNIUSD": { rank: 34, n: 34, wr: 79, net: 11.04 },
  "MTF_4h_TripleTopBottom_Long|AVAXUSD": { rank: 35, n: 21, wr: 78, net: 11.0 },
  "MTF_10m_OptimalTradeEntry_Long|UNIUSD": { rank: 37, n: 26, wr: 78, net: 10.78 },
  "MTF_10m_FairValueGap_Long|LTCUSD": { rank: 40, n: 39, wr: 80, net: 10.64 },
};

function fmtUSD(v: number | undefined): string {
  if (v === undefined || Number.isNaN(v)) return "—";
  const abs = Math.abs(v);
  const dp = abs > 0 && abs < 1 ? 4 : 2;
  return `${v < 0 ? "-" : ""}$${abs.toFixed(dp)}`;
}
function fmtSigned(v: number | undefined): string {
  if (v === undefined || Number.isNaN(v)) return "—";
  return `${v < 0 ? "-" : "+"}$${Math.abs(v).toFixed(2)}`;
}
/** These symbols span DOGE at $0.2 and BTC at $86,000, so precision is by size. */
function fmtPrice(v: number | undefined): string {
  if (v === undefined || Number.isNaN(v) || v === 0) return "—";
  if (v >= 1000) return v.toLocaleString("en-US", { maximumFractionDigits: 1 });
  if (v >= 1) return v.toFixed(3);
  return v.toFixed(5);
}
function pnlTone(v: number): string {
  return v > 0 ? "desk-pnl-positive" : v < 0 ? "desk-pnl-negative" : "desk-pnl-neutral";
}
function ageLabel(iso?: string): string {
  if (!iso) return "—";
  const ms = Date.now() - new Date(iso).getTime();
  if (Number.isNaN(ms)) return "—";
  const s = Math.max(0, Math.round(ms / 1000));
  if (s < 60) return `${s}s ago`;
  const m = Math.round(s / 60);
  return m < 90 ? `${m}m ago` : `${Math.round(m / 60)}h ago`;
}
function key(strategy: string, symbol: string): string {
  return `${strategy}|${symbol}`;
}

export default function PaperTradingTab() {
  const [book, setBook] = useState<Book | null>(null);
  const [error, setError] = useState<string>("");
  const [missing, setMissing] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(true);
  const [updatedAt, setUpdatedAt] = useState<string>("");

  const refresh = useCallback(async () => {
    try {
      const r = await fetch("/api/scalp-highvol/scalp/live/paper", { cache: "no-store" });
      if (!r.ok) {
        setError(`desk unreachable (HTTP ${r.status})`);
        return;
      }
      const body = (await r.json()) as { accounts?: Book[] };
      const b = (body.accounts ?? []).find((a) => a.account === BOOK_ID) ?? null;
      setBook(b);
      // "The engine has no such book" and "the book has not traded yet" render
      // identically as an empty page and mean opposite things — one is a
      // deployment that did not take, the other is a quiet market. Reported
      // separately from a reach failure for the same reason.
      setMissing(!b);
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
    const t = setInterval(() => void refresh(), 20_000);
    return () => clearInterval(t);
  }, [refresh]);

  const streams = book?.accounts ?? [];
  const open = book?.openPositions ?? [];
  const closed = book?.recentTrades ?? [];
  const traded = streams.filter((s) => s.trades > 0);
  const trades = streams.reduce((a, x) => a + x.trades, 0);
  const wins = streams.reduce((a, x) => a + x.wins, 0);
  const gross = streams.reduce((a, x) => a + x.grossUsd, 0);
  const fees = streams.reduce((a, x) => a + x.feesUsd, 0);
  const status: DeskEngineStatus = error || missing ? "degraded" : book ? "live" : "syncing";
  const slots = book?.maxConcurrent ?? 3;

  const streamColumns: DeskColumn<Stream>[] = [
    {
      id: "strategy",
      header: "Strategy",
      cell: (r) => <span style={{ fontWeight: 600 }}>{r.strategy}</span>,
    },
    { id: "symbol", header: "Symbol", cell: (r) => <DeskChip tone="default">{r.symbol.replace(/USD$/, "")}</DeskChip> },
    {
      id: "picked",
      align: "right",
      header: "Board Rank",
      // Where it sat on the desk board when it was chosen. Kept beside the live
      // figures so the selection can be judged rather than assumed.
      cell: (r) => {
        const s = SELECTED_AT[key(r.strategy, r.symbol)];
        return s ? <span className="desk-mono">#{s.rank}</span> : <span style={{ opacity: 0.5 }}>—</span>;
      },
    },
    {
      id: "then",
      align: "right",
      header: "Net at Selection",
      cell: (r) => {
        const s = SELECTED_AT[key(r.strategy, r.symbol)];
        return s ? (
          <span className="desk-mono" style={{ opacity: 0.75 }}>
            {fmtSigned(s.net)} <span style={{ fontSize: "0.75em" }}>({s.n}t)</span>
          </span>
        ) : (
          "—"
        );
      },
    },
    { id: "n", align: "right", header: "Trades", cell: (r) => (r.trades ? r.trades : <span style={{ opacity: 0.5 }}>0</span>) },
    { id: "wr", align: "right", header: "WR %", cell: (r) => (r.trades ? ((100 * r.wins) / r.trades).toFixed(1) : "—") },
    { id: "gross", align: "right", header: "Gross", cell: (r) => <span className={pnlTone(r.grossUsd)}>{fmtUSD(r.grossUsd)}</span> },
    {
      id: "fees",
      align: "right",
      header: "− Taker Fees",
      cell: (r) => (r.feesUsd ? <span className="desk-pnl-negative">{fmtUSD(-r.feesUsd)}</span> : "—"),
    },
    {
      id: "net",
      align: "right",
      header: "= Net Now",
      cell: (r) => (
        <span className={pnlTone(r.netUsd)} style={{ fontWeight: 700 }}>
          {r.trades ? fmtUSD(r.netUsd) : "—"}
        </span>
      ),
    },
    {
      id: "drag",
      align: "right",
      header: "Fee Drag",
      // The share of gross handed to the venue. It decided every result on the
      // sibling desks and it is the first number to look at here.
      cell: (r) => (r.grossUsd > 0 ? `${((r.feesUsd / r.grossUsd) * 100).toFixed(0)}%` : "—"),
    },
    {
      id: "share",
      align: "right",
      header: "Share of Book",
      // Against the $500, not against an imaginary per-strategy account.
      cell: (r) =>
        r.trades ? (
          <span className={pnlTone(r.shareOfEquityPct)}>
            {`${r.shareOfEquityPct >= 0 ? "+" : ""}${r.shareOfEquityPct.toFixed(2)}%`}
          </span>
        ) : (
          "—"
        ),
    },
    {
      id: "verdict",
      header: "Worth Real Money?",
      cell: (r) =>
        r.trades === 0 ? (
          <DeskChip tone="default">no fills yet</DeskChip>
        ) : r.trades < 200 ? (
          <DeskChip tone="default">too few trades</DeskChip>
        ) : r.netUsd > 0 ? (
          <DeskChip tone="success" style={{ fontWeight: 700 }}>
            positive
          </DeskChip>
        ) : (
          <DeskChip tone="danger">negative</DeskChip>
        ),
    },
  ];

  const openColumns: DeskColumn<OpenPos>[] = [
    { id: "strategy", header: "Strategy", cell: (r) => r.strategy },
    { id: "symbol", header: "Symbol", cell: (r) => r.symbol.replace(/USD$/, "") },
    {
      id: "dir",
      header: "Side",
      cell: (r) => (
        <DeskChip tone={r.dir?.toUpperCase() === "LONG" ? "success" : "danger"}>{(r.dir || "?").toUpperCase()}</DeskChip>
      ),
    },
    { id: "entry", align: "right", header: "Entry", cell: (r) => fmtPrice(r.entry) },
    { id: "mark", align: "right", header: "Mark", cell: (r) => fmtPrice(r.mark) },
    { id: "stop", align: "right", header: "Stop", cell: (r) => fmtPrice(r.stop) },
    { id: "target", align: "right", header: "Target", cell: (r) => fmtPrice(r.target) },
    {
      id: "notional",
      align: "right",
      header: "Notional",
      cell: (r) => <span className="desk-mono">{fmtUSD(r.entry * r.contracts)}</span>,
    },
    {
      id: "upnl",
      align: "right",
      header: "Unrealised",
      // NET of the fee the exit will pay. Gross would show a winner on trades
      // that book a loss, which is the discrepancy these books exist to stop.
      cell: (r) =>
        r.mark > 0 ? (
          <span className={pnlTone(r.unrealisedUsd)} style={{ fontWeight: 700 }}>
            {fmtUSD(r.unrealisedUsd)}
          </span>
        ) : (
          "—"
        ),
    },
    {
      id: "upct",
      align: "right",
      header: "Move %",
      cell: (r) =>
        r.mark > 0 ? (
          <span className={pnlTone(r.unrealisedPct)}>{`${r.unrealisedPct >= 0 ? "+" : ""}${r.unrealisedPct.toFixed(3)}%`}</span>
        ) : (
          "—"
        ),
    },
    { id: "at", header: "Opened", cell: (r) => ageLabel(r.openedAt) },
  ];

  const tradeColumns: DeskColumn<ClosedTrade>[] = [
    { id: "at", header: "Closed (IST)", cell: (r) => fmtIST(r.closedAt) },
    { id: "strategy", header: "Strategy", cell: (r) => r.strategy },
    { id: "symbol", header: "Symbol", cell: (r) => r.symbol.replace(/USD$/, "") },
    { id: "dir", header: "Side", cell: (r) => (r.dir || "").toUpperCase() },
    { id: "entry", align: "right", header: "Entry", cell: (r) => fmtPrice(r.entry) },
    { id: "exit", align: "right", header: "Exit", cell: (r) => fmtPrice(r.exit) },
    {
      id: "reason",
      header: "Exit",
      cell: (r) => (
        <DeskChip tone={r.reason === "TP" ? "success" : r.reason === "SL" || r.reason === "LIQUIDATED" ? "danger" : "default"}>
          {r.reason}
        </DeskChip>
      ),
    },
    { id: "hold", align: "right", header: "Held", cell: (r) => `${r.holdMin}m` },
    { id: "gross", align: "right", header: "Gross", cell: (r) => <span className={pnlTone(r.grossUsd)}>{fmtUSD(r.grossUsd)}</span> },
    { id: "fees", align: "right", header: "− Fees", cell: (r) => <span className="desk-pnl-negative">{fmtUSD(-r.feesUsd)}</span> },
    {
      id: "net",
      align: "right",
      header: "= Net",
      cell: (r) => (
        <span className={pnlTone(r.netUsd)} style={{ fontWeight: 600 }}>
          {fmtUSD(r.netUsd)}
        </span>
      ),
    },
  ];

  return (
    <div style={{ minHeight: "100%", background: "var(--desk-surface-dim)" }}>
      <DeskLinearProgress visible={loading} />
      <main className="desk-page">
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 6, fontSize: "0.8125rem" }}>
            <Link href="/terminal" className="desk-label-md" style={{ fontWeight: 400, textDecoration: "none" }}>
              Home
            </Link>
            <span style={{ color: "var(--desk-outline)" }}>›</span>
            <span className="desk-label-md" style={{ fontWeight: 400 }}>
              High Volume Crypto Trading
            </span>
            <span style={{ color: "var(--desk-outline)" }}>›</span>
            <span className="desk-body-md" style={{ fontWeight: 500 }}>
              Paper Trading
            </span>
          </div>
          <div style={{ marginTop: 8, display: "flex", flexWrap: "wrap", alignItems: "center", gap: 12 }}>
            <h1 className="desk-display-lg" style={{ fontSize: "2rem" }}>
              High Volume Crypto Paper Trading
            </h1>
            <StatusBadge status={status} />
            <DeskChip tone="primary" style={{ fontWeight: 700 }}>
              PAPER MONEY · ${book?.startingEquityUsd?.toFixed(0) ?? 500} BOOK
            </DeskChip>
          </div>
          <p className="desk-body-md" style={{ marginTop: 6, maxWidth: 900, color: "var(--desk-on-surface-variant)" }}>
            Nine streams picked off the desk board next door, trading a single{" "}
            <strong>${book?.startingEquityUsd?.toFixed(0) ?? 500}</strong> balance at real Delta prices with Delta&apos;s
            real taker fee on both legs. One book, not nine — a win here funds the next position, exactly as one wallet
            does. Only the money is simulated: the prices, fees, size caps and margin rules are the ones the real
            account runs under.
          </p>
        </div>

        {error && <DeskBanner variant="warning">{error} — retrying every 20s</DeskBanner>}
        {missing && !error && (
          <DeskBanner variant="warning">
            No <strong>{BOOK_ID}</strong> book in the engine payload. The desk is running a build without this module,
            or the container was started without <code>SCALP_HV_PAPER=true</code>. This is a deployment state, not a
            quiet market — nothing is being traded on these nine streams.
          </DeskBanner>
        )}

        <DeskBanner variant="info">
          <strong>A leaderboard is a sort, not a test.</strong> These nine were the top rows of 20,088 streams on a desk
          that was <strong>-$21,837 over 45,158 trades</strong> at the time, with 18-39 closed trades each against a
          gate that asks for 200. That is what the right tail of a large sample looks like. This book exists to find out
          whether it is more than that — and being in it grants nothing at the venue.
        </DeskBanner>

        <DeskCard>
          <DeskSectionHeader
            title="The Book"
            subtitle="One shared balance across all nine streams — ring-fenced from every other book, so neither can fund the other."
            actions={
              <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                {updatedAt ? `updated ${updatedAt}` : "—"}
              </span>
            }
          />
          <div className="desk-metrics-row">
            <DeskMetricTile
              label="Book Balance"
              value={book ? `$${book.equityUsd.toFixed(2)}` : "—"}
              valueClassName={book ? pnlTone(book.netUsd) : undefined}
              sub={`started at $${book?.startingEquityUsd?.toFixed(0) ?? 500} · ${streams.length} streams watched`}
              highlight
            />
            <DeskMetricTile
              compact
              label="Net P&L"
              value={book ? fmtUSD(book.netUsd) : "—"}
              valueClassName={book ? pnlTone(book.netUsd) : undefined}
              sub={book ? `${book.roiPct >= 0 ? "+" : ""}${book.roiPct.toFixed(2)}% · gross ${fmtUSD(gross)}` : "—"}
            />
            <DeskMetricTile
              compact
              label="Taker Fees"
              value={fees ? fmtUSD(-fees) : "—"}
              valueClassName={fees > 0 ? "desk-pnl-negative" : undefined}
              sub={gross > 0 ? `${((fees / gross) * 100).toFixed(0)}% of gross` : "both legs, on every round trip"}
            />
            <DeskMetricTile
              compact
              label="Open P&L"
              value={book ? fmtUSD(book.openUnrealisedUsd) : "—"}
              valueClassName={book ? pnlTone(book.openUnrealisedUsd) : undefined}
              sub={`${open.length} open · ${fmtUSD(book?.openNotionalUsd)} deployed`}
            />
            <DeskMetricTile
              compact
              label="Trades"
              value={trades ? String(trades) : "—"}
              sub={trades ? `win rate ${((100 * wins) / trades).toFixed(1)}%` : "waiting for the first signal"}
            />
            <DeskMetricTile
              compact
              label="Uptime"
              value={book ? `${Math.floor(book.uptimeMin / 60)}h ${book.uptimeMin % 60}m` : "—"}
              sub="since the book was opened"
            />
          </div>

          {/* The rules the book spends under, stated as numbers rather than
              left for a reader to infer from a position size. */}
          <div style={{ marginTop: 14, display: "flex", flexWrap: "wrap", gap: 8 }}>
            <DeskChip tone="default">
              {book?.maxConcurrent === -1 ? "unlimited concurrent positions" : `${slots} concurrent positions`}
            </DeskChip>
            <DeskChip tone="default">
              {book?.positionUsd && book.positionUsd > 0 ? `${fmtUSD(book.positionUsd)} per position` : "per-position size from the budget"}
            </DeskChip>
            <DeskChip tone="default">
              {book?.maxNotionalUsd && book.maxNotionalUsd > 0
                ? `${fmtUSD(book.maxNotionalUsd)} aggregate ceiling`
                : "no aggregate ceiling"}
            </DeskChip>
            <DeskChip tone="default">{book?.productLeverage ?? 10}x product leverage at the venue</DeskChip>
            <DeskChip tone="default">
              liquidation ≈ {(book?.liquidationDistPct ?? 9.5).toFixed(1)}% away
            </DeskChip>
            <DeskChip tone="default">
              taker {((book?.feeRatePerSide ?? 0.00059) * 100).toFixed(3)}% per side
            </DeskChip>
          </div>
          <p className="desk-body-md" style={{ marginTop: 12, maxWidth: 900, color: "var(--desk-on-surface-variant)" }}>
            <strong>
              Nine streams, {book?.maxConcurrent === -1 ? "unlimited" : slots} slots.
            </strong>{" "}
            They queue for the same positions, so the fastest-signalling streams take most of the fills — the three 10m
            rows will trade far more often than the 1d one, whatever their records say. That is a property of the clock,
            not of the edge, and it is the main reason a thin sample here will still mislead.
          </p>
        </DeskCard>

        <DeskCard padding="md">
          <DeskSectionHeader
            title="The Nine Streams"
            subtitle="Each one's contribution to the book — gross, minus the taker fee, equals net. Beside it, the board record it was picked on."
            actions={
              <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                {traded.length} traded · {streams.length} watched
              </span>
            }
          />
          <DeskDataTable
            columns={streamColumns}
            rows={[...streams].sort((a, b) => b.trades - a.trades || b.netUsd - a.netUsd)}
            getRowKey={(r) => key(r.strategy, r.symbol)}
            minWidth={1280}
            stickyHeader
            empty={
              <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "10px 2px" }}>
                No streams registered in this book. The engine registers them at boot — if this stays empty while the
                desk tab is healthy, the container is running without <code>SCALP_HV_PAPER=true</code>.
              </p>
            }
          />
        </DeskCard>

        <DeskCard padding="md">
          <DeskSectionHeader
            title="Open Positions"
            subtitle="Marked against the latest real Delta price. Unrealised is NET of the fee the exit will pay."
            actions={
              <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                {open.length} of {book?.maxConcurrent === -1 ? "∞" : slots} slots used
              </span>
            }
          />
          <DeskDataTable
            columns={openColumns}
            rows={open}
            getRowKey={(r) => key(r.strategy, r.symbol)}
            minWidth={1250}
            empty={
              <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "10px 2px" }}>
                Nothing open. Every position in this book is opened by a signal from one of the nine streams and closed
                by its own stop, target, time limit or the venue&apos;s liquidation price — never by hand.
              </p>
            }
          />
        </DeskCard>

        <DeskCard padding="md">
          <DeskSectionHeader
            title="Closed Trades"
            subtitle="Every round trip this book has taken, newest first — with the exit that closed it and the fee it paid."
            actions={
              <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                {closed.length} closed
              </span>
            }
          />
          <DeskDataTable
            columns={tradeColumns}
            rows={closed}
            getRowKey={(r, i) => `${r.strategy}-${r.closedAt}-${i}`}
            minWidth={1250}
            empty={
              <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "10px 2px" }}>
                No closed trades yet. The record starts the day the book opens — nothing from the leaderboard carries
                over, because those trades were taken under different rules against different competition.
              </p>
            }
          />
          <p className="desk-body-md" style={{ marginTop: 12, maxWidth: 900, color: "var(--desk-on-surface-variant)" }}>
            A taker round trip costs {((book?.feeRatePerSide ?? 0.00059) * 200).toFixed(3)}% of notional. The nine were
            selected with fee drags of 21% to 56% of gross, so roughly half of everything the better half of them earned
            went to the venue. Read every net figure here as what survived that, and the gross beside it as what did
            not.
          </p>
        </DeskCard>
      </main>
    </div>
  );
}
