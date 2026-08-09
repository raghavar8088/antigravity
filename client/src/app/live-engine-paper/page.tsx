"use client";

/**
 * Live Engine Paper Desk — the promoted strategies on paper money.
 *
 * Its own page rather than a card on the Live Engine, because it answers a
 * different question. The Live Engine page shows what real capital is doing
 * right now; this shows whether those strategies deserve it.
 *
 * Everything except the money is real: the same allow-list the venue gates on,
 * the same Delta prices, the same 0.059% taker fee on both legs, the same
 * shared $100, the same 3x size cap and 3-position limit, and the same 10x
 * margin setting that decides where Delta would force-close a position.
 *
 * It exists because the two records that mattered disagreed and neither could
 * explain the other: the scalp leaderboard said 79.7% wins and +$37 gross where
 * the same streams returned 33.3% and -$13.91 with money. The scalp desk runs
 * 66,000 streams on different levels and a different fee model, so it was never
 * answering that question. When THIS desk disagrees with the live bridge, the
 * difference is execution — slippage, latency, partial fills — and nothing else,
 * because every other variable is held equal by construction.
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

type PaperAccount = {
  strategy: string;
  /** The unit watched is the STREAM — the same strategy on two symbols is two bets. */
  symbol: string;
  live: boolean;
  /** Contribution to the SHARED balance, not an account of its own. */
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
  mark: number;
  /** NET of the round-trip taker fee it will pay to close. */
  unrealisedUsd: number;
  unrealisedPct: number;
  /** True when this stream is ALSO on the venue allow-list. */
  live: boolean;
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
  live: boolean;
};
type PaperDesk = {
  /** Which book this is — "01" or "02". Two independent $100 accounts. */
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
  accounts?: PaperAccount[];
  openPositions?: PaperOpen[];
  recentTrades?: PaperTrade[];
  uptimeMin: number;
};

function fmtUSD(v: number | undefined): string {
  if (v === undefined || Number.isNaN(v)) return "—";
  const abs = Math.abs(v);
  const dp = abs > 0 && abs < 1 ? 4 : 2;
  return `${v < 0 ? "-" : ""}$${abs.toFixed(dp)}`;
}
function fmtPrice(v: number | undefined): string {
  if (v === undefined || Number.isNaN(v) || v === 0) return "—";
  const abs = Math.abs(v);
  return v.toFixed(abs >= 1000 ? 1 : abs >= 1 ? 3 : 5);
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

/**
 * One paper book. Account 01 and 02 render identically — same columns, same
 * ordering, same wording — so a difference between them is a difference in the
 * strategies, never in how they are presented.
 */
function AccountBook({ d, updatedAt }: { d: PaperDesk; updatedAt: string }) {
  const accts = d.accounts ?? [];
  const trades = accts.reduce((a, x) => a + x.trades, 0);
  const wins = accts.reduce((a, x) => a + x.wins, 0);
  const gross = accts.reduce((a, x) => a + x.grossUsd, 0);
  const fees = accts.reduce((a, x) => a + x.feesUsd, 0);

  const accountColumns: DeskColumn<PaperAccount>[] = [
    {
      id: "strategy",
      header: "Strategy",
      cell: (r) => (
        <span className="desk-body-md" style={{ fontWeight: 600 }}>
          {r.strategy}
        </span>
      ),
    },
    { id: "symbol", header: "Symbol", cell: (r) => r.symbol },
    {
      id: "route",
      header: "Route",
      cell: (r) =>
        r.live ? (
          <DeskChip tone="primary" style={{ fontWeight: 700 }}>LIVE</DeskChip>
        ) : (
          <DeskChip tone="default">candidate</DeskChip>
        ),
    },
    {
      id: "share",
      align: "right",
      header: "Share of Balance",
      // A CONTRIBUTION, not a balance. There is one $100; showing a per-strategy
      // equity implied several accounts and several times the capital.
      cell: (r) => (
        <span className={pnlTone(r.netUsd)} style={{ fontWeight: 700 }}>
          {`${r.shareOfEquityPct >= 0 ? "+" : ""}${r.shareOfEquityPct.toFixed(2)}%`}
        </span>
      ),
    },
    { id: "n", align: "right", header: "Trades", cell: (r) => r.trades },
    {
      id: "wr",
      align: "right",
      header: "WR %",
      cell: (r) => (r.trades ? ((100 * r.wins) / r.trades).toFixed(1) : "—"),
    },
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
      id: "verdict",
      header: "Worth Real Money?",
      // Deliberately a question. Positive net is necessary and nowhere near
      // sufficient — the gate wants 200 trades, and a green chip at n=3 is how
      // a leaderboard starts lying.
      cell: (r) =>
        r.trades < 30 ? (
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

  const openColumns: DeskColumn<PaperOpen>[] = [
    { id: "strategy", header: "Strategy", cell: (r) => r.strategy },
    { id: "symbol", header: "Symbol", cell: (r) => r.symbol },
    {
      id: "dir",
      header: "Side",
      cell: (r) => (
        <DeskChip tone={r.dir?.toUpperCase() === "LONG" ? "success" : "danger"}>{(r.dir || "?").toUpperCase()}</DeskChip>
      ),
    },
    { id: "entry", align: "right", header: "Entry", cell: (r) => fmtPrice(r.entry) },
    { id: "stop", align: "right", header: "Stop", cell: (r) => fmtPrice(r.stop) },
    { id: "target", align: "right", header: "Target", cell: (r) => fmtPrice(r.target) },
    {
      id: "rr",
      align: "right",
      header: "R:R",
      cell: (r) => {
        const risk = Math.abs(r.entry - r.stop);
        return risk > 0 ? `1:${(Math.abs(r.target - r.entry) / risk).toFixed(2)}` : "—";
      },
    },
    { id: "notional", align: "right", header: "Size", cell: (r) => fmtUSD(r.entry * r.contracts) },
    { id: "mark", align: "right", header: "Mark", cell: (r) => (r.mark > 0 ? fmtPrice(r.mark) : "—") },
    {
      id: "upnl",
      align: "right",
      header: "Unrealised",
      // NET of the round-trip fee it will pay to close. A gross figure would
      // show a winner where the close books a loss — most of these strategies
      // target moves smaller than the 0.118% it costs to trade.
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
          <span className={pnlTone(r.unrealisedPct)}>
            {`${r.unrealisedPct >= 0 ? "+" : ""}${r.unrealisedPct.toFixed(3)}%`}
          </span>
        ) : (
          "—"
        ),
    },
    {
      id: "route",
      header: "Route",
      // A candidate's result is NOT evidence about real money. Without this the
      // two are indistinguishable on the page.
      cell: (r: PaperOpen) =>
        r.live ? (
          <DeskChip tone="primary" style={{ fontWeight: 700 }}>
            LIVE
          </DeskChip>
        ) : (
          <DeskChip tone="default">candidate</DeskChip>
        ),
    },
    { id: "at", header: "Opened", cell: (r) => ageLabel(r.openedAt) },
  ];

  const tradeColumns: DeskColumn<PaperTrade>[] = [
    { id: "at", header: "Closed", cell: (r) => r.closedAt?.slice(5, 16).replace("T", " ") },
    { id: "strategy", header: "Strategy", cell: (r) => r.strategy },
    { id: "symbol", header: "Symbol", cell: (r) => r.symbol },
    {
      id: "route",
      header: "Route",
      // A candidate's result is NOT evidence about real money. Without this the
      // two are indistinguishable on the page.
      cell: (r: PaperTrade) =>
        r.live ? (
          <DeskChip tone="primary" style={{ fontWeight: 700 }}>
            LIVE
          </DeskChip>
        ) : (
          <DeskChip tone="default">candidate</DeskChip>
        ),
    },
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
    <>
      <DeskCard>
        <DeskSectionHeader
          title={`Account ${d.account}`}
          subtitle="One shared balance within this book — separate from the other account, so neither can fund the other."
          actions={
            <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
              {updatedAt ? `updated ${updatedAt}` : "—"}
            </span>
          }
        />
        <div className="desk-metrics-row">
          <DeskMetricTile
            label="Account Balance"
            value={`$${d.equityUsd.toFixed(2)}`}
            valueClassName={pnlTone(d.netUsd)}
            sub={`started at $${d.startingEquityUsd.toFixed(0)} · ${accts.length} streams watched`}
            highlight
          />
          <DeskMetricTile compact label="Net P&L" value={fmtUSD(d.netUsd)} valueClassName={pnlTone(d.netUsd)}
            sub={`${d.roiPct >= 0 ? "+" : ""}${d.roiPct.toFixed(2)}% · gross ${fmtUSD(gross)}`} />
          <DeskMetricTile compact label="Taker Fees" value={fees ? fmtUSD(-fees) : "—"}
            valueClassName={fees > 0 ? "desk-pnl-negative" : undefined}
            sub={gross > 0 ? `${((fees / gross) * 100).toFixed(0)}% of gross` : "both legs"} />
          <DeskMetricTile compact label="Deployed" value={fmtUSD(d.openNotionalUsd)}
            sub={`of ${fmtUSD(d.maxNotionalUsd)} max · ${d.maxConcurrent} at once`} />
          <DeskMetricTile compact label="Open P&L" value={fmtUSD(d.openUnrealisedUsd)}
            valueClassName={pnlTone(d.openUnrealisedUsd)} sub="unrealised, net of exit fee" />
          <DeskMetricTile compact label="Trades" value={trades ? String(trades) : "—"}
            sub={trades ? `win rate ${((100 * wins) / trades).toFixed(1)}%` : "waiting for signals"} />
        </div>
        <p className="desk-body-md" style={{ marginTop: 14, maxWidth: 820, color: "var(--desk-on-surface-variant)" }}>
          <strong>{d.maxLeverage}× limits SIZE</strong> — at most {fmtUSD(d.maxNotionalUsd)} of positions across{" "}
          {d.maxConcurrent} at a time. <strong>{d.productLeverage}× is the MARGIN setting</strong> — it decides how much
          cash Delta freezes, and therefore that the venue would not force-close until price moved{" "}
          {d.liquidationDistPct.toFixed(1)}% against the position. Stops sit near 0.7%, so the strategy closes the trade
          rather than the venue.
        </p>
      </DeskCard>

      <DeskCard padding="md">
        <DeskSectionHeader
          title={`Account ${d.account} — Strategy Leaderboard`}
          subtitle="Each stream's contribution to this book's $100 — gross, minus taker fees, equals net."
          actions={
            <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
              {accts.filter((a) => a.trades > 0).length} traded · {accts.length} watched
            </span>
          }
        />
        <DeskDataTable columns={accountColumns} rows={accts} getRowKey={(r) => `${r.strategy}|${r.symbol}`}
          minWidth={1120}
          empty={<p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "10px 2px" }}>No streams configured for this account.</p>} />
      </DeskCard>

      <DeskCard padding="md">
        <DeskSectionHeader
          title={`Account ${d.account} — Open Positions`}
          subtitle="Marked against the latest real Delta price. Unrealised is NET of the fee the exit will pay."
          actions={
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                {d.openPositions?.length ?? 0} open · {fmtUSD(d.openNotionalUsd)} deployed
              </span>
              <span className={`desk-mono desk-label-md ${pnlTone(d.openUnrealisedUsd)}`} style={{ fontWeight: 700 }}>
                {fmtUSD(d.openUnrealisedUsd)}
              </span>
            </div>
          }
        />
        <DeskDataTable columns={openColumns} rows={d.openPositions ?? []}
          getRowKey={(r) => `${r.strategy}|${r.symbol}`} minWidth={1320}
          empty={<p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "10px 2px" }}>No open paper positions.</p>} />
      </DeskCard>

      <DeskCard padding="md">
        <DeskSectionHeader
          title={`Account ${d.account} — Closed Trades`}
          subtitle="Every paper round trip, newest first — with the exit that closed it and the fee it paid."
          actions={<span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>{d.recentTrades?.length ?? 0} closed</span>}
        />
        <DeskDataTable columns={tradeColumns} rows={d.recentTrades ?? []}
          getRowKey={(r, i) => `${r.strategy}-${r.closedAt}-${i}`} minWidth={1220}
          empty={<p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "10px 2px" }}>No closed paper trades yet.</p>} />
      </DeskCard>
    </>
  );
}

export default function LiveEnginePaperDeskPage() {
  const [books, setBooks] = useState<PaperDesk[]>([]);
  const [error, setError] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);
  const [updatedAt, setUpdatedAt] = useState<string>("");

  const refresh = useCallback(async () => {
    try {
      const r = await fetch("/api/scalp/scalp/live/paper", { cache: "no-store" });
      if (!r.ok) {
        setError(`desk unreachable (HTTP ${r.status})`);
        return;
      }
      const body = (await r.json()) as { accounts?: PaperDesk[] };
      setBooks(body.accounts ?? []);
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
    const t = setInterval(() => void refresh(), 20_000);
    return () => clearInterval(t);
  }, [refresh]);

  const status: DeskEngineStatus = error ? "degraded" : books.length ? "live" : "syncing";
  const combined = books.reduce((a, b) => a + b.netUsd, 0);

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
              Live Engine Paper Desk
            </span>
          </div>
          <div style={{ marginTop: 8, display: "flex", flexWrap: "wrap", alignItems: "center", gap: 12 }}>
            <h1 className="desk-display-lg" style={{ fontSize: "2rem" }}>
              Live Engine Paper Desk
            </h1>
            <StatusBadge status={status} />
            <DeskChip tone="primary" style={{ fontWeight: 700 }}>
              PAPER MONEY · LIVE DATA
            </DeskChip>
          </div>
          <p className="desk-body-md" style={{ marginTop: 6, maxWidth: 800, color: "var(--desk-on-surface-variant)" }}>
            Two independent books, each starting at $100, against real Delta prices with Delta&apos;s real taker fee
            on both legs. They are separate accounts, not one list split in two: a winner in one cannot fund a position
            in the other, so the better set cannot subsidise the worse and hide it. Rows marked <strong>LIVE</strong>
            are also on the real-money allow-list; rows marked <strong>candidate</strong> are watched on live terms
            BEFORE anyone decides they deserve money and cannot reach the venue. Only the money is simulated — the
            prices, fees, size caps and margin rules are the ones the real account runs under.
          </p>
        </div>

        {error && <DeskBanner variant="warning">{error} — retrying every 20s</DeskBanner>}

        {books.length > 1 && (
          <DeskCard>
            <DeskSectionHeader
              title="Both Accounts"
              subtitle="Two competing sets of streams, each on its own $100. The comparison is the point."
            />
            <div className="desk-metrics-row">
              {books.map((b) => (
                <DeskMetricTile
                  key={b.account}
                  compact
                  label={`Account ${b.account}`}
                  value={`$${b.equityUsd.toFixed(2)}`}
                  valueClassName={pnlTone(b.netUsd)}
                  sub={`${b.roiPct >= 0 ? "+" : ""}${b.roiPct.toFixed(2)}% · ${
                    (b.accounts ?? []).filter((a) => a.trades > 0).length
                  } of ${(b.accounts ?? []).length} streams traded`}
                />
              ))}
              <DeskMetricTile
                compact
                label="Combined"
                value={fmtUSD(combined)}
                valueClassName={pnlTone(combined)}
                sub={`across $${(books.length * 100).toFixed(0)} of paper capital`}
              />
            </div>
          </DeskCard>
        )}

        {books.map((b) => (
          <AccountBook key={b.account} d={b} updatedAt={updatedAt} />
        ))}

      </main>
    </div>
  );
}
