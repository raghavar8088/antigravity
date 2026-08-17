"use client";

/**
 * Top Crypto Trading — the full pattern catalogue, one book per symbol.
 *
 * Ten accounts, one per instrument, each starting at its own $100 and spending
 * only from its own balance. Per-symbol rather than per-strategy because the
 * question this module asks is which INSTRUMENT suits these patterns, and a
 * shared balance would let BTC's results pay for a loss on DOGE and hide it.
 *
 * The basket was chosen on 400-day CUMULATIVE turnover, not a 24-hour snapshot.
 * That distinction is the module's premise: on the day it was built, four of
 * the top ten by 24h volume did not survive into the cumulative top fourteen,
 * and H looked like the fourth-largest market on a day it traded 3.5x its
 * 398-day average. A basket rebuilt from yesterday is a basket of whatever
 * spiked yesterday.
 *
 * Reward:risk is 1:6 here, set per-container. The other three desks running
 * this binary stay at 1:3.
 *
 * PAPER ONLY. No stream in any of these books is on the venue allow-list, so
 * nothing here can place a real order however well it performs — real money is
 * a separate, deliberate decision made against these records.
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
  /** Notional of one position. Explicit here because the concurrency cap is off. */
  positionUsd: number;
  feeRatePerSide: number;
  accounts?: Stream[];
  openPositions?: OpenPos[];
  recentTrades?: ClosedTrade[];
  uptimeMin: number;
};

/** The numbered/gold books belong to other modules and are filtered out here. */
const NON_SYMBOL_BOOKS = new Set(["01", "02", "03", "04", "05", "06", "GOLD"]);

function fmtUSD(v: number | undefined): string {
  if (v === undefined || Number.isNaN(v)) return "—";
  const abs = Math.abs(v);
  const dp = abs > 0 && abs < 1 ? 4 : 2;
  return `${v < 0 ? "-" : ""}$${abs.toFixed(dp)}`;
}
/** Prices here span six orders of magnitude — BTC at 64,000 and tokens at $0.005. */
function fmtPx(v: number | undefined): string {
  if (v === undefined || Number.isNaN(v) || v === 0) return "—";
  const abs = Math.abs(v);
  if (abs >= 1000) return v.toFixed(1);
  if (abs >= 1) return v.toFixed(3);
  return v.toFixed(6);
}
/**
 * The engine reports -1 for an UNLIMITED cap, distinct from 0 which would read
 * as "nothing allowed" — the opposite. Rendering the raw -1 would look like a
 * bug, so it becomes a word.
 */
function fmtCap(v: number | undefined, unit: string): string {
  if (v === undefined) return "—";
  if (v < 0) return "unlimited";
  return unit === "$" ? `$${v.toFixed(0)}` : `${v}${unit}`;
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
/** "MTF_45m_Wedge_Short" -> timeframe + template, for grouping. */
function parseStrategy(name: string): { tf: string; template: string } {
  const p = name.split("_");
  if (p.length >= 3 && p[0] === "MTF") {
    return { tf: p[1] ?? "", template: p.slice(2).join("_") };
  }
  return { tf: "", template: name };
}

export default function TopCryptoTradingPage() {
  const [books, setBooks] = useState<Book[]>([]);
  const [active, setActive] = useState<string>("");
  const [error, setError] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);
  const [updatedAt, setUpdatedAt] = useState<string>("");
  const [tfFilter, setTfFilter] = useState<string>("ALL");
  const [tradedOnly, setTradedOnly] = useState<boolean>(true);

  const refresh = useCallback(async () => {
    try {
      const r = await fetch("/api/scalp-topcrypto/scalp/live/paper", { cache: "no-store" });
      if (!r.ok) {
        setError(`Top Crypto engine unreachable (HTTP ${r.status})`);
        return;
      }
      const body = (await r.json()) as { accounts?: Book[] };
      const symbolBooks = (body.accounts ?? []).filter((b) => !NON_SYMBOL_BOOKS.has(b.account));
      setBooks(symbolBooks);
      setActive((cur) => (cur && symbolBooks.some((b) => b.account === cur) ? cur : symbolBooks[0]?.account ?? ""));
      // An engine with no symbol books is a configuration problem, not an empty
      // desk. Said plainly, because the two render identically.
      setError(
        symbolBooks.length
          ? ""
          : "the engine returned no per-symbol books — SCALP_SYMBOL_BOOKS is not enabled on this container",
      );
      setUpdatedAt(fmtISTClock(Date.now()));
    } catch {
      setError("Top Crypto engine unreachable");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
    const t = setInterval(() => void refresh(), 20_000);
    return () => clearInterval(t);
  }, [refresh]);

  const book = books.find((b) => b.account === active) ?? null;
  const allStreams = book?.accounts ?? [];
  const timeframes = Array.from(new Set(allStreams.map((s) => parseStrategy(s.strategy).tf).filter(Boolean))).sort(
    (a, b) => {
      const order = ["1m", "5m", "10m", "15m", "30m", "45m", "1h", "4h", "1d"];
      return order.indexOf(a) - order.indexOf(b);
    },
  );
  let streams = tfFilter === "ALL" ? allStreams : allStreams.filter((s) => parseStrategy(s.strategy).tf === tfFilter);
  if (tradedOnly) streams = streams.filter((s) => s.trades > 0);

  const trades = streams.reduce((a, x) => a + x.trades, 0);
  const wins = streams.reduce((a, x) => a + x.wins, 0);
  const gross = streams.reduce((a, x) => a + x.grossUsd, 0);
  const fees = streams.reduce((a, x) => a + x.feesUsd, 0);
  const status: DeskEngineStatus = error ? "degraded" : books.length ? "live" : "syncing";
  const totalEquity = books.reduce((a, b) => a + b.equityUsd, 0);
  const totalStart = books.reduce((a, b) => a + b.startingEquityUsd, 0);

  const streamColumns: DeskColumn<Stream>[] = [
    {
      id: "strategy",
      header: "Strategy",
      cell: (r) => {
        const { tf, template } = parseStrategy(r.strategy);
        return (
          <span>
            {tf && (
              <DeskChip tone="default" style={{ marginRight: 6 }}>
                {tf}
              </DeskChip>
            )}
            <span style={{ fontWeight: 600 }}>{template}</span>
          </span>
        );
      },
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

  const openColumns: DeskColumn<OpenPos>[] = [
    { id: "strategy", header: "Strategy", cell: (r) => r.strategy },
    {
      id: "dir",
      header: "Side",
      cell: (r) => <DeskChip tone={r.dir?.toUpperCase() === "LONG" ? "success" : "danger"}>{(r.dir || "?").toUpperCase()}</DeskChip>,
    },
    { id: "entry", align: "right", header: "Entry", cell: (r) => fmtPx(r.entry) },
    { id: "mark", align: "right", header: "Mark", cell: (r) => fmtPx(r.mark) },
    { id: "stop", align: "right", header: "Stop", cell: (r) => fmtPx(r.stop) },
    { id: "target", align: "right", header: "Target", cell: (r) => fmtPx(r.target) },
    {
      id: "rr",
      align: "right",
      header: "R:R",
      // Shown per position because 1:6 is this module's headline setting, and a
      // row that is not 1:6 is the fastest way to notice the override did not
      // reach this container.
      cell: (r) => {
        const risk = Math.abs(r.entry - r.stop);
        return risk > 0 ? `1:${(Math.abs(r.target - r.entry) / risk).toFixed(2)}` : "—";
      },
    },
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
    { id: "at", header: "Opened", cell: (r) => ageLabel(r.openedAt) },
  ];

  const tradeColumns: DeskColumn<ClosedTrade>[] = [
    { id: "at", header: "Closed (IST)", cell: (r) => fmtIST(r.closedAt) },
    { id: "strategy", header: "Strategy", cell: (r) => r.strategy },
    { id: "dir", header: "Side", cell: (r) => (r.dir || "").toUpperCase() },
    { id: "entry", align: "right", header: "Entry", cell: (r) => fmtPx(r.entry) },
    { id: "exit", align: "right", header: "Exit", cell: (r) => fmtPx(r.exit) },
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
              Top Crypto Trading
            </span>
          </div>
          <div style={{ marginTop: 8, display: "flex", flexWrap: "wrap", alignItems: "center", gap: 12 }}>
            <h1 className="desk-display-lg" style={{ fontSize: "2rem" }}>
              Top Crypto Trading
            </h1>
            <StatusBadge status={status} />
            <DeskChip tone="primary" style={{ fontWeight: 700 }}>
              PAPER MONEY · 1:6 REWARD:RISK
            </DeskChip>
          </div>
          <p className="desk-body-md" style={{ marginTop: 6, maxWidth: 900, color: "var(--desk-on-surface-variant)" }}>
            The full pattern catalogue — 41 chart, candlestick and price-structure templates across nine timeframes,
            both directions — on the ten highest instruments by <strong>400-day cumulative turnover</strong>. One book
            per symbol, each with its own ${books[0]?.startingEquityUsd?.toFixed(0) ?? 100}, so a winner on one cannot
            fund a loss on another.
          </p>
        </div>

        {error && <DeskBanner variant="warning">{error}</DeskBanner>}

        <DeskBanner variant="info">
          <strong>Paper only, structurally.</strong> No stream in these books is on the venue allow-list, so nothing
          here can place a real order however well it performs. Ranked on cumulative turnover rather than a 24-hour
          snapshot on purpose — four of the top ten by yesterday&apos;s volume did not survive into the cumulative top
          fourteen.
        </DeskBanner>

        {/* One tab per symbol. */}
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
          {books.map((b) => {
            const on = b.account === active;
            return (
              <button
                key={b.account}
                type="button"
                onClick={() => setActive(b.account)}
                style={{
                  cursor: "pointer",
                  padding: "10px 16px",
                  borderRadius: 8,
                  border: "1px solid var(--desk-outline)",
                  background: on ? "var(--desk-primary)" : "transparent",
                  color: on ? "var(--desk-on-primary)" : "var(--desk-on-surface-variant)",
                  fontWeight: 700,
                  display: "flex",
                  alignItems: "center",
                  gap: 8,
                }}
              >
                <span>{b.account.replace(/USD$/, "")}</span>
                <span
                  className="desk-mono"
                  style={{ fontWeight: 500, opacity: on ? 0.9 : 1 }}
                >
                  {b.netUsd === 0 ? "—" : fmtUSD(b.netUsd)}
                </span>
              </button>
            );
          })}
        </div>

        {book && (
          <>
            <DeskCard>
              <DeskSectionHeader
                title={`${book.account} — Account`}
                subtitle="One balance for this symbol only, ring-fenced from the other nine books."
                actions={
                  <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                    {updatedAt ? `updated ${updatedAt}` : "—"}
                  </span>
                }
              />
              <div className="desk-metrics-row">
                <DeskMetricTile
                  label="Account Balance"
                  value={`$${book.equityUsd.toFixed(2)}`}
                  valueClassName={pnlTone(book.netUsd)}
                  sub={`started at $${book.startingEquityUsd.toFixed(0)} · ${allStreams.length} strategies watched`}
                  highlight
                />
                <DeskMetricTile
                  compact
                  label="Net P&L"
                  value={fmtUSD(book.netUsd)}
                  valueClassName={pnlTone(book.netUsd)}
                  sub={`${book.roiPct >= 0 ? "+" : ""}${book.roiPct.toFixed(2)}% · gross ${fmtUSD(gross)}`}
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
                  value={fmtUSD(book.openUnrealisedUsd)}
                  valueClassName={pnlTone(book.openUnrealisedUsd)}
                  sub="unrealised, net of exit fee"
                />
                <DeskMetricTile
                  compact
                  label="Trades"
                  value={trades ? String(trades) : "—"}
                  sub={trades ? `win rate ${((100 * wins) / trades).toFixed(1)}%` : "no fills on this symbol yet"}
                />
              </div>
              <p className="desk-body-md" style={{ marginTop: 12, color: "var(--desk-on-surface-variant)" }}>
                All ten books combined: <strong>{fmtUSD(totalEquity)}</strong> on {fmtUSD(totalStart)} deployed. Shown
                as a sum, never as a single account — there is no pooled balance, and treating one would let the best
                book disguise the worst.
              </p>
              <p className="desk-body-md" style={{ marginTop: 8, maxWidth: 900, color: "var(--desk-on-surface-variant)" }}>
                <strong>Open positions: {fmtCap(book.maxConcurrent, "")}</strong> · aggregate leverage cap{" "}
                {fmtCap(book.maxLeverage, "x")} · {fmtUSD(book.positionUsd)} notional per position. Uncapped
                concurrency is what lets all {allStreams.length} streams express themselves instead of the first three
                to signal — but it also means aggregate exposure is bounded only by how many setups appear at once,
                so read a drawdown here against the position COUNT, not just the balance.
              </p>
            </DeskCard>

            <DeskCard padding="md">
              <DeskSectionHeader
                title={`${book.account} — Strategy Leaderboard`}
                subtitle="Every template on every timeframe, ranked by what it actually returned after fees."
                actions={
                  <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                    <button
                      type="button"
                      onClick={() => setTradedOnly((v) => !v)}
                      className="desk-label-md"
                      style={{
                        cursor: "pointer",
                        padding: "5px 12px",
                        borderRadius: 6,
                        border: "1px solid var(--desk-outline)",
                        background: tradedOnly ? "var(--desk-primary-container)" : "transparent",
                        color: tradedOnly ? "var(--desk-on-primary-container)" : "var(--desk-on-surface-variant)",
                        fontWeight: 600,
                      }}
                    >
                      {tradedOnly ? "traded only" : "all streams"}
                    </button>
                    {["ALL", ...timeframes].map((tf) => (
                      <button
                        key={tf}
                        type="button"
                        onClick={() => setTfFilter(tf)}
                        className="desk-label-md"
                        style={{
                          cursor: "pointer",
                          padding: "5px 10px",
                          borderRadius: 6,
                          border: "1px solid var(--desk-outline)",
                          background: tfFilter === tf ? "var(--desk-primary)" : "transparent",
                          color: tfFilter === tf ? "var(--desk-on-primary)" : "var(--desk-on-surface-variant)",
                          fontWeight: 600,
                        }}
                      >
                        {tf}
                      </button>
                    ))}
                  </div>
                }
              />
              <DeskDataTable
                columns={streamColumns}
                rows={[...streams].sort((a, b) => b.netUsd - a.netUsd || b.trades - a.trades).slice(0, 250)}
                getRowKey={(r) => `${r.strategy}|${r.symbol}`}
                minWidth={1000}
                stickyHeader
                empty={
                  <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "10px 2px" }}>
                    {tradedOnly
                      ? `No strategy has filled on ${book.account} yet — switch to "all streams" to see the ${allStreams.length} being watched.`
                      : "No strategies registered for this symbol."}
                  </p>
                }
              />
              {streams.length > 250 && (
                <p className="desk-body-md" style={{ marginTop: 10, color: "var(--desk-on-surface-variant)" }}>
                  Showing the top 250 of {streams.length}.
                </p>
              )}
            </DeskCard>

            <DeskCard padding="md">
              <DeskSectionHeader
                title={`${book.account} — Open Positions`}
                subtitle="Marked against the latest real Delta price. Unrealised is NET of the fee the exit will pay."
                actions={
                  <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                    {book.openPositions?.length ?? 0} open · {fmtUSD(book.openNotionalUsd)} deployed
                  </span>
                }
              />
              <DeskDataTable
                columns={openColumns}
                rows={book.openPositions ?? []}
                getRowKey={(r) => `${r.strategy}|${r.symbol}`}
                minWidth={1100}
                empty={
                  <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "10px 2px" }}>
                    No open positions on {book.account}.
                  </p>
                }
              />
            </DeskCard>

            <DeskCard padding="md">
              <DeskSectionHeader
                title={`${book.account} — Closed Trades`}
                subtitle="Every paper round trip on this symbol, newest first."
                actions={
                  <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                    {book.recentTrades?.length ?? 0} closed
                  </span>
                }
              />
              <DeskDataTable
                columns={tradeColumns}
                rows={book.recentTrades ?? []}
                getRowKey={(r, i) => `${r.strategy}-${r.closedAt}-${i}`}
                minWidth={1100}
                empty={
                  <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "10px 2px" }}>
                    No closed trades on {book.account} yet.
                  </p>
                }
              />
              <p className="desk-body-md" style={{ marginTop: 12, maxWidth: 880, color: "var(--desk-on-surface-variant)" }}>
                At 1:6 a strategy needs only a <strong>14.3% win rate</strong> to break even before costs — but the
                target sits six stops away, and whether price travels that far inside the holding period is a different
                question from whether the pattern was right. Watch the exit-reason mix: a book resolving mostly on time
                stops is one whose target is out of reach, not one whose patterns are wrong.
              </p>
            </DeskCard>
          </>
        )}
      </main>
    </div>
  );
}
