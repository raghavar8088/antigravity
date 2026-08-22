"use client";

/**
 * The trading terminal, shared by the Delta and Forex paper desks.
 *
 * ONE COMPONENT, TWO VENUES. The two modules differ in what they trade, not in
 * how trading works, so the terminal is parameterised by venue rather than
 * copied. Everything venue-specific — the size unit, the leverage ladder, the
 * position mode, whether the market is open, whether the spread is quoted or
 * modelled — arrives in the snapshot and is rendered from there. A second copy
 * of this file would drift within a week.
 *
 * WHAT THE UI IS OBLIGED TO SAY OUT LOUD:
 *
 *   - that the money is paper, on every screen, permanently;
 *   - when a spread is MODELLED rather than quoted, because the forex desk's
 *     entire cost base is that number and no free feed publishes a broker's
 *     bid and ask;
 *   - when the book being shown is synthetic rather than real depth;
 *   - when the market is closed, and that market orders are refused rather
 *     than filled against a stale print;
 *   - why an order was rejected, in the venue's own terms.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createChart, CandlestickSeries, LineStyle } from "lightweight-charts";
import type { CandlestickData, IChartApi, ISeriesApi, Time } from "lightweight-charts";
import {
  DeskBanner,
  DeskButton,
  DeskCard,
  DeskChip,
  DeskDataTable,
  DeskLinearProgress,
  DeskMetricTile,
  DeskSearchField,
  DeskTabs,
  type DeskColumn,
  type DeskTabItem,
} from "@/components/desk/ui";
import { fmtISTClock } from "@/lib/istTime";

// ── types mirroring the API ─────────────────────────────────────────────────

type Instrument = {
  symbol: string;
  displayName: string;
  kind: string;
  contractSize: number;
  sizeUnit: "contracts" | "lots";
  minSize: number;
  sizeStep: number;
  tickSize: number;
  pricePrecision: number;
  maxLeverage: number;
  maintenanceMarginPct: number;
  takerFeeRate: number;
  commissionPerLotUsd: number;
  carryKind: "funding" | "swap";
  fundingRatePct8h: number | null;
  swapLongPointsPerDay: number | null;
  swapShortPointsPerDay: number | null;
  last: number;
  bid: number;
  ask: number;
  markPrice: number;
  change24hPct: number | null;
  high24h: number | null;
  low24h: number | null;
  spreadIsModelled: boolean;
  source: string;
};

type Row = Record<string, unknown>;

type VenueMeta = {
  id: string;
  label: string;
  dataNote: string;
  positionMode: "netting" | "hedging";
  sizeUnit: "contracts" | "lots";
  leverageChoices: number[];
  accountTypes: { key: string; label: string; note: string }[];
  stopOutLevelPct: number;
  marginCallLevelPct: number;
  resolutions: { key: string; label: string; seconds: number }[];
  marketOpen: boolean;
  marketClosedNote: string;
};

type Snapshot = {
  ok: boolean;
  configured: boolean;
  reason?: string;
  venue: VenueMeta;
  account?: {
    account: { leverage: number; accountType: string; startingBalance: number; resetCount: number };
    balance: number;
    equity: number;
    usedMargin: number;
    freeMargin: number;
    marginLevelPct: number | null;
    unrealisedPnlUsd: number;
    openPositions: number;
    openOrders: number;
  };
  marginCall?: boolean;
  instruments?: Instrument[];
  positions?: Row[];
  orders?: Row[];
  trades?: Row[];
  stats?: Record<string, number | null>;
  tick?: { lastTickAt: number | null; ticks: number; lastError: string | null; note: string };
};

type Book = { bids: { price: number; size: number }[]; asks: { price: number; size: number }[]; modelled: boolean };

// ── formatting ──────────────────────────────────────────────────────────────

function px(v: number | null | undefined, precision = 2): string {
  if (v === null || v === undefined || !Number.isFinite(v)) return "—";
  return v.toLocaleString(undefined, { minimumFractionDigits: precision, maximumFractionDigits: precision });
}
function usd(v: number | null | undefined, dp = 2): string {
  if (v === null || v === undefined || !Number.isFinite(v)) return "—";
  return `${v < 0 ? "-" : ""}$${Math.abs(v).toLocaleString(undefined, { minimumFractionDigits: dp, maximumFractionDigits: dp })}`;
}
function pct(v: number | null | undefined, dp = 2): string {
  if (v === null || v === undefined || !Number.isFinite(v)) return "—";
  return `${v >= 0 ? "+" : ""}${v.toFixed(dp)}%`;
}
function pnlClass(v: number | null | undefined): string {
  if (v === null || v === undefined || !Number.isFinite(v) || v === 0) return "";
  return v > 0 ? "desk-pnl-positive" : "desk-pnl-negative";
}

// ── page ────────────────────────────────────────────────────────────────────

export default function TradingTerminal({ venue }: { venue: "delta" | "forex" }) {
  const [snap, setSnap] = useState<Snapshot | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [symbol, setSymbol] = useState<string>("");
  const [book, setBook] = useState<Book | null>(null);
  const [resolution, setResolution] = useState("5m");
  const [notice, setNotice] = useState<{ tone: "success" | "error"; text: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const [tab, setTab] = useState<"positions" | "orders" | "history" | "account">("positions");

  const api = useCallback(
    async (action: string, init?: RequestInit) => {
      const r = await fetch(`/api/paper-trading/${venue}/${action}`, { cache: "no-store", ...init });
      const body = (await r.json().catch(() => ({}))) as Record<string, unknown>;
      if (!r.ok) throw new Error(typeof body.error === "string" ? body.error : `HTTP ${r.status}`);
      return body;
    },
    [venue],
  );

  const refresh = useCallback(async () => {
    try {
      const s = (await api("snapshot")) as unknown as Snapshot;
      setSnap(s);
      setError("");
      setSymbol((cur) => cur || s.instruments?.[0]?.symbol || "");
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to load");
    } finally {
      setLoading(false);
    }
  }, [api]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // The book is the fastest-moving thing on the page and the cheapest to
  // fetch, so it refreshes on its own rather than waiting for a full snapshot.
  useEffect(() => {
    if (!symbol) return undefined;
    let live = true;
    const pull = async () => {
      try {
        const b = (await api(`book?symbol=${encodeURIComponent(symbol)}&depth=12`)) as { book?: Book };
        if (live && b.book) setBook(b.book);
      } catch {
        if (live) setBook(null);
      }
    };
    void pull();
    const t = setInterval(pull, 5_000);
    return () => {
      live = false;
      clearInterval(t);
    };
  }, [symbol, api]);

  const instrument = useMemo(
    () => snap?.instruments?.find((i) => i.symbol === symbol) ?? null,
    [snap, symbol],
  );

  const act = useCallback(
    async (action: string, payload: unknown, okText: string) => {
      setBusy(true);
      setNotice(null);
      try {
        await api(action, {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify(payload),
        });
        setNotice({ tone: "success", text: okText });
        await refresh();
      } catch (e) {
        setNotice({ tone: "error", text: e instanceof Error ? e.message : "request failed" });
      } finally {
        setBusy(false);
      }
    },
    [api, refresh],
  );

  if (loading) return <DeskLinearProgress />;

  if (error) {
    return (
      <DeskBanner variant="error" title="Terminal unavailable">
        {error}
      </DeskBanner>
    );
  }

  if (snap && !snap.configured) {
    return (
      <DeskBanner variant="warning" title="This terminal is not configured on this deployment">
        {snap.reason}
      </DeskBanner>
    );
  }

  const v = snap!.venue;
  const acct = snap!.account!;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--desk-space-4)" }}>
      {!v.marketOpen ? (
        <DeskBanner variant="warning" title="Market closed">
          {v.marketClosedNote}
        </DeskBanner>
      ) : null}

      {snap!.marginCall ? (
        <DeskBanner variant="error" title="Margin call">
          Margin level is {pct(acct.marginLevelPct, 1)}, below this account&apos;s {v.marginCallLevelPct}%
          call level. At {v.stopOutLevelPct}% the desk starts closing the worst position until the level
          recovers.
        </DeskBanner>
      ) : null}

      {notice ? (
        <DeskBanner variant={notice.tone === "success" ? "success" : "error"} title={notice.tone === "success" ? "Done" : "Rejected"}>
          {notice.text}
        </DeskBanner>
      ) : null}

      <AccountStrip acct={acct} venue={v} stats={snap!.stats ?? {}} />

      <div style={{ display: "grid", gridTemplateColumns: "minmax(0, 1fr) 330px", gap: "var(--desk-space-4)", alignItems: "start" }}>
        <div style={{ display: "flex", flexDirection: "column", gap: "var(--desk-space-4)", minWidth: 0 }}>
          <DeskCard padding="md">
            <InstrumentPicker
              instruments={snap!.instruments ?? []}
              symbol={symbol}
              onPick={setSymbol}
              sizeUnit={v.sizeUnit}
            />
            {instrument ? <TickerStrip inst={instrument} /> : null}
          </DeskCard>

          <DeskCard padding="md">
            <div style={{ display: "flex", gap: 6, marginBottom: 10, flexWrap: "wrap" }}>
              {v.resolutions.map((r) => (
                <button
                  key={r.key}
                  type="button"
                  onClick={() => setResolution(r.key)}
                  style={{
                    padding: "4px 12px",
                    minHeight: 30,
                    borderRadius: "var(--desk-radius-chip)",
                    border: `1px solid ${r.key === resolution ? "transparent" : "var(--desk-outline)"}`,
                    background: r.key === resolution ? "var(--desk-primary-container)" : "transparent",
                    color: r.key === resolution ? "var(--desk-on-primary-container)" : "var(--desk-on-surface-variant)",
                    fontSize: "0.75rem",
                    fontWeight: r.key === resolution ? 700 : 500,
                    cursor: "pointer",
                  }}
                >
                  {r.label}
                </button>
              ))}
            </div>
            <PriceChart venue={venue} symbol={symbol} resolution={resolution} positions={snap!.positions ?? []} />
          </DeskCard>
        </div>

        <div style={{ display: "flex", flexDirection: "column", gap: "var(--desk-space-4)" }}>
          {instrument ? (
            <OrderTicket
              // Keyed by symbol so switching instrument REMOUNTS the ticket
              // with fresh state. Clearing it from an effect instead means a
              // render where the new instrument is already showing while the
              // old instrument's limit price is still in the box — and that is
              // a render the trader can click Buy in.
              key={instrument.symbol}
              inst={instrument}
              venue={v}
              accountLeverage={acct.account.leverage}
              freeMargin={acct.freeMargin}
              busy={busy}
              onPlace={(payload) => act("order", payload, "Order accepted.")}
            />
          ) : null}
          <BookLadder book={book} inst={instrument} />
        </div>
      </div>

      <DeskCard padding="md">
        <DeskTabs
          items={
            [
              { key: "positions", label: `Positions (${snap!.positions?.length ?? 0})` },
              { key: "orders", label: `Orders (${snap!.orders?.length ?? 0})` },
              { key: "history", label: `History (${snap!.trades?.length ?? 0})` },
              { key: "account", label: "Account" },
            ] as DeskTabItem<typeof tab>[]
          }
          active={tab}
          onChange={setTab}
        />
        <div style={{ marginTop: "var(--desk-space-4)" }}>
          {tab === "positions" ? (
            <PositionsTable
              rows={snap!.positions ?? []}
              venue={v}
              busy={busy}
              onClose={(positionId) => act("close", { positionId }, "Position closed.")}
              onModify={(positionId, patch) => act("modify", { positionId, ...patch }, "Levels updated.")}
            />
          ) : null}
          {tab === "orders" ? (
            <OrdersTable rows={snap!.orders ?? []} busy={busy} onCancel={(orderId) => act("cancel", { orderId }, "Order cancelled.")} />
          ) : null}
          {tab === "history" ? <HistoryTable rows={snap!.trades ?? []} /> : null}
          {tab === "account" ? (
            <AccountPanel
              venue={v}
              acct={acct}
              busy={busy}
              tick={snap!.tick}
              onSettings={(patch) => act("settings", patch, "Account updated.")}
              onTick={() => act("tick", {}, "Cycle run.")}
              onReset={async () => {
                setBusy(true);
                setNotice(null);
                try {
                  await api("reset?confirm=true", { method: "POST" });
                  setNotice({ tone: "success", text: "Account reset to its opening balance." });
                  await refresh();
                } catch (e) {
                  setNotice({ tone: "error", text: e instanceof Error ? e.message : "reset failed" });
                } finally {
                  setBusy(false);
                }
              }}
            />
          ) : null}
        </div>
      </DeskCard>

      <p className="desk-label-md" style={{ fontWeight: 400, opacity: 0.72, lineHeight: 1.55 }}>
        {v.dataNote}
      </p>
    </div>
  );
}

// ── account strip ───────────────────────────────────────────────────────────

function AccountStrip({
  acct,
  venue,
  stats,
}: {
  acct: NonNullable<Snapshot["account"]>;
  venue: VenueMeta;
  stats: Record<string, number | null>;
}) {
  const level = acct.marginLevelPct;
  const levelTone =
    level === null ? "" : level < venue.stopOutLevelPct ? "desk-pnl-negative" : level < venue.marginCallLevelPct ? "" : "desk-pnl-positive";
  return (
    <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(150px, 1fr))", gap: "var(--desk-space-3)" }}>
      <DeskMetricTile label="Balance" value={usd(acct.balance)} detail="realised cash" compact />
      <DeskMetricTile
        label="Equity"
        value={usd(acct.equity)}
        detail="balance + open P&L"
        valueClassName={pnlClass(acct.equity - acct.account.startingBalance)}
        compact
      />
      <DeskMetricTile
        label="Unrealised"
        value={usd(acct.unrealisedPnlUsd)}
        detail="net of exit costs"
        valueClassName={pnlClass(acct.unrealisedPnlUsd)}
        compact
        title="Marked at the price the position would CLOSE at — a long exits on the bid — and net of the fees and carry already accrued."
      />
      <DeskMetricTile label="Used margin" value={usd(acct.usedMargin)} detail={`${acct.openPositions} open`} compact />
      <DeskMetricTile label="Free margin" value={usd(acct.freeMargin)} detail="available to commit" compact />
      <DeskMetricTile
        label="Margin level"
        value={level === null ? "—" : `${level.toFixed(1)}%`}
        detail={venue.stopOutLevelPct > 0 ? `stop out at ${venue.stopOutLevelPct}%` : "no account-wide stop out"}
        valueClassName={levelTone}
        compact
        title={
          venue.stopOutLevelPct > 0
            ? "Equity over used margin. Below the stop-out level the desk closes the worst position and re-checks, repeating until the level recovers."
            : "This venue liquidates position by position against maintenance margin rather than on an account-wide level."
        }
      />
      <DeskMetricTile
        label="Closed trades"
        value={String(stats.trades ?? 0)}
        detail={stats.winRate !== null && stats.winRate !== undefined ? `${stats.winRate}% won` : "none yet"}
        compact
      />
      <DeskMetricTile
        label="Realised P&L"
        value={usd(stats.netPnlUsd ?? 0)}
        detail={stats.profitFactor ? `PF ${stats.profitFactor}` : "—"}
        valueClassName={pnlClass(stats.netPnlUsd ?? 0)}
        compact
      />
    </div>
  );
}

// ── instrument picker + ticker ──────────────────────────────────────────────

function InstrumentPicker({
  instruments,
  symbol,
  onPick,
  sizeUnit,
}: {
  instruments: Instrument[];
  symbol: string;
  onPick: (s: string) => void;
  sizeUnit: string;
}) {
  const [q, setQ] = useState("");
  const shown = useMemo(() => {
    const needle = q.trim().toUpperCase();
    const list = needle
      ? instruments.filter((i) => i.symbol.includes(needle) || i.displayName.toUpperCase().includes(needle))
      : instruments;
    return list.slice(0, 40);
  }, [instruments, q]);

  return (
    <div>
      <div style={{ display: "flex", gap: 10, alignItems: "center", marginBottom: 10 }}>
        <div style={{ minWidth: 190 }}>
          <DeskSearchField label="Instrument" placeholder="search" value={q} onChange={(e) => setQ(e.target.value)} />
        </div>
        <span className="desk-label-md" style={{ fontWeight: 400, opacity: 0.65 }}>
          {instruments.length} listed · size in {sizeUnit}
        </span>
      </div>
      <div style={{ display: "flex", gap: 6, overflowX: "auto", paddingBottom: 4 }}>
        {shown.map((i) => (
          <button
            key={i.symbol}
            type="button"
            onClick={() => onPick(i.symbol)}
            title={i.displayName}
            style={{
              flexShrink: 0,
              padding: "6px 12px",
              minHeight: 40,
              borderRadius: "var(--desk-radius-chip)",
              border: `1px solid ${i.symbol === symbol ? "transparent" : "var(--desk-outline)"}`,
              background: i.symbol === symbol ? "var(--desk-primary-container)" : "transparent",
              color: i.symbol === symbol ? "var(--desk-on-primary-container)" : "var(--desk-on-surface-variant)",
              cursor: "pointer",
              display: "flex",
              flexDirection: "column",
              alignItems: "flex-start",
              gap: 1,
            }}
          >
            <span style={{ fontWeight: 700, fontSize: "0.8125rem" }}>{i.symbol}</span>
            <span className={pnlClass(i.change24hPct)} style={{ fontSize: "0.6875rem" }}>
              {pct(i.change24hPct, 2)}
            </span>
          </button>
        ))}
      </div>
    </div>
  );
}

function TickerStrip({ inst }: { inst: Instrument }) {
  const spread = inst.ask - inst.bid;
  const spreadPts = inst.tickSize > 0 ? spread / inst.tickSize : 0;
  return (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 18, alignItems: "center", marginTop: 12, paddingTop: 12, borderTop: "1px solid var(--desk-outline-variant)" }}>
      <div>
        <div style={{ fontWeight: 800, fontSize: "1.35rem", fontFamily: "var(--desk-font-mono)" }}>
          {px(inst.last, inst.pricePrecision)}
        </div>
        <div className={pnlClass(inst.change24hPct)} style={{ fontSize: "0.8125rem", fontWeight: 600 }}>
          {pct(inst.change24hPct, 2)} · 24h
        </div>
      </div>
      <Field label="Bid" value={px(inst.bid, inst.pricePrecision)} />
      <Field label="Ask" value={px(inst.ask, inst.pricePrecision)} />
      <Field
        label={inst.spreadIsModelled ? "Spread (modelled)" : "Spread"}
        value={`${spreadPts.toFixed(1)} pts`}
        warn={inst.spreadIsModelled}
        title={
          inst.spreadIsModelled
            ? "MODELLED, not quoted. No free feed publishes a retail broker's bid and ask, so this spread comes from a per-instrument table scaled by account tier. The mid price it brackets is real."
            : "Quoted by the venue — this is the real top of book."
        }
      />
      <Field label="24h High" value={px(inst.high24h, inst.pricePrecision)} />
      <Field label="24h Low" value={px(inst.low24h, inst.pricePrecision)} />
      {inst.carryKind === "funding" ? (
        <Field
          label="Funding / 8h"
          value={inst.fundingRatePct8h === null ? "—" : `${inst.fundingRatePct8h.toFixed(4)}%`}
          title="Published by the venue. Positive means longs pay shorts, every eight hours."
        />
      ) : (
        <Field
          label="Swap L/S (modelled)"
          value={`${inst.swapLongPointsPerDay ?? "—"} / ${inst.swapShortPointsPerDay ?? "—"}`}
          warn
          title="Points per lot per day, charged at 21:00 UTC rollover and TRIPLED on Wednesday for the weekend value date. Broker-specific and therefore modelled."
        />
      )}
      <Field label="Max leverage" value={`${inst.maxLeverage}x`} />
    </div>
  );
}

function Field({ label, value, title, warn }: { label: string; value: string; title?: string; warn?: boolean }) {
  return (
    <div title={title}>
      <div className="desk-label-md" style={{ fontWeight: 400, opacity: 0.65, fontSize: "0.6875rem" }}>
        {label}
        {warn ? <span style={{ color: "var(--desk-warning)" }}> ⚠</span> : null}
      </div>
      <div style={{ fontFamily: "var(--desk-font-mono)", fontSize: "0.875rem", fontWeight: 600 }}>{value}</div>
    </div>
  );
}

// ── chart ───────────────────────────────────────────────────────────────────

function PriceChart({
  venue,
  symbol,
  resolution,
  positions,
}: {
  venue: string;
  symbol: string;
  resolution: string;
  positions: Row[];
}) {
  const ref = useRef<HTMLDivElement | null>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const seriesRef = useRef<ISeriesApi<"Candlestick"> | null>(null);
  const [err, setErr] = useState("");

  useEffect(() => {
    if (!ref.current) return undefined;
    const chart = createChart(ref.current, {
      height: 380,
      layout: { background: { color: "transparent" }, textColor: "#6b7280", fontSize: 11 },
      grid: {
        vertLines: { color: "rgba(128,128,128,0.10)", style: LineStyle.Solid },
        horzLines: { color: "rgba(128,128,128,0.10)", style: LineStyle.Solid },
      },
      rightPriceScale: { borderColor: "rgba(128,128,128,0.25)" },
      timeScale: { borderColor: "rgba(128,128,128,0.25)", timeVisible: true, secondsVisible: false },
      autoSize: true,
    });
    const series = chart.addSeries(CandlestickSeries, {
      upColor: "#16a34a",
      downColor: "#dc2626",
      borderUpColor: "#16a34a",
      borderDownColor: "#dc2626",
      wickUpColor: "#16a34a",
      wickDownColor: "#dc2626",
    });
    chartRef.current = chart;
    seriesRef.current = series;
    return () => {
      chart.remove();
      chartRef.current = null;
      seriesRef.current = null;
    };
  }, []);

  useEffect(() => {
    if (!symbol || !seriesRef.current) return undefined;
    let live = true;
    const pull = async () => {
      try {
        const r = await fetch(
          `/api/paper-trading/${venue}/candles?symbol=${encodeURIComponent(symbol)}&resolution=${resolution}&bars=300`,
          { cache: "no-store" },
        );
        const body = (await r.json()) as { ok?: boolean; error?: string; candles?: { time: number; open: number; high: number; low: number; close: number }[] };
        if (!r.ok) throw new Error(body.error ?? `HTTP ${r.status}`);
        if (!live || !seriesRef.current) return;
        const data = (body.candles ?? []).map((c) => ({
          time: c.time as Time,
          open: c.open,
          high: c.high,
          low: c.low,
          close: c.close,
        })) as CandlestickData[];
        seriesRef.current.setData(data);
        chartRef.current?.timeScale().fitContent();
        setErr(data.length === 0 ? "The venue returned no bars for this instrument and interval." : "");
      } catch (e) {
        if (live) setErr(e instanceof Error ? e.message : "chart failed");
      }
    };
    void pull();
    const t = setInterval(pull, 20_000);
    return () => {
      live = false;
      clearInterval(t);
    };
  }, [venue, symbol, resolution]);

  // Entry, take-profit and stop lines for whatever is open on this instrument,
  // so the chart shows the position rather than just the market.
  useEffect(() => {
    const series = seriesRef.current;
    if (!series) return undefined;
    const mine = positions.filter((p) => p.symbol === symbol);
    const lines = mine.flatMap((p) => {
      const out = [
        series.createPriceLine({
          price: Number(p.entryPrice),
          color: "#6b7280",
          lineWidth: 1,
          lineStyle: LineStyle.Solid,
          axisLabelVisible: true,
          title: `${String(p.side).toUpperCase()} ${p.size}`,
        }),
      ];
      if (p.takeProfit != null) {
        out.push(series.createPriceLine({ price: Number(p.takeProfit), color: "#16a34a", lineWidth: 1, lineStyle: LineStyle.Dashed, axisLabelVisible: true, title: "TP" }));
      }
      if (p.stopLoss != null) {
        out.push(series.createPriceLine({ price: Number(p.stopLoss), color: "#dc2626", lineWidth: 1, lineStyle: LineStyle.Dashed, axisLabelVisible: true, title: "SL" }));
      }
      if (p.liquidationPrice != null) {
        out.push(series.createPriceLine({ price: Number(p.liquidationPrice), color: "#f59e0b", lineWidth: 1, lineStyle: LineStyle.Dotted, axisLabelVisible: true, title: "LIQ" }));
      }
      return out;
    });
    return () => {
      for (const l of lines) {
        try {
          series.removePriceLine(l);
        } catch {
          /* the series was torn down first */
        }
      }
    };
  }, [positions, symbol]);

  return (
    <div>
      <div ref={ref} style={{ width: "100%", height: 380 }} />
      {err ? (
        <p className="desk-label-md" style={{ fontWeight: 400, color: "var(--desk-error)", marginTop: 8 }}>
          {err}
        </p>
      ) : null}
    </div>
  );
}

// ── order book ──────────────────────────────────────────────────────────────

function BookLadder({ book, inst }: { book: Book | null; inst: Instrument | null }) {
  if (!book || !inst) {
    return (
      <DeskCard padding="md">
        <div className="desk-label-md" style={{ opacity: 0.6 }}>
          No order book available for this instrument right now.
        </div>
      </DeskCard>
    );
  }
  const maxSize = Math.max(...book.bids.map((b) => b.size), ...book.asks.map((a) => a.size), 1);
  const spread = (book.asks[0]?.price ?? 0) - (book.bids[0]?.price ?? 0);

  const Row = ({ price, size, side }: { price: number; size: number; side: "bid" | "ask" }) => (
    <div style={{ position: "relative", display: "flex", justifyContent: "space-between", padding: "2px 8px", fontSize: "0.75rem", fontFamily: "var(--desk-font-mono)" }}>
      <div
        aria-hidden
        style={{
          position: "absolute",
          inset: 0,
          width: `${Math.min(100, (size / maxSize) * 100)}%`,
          background: side === "bid" ? "rgba(22,163,74,0.13)" : "rgba(220,38,38,0.13)",
          [side === "bid" ? "left" : "right"]: 0,
        }}
      />
      <span style={{ position: "relative", color: side === "bid" ? "var(--desk-success)" : "var(--desk-error)" }}>
        {px(price, inst.pricePrecision)}
      </span>
      <span style={{ position: "relative", opacity: 0.75 }}>{size.toLocaleString()}</span>
    </div>
  );

  return (
    <DeskCard padding="md">
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
        <span className="desk-label-md" style={{ fontWeight: 700 }}>
          Order book
        </span>
        {book.modelled ? (
          <DeskChip
            tone="warning"
            title="This venue publishes no depth. The ladder is a synthetic decay around a real mid price and exists so the ticket has a bid and an ask — it is not anyone's resting liquidity."
          >
            modelled
          </DeskChip>
        ) : (
          <DeskChip tone="success" title="Real resting depth, straight from the venue.">
            live depth
          </DeskChip>
        )}
      </div>
      <div style={{ display: "flex", flexDirection: "column-reverse" }}>
        {book.asks.slice(0, 8).map((a, i) => (
          <Row key={`a${i}`} price={a.price} size={a.size} side="ask" />
        ))}
      </div>
      <div style={{ padding: "6px 8px", textAlign: "center", fontSize: "0.75rem", opacity: 0.7, borderTop: "1px solid var(--desk-outline-variant)", borderBottom: "1px solid var(--desk-outline-variant)", margin: "4px 0" }}>
        spread {px(spread, inst.pricePrecision)}
      </div>
      <div>
        {book.bids.slice(0, 8).map((b, i) => (
          <Row key={`b${i}`} price={b.price} size={b.size} side="bid" />
        ))}
      </div>
    </DeskCard>
  );
}

// ── order ticket ────────────────────────────────────────────────────────────

type TicketPayload = {
  symbol: string;
  side: "buy" | "sell";
  type: string;
  size: number;
  limitPrice: number | null;
  stopPrice: number | null;
  leverage: number;
  takeProfit: number | null;
  stopLoss: number | null;
  reduceOnly: boolean;
  postOnly: boolean;
};

function OrderTicket({
  inst,
  venue,
  accountLeverage,
  freeMargin,
  busy,
  onPlace,
}: {
  inst: Instrument;
  venue: VenueMeta;
  accountLeverage: number;
  freeMargin: number;
  busy: boolean;
  onPlace: (p: TicketPayload) => void;
}) {
  const [side, setSide] = useState<"buy" | "sell">("buy");
  const [type, setType] = useState("market");
  const [size, setSize] = useState(String(inst.sizeUnit === "lots" ? 0.1 : 10));
  const [limitPrice, setLimitPrice] = useState("");
  const [stopPrice, setStopPrice] = useState("");
  const [leverage, setLeverage] = useState(accountLeverage);
  const [tp, setTp] = useState("");
  const [sl, setSl] = useState("");
  const [reduceOnly, setReduceOnly] = useState(false);
  const [postOnly, setPostOnly] = useState(false);

  const sizeNum = Number(size) || 0;
  const lev = Math.min(leverage, inst.maxLeverage);
  const refPrice = type === "limit" || type === "stop_limit" ? Number(limitPrice) || inst.ask : side === "buy" ? inst.ask : inst.bid;

  // Estimated up front, so the trader sees the commitment before committing.
  const notional = sizeNum * inst.contractSize * refPrice;
  const margin = lev > 0 ? notional / lev : notional;
  const fee = notional * inst.takerFeeRate + inst.commissionPerLotUsd * sizeNum;
  const cushion = 1 / lev - inst.maintenanceMarginPct / 100;
  const liq =
    inst.maintenanceMarginPct > 0 && cushion > 0
      ? side === "buy"
        ? refPrice * (1 - cushion)
        : refPrice * (1 + cushion)
      : null;
  const affordable = margin <= freeMargin;

  const numOrNull = (s: string) => (s.trim() === "" ? null : Number(s));

  return (
    <DeskCard padding="md">
      <div style={{ display: "flex", gap: 6, marginBottom: 12 }}>
        {(["buy", "sell"] as const).map((s) => (
          <button
            key={s}
            type="button"
            onClick={() => setSide(s)}
            style={{
              flex: 1,
              minHeight: 42,
              borderRadius: "var(--desk-radius-button)",
              border: "none",
              cursor: "pointer",
              fontWeight: 700,
              fontSize: "0.875rem",
              background:
                side === s ? (s === "buy" ? "var(--desk-success-container)" : "var(--desk-error-container)") : "var(--desk-surface-container)",
              color: side === s ? (s === "buy" ? "var(--desk-success)" : "var(--desk-error)") : "var(--desk-on-surface-variant)",
            }}
          >
            {s === "buy" ? "Buy / Long" : "Sell / Short"}
          </button>
        ))}
      </div>

      <div style={{ display: "flex", gap: 4, marginBottom: 12, flexWrap: "wrap" }}>
        {[
          { k: "market", l: "Market" },
          { k: "limit", l: "Limit" },
          { k: "stop_market", l: "Stop" },
          { k: "stop_limit", l: "Stop-Limit" },
        ].map((t) => (
          <button
            key={t.k}
            type="button"
            onClick={() => setType(t.k)}
            style={{
              padding: "4px 10px",
              minHeight: 30,
              borderRadius: "var(--desk-radius-chip)",
              border: `1px solid ${t.k === type ? "transparent" : "var(--desk-outline)"}`,
              background: t.k === type ? "var(--desk-primary-container)" : "transparent",
              color: t.k === type ? "var(--desk-on-primary-container)" : "var(--desk-on-surface-variant)",
              fontSize: "0.75rem",
              fontWeight: t.k === type ? 700 : 500,
              cursor: "pointer",
            }}
          >
            {t.l}
          </button>
        ))}
      </div>

      <Input label={`Size (${inst.sizeUnit})`} value={size} onChange={setSize} hint={`min ${inst.minSize}, step ${inst.sizeStep}`} />
      {(type === "limit" || type === "stop_limit") && (
        <Input label="Limit price" value={limitPrice} onChange={setLimitPrice} hint={`tick ${inst.tickSize}`} />
      )}
      {(type === "stop_market" || type === "stop_limit") && (
        <Input label="Stop price" value={stopPrice} onChange={setStopPrice} hint="triggers on touch" />
      )}

      <div style={{ margin: "10px 0" }}>
        <div className="desk-label-md" style={{ fontWeight: 400, opacity: 0.7, marginBottom: 4 }}>
          Leverage — max {inst.maxLeverage}x here
        </div>
        <div style={{ display: "flex", gap: 4, flexWrap: "wrap" }}>
          {venue.leverageChoices
            .filter((l) => l <= inst.maxLeverage)
            .map((l) => (
              <button
                key={l}
                type="button"
                onClick={() => setLeverage(l)}
                style={{
                  padding: "3px 9px",
                  minHeight: 28,
                  borderRadius: "var(--desk-radius-chip)",
                  border: `1px solid ${l === lev ? "transparent" : "var(--desk-outline)"}`,
                  background: l === lev ? "var(--desk-primary-container)" : "transparent",
                  color: l === lev ? "var(--desk-on-primary-container)" : "var(--desk-on-surface-variant)",
                  fontSize: "0.6875rem",
                  fontWeight: l === lev ? 700 : 500,
                  cursor: "pointer",
                }}
              >
                {l}x
              </button>
            ))}
        </div>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
        <Input label="Take profit" value={tp} onChange={setTp} hint="optional" />
        <Input label="Stop loss" value={sl} onChange={setSl} hint="optional" />
      </div>

      {venue.id === "delta" ? (
        <div style={{ display: "flex", gap: 14, margin: "10px 0", fontSize: "0.75rem" }}>
          <label style={{ display: "flex", gap: 5, alignItems: "center", cursor: "pointer" }}>
            <input type="checkbox" checked={reduceOnly} onChange={(e) => setReduceOnly(e.target.checked)} />
            Reduce only
          </label>
          <label style={{ display: "flex", gap: 5, alignItems: "center", cursor: "pointer" }}>
            <input type="checkbox" checked={postOnly} onChange={(e) => setPostOnly(e.target.checked)} />
            Post only
          </label>
        </div>
      ) : null}

      <div
        style={{
          background: "var(--desk-surface-container)",
          borderRadius: "var(--desk-radius-chip)",
          padding: "10px 12px",
          margin: "12px 0",
          fontSize: "0.75rem",
          display: "flex",
          flexDirection: "column",
          gap: 4,
        }}
      >
        <Est label="Notional" value={usd(notional)} />
        <Est label="Margin required" value={usd(margin)} warn={!affordable} />
        <Est label={inst.commissionPerLotUsd > 0 ? "Fee + commission" : "Fee"} value={usd(fee, 4)} />
        <Est
          label="Liquidation"
          value={liq === null ? "n/a on this venue" : px(liq, inst.pricePrecision)}
          title={
            liq === null
              ? "This venue has no per-position maintenance tier — the account is managed on margin level and stopped out there."
              : "Where the venue closes this position regardless of your own stop."
          }
        />
      </div>

      {!affordable ? (
        <p className="desk-label-md" style={{ fontWeight: 400, color: "var(--desk-error)", marginBottom: 8 }}>
          Free margin is {usd(freeMargin)}; this needs {usd(margin)}.
        </p>
      ) : null}

      <DeskButton
        variant="filled"
        disabled={busy || !(sizeNum > 0)}
        style={{
          width: "100%",
          background: side === "buy" ? "var(--desk-success)" : "var(--desk-error)",
          color: "#fff",
        }}
        onClick={() =>
          onPlace({
            symbol: inst.symbol,
            side,
            type,
            size: sizeNum,
            limitPrice: numOrNull(limitPrice),
            stopPrice: numOrNull(stopPrice),
            leverage: lev,
            takeProfit: numOrNull(tp),
            stopLoss: numOrNull(sl),
            reduceOnly,
            postOnly,
          })
        }
      >
        {busy ? "Placing…" : `${side === "buy" ? "Buy" : "Sell"} ${size} ${inst.sizeUnit}`}
      </DeskButton>

      <p className="desk-label-md" style={{ fontWeight: 400, opacity: 0.6, marginTop: 8, fontSize: "0.6875rem", lineHeight: 1.5 }}>
        Paper money. A market order is walked through the book shown above and receives the
        size-weighted price it would actually have paid.
      </p>
    </DeskCard>
  );
}

function Input({ label, value, onChange, hint }: { label: string; value: string; onChange: (v: string) => void; hint?: string }) {
  return (
    <label style={{ display: "block", marginBottom: 8 }}>
      <span className="desk-label-md" style={{ fontWeight: 400, opacity: 0.7, fontSize: "0.6875rem" }}>
        {label}
        {hint ? <span style={{ opacity: 0.7 }}> · {hint}</span> : null}
      </span>
      <input
        value={value}
        inputMode="decimal"
        onChange={(e) => onChange(e.target.value)}
        style={{
          width: "100%",
          marginTop: 3,
          padding: "8px 10px",
          minHeight: 38,
          borderRadius: "var(--desk-radius-chip)",
          border: "1px solid var(--desk-outline)",
          background: "var(--desk-surface)",
          color: "var(--desk-on-surface)",
          fontFamily: "var(--desk-font-mono)",
          fontSize: "0.8125rem",
        }}
      />
    </label>
  );
}

function Est({ label, value, warn, title }: { label: string; value: string; warn?: boolean; title?: string }) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between" }} title={title}>
      <span style={{ opacity: 0.7 }}>{label}</span>
      <span style={{ fontFamily: "var(--desk-font-mono)", fontWeight: 600, color: warn ? "var(--desk-error)" : undefined }}>{value}</span>
    </div>
  );
}

// ── tables ──────────────────────────────────────────────────────────────────

function PositionsTable({
  rows,
  venue,
  busy,
  onClose,
  onModify,
}: {
  rows: Row[];
  venue: VenueMeta;
  busy: boolean;
  onClose: (id: string) => void;
  onModify: (id: string, patch: { takeProfit?: number | null; stopLoss?: number | null }) => void;
}) {
  const cols: DeskColumn<Row>[] = [
    {
      id: "symbol",
      header: "Instrument",
      cell: (r) => (
        <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <span style={{ fontWeight: 700 }}>{String(r.symbol)}</span>
          <DeskChip tone={r.side === "long" ? "success" : "error"}>
            {String(r.side).toUpperCase()} {String(r.size)} · {String(r.leverage)}x
          </DeskChip>
        </div>
      ),
      sortValue: (r) => String(r.symbol),
    },
    { id: "entry", header: "Entry", align: "right", cell: (r) => px(r.entryPrice as number, 6) },
    { id: "mark", header: "Mark", align: "right", cell: (r) => px(r.mark as number, 6) },
    {
      id: "pnl",
      header: "Unrealised",
      align: "right",
      cell: (r) => (
        <strong className={pnlClass(r.unrealisedUsd as number)}>
          {usd(r.unrealisedUsd as number)}{" "}
          <span style={{ fontSize: "0.6875rem", opacity: 0.75 }}>({pct(r.unrealisedPct as number, 1)})</span>
        </strong>
      ),
      sortValue: (r) => (r.unrealisedUsd as number) ?? null,
    },
    {
      id: "pips",
      header: venue.sizeUnit === "lots" ? "Pips" : "Ticks",
      align: "right",
      cell: (r) => <span className={pnlClass(r.pips as number)}>{r.pips === null ? "—" : (r.pips as number).toFixed(1)}</span>,
      sortValue: (r) => (r.pips as number) ?? null,
    },
    { id: "margin", header: "Margin", align: "right", cell: (r) => usd(r.marginUsd as number), sortValue: (r) => (r.marginUsd as number) ?? null },
    {
      id: "liq",
      header: "Liquidation",
      align: "right",
      cell: (r) =>
        r.liquidationPrice == null ? (
          <span style={{ opacity: 0.45 }} title="This venue stops the account out on margin level rather than liquidating each position.">
            n/a
          </span>
        ) : (
          <span style={{ color: "var(--desk-warning)" }}>{px(r.liquidationPrice as number, 6)}</span>
        ),
    },
    {
      id: "levels",
      header: "TP / SL",
      cell: (r) => (
        <div style={{ display: "flex", gap: 4, alignItems: "center", fontSize: "0.75rem" }}>
          <span style={{ color: "var(--desk-success)" }}>{r.takeProfit == null ? "—" : px(r.takeProfit as number, 6)}</span>
          <span style={{ opacity: 0.4 }}>/</span>
          <span style={{ color: "var(--desk-error)" }}>{r.stopLoss == null ? "—" : px(r.stopLoss as number, 6)}</span>
          <button
            type="button"
            disabled={busy}
            onClick={() => {
              const tp = window.prompt("Take profit (blank to clear)", r.takeProfit == null ? "" : String(r.takeProfit));
              if (tp === null) return;
              const sl = window.prompt("Stop loss (blank to clear)", r.stopLoss == null ? "" : String(r.stopLoss));
              if (sl === null) return;
              onModify(String(r.positionId), {
                takeProfit: tp.trim() === "" ? null : Number(tp),
                stopLoss: sl.trim() === "" ? null : Number(sl),
              });
            }}
            style={{ marginLeft: 4, background: "transparent", border: "1px solid var(--desk-outline)", borderRadius: 6, padding: "1px 6px", cursor: "pointer", fontSize: "0.6875rem" }}
          >
            edit
          </button>
        </div>
      ),
      sortable: false,
    },
    {
      id: "carry",
      header: venue.sizeUnit === "lots" ? "Swap" : "Funding",
      align: "right",
      cell: (r) => <span className={pnlClass(-(r.carryUsd as number))}>{usd(r.carryUsd as number, 4)}</span>,
      sortValue: (r) => (r.carryUsd as number) ?? null,
    },
    {
      id: "act",
      header: "",
      cell: (r) => (
        <DeskButton variant="danger-tonal" disabled={busy} onClick={() => onClose(String(r.positionId))} style={{ minHeight: 32, padding: "0 12px", fontSize: "0.75rem" }}>
          Close
        </DeskButton>
      ),
      sortable: false,
    },
  ];

  return (
    <DeskDataTable
      columns={cols}
      rows={rows}
      getRowKey={(r) => String(r.positionId)}
      minWidth={1250}
      empty={
        <DeskBanner variant="info" title="No open positions">
          Place an order from the ticket to open one.{" "}
          {venue.positionMode === "hedging"
            ? "This venue hedges: a long and a short on the same instrument are separate tickets."
            : "This venue nets: a second order on the same instrument merges into one averaged position."}
        </DeskBanner>
      }
    />
  );
}

function OrdersTable({ rows, busy, onCancel }: { rows: Row[]; busy: boolean; onCancel: (id: string) => void }) {
  const cols: DeskColumn<Row>[] = [
    { id: "symbol", header: "Instrument", cell: (r) => <strong>{String(r.symbol)}</strong>, sortValue: (r) => String(r.symbol) },
    {
      id: "side",
      header: "Order",
      cell: (r) => (
        <DeskChip tone={r.side === "buy" ? "success" : "error"}>
          {String(r.side).toUpperCase()} {String(r.type).replace("_", " ")}
        </DeskChip>
      ),
      sortValue: (r) => String(r.type),
    },
    { id: "size", header: "Size", align: "right", cell: (r) => String(r.size) },
    { id: "limit", header: "Limit", align: "right", cell: (r) => (r.limitPrice == null ? "—" : px(r.limitPrice as number, 6)) },
    { id: "stop", header: "Stop", align: "right", cell: (r) => (r.stopPrice == null ? "—" : px(r.stopPrice as number, 6)) },
    {
      id: "trig",
      header: "State",
      cell: (r) => (
        <DeskChip tone={r.triggered ? "warning" : "default"} title={r.triggered ? "The stop has been touched; it is now working as a market or limit order." : "Resting. Resolved by bar replay when the desk next ticks."}>
          {r.triggered ? "triggered" : "resting"}
        </DeskChip>
      ),
      sortValue: (r) => (r.triggered ? 1 : 0),
    },
    { id: "created", header: "Placed", align: "right", cell: (r) => fmtISTClock((r.createdAt as number) * 1000), sortValue: (r) => (r.createdAt as number) ?? null },
    {
      id: "act",
      header: "",
      cell: (r) => (
        <DeskButton variant="outlined" disabled={busy} onClick={() => onCancel(String(r.orderId))} style={{ minHeight: 32, padding: "0 12px", fontSize: "0.75rem" }}>
          Cancel
        </DeskButton>
      ),
      sortable: false,
    },
  ];
  return (
    <DeskDataTable
      columns={cols}
      rows={rows}
      getRowKey={(r) => String(r.orderId)}
      minWidth={1000}
      empty={
        <DeskBanner variant="info" title="No working orders">
          A limit or stop order rests here until the market reaches it. It is resolved by replaying the
          bars since the desk last looked, so a fill that happened overnight is recorded at that bar&apos;s
          price and time rather than at the current one.
        </DeskBanner>
      }
    />
  );
}

function HistoryTable({ rows }: { rows: Row[] }) {
  const cols: DeskColumn<Row>[] = [
    { id: "symbol", header: "Instrument", cell: (r) => <strong>{String(r.symbol)}</strong>, sortValue: (r) => String(r.symbol) },
    {
      id: "side",
      header: "Side",
      cell: (r) => (
        <DeskChip tone={r.side === "long" ? "success" : "error"}>
          {String(r.side).toUpperCase()} {String(r.size)}
        </DeskChip>
      ),
      sortValue: (r) => String(r.side),
    },
    { id: "entry", header: "Entry", align: "right", cell: (r) => px(r.entryPrice as number, 6) },
    { id: "exit", header: "Exit", align: "right", cell: (r) => px(r.exitPrice as number, 6) },
    {
      id: "reason",
      header: "Closed by",
      cell: (r) => {
        const reason = String(r.exitReason);
        const tone = reason === "take_profit" ? "success" : reason === "manual" ? "default" : "error";
        return (
          <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <DeskChip tone={tone}>{reason.replace("_", " ")}</DeskChip>
            {r.ambiguous ? (
              <span style={{ fontSize: "0.625rem", color: "var(--desk-warning)" }} title="The take-profit and the stop both fell inside one replay bar. OHLC cannot say which printed first, so the STOP was assumed — the unfavourable branch.">
                assumed stop first
              </span>
            ) : null}
            {r.gapped ? (
              <span style={{ fontSize: "0.625rem", color: "var(--desk-warning)" }} title="Price opened beyond the level, so the fill is the bar's open — which is what the order would actually have received.">
                gapped fill
              </span>
            ) : null}
          </div>
        );
      },
      sortValue: (r) => String(r.exitReason),
    },
    {
      id: "pnl",
      header: "Net P&L",
      align: "right",
      cell: (r) => <strong className={pnlClass(r.netPnlUsd as number)}>{usd(r.netPnlUsd as number)}</strong>,
      sortValue: (r) => (r.netPnlUsd as number) ?? null,
    },
    { id: "ret", header: "Return", align: "right", cell: (r) => <span className={pnlClass(r.returnPct as number)}>{pct(r.returnPct as number, 1)}</span>, sortValue: (r) => (r.returnPct as number) ?? null },
    { id: "costs", header: "Fees / carry", align: "right", cell: (r) => `${usd(r.feesUsd as number, 3)} / ${usd(r.carryUsd as number, 3)}` },
    { id: "hold", header: "Hold", align: "right", cell: (r) => `${(r.holdHours as number).toFixed(1)}h`, sortValue: (r) => (r.holdHours as number) ?? null },
    { id: "closed", header: "Closed", align: "right", cell: (r) => fmtISTClock((r.closedAt as number) * 1000), sortValue: (r) => (r.closedAt as number) ?? null },
  ];
  return (
    <DeskDataTable
      columns={cols}
      rows={rows}
      getRowKey={(r, i) => `${r.positionId}-${i}`}
      minWidth={1250}
      defaultSort={{ id: "closed", dir: "desc" }}
      empty={<DeskBanner variant="info" title="No closed trades yet">Closed positions land here with what they cost to hold.</DeskBanner>}
    />
  );
}

function AccountPanel({
  venue,
  acct,
  busy,
  tick,
  onSettings,
  onTick,
  onReset,
}: {
  venue: VenueMeta;
  acct: NonNullable<Snapshot["account"]>;
  busy: boolean;
  tick: Snapshot["tick"];
  onSettings: (p: Record<string, unknown>) => void;
  onTick: () => void;
  onReset: () => void;
}) {
  const [confirming, setConfirming] = useState(false);
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--desk-space-4)" }}>
      <div>
        <div className="desk-label-md" style={{ fontWeight: 700, marginBottom: 6 }}>
          Default leverage
        </div>
        <div style={{ display: "flex", gap: 5, flexWrap: "wrap" }}>
          {venue.leverageChoices.map((l) => (
            <button
              key={l}
              type="button"
              disabled={busy}
              onClick={() => onSettings({ leverage: l })}
              style={{
                padding: "5px 12px",
                minHeight: 34,
                borderRadius: "var(--desk-radius-chip)",
                border: `1px solid ${l === acct.account.leverage ? "transparent" : "var(--desk-outline)"}`,
                background: l === acct.account.leverage ? "var(--desk-primary-container)" : "transparent",
                color: l === acct.account.leverage ? "var(--desk-on-primary-container)" : "var(--desk-on-surface-variant)",
                fontWeight: l === acct.account.leverage ? 700 : 500,
                fontSize: "0.75rem",
                cursor: "pointer",
              }}
            >
              1:{l}
            </button>
          ))}
        </div>
      </div>

      {venue.accountTypes.length > 1 ? (
        <div>
          <div className="desk-label-md" style={{ fontWeight: 700, marginBottom: 6 }}>
            Account type — this changes the spread and the commission
          </div>
          <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
            {venue.accountTypes.map((a) => (
              <button
                key={a.key}
                type="button"
                disabled={busy}
                title={a.note}
                onClick={() => onSettings({ accountType: a.key })}
                style={{
                  padding: "6px 14px",
                  minHeight: 38,
                  borderRadius: "var(--desk-radius-chip)",
                  border: `1px solid ${a.key === acct.account.accountType ? "transparent" : "var(--desk-outline)"}`,
                  background: a.key === acct.account.accountType ? "var(--desk-primary-container)" : "transparent",
                  color: a.key === acct.account.accountType ? "var(--desk-on-primary-container)" : "var(--desk-on-surface-variant)",
                  fontWeight: a.key === acct.account.accountType ? 700 : 500,
                  fontSize: "0.8125rem",
                  cursor: "pointer",
                }}
              >
                {a.label}
              </button>
            ))}
          </div>
          <p className="desk-label-md" style={{ fontWeight: 400, opacity: 0.7, marginTop: 6, fontSize: "0.6875rem" }}>
            {venue.accountTypes.find((a) => a.key === acct.account.accountType)?.note}
          </p>
        </div>
      ) : null}

      {tick ? (
        <DeskBanner variant={tick.lastError ? "error" : "info"} title="How this terminal keeps time">
          {tick.lastError ? `Last cycle errored: ${tick.lastError}. ` : ""}
          {tick.note}
          {tick.lastTickAt ? ` Last cycle ${fmtISTClock(tick.lastTickAt)} IST (${tick.ticks} total).` : ""}
        </DeskBanner>
      ) : null}

      <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
        <DeskButton variant="outlined" disabled={busy} onClick={onTick}>
          Run a cycle now
        </DeskButton>
        {confirming ? (
          <>
            <DeskButton variant="danger-tonal" disabled={busy} onClick={onReset}>
              Yes, wipe this account
            </DeskButton>
            <DeskButton variant="text" onClick={() => setConfirming(false)}>
              Cancel
            </DeskButton>
          </>
        ) : (
          <DeskButton variant="text" onClick={() => setConfirming(true)}>
            Reset account
          </DeskButton>
        )}
        <span className="desk-label-md" style={{ fontWeight: 400, opacity: 0.65 }}>
          Opened at {usd(acct.account.startingBalance, 0)}
          {acct.account.resetCount > 0 ? ` · reset ${acct.account.resetCount}x` : ""}
        </span>
      </div>
      {confirming ? (
        <p className="desk-label-md" style={{ fontWeight: 400, color: "var(--desk-error)" }}>
          This deletes every order, position and closed trade on this desk and returns the balance to its
          opening figure. The trade log is the only record of what the account did, and it is not
          recoverable.
        </p>
      ) : null}
    </div>
  );
}
