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
  equityUsd: number;
  netUsd: number;
  roiPct: number;
  openNotionalUsd: number;
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

export default function LiveEnginePaperDeskPage() {
  const [paper, setPaper] = useState<PaperDesk | null>(null);
  const [tab, setTab] = useState<string>("accounts");
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
      setPaper((await r.json()) as PaperDesk);
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

  const accts = paper?.accounts ?? [];
  const trades = accts.reduce((a, x) => a + x.trades, 0);
  const wins = accts.reduce((a, x) => a + x.wins, 0);
  const gross = accts.reduce((a, x) => a + x.grossUsd, 0);
  const fees = accts.reduce((a, x) => a + x.feesUsd, 0);
  const status: DeskEngineStatus = error ? "degraded" : paper ? "live" : "syncing";

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
    { id: "at", header: "Opened", cell: (r) => ageLabel(r.openedAt) },
  ];

  const tradeColumns: DeskColumn<PaperTrade>[] = [
    { id: "at", header: "Closed", cell: (r) => r.closedAt?.slice(5, 16).replace("T", " ") },
    { id: "strategy", header: "Strategy", cell: (r) => r.strategy },
    { id: "symbol", header: "Symbol", cell: (r) => r.symbol },
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
            The strategies promoted to the Live Engine, traded on one shared $100 of paper money against real Delta
            prices, with Delta&apos;s real taker fee on both legs. Only the money is simulated — the allow-list, the
            prices, the fees, the size caps and the margin rules are the same ones the real account runs under. Where
            this desk and the live record disagree, the difference is <strong>execution</strong>.
          </p>
        </div>

        {error && <DeskBanner variant="warning">{error} — retrying every 20s</DeskBanner>}

        <DeskCard>
          <DeskSectionHeader
            title="Account"
            subtitle="One shared balance, not one per strategy — the live bridge has one Delta wallet and so does this."
            actions={
              <span className="desk-mono desk-label-md" style={{ fontWeight: 400 }}>
                {updatedAt ? `updated ${updatedAt}` : "—"}
              </span>
            }
          />
          <div className="desk-metrics-row">
            <DeskMetricTile
              label="Account Balance"
              value={paper ? `$${paper.equityUsd.toFixed(2)}` : "—"}
              valueClassName={paper ? pnlTone(paper.netUsd) : undefined}
              sub={`started at $${paper?.startingEquityUsd.toFixed(0) ?? 100} · ${accts.length} strategies drawing from it`}
              highlight
            />
            <DeskMetricTile
              compact
              label="Net P&L"
              value={paper ? fmtUSD(paper.netUsd) : "—"}
              valueClassName={paper ? pnlTone(paper.netUsd) : undefined}
              sub={paper ? `${paper.roiPct >= 0 ? "+" : ""}${paper.roiPct.toFixed(2)}% · gross ${fmtUSD(gross)}` : "—"}
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
              label="Deployed"
              value={paper ? fmtUSD(paper.openNotionalUsd) : "—"}
              sub={paper ? `of ${fmtUSD(paper.maxNotionalUsd)} max · ${paper.maxConcurrent} at once` : "—"}
            />
            <DeskMetricTile
              compact
              label="Trades"
              value={trades ? String(trades) : "—"}
              sub={trades ? `win rate ${((100 * wins) / trades).toFixed(1)}%` : "waiting for signals"}
            />
          </div>

          {/* The two leverage numbers, named. They sound alike and do different
              jobs, and confusing them is how an operator reads 3x exposure as
              10x exposure. */}
          <p className="desk-body-md" style={{ marginTop: 14, maxWidth: 820, color: "var(--desk-on-surface-variant)" }}>
            <strong>{paper?.maxLeverage ?? 3}× limits SIZE</strong> — at most $
            {(paper?.maxNotionalUsd ?? 300).toFixed(0)} of positions across {paper?.maxConcurrent ?? 3} at a time.{" "}
            <strong>{paper?.productLeverage ?? 10}× is the MARGIN setting</strong> — it decides how much cash Delta
            freezes, and therefore that the venue would not force-close until price moved{" "}
            {paper?.liquidationDistPct?.toFixed(1) ?? "9.5"}% against the position. Stops sit near 0.7%, so the strategy
            closes the trade rather than the venue. This desk refuses any signal whose stop sits past that line, exactly
            as the live bridge does.
          </p>
        </DeskCard>

        <DeskCard padding="md">
          <DeskSectionHeader
            title="Strategies"
            actions={
              <div style={{ display: "flex", gap: 8 }}>
                {[
                  ["accounts", `Strategies (${accts.length})`],
                  ["open", `Open (${paper?.openPositions?.length ?? 0})`],
                  ["trades", `Closed (${paper?.recentTrades?.length ?? 0})`],
                ].map(([id, label]) => (
                  <button
                    key={id}
                    type="button"
                    onClick={() => setTab(id)}
                    className="desk-label-md"
                    style={{
                      cursor: "pointer",
                      padding: "5px 12px",
                      borderRadius: 6,
                      border: "1px solid var(--desk-outline)",
                      background: tab === id ? "var(--desk-primary)" : "transparent",
                      color: tab === id ? "var(--desk-on-primary)" : "var(--desk-on-surface-variant)",
                      fontWeight: 600,
                    }}
                  >
                    {label}
                  </button>
                ))}
              </div>
            }
          />

          {tab === "accounts" && (
            <DeskDataTable
              columns={accountColumns}
              rows={accts}
              getRowKey={(r) => r.strategy}
              minWidth={980}
              empty={
                <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "10px 2px" }}>
                  No strategy has traded yet. Rows appear on the first paper fill.
                </p>
              }
            />
          )}
          {tab === "open" && (
            <DeskDataTable
              columns={openColumns}
              rows={paper?.openPositions ?? []}
              getRowKey={(r) => `${r.strategy}|${r.symbol}`}
              minWidth={1000}
              empty={
                <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "10px 2px" }}>
                  No open paper positions.
                </p>
              }
            />
          )}
          {tab === "trades" && (
            <DeskDataTable
              columns={tradeColumns}
              rows={paper?.recentTrades ?? []}
              getRowKey={(r, i) => `${r.strategy}-${r.closedAt}-${i}`}
              minWidth={1100}
              empty={
                <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", margin: "10px 2px" }}>
                  No closed paper trades yet.
                </p>
              }
            />
          )}

          <p className="desk-body-md" style={{ marginTop: 14, maxWidth: 820, color: "var(--desk-on-surface-variant)" }}>
            &ldquo;Worth real money?&rdquo; is a question, not a verdict. Positive net is necessary and nowhere near
            sufficient: the pre-registered gate wants 200 trades per stream, and a profit over 30 is still mostly noise.
            Fee drag is the number to watch — a taker round trip costs {((paper?.feeRatePerSide ?? 0.00059) * 200).toFixed(3)}
            % of notional, which is larger than the move most of these strategies target.
          </p>
        </DeskCard>
      </main>
    </div>
  );
}
