"use client";

/**
 * Gold Desk — gold strategies on paper, at real venue prices.
 *
 * Gold trades here on the same Delta perpetuals the crypto desks use, which is
 * why this exists at all: XAUTUSD and PAXGUSD were already inside the resolved
 * symbol universe, already priced, already carrying the real taker fee. Nothing
 * new had to be plumbed to trade gold — only a book to keep its results out of
 * a leaderboard that ranks altcoin scalps.
 *
 * PAPER ONLY, and structurally so. No gold stream is on the venue allow-list,
 * so nothing on this page can place a real order however well it performs. Gold
 * reaches real money by being added to the live roster deliberately, which is a
 * separate decision made against this book's record — the same promotion path
 * the crypto books follow, and the reason they exist.
 *
 * The instruments are gold-backed TOKENS, not interbank XAU/USD. They track
 * spot gold per troy ounce and they are the only gold this venue can fill, but
 * the basis between them is real: the two have quoted $15 apart on the same
 * ounce. Anything read against a forex gold chart has to account for that,
 * which is why both symbols are shown separately and never averaged.
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

type GoldStream = {
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
type GoldOpen = {
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
type GoldTrade = {
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
type GoldBook = {
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
  productLeverage: number;
  liquidationDistPct: number;
  feeRatePerSide: number;
  accounts?: GoldStream[];
  openPositions?: GoldOpen[];
  recentTrades?: GoldTrade[];
  uptimeMin: number;
};

function fmtUSD(v: number | undefined): string {
  if (v === undefined || Number.isNaN(v)) return "—";
  const abs = Math.abs(v);
  const dp = abs > 0 && abs < 1 ? 4 : 2;
  return `${v < 0 ? "-" : ""}$${abs.toFixed(dp)}`;
}
/** Gold quotes near $4,400 an ounce, so two decimals is the venue's own tick. */
function fmtGold(v: number | undefined): string {
  if (v === undefined || Number.isNaN(v) || v === 0) return "—";
  return `$${v.toFixed(2)}`;
}
function pnlTone(v: number): string {
  return v > 0 ? "desk-pnl-positive" : v < 0 ? "desk-pnl-negative" : "desk-pnl-neutral";
}
function ageLabel(iso?: string): string {
  if (!iso) return "—";
  const ms = Date.now() - new Date(iso).getTime();
  if (Number.isNaN(ms)) return "—";
  const s = Math.max(0, Math.round(ms / 1000));
  return s < 60 ? `${s}s ago` : `${Math.round(s / 60)}m ago`;
}

export default function GoldDeskPage() {
  const [book, setBook] = useState<GoldBook | null>(null);
  const [symbolFilter, setSymbolFilter] = useState<string>("ALL");
  const [error, setError] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);
  const [updatedAt, setUpdatedAt] = useState<string>("");

  const refresh = useCallback(async () => {
    try {
      const r = await fetch("/api/scalp/scalp/live/paper", { cache: "no-store" });
      if (!r.ok) {
        setError(`gold desk unreachable (HTTP ${r.status})`);
        return;
      }
      const body = (await r.json()) as { accounts?: GoldBook[] };
      // The gold book travels in the same payload as the crypto books and is
      // picked out by id. Missing is reported rather than rendered as an empty
      // desk: "the engine has no gold book" and "gold has not traded yet" look
      // identical on screen and mean completely different things.
      const g = (body.accounts ?? []).find((a) => a.account === "GOLD") ?? null;
      setBook(g);
      setError(g ? "" : "no GOLD book in the engine payload — the desk is running a build without the gold module");
      setUpdatedAt(fmtISTClock(Date.now()));
    } catch {
      setError("gold desk unreachable");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
    const t = setInterval(() => void refresh(), 20_000);
    return () => clearInterval(t);
  }, [refresh]);

  const allStreams = book?.accounts ?? [];
  const symbols = Array.from(new Set(allStreams.map((s) => s.symbol))).sort();
  const streams = symbolFilter === "ALL" ? allStreams : allStreams.filter((s) => s.symbol === symbolFilter);
  const traded = streams.filter((s) => s.trades > 0);
  const trades = streams.reduce((a, x) => a + x.trades, 0);
  const wins = streams.reduce((a, x) => a + x.wins, 0);
  const gross = streams.reduce((a, x) => a + x.grossUsd, 0);
  const fees = streams.reduce((a, x) => a + x.feesUsd, 0);
  const status: DeskEngineStatus = error ? "degraded" : book ? "live" : "syncing";

  /** Latest mark per symbol, straight off the open positions. */
  const marks = new Map<string, number>();
  for (const p of book?.openPositions ?? []) if (p.mark > 0) marks.set(p.symbol, p.mark);

  const streamColumns: DeskColumn<GoldStream>[] = [
    { id: "strategy", header: "Strategy", cell: (r) => <span style={{ fontWeight: 600 }}>{r.strategy}</span> },
    {
      id: "symbol",
      header: "Symbol",
      cell: (r) => <DeskChip tone={r.symbol === "XAUTUSD" ? "primary" : "default"}>{r.symbol}</DeskChip>,
    },
    { id: "n", align: "right", header: "Trades", cell: (r) => r.trades },
    { id: "wr", align: "right", header: "WR %", cell: (r) => (r.trades ? ((100 * r.wins) / r.trades).toFixed(1) : "—") },
    { id: "gross", align: "right", header: "Gross", cell: (r) => <span className={pnlTone(r.grossUsd)}>{fmtUSD(r.grossUsd)}</span> },
    { id: "fees", align: "right", header: "− Taker Fees", cell: (r) => <span className="desk-pnl-negative">{fmtUSD(-r.feesUsd)}</span> },
    {
      id: "net",
      align: "right",
      header: "= Net",
      cell: (r) => (
        <span className={pnlTone(r.netUsd)} style={{ fontWeight: 700 }}>
          {fmtUSD(r.netUsd)}
        </span>
      ),
    },
    {
      id: "drag",
      align: "right",
      header: "Fee Drag",
      // The share of gross handed to the venue. On gold this is the number that
      // decides whether a strategy is worth anything: a 0.118% round trip
      // against moves an ounce of gold makes in minutes is a high bar.
      cell: (r) => (r.grossUsd > 0 ? `${((r.feesUsd / r.grossUsd) * 100).toFixed(0)}%` : "—"),
    },
    {
      id: "verdict",
      header: "Worth Real Money?",
      cell: (r) =>
        r.trades === 0 ? (
          <DeskChip tone="default">no fills yet</DeskChip>
        ) : r.trades < 30 ? (
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

  const openColumns: DeskColumn<GoldOpen>[] = [
    { id: "strategy", header: "Strategy", cell: (r) => r.strategy },
    { id: "symbol", header: "Symbol", cell: (r) => r.symbol },
    {
      id: "dir",
      header: "Side",
      cell: (r) => <DeskChip tone={r.dir?.toUpperCase() === "LONG" ? "success" : "danger"}>{(r.dir || "?").toUpperCase()}</DeskChip>,
    },
    { id: "entry", align: "right", header: "Entry", cell: (r) => fmtGold(r.entry) },
    { id: "mark", align: "right", header: "Mark", cell: (r) => fmtGold(r.mark) },
    { id: "stop", align: "right", header: "Stop", cell: (r) => fmtGold(r.stop) },
    { id: "target", align: "right", header: "Target", cell: (r) => fmtGold(r.target) },
    {
      id: "upnl",
      align: "right",
      header: "Unrealised",
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

  const tradeColumns: DeskColumn<GoldTrade>[] = [
    { id: "at", header: "Closed (IST)", cell: (r) => fmtIST(r.closedAt) },
    { id: "strategy", header: "Strategy", cell: (r) => r.strategy },
    { id: "symbol", header: "Symbol", cell: (r) => r.symbol },
    { id: "dir", header: "Side", cell: (r) => (r.dir || "").toUpperCase() },
    { id: "entry", align: "right", header: "Entry", cell: (r) => fmtGold(r.entry) },
    { id: "exit", align: "right", header: "Exit", cell: (r) => fmtGold(r.exit) },
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
            <span className="desk-body-md" style={{ fontWeight: 500 }}>
              Gold Desk
            </span>
          </div>
          <div style={{ marginTop: 8, display: "flex", flexWrap: "wrap", alignItems: "center", gap: 12 }}>
            <h1 className="desk-display-lg" style={{ fontSize: "2rem" }}>
              Gold Desk
            </h1>
            <StatusBadge status={status} />
            <DeskChip tone="primary" style={{ fontWeight: 700 }}>
              PAPER MONEY · LIVE GOLD PRICES
            </DeskChip>
          </div>
          <p className="desk-body-md" style={{ marginTop: 6, maxWidth: 860, color: "var(--desk-on-surface-variant)" }}>
            Every strategy the desk runs, traded on gold at real Delta prices with Delta&apos;s real taker fee on both
            legs, out of its own ${book?.startingEquityUsd?.toFixed(0) ?? 100} book. Only the money is simulated — the
            prices, fees, size caps and margin rules are the ones the real account runs under.
          </p>
        </div>

        {error && <DeskBanner variant="warning">{error}</DeskBanner>}

        {/* The safety statement, not a disclaimer. It is a structural fact about
            the roster, and it is the reason this desk can watch every strategy
            on gold without anyone approving each one. */}
        <DeskBanner variant="info">
          <strong>Paper only, structurally.</strong> No gold stream is on the venue allow-list, so nothing here can
          place a real order however well it performs. Gold reaches real money only by being added to the live roster
          deliberately — a separate decision, made against this book&apos;s record.
        </DeskBanner>

        <DeskCard>
          <DeskSectionHeader
            title="Gold Book"
            subtitle="One shared balance across every gold stream — ring-fenced from the crypto books, so neither can fund the other."
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
              sub={`started at $${book?.startingEquityUsd?.toFixed(0) ?? 100} · ${allStreams.length} gold streams watched`}
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
              sub={gross > 0 ? `${((fees / gross) * 100).toFixed(0)}% of gross` : "both legs"}
            />
            <DeskMetricTile
              compact
              label="Open P&L"
              value={book ? fmtUSD(book.openUnrealisedUsd) : "—"}
              valueClassName={book ? pnlTone(book.openUnrealisedUsd) : undefined}
              sub="unrealised, net of exit fee"
            />
            <DeskMetricTile
              compact
              label="Trades"
              value={trades ? String(trades) : "—"}
              sub={trades ? `win rate ${((100 * wins) / trades).toFixed(1)}%` : "waiting for the first gold signal"}
            />
          </div>

          {/* Both marks, side by side and never averaged — the spread between
              them IS the token basis, and it is the number that separates this
              desk from a forex gold chart. */}
          <div style={{ marginTop: 14, display: "flex", flexWrap: "wrap", gap: 24 }}>
            {["XAUTUSD", "PAXGUSD"].map((sym) => (
              <div key={sym}>
                <div className="desk-label-md" style={{ color: "var(--desk-on-surface-variant)" }}>
                  {sym} {sym === "XAUTUSD" ? "(Tether Gold)" : "(PAX Gold)"}
                </div>
                <div className="desk-mono" style={{ fontSize: "1.05rem", fontWeight: 700 }}>
                  {marks.has(sym) ? fmtGold(marks.get(sym)) : "— no open position to mark"}
                </div>
              </div>
            ))}
          </div>
          <p className="desk-body-md" style={{ marginTop: 12, maxWidth: 860, color: "var(--desk-on-surface-variant)" }}>
            Both are gold-backed <strong>tokens</strong> redeemable for a troy ounce, not interbank XAU/USD. They track
            spot gold closely and they are the only gold this venue can fill, but the basis between them is real — the
            two have quoted <strong>$15 apart on the same ounce</strong>. They also trade 24/7, so there is no weekend
            gap where forex gold has one.
          </p>
        </DeskCard>

        <DeskCard padding="md">
          <DeskSectionHeader
            title="Gold Strategy Leaderboard"
            subtitle="Each stream's contribution to the gold book — gross, minus taker fees, equals net."
            actions={
              <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                  {traded.length} traded · {streams.length} watched
                </span>
                {["ALL", ...symbols].map((sym) => (
                  <button
                    key={sym}
                    type="button"
                    onClick={() => setSymbolFilter(sym)}
                    className="desk-label-md"
                    style={{
                      cursor: "pointer",
                      padding: "5px 12px",
                      borderRadius: 6,
                      border: "1px solid var(--desk-outline)",
                      background: symbolFilter === sym ? "var(--desk-primary)" : "transparent",
                      color: symbolFilter === sym ? "var(--desk-on-primary)" : "var(--desk-on-surface-variant)",
                      fontWeight: 600,
                    }}
                  >
                    {sym}
                  </button>
                ))}
              </div>
            }
          />
          {/* Streams that have traded first: a board that opens on hundreds of
              zero rows buries the handful that carry information. */}
          <DeskDataTable
            columns={streamColumns}
            rows={[...streams].sort((a, b) => b.trades - a.trades || b.netUsd - a.netUsd).slice(0, 200)}
            getRowKey={(r) => `${r.strategy}|${r.symbol}`}
            minWidth={1080}
            stickyHeader
            empty={
              <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "10px 2px" }}>
                No gold streams registered. The desk registers them at boot from the resolved symbol universe — if this
                stays empty, XAUTUSD and PAXGUSD did not clear the turnover floor.
              </p>
            }
          />
          {streams.length > 200 && (
            <p className="desk-body-md" style={{ marginTop: 10, color: "var(--desk-on-surface-variant)" }}>
              Showing the 200 most active of {streams.length} gold streams.
            </p>
          )}
        </DeskCard>

        <DeskCard padding="md">
          <DeskSectionHeader
            title="Open Gold Positions"
            subtitle="Marked against the latest real Delta price. Unrealised is NET of the fee the exit will pay."
            actions={
              <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                {book?.openPositions?.length ?? 0} open · {fmtUSD(book?.openNotionalUsd)} deployed
              </span>
            }
          />
          <DeskDataTable
            columns={openColumns}
            rows={book?.openPositions ?? []}
            getRowKey={(r) => `${r.strategy}|${r.symbol}`}
            minWidth={1150}
            empty={
              <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "10px 2px" }}>
                No open gold positions.
              </p>
            }
          />
        </DeskCard>

        <DeskCard padding="md">
          <DeskSectionHeader
            title="Closed Gold Trades"
            subtitle="Every paper round trip on gold, newest first — with the exit that closed it and the fee it paid."
            actions={
              <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                {book?.recentTrades?.length ?? 0} closed
              </span>
            }
          />
          <DeskDataTable
            columns={tradeColumns}
            rows={book?.recentTrades ?? []}
            getRowKey={(r, i) => `${r.strategy}-${r.closedAt}-${i}`}
            minWidth={1150}
            empty={
              <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "10px 2px" }}>
                No closed gold trades yet.
              </p>
            }
          />
          <p className="desk-body-md" style={{ marginTop: 12, maxWidth: 860, color: "var(--desk-on-surface-variant)" }}>
            Gold moves in far smaller percentage steps than the altcoins these strategies were written against, while
            the fee is identical — a taker round trip costs{" "}
            {((book?.feeRatePerSide ?? 0.00059) * 200).toFixed(3)}% of notional either way. Expect fewer signals and a
            harsher fee drag here than on the crypto books, and read a thin sample accordingly.
          </p>
        </DeskCard>
      </main>
    </div>
  );
}
