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

import { useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState } from "react";
import { createChart, CandlestickSeries, LineStyle } from "lightweight-charts";
import type { CandlestickData, IChartApi, ISeriesApi, Time } from "lightweight-charts";
import {
  DeskBanner,
  DeskButton,
  DeskCard,
  DeskChip,
  DeskDataTable,
  DeskLinearProgress,
  DeskSearchField,
  DeskTabs,
  type DeskColumn,
  type DeskTabItem,
} from "@/components/desk/ui";
import { fmtISTClock } from "@/lib/istTime";
import {
  DepthChart,
  FundingCountdown,
  Kbd,
  LiveDot,
  Meter,
  Panel,
  Segmented,
  SplitMeter,
  Stat,
  Ticking,
} from "@/components/terminalkit";

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
  pipSize: number;
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

type Book = { bids: { price: number; size: number }[]; asks: { price: number; size: number }[]; modelled: boolean; asOf?: number };

type Tape = { derived: boolean; trades: { price: number; size: number; side: "buy" | "sell"; at: number }[] };

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
  const [tape, setTape] = useState<Tape | null>(null);
  const [resolution, setResolution] = useState("5m");
  /** Lets the ladder push a clicked level straight into the ticket. */
  const ticketRef = useRef<TicketHandle | null>(null);
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

  // The tape polls on its own, slower than the book: it is a longer request and
  // a print older than a few seconds is still a print, whereas a stale book
  // would mislead the fill estimate on the ticket.
  useEffect(() => {
    if (!symbol) return undefined;
    let live = true;
    const pull = async () => {
      try {
        const t = (await api(`trades?symbol=${encodeURIComponent(symbol)}&limit=32`)) as unknown as Tape;
        if (live) setTape(t);
      } catch {
        if (live) setTape(null);
      }
    };
    void pull();
    const t = setInterval(pull, 9_000);
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

      <AccountStrip acct={acct} venue={v} stats={snap!.stats ?? {}} bookAsOf={book?.asOf ?? null} />

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
            <PriceChart
              venue={venue}
              symbol={symbol}
              resolution={resolution}
              positions={snap!.positions ?? []}
              venueResolutions={v.resolutions}
            />
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
              equity={acct.equity}
              busy={busy}
              book={book}
              ref={ticketRef}
              onPlace={(payload) => act("order", payload, "Order accepted.")}
            />
          ) : null}
          <MarketDepth book={book} tape={tape} inst={instrument} onPick={(p) => ticketRef.current?.applyPrice(p)} />
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
              onClose={(positionId, size) =>
                act("close", size === undefined ? { positionId } : { positionId, size }, size === undefined ? "Position closed." : `Closed ${size}.`)
              }
              onModify={(positionId, patch) => act("modify", { positionId, ...patch }, "Levels updated.")}
            />
          ) : null}
          {tab === "orders" ? (
            <OrdersTable rows={snap!.orders ?? []} busy={busy} onCancel={(orderId) => act("cancel", { orderId }, "Order cancelled.")} />
          ) : null}
          {tab === "history" ? (
            <>
              <ArchivePanel venue={venue} busy={busy} onRestored={refresh} />
              <HistoryTable rows={snap!.trades ?? []} />
            </>
          ) : null}
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

/**
 * The account hero.
 *
 * One dense strip rather than eight cards: on a trading screen the account
 * summary is glanced at between decisions, and a grid of large tiles pushes the
 * chart and the ticket — the things being decided with — below the fold.
 */
function AccountStrip({
  acct,
  venue,
  stats,
  bookAsOf,
}: {
  acct: NonNullable<Snapshot["account"]>;
  venue: VenueMeta;
  stats: Record<string, number | null>;
  bookAsOf: number | null;
}) {
  const level = acct.marginLevelPct;
  const levelTone: "success" | "error" | "warning" | null =
    level === null
      ? null
      : level < venue.stopOutLevelPct
        ? "error"
        : level < venue.marginCallLevelPct
          ? "warning"
          : "success";
  const pnl = acct.equity - acct.account.startingBalance;

  return (
    <section className="tk-panel tk-grid" style={{ padding: "14px 18px" }}>
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fit, minmax(104px, 1fr))",
          gap: 16,
          alignItems: "start",
        }}
      >
        <Stat
          label="Equity"
          value={<Ticking value={acct.equity} format={(x) => usd(x)} />}
          sub={`${pnl >= 0 ? "+" : ""}${usd(pnl)} all time`}
          tone={pnl > 0 ? "success" : pnl < 0 ? "error" : null}
        />
        <Stat label="Balance" value={usd(acct.balance)} sub="realised cash" />
        <Stat
          label="Unrealised"
          value={<Ticking value={acct.unrealisedPnlUsd} format={(x) => usd(x)} />}
          sub="net of exit costs"
          tone={acct.unrealisedPnlUsd > 0 ? "success" : acct.unrealisedPnlUsd < 0 ? "error" : null}
          title="Marked at the price the position would CLOSE at — a long exits on the bid — and net of the fees and carry already accrued."
        />
        <Stat label="Used margin" value={usd(acct.usedMargin)} sub={`${acct.openPositions} open`} />
        <Stat label="Free margin" value={usd(acct.freeMargin)} sub="available" />
        <div>
          <Stat
            label="Margin level"
            value={level === null ? "—" : `${level.toFixed(1)}%`}
            tone={levelTone}
            title={
              venue.stopOutLevelPct > 0
                ? "Equity over used margin. Below the stop-out level the desk closes the worst position and re-checks, repeating until the level recovers."
                : "This venue liquidates position by position against maintenance margin rather than on an account-wide level."
            }
          />
          {level !== null && venue.stopOutLevelPct > 0 ? (
            <div style={{ marginTop: 4 }}>
              {/* Scaled against 5x the stop-out level, so the bar is nearly full
                  at a comfortable margin and visibly draining as it approaches
                  the threshold that actually closes positions. */}
              <Meter
                pct={Math.min(100, (level / (venue.stopOutLevelPct * 5)) * 100)}
                tone={levelTone === "error" ? "error" : levelTone === "warning" ? "warning" : "success"}
                title={`Stop out at ${venue.stopOutLevelPct}%`}
              />
            </div>
          ) : null}
        </div>
        <Stat
          label="Realised"
          value={usd(stats.netPnlUsd ?? 0)}
          sub={`${stats.trades ?? 0} trades${stats.winRate != null ? ` · ${stats.winRate}% won` : ""}`}
          tone={(stats.netPnlUsd ?? 0) > 0 ? "success" : (stats.netPnlUsd ?? 0) < 0 ? "error" : null}
        />
        <Stat
          label="Feed"
          value={
            <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
              <LiveDot asOfMs={bookAsOf ? bookAsOf * 1000 : null} />
              <span style={{ fontSize: "0.8rem" }}>{bookAsOf ? "live" : "—"}</span>
            </span>
          }
          sub="book age"
        />
      </div>
    </section>
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
  // Quoted in the instrument's OWN unit — pips on FX, ticks on a perpetual.
  // Measuring it in fractional-pip "points" instead put "25000.0 pts" on a
  // $77,000 CFD, which reads as a broken number rather than as $25.
  const unit = inst.pipSize > 0 ? inst.pipSize : inst.tickSize;
  const spreadUnits = unit > 0 ? spread / unit : 0;
  const unitLabel = inst.sizeUnit === "lots" ? "pips" : "ticks";
  return (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 18, alignItems: "center", marginTop: 12, paddingTop: 12, borderTop: "1px solid var(--desk-outline-variant)" }}>
      <div>
        <Ticking
          as="div"
          value={inst.last}
          format={(v) => px(v, inst.pricePrecision)}
          className="tk-num-lg"
        />
        <div className={pnlClass(inst.change24hPct)} style={{ fontSize: "0.8125rem", fontWeight: 600 }}>
          {pct(inst.change24hPct, 2)} · 24h
        </div>
      </div>
      <Field label="Bid" value={px(inst.bid, inst.pricePrecision)} />
      <Field label="Ask" value={px(inst.ask, inst.pricePrecision)} />
      <Field
        label={inst.spreadIsModelled ? "Spread (modelled)" : "Spread"}
        value={`${spreadUnits.toFixed(1)} ${unitLabel} · ${px(spread, inst.pricePrecision)}`}
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
        <>
          <Field
            label="Funding / 8h"
            value={inst.fundingRatePct8h === null ? "—" : `${inst.fundingRatePct8h.toFixed(4)}%`}
            title="Published by the venue. Positive means longs pay shorts, every eight hours."
          />
          <div title="Funding settles at the stamp, not pro-rata — a position opened a minute before one still pays a full interval.">
            <div className="desk-label-md" style={{ fontWeight: 400, opacity: 0.65, fontSize: "0.6875rem" }}>
              Next funding
            </div>
            <div style={{ fontSize: "0.875rem" }}>
              <FundingCountdown />
            </div>
          </div>
        </>
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
  venueResolutions,
}: {
  venue: string;
  symbol: string;
  resolution: string;
  positions: Row[];
  venueResolutions: { key: string; label: string; seconds: number }[];
}) {
  const ref = useRef<HTMLDivElement | null>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const seriesRef = useRef<ISeriesApi<"Candlestick"> | null>(null);
  const [err, setErr] = useState("");
  /** Age of the newest bar, in ms. Non-null only when it is meaningfully old. */
  const [staleMs, setStaleMs] = useState<number | null>(null);

  // Derived to a PRIMITIVE outside the effect. `venueResolutions` is rebuilt on
  // every snapshot poll, so depending on the array itself would tear down and
  // restart the candle fetch every few seconds; the number it yields is stable.
  const intervalMs = useMemo(
    () => (venueResolutions.find((r) => r.key === resolution)?.seconds ?? 300) * 1000,
    [venueResolutions, resolution],
  );

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

        // A closed market is not a failure. When the newest bar is older than a
        // few intervals the chart says how old it is, rather than either
        // pretending it is current or reporting an error it is not.
        const newest = data.length > 0 ? Number(data[data.length - 1]!.time) * 1000 : null;
        setStaleMs(newest !== null && Date.now() - newest > intervalMs * 3 ? Date.now() - newest : null);
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
  }, [venue, symbol, resolution, intervalMs]);

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
      ) : staleMs !== null ? (
        <p
          className="desk-label-md"
          style={{ fontWeight: 400, color: "var(--desk-warning)", marginTop: 8, display: "flex", alignItems: "center", gap: 6 }}
          title="This market is not trading right now, so the chart shows the most recent session rather than the last few hours."
        >
          <span className="tk-live-dot" data-state="stale" />
          Last session — the newest bar is {formatAge(staleMs)} old. This market is closed.
        </p>
      ) : null}
    </div>
  );
}

// ── market depth: ladder, cumulative curve, imbalance and tape ──────────────

function MarketDepth({
  book,
  tape,
  inst,
  onPick,
}: {
  book: Book | null;
  tape: Tape | null;
  inst: Instrument | null;
  onPick: (p: number) => void;
}) {
  const [view, setView] = useState<"book" | "tape">("book");

  if (!book || !inst) {
    return (
      <Panel title="Market depth">
        <div className="desk-label-md" style={{ opacity: 0.6 }}>
          No order book available for this instrument right now.
        </div>
      </Panel>
    );
  }

  const bids = book.bids.slice(0, 9);
  const asks = book.asks.slice(0, 9);
  const maxSize = Math.max(...bids.map((b) => b.size), ...asks.map((a) => a.size), 1);
  const spread = (asks[0]?.price ?? 0) - (bids[0]?.price ?? 0);

  // Imbalance over the VISIBLE ladder rather than the touch alone: one large
  // order at the best price is noise, and the shape of the near book is the
  // reading people actually mean by "bid heavy".
  const bidVol = bids.reduce((t, b) => t + b.size, 0);
  const askVol = asks.reduce((t, a) => t + a.size, 0);
  const imbalance = bidVol + askVol > 0 ? (bidVol - askVol) / (bidVol + askVol) : 0;

  const Row = ({ price, size, side }: { price: number; size: number; side: "bid" | "ask" }) => (
    <div
      className="tk-book-row"
      onClick={() => onPick(price)}
      title={`Click to load ${px(price, inst.pricePrecision)} into the ticket's limit price`}
    >
      <span
        className="tk-book-depth"
        style={{
          width: `${Math.min(100, (size / maxSize) * 100)}%`,
          [side === "bid" ? "left" : "right"]: 0,
          background: side === "bid" ? "color-mix(in srgb, var(--desk-success) 16%, transparent)" : "color-mix(in srgb, var(--desk-error) 16%, transparent)",
        }}
      />
      <span className="tk-num" style={{ position: "relative", color: side === "bid" ? "var(--desk-success)" : "var(--desk-error)" }}>
        {px(price, inst.pricePrecision)}
      </span>
      <span className="tk-num" style={{ position: "relative", opacity: 0.75 }}>{size.toLocaleString()}</span>
      <span />
    </div>
  );

  return (
    <Panel
      title="Market depth"
      padding={0}
      actions={
        <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
          {book.modelled ? (
            <DeskChip tone="warning" title="This venue publishes no depth. The ladder is a synthetic decay around a real mid price and exists so the ticket has a bid and an ask — it is not anyone's resting liquidity.">
              modelled
            </DeskChip>
          ) : (
            <DeskChip tone="success" title="Real resting depth, straight from the venue.">live</DeskChip>
          )}
          <Segmented
            size="sm"
            options={[
              { key: "book", label: "Book" },
              { key: "tape", label: "Tape" },
            ]}
            value={view}
            onChange={setView}
          />
        </div>
      }
    >
      {view === "book" ? (
        <div>
          <div style={{ padding: "8px 12px 4px" }}>
            <SplitMeter
              value={imbalance}
              title={`Resting size across the visible ladder is ${(Math.abs(imbalance) * 100).toFixed(0)}% skewed to the ${imbalance >= 0 ? "bid" : "ask"}.`}
            />
            <div style={{ display: "flex", justifyContent: "space-between", fontSize: "0.62rem", opacity: 0.62, marginTop: 3 }}>
              <span style={{ color: "var(--desk-success)" }}>{bidVol.toLocaleString()} bid</span>
              <span>{imbalance >= 0 ? "bid" : "ask"} heavy {(Math.abs(imbalance) * 100).toFixed(0)}%</span>
              <span style={{ color: "var(--desk-error)" }}>{askVol.toLocaleString()} ask</span>
            </div>
          </div>

          <div style={{ display: "flex", flexDirection: "column-reverse" }}>
            {asks.map((a, i) => <Row key={`a${i}`} price={a.price} size={a.size} side="ask" />)}
          </div>
          <div
            style={{
              padding: "6px 12px",
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              fontSize: "0.72rem",
              borderTop: "1px solid var(--desk-outline-variant)",
              borderBottom: "1px solid var(--desk-outline-variant)",
              margin: "3px 0",
              background: "var(--desk-surface-container)",
            }}
          >
            <span style={{ opacity: 0.65 }}>spread</span>
            <span className="tk-num" style={{ fontWeight: 700 }}>
              {px(spread, inst.pricePrecision)}
              <span style={{ opacity: 0.6, fontWeight: 400 }}> · {inst.tickSize > 0 ? (spread / inst.tickSize).toFixed(1) : "—"} ticks</span>
            </span>
          </div>
          <div>
            {bids.map((b, i) => <Row key={`b${i}`} price={b.price} size={b.size} side="bid" />)}
          </div>

          <div style={{ padding: "10px 12px 4px", borderTop: "1px solid var(--desk-outline-variant)", marginTop: 4 }}>
            <DepthChart bids={book.bids.slice(0, 60)} asks={book.asks.slice(0, 60)} height={78} precision={inst.pricePrecision} />
          </div>
        </div>
      ) : (
        <div style={{ maxHeight: 380, overflowY: "auto" }}>
          {tape?.derived ? (
            <div style={{ padding: "7px 12px", fontSize: "0.64rem", color: "var(--desk-warning)", lineHeight: 1.45 }}>
              Reconstructed from 1-minute bars — this venue publishes no public tape, so each row is one
              closed minute rather than an executed print.
            </div>
          ) : null}
          {(tape?.trades ?? []).length === 0 ? (
            <div style={{ padding: 12, opacity: 0.6, fontSize: "0.75rem" }}>No recent prints.</div>
          ) : (
            (tape?.trades ?? []).map((t, i) => (
              <div key={`${t.at}-${i}`} className="tk-tape-row">
                <span className="tk-num" style={{ color: t.side === "buy" ? "var(--desk-success)" : "var(--desk-error)" }}>
                  {px(t.price, inst.pricePrecision)}
                </span>
                <span className="tk-num" style={{ opacity: 0.75 }}>{t.size.toLocaleString()}</span>
                <span className="tk-num" style={{ opacity: 0.45, fontSize: "0.62rem" }}>
                  {new Date(t.at * 1000).toLocaleTimeString(undefined, { hour12: false })}
                </span>
              </div>
            ))
          )}
        </div>
      )}
    </Panel>
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

/** What the ladder can ask the ticket to do when a level is clicked. */
export type TicketHandle = { applyPrice: (price: number) => void };

function OrderTicket({
  inst,
  venue,
  accountLeverage,
  freeMargin,
  equity,
  busy,
  book,
  ref,
  onPlace,
}: {
  inst: Instrument;
  venue: VenueMeta;
  accountLeverage: number;
  freeMargin: number;
  equity: number;
  busy: boolean;
  book: Book | null;
  ref?: React.Ref<TicketHandle>;
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
  /** When set, size is DERIVED from this risk budget and the stop distance. */
  const [riskPct, setRiskPct] = useState<number | null>(null);

  /**
   * A price clicked in the ladder lands in whichever box is currently in play.
   *
   * Exposed imperatively rather than passed down as a `pickedPrice` prop and
   * synced with an effect. Clicking a level is an EVENT, not a piece of state
   * the ticket should mirror — syncing it meant the parent had to hold the
   * value, hand it over, and then be told to clear it, with a render in between
   * where the ticket and the parent disagreed about what was in the box.
   */
  useImperativeHandle(
    ref,
    () => ({
      applyPrice: (price: number) => {
        const v = price.toFixed(inst.pricePrecision + 1);
        setType((cur) => {
          if (cur === "stop_market" || cur === "stop_limit") {
            setStopPrice(v);
            return cur;
          }
          setLimitPrice(v);
          // A click on the ladder is a request for a resting order; leaving the
          // ticket on Market would ignore the price just chosen.
          return cur === "market" ? "limit" : cur;
        });
      },
    }),
    [inst.pricePrecision],
  );

  const lev = Math.min(leverage, inst.maxLeverage);
  const refPrice =
    type === "limit" || type === "stop_limit"
      ? Number(limitPrice) || inst.ask
      : type === "stop_market"
        ? Number(stopPrice) || inst.ask
        : side === "buy"
          ? inst.ask
          : inst.bid;

  const slNum = sl.trim() === "" ? null : Number(sl);

  /**
   * RISK-BASED SIZING.
   *
   * The box a trader actually wants: "risk 1% of the account on this idea" —
   * the size falls out of the stop distance rather than being guessed and then
   * checked. It needs a stop, so it is disabled without one rather than
   * silently sizing off something else, and it recomputes whenever either input
   * moves.
   */
  const derivedSize = useMemo(() => {
    if (riskPct === null || slNum === null || !(refPrice > 0)) return null;
    const stopDist = Math.abs(refPrice - slNum);
    if (!(stopDist > 0)) return null;
    const riskUsd = (equity * riskPct) / 100;
    const perUnit = stopDist * inst.contractSize;
    if (!(perUnit > 0)) return null;
    const raw = riskUsd / perUnit;
    const stepped = Math.floor(raw / inst.sizeStep) * inst.sizeStep;
    const rounded = Math.round(stepped * 1e8) / 1e8;
    return rounded >= inst.minSize ? rounded : null;
  }, [riskPct, slNum, refPrice, equity, inst.contractSize, inst.sizeStep, inst.minSize]);

  const sizeNum = derivedSize ?? (Number(size) || 0);

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

  // What a market order of this size would ACTUALLY average, walked through the
  // book on screen. Shown before the click rather than discovered after it.
  const estFill = useMemo(() => {
    if (type !== "market" || !book || !(sizeNum > 0)) return null;
    const levels = side === "buy" ? book.asks : book.bids;
    if (levels.length === 0) return null;
    let remaining = sizeNum;
    let cost = 0;
    for (const l of levels) {
      if (remaining <= 0) break;
      const take = Math.min(remaining, l.size);
      cost += take * l.price;
      remaining -= take;
    }
    const filled = sizeNum - remaining;
    if (filled <= 0) return null;
    const avg = cost / filled;
    const touch = levels[0]!.price;
    return { avg, slipPts: inst.tickSize > 0 ? Math.abs(avg - touch) / inst.tickSize : 0, short: remaining > 0 };
  }, [type, book, sizeNum, side, inst.tickSize]);

  const riskUsd = slNum !== null && refPrice > 0 ? Math.abs(refPrice - slNum) * sizeNum * inst.contractSize : null;
  const rewardUsd =
    tp.trim() !== "" && refPrice > 0 ? Math.abs(Number(tp) - refPrice) * sizeNum * inst.contractSize : null;
  const rr = riskUsd && rewardUsd && riskUsd > 0 ? rewardUsd / riskUsd : null;

  const numOrNull = (x: string) => (x.trim() === "" ? null : Number(x));

  const submit = useCallback(() => {
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
    });
  }, [onPlace, inst.symbol, side, type, sizeNum, limitPrice, stopPrice, lev, tp, sl, reduceOnly, postOnly]);

  /**
   * Hotkeys.
   *
   * Bound on the window but ignored while a field has focus — otherwise typing
   * "0.5" into the size box would flip the side to Sell on the "s". That guard
   * is the whole reason terminal hotkeys are usually more annoying than useful.
   */
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = e.target as HTMLElement | null;
      if (el && (el.tagName === "INPUT" || el.tagName === "TEXTAREA" || el.isContentEditable)) return;
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      const k = e.key.toLowerCase();
      if (k === "b") { setSide("buy"); e.preventDefault(); }
      else if (k === "s") { setSide("sell"); e.preventDefault(); }
      else if (k === "m") { setType("market"); e.preventDefault(); }
      else if (k === "l") { setType("limit"); e.preventDefault(); }
      else if (k === "enter" && !busy && sizeNum > 0) { submit(); e.preventDefault(); }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [busy, sizeNum, submit]);

  return (
    <Panel
      title="Order ticket"
      accent
      actions={
        <span style={{ display: "flex", gap: 4, alignItems: "center", opacity: 0.7 }} title="Hotkeys work when no field has focus">
          <Kbd>B</Kbd><Kbd>S</Kbd><Kbd>M</Kbd><Kbd>L</Kbd><Kbd>⏎</Kbd>
        </span>
      }
    >
      <div style={{ display: "flex", gap: 7, marginBottom: 12 }}>
        {(["buy", "sell"] as const).map((sd) => (
          <button
            key={sd}
            type="button"
            className="tk-side-btn"
            data-side={sd}
            data-active={side === sd}
            onClick={() => setSide(sd)}
          >
            {sd === "buy" ? "Buy / Long" : "Sell / Short"}
          </button>
        ))}
      </div>

      <div style={{ marginBottom: 12 }}>
        <Segmented
          size="sm"
          options={[
            { key: "market", label: "Market" },
            { key: "limit", label: "Limit" },
            { key: "stop_market", label: "Stop" },
            { key: "stop_limit", label: "Stop-Limit" },
          ]}
          value={type}
          onChange={setType}
        />
      </div>

      {/* Risk-based sizing. Off by default, because a trader who types a size
          means that size — the toggle is opt-in rather than a mode the ticket
          silently starts in. */}
      <div style={{ marginBottom: 10 }}>
        <div className="desk-label-md" style={{ fontWeight: 400, opacity: 0.7, fontSize: "0.66rem", marginBottom: 4 }}>
          Size by risk {riskPct !== null && derivedSize === null ? <span style={{ color: "var(--desk-warning)" }}>· needs a stop loss</span> : null}
        </div>
        <div style={{ display: "flex", gap: 4, flexWrap: "wrap" }}>
          <button type="button" className="tk-seg-btn" aria-pressed={riskPct === null} onClick={() => setRiskPct(null)} style={{ minHeight: 28, fontSize: "0.68rem", border: "1px solid var(--desk-outline)" }}>
            manual
          </button>
          {[0.5, 1, 2, 5].map((r) => (
            <button
              key={r}
              type="button"
              className="tk-seg-btn"
              aria-pressed={riskPct === r}
              onClick={() => setRiskPct(r)}
              title={`Risk ${r}% of ${usd(equity)} equity to the stop — the size follows from the stop distance`}
              style={{ minHeight: 28, fontSize: "0.68rem", border: "1px solid var(--desk-outline)" }}
            >
              {r}%
            </button>
          ))}
        </div>
      </div>

      {riskPct !== null && derivedSize !== null ? (
        <div style={{ marginBottom: 8, padding: "7px 10px", borderRadius: 8, background: "var(--desk-surface-container)", fontSize: "0.72rem" }}>
          Risking <strong className="tk-num">{usd((equity * riskPct) / 100)}</strong> → size{" "}
          <strong className="tk-num">{derivedSize}</strong> {inst.sizeUnit}
        </div>
      ) : (
        <Input label={`Size (${inst.sizeUnit})`} value={size} onChange={setSize} hint={`min ${inst.minSize}, step ${inst.sizeStep}`} />
      )}

      {(type === "limit" || type === "stop_limit") && (
        <Input label="Limit price" value={limitPrice} onChange={setLimitPrice} hint="click the ladder to fill" />
      )}
      {(type === "stop_market" || type === "stop_limit") && (
        <Input label="Stop price" value={stopPrice} onChange={setStopPrice} hint="triggers on touch" />
      )}

      <div style={{ margin: "10px 0" }}>
        <div className="desk-label-md" style={{ fontWeight: 400, opacity: 0.7, marginBottom: 4, fontSize: "0.66rem" }}>
          Leverage — max {inst.maxLeverage}x here
        </div>
        <div style={{ display: "flex", gap: 4, flexWrap: "wrap" }}>
          {venue.leverageChoices
            .filter((l) => l <= inst.maxLeverage)
            .map((l) => (
              <button
                key={l}
                type="button"
                className="tk-seg-btn"
                aria-pressed={l === lev}
                onClick={() => setLeverage(l)}
                style={{ minHeight: 27, fontSize: "0.66rem", padding: "2px 9px", border: "1px solid var(--desk-outline)" }}
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
        <div style={{ display: "flex", gap: 14, margin: "8px 0", fontSize: "0.72rem" }}>
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
          fontSize: "0.73rem",
          display: "flex",
          flexDirection: "column",
          gap: 4,
        }}
      >
        {estFill ? (
          <Est
            label="Est. fill"
            value={`${px(estFill.avg, inst.pricePrecision)}`}
            warn={estFill.short}
            title={
              estFill.short
                ? "The visible book cannot cover this size — the order would be refused rather than filled beyond published depth."
                : `Walked through the book on screen: ${estFill.slipPts.toFixed(1)} ticks of slippage against the touch.`
            }
          />
        ) : null}
        <Est label="Notional" value={usd(notional)} />
        <Est label="Margin required" value={usd(margin)} warn={!affordable} />
        <Est label={inst.commissionPerLotUsd > 0 ? "Fee + commission" : "Fee"} value={usd(fee, 4)} />
        {riskUsd !== null ? (
          <Est label="Risk to stop" value={usd(riskUsd)} title="What this position loses if the stop fills exactly." />
        ) : null}
        {rr !== null ? (
          <Est
            label="Reward : risk"
            value={`${rr.toFixed(2)} : 1`}
            warn={rr < 1}
            title="Gross of costs — the fee and any carry come out of the reward side."
          />
        ) : null}
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
        <p className="desk-label-md" style={{ fontWeight: 400, color: "var(--desk-error)", marginBottom: 8, fontSize: "0.7rem" }}>
          Free margin is {usd(freeMargin)}; this needs {usd(margin)}.
        </p>
      ) : null}

      <button type="button" className="tk-submit" data-side={side} disabled={busy || !(sizeNum > 0)} onClick={submit}>
        {busy ? "Placing…" : `${side === "buy" ? "Buy" : "Sell"} ${sizeNum || 0} ${inst.sizeUnit}`}
      </button>

      <p className="desk-label-md" style={{ fontWeight: 400, opacity: 0.6, marginTop: 8, fontSize: "0.66rem", lineHeight: 1.5 }}>
        Paper money. A market order is walked through the book shown alongside and receives the
        size-weighted price it would actually have paid.
      </p>
    </Panel>
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
  onClose: (id: string, size?: number) => void;
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
      header: "Close",
      cell: (r) => {
        const full = Number(r.size);
        // Partial closes in quarters. A trader scaling out does not want to
        // compute 37.5% of a lot size in their head, and a free-text box for it
        // is a keystroke away from closing the wrong amount.
        return (
          <div style={{ display: "flex", gap: 3 }}>
            {[25, 50, 75].map((pctOf) => {
              const part = Math.max(0, Math.round((full * pctOf) / 100 * 1e8) / 1e8);
              return (
                <button
                  key={pctOf}
                  type="button"
                  className="tk-seg-btn"
                  disabled={busy || !(part > 0)}
                  onClick={() => onClose(String(r.positionId), part)}
                  title={`Close ${pctOf}% — ${part} of ${full}`}
                  style={{ minHeight: 28, padding: "0 7px", fontSize: "0.65rem", border: "1px solid var(--desk-outline)" }}
                >
                  {pctOf}%
                </button>
              );
            })}
            <DeskButton
              variant="danger-tonal"
              disabled={busy}
              onClick={() => onClose(String(r.positionId))}
              style={{ minHeight: 28, padding: "0 10px", fontSize: "0.7rem" }}
            >
              All
            </DeskButton>
          </div>
        );
      },
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
        <p className="desk-label-md" style={{ fontWeight: 400, opacity: 0.85 }}>
          This returns the balance to its opening figure and clears the desk. <strong>Nothing is
          deleted</strong> — the current orders, positions and trades are archived as a numbered
          generation and can be restored from the History tab.
        </p>
      ) : null}
    </div>
  );
}

/**
 * Past lives of this desk.
 *
 * Exists because a reset used to delete the trade log outright, and one did —
 * an operator clearing what they thought were their own test rows took a real
 * account's history with it, and there was no way to get it back. A reset now
 * archives instead, and this is where the archived generations are read and
 * restored from.
 */
function ArchivePanel({ venue, busy, onRestored }: { venue: string; busy: boolean; onRestored: () => void }) {
  const [gens, setGens] = useState<
    { generation: number; archivedAt: number | null; trades: number; orders: number; positions: number; netPnlUsd: number }[]
  >([]);
  const [note, setNote] = useState("");
  const [working, setWorking] = useState(false);

  const load = useCallback(async () => {
    try {
      const r = await fetch(`/api/paper-trading/${venue}/archive`, { cache: "no-store" });
      const b = (await r.json()) as { generations?: typeof gens };
      setGens(b.generations ?? []);
    } catch {
      setGens([]);
    }
  }, [venue]);

  useEffect(() => {
    void load();
  }, [load]);

  const restore = useCallback(
    async (generation: number) => {
      setWorking(true);
      setNote("");
      try {
        const r = await fetch(`/api/paper-trading/${venue}/restore`, {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ generation }),
        });
        const b = (await r.json()) as { ok?: boolean; error?: string; restored?: { trades: number } };
        if (!r.ok) throw new Error(b.error ?? `HTTP ${r.status}`);
        setNote(`Restored generation ${generation} — ${b.restored?.trades ?? 0} trades back on the desk.`);
        await load();
        onRestored();
      } catch (e) {
        setNote(e instanceof Error ? e.message : "restore failed");
      } finally {
        setWorking(false);
      }
    },
    [venue, load, onRestored],
  );

  if (gens.length === 0) return null;

  return (
    <div style={{ marginBottom: "var(--desk-space-4)" }}>
      <Panel title={`Archived accounts (${gens.length})`}>
        <p className="desk-label-md" style={{ fontWeight: 400, opacity: 0.72, marginBottom: 10, lineHeight: 1.5 }}>
          Every reset archives the desk rather than deleting it. Restoring puts a generation&apos;s
          trades, orders and balance back — only onto an empty desk, because merging two accounts
          started from different balances would make every statistic over the combined book meaningless.
        </p>
        {note ? (
          <div style={{ marginBottom: 10 }}>
            <DeskBanner variant={note.startsWith("Restored") ? "success" : "warning"}>{note}</DeskBanner>
          </div>
        ) : null}
        <div style={{ display: "flex", flexDirection: "column", gap: 7 }}>
          {gens.map((g) => (
            <div
              key={g.generation}
              style={{
                display: "flex",
                alignItems: "center",
                gap: 12,
                flexWrap: "wrap",
                padding: "9px 12px",
                borderRadius: 8,
                background: "var(--desk-surface-container)",
              }}
            >
              <strong style={{ fontSize: "0.8rem" }}>Generation {g.generation}</strong>
              <span className="tk-num" style={{ fontSize: "0.75rem", opacity: 0.8 }}>
                {g.trades} trades · {g.orders} orders · {g.positions} positions
              </span>
              <span className={`tk-num ${pnlClass(g.netPnlUsd)}`} style={{ fontSize: "0.78rem", fontWeight: 700 }}>
                {usd(g.netPnlUsd)}
              </span>
              <span style={{ fontSize: "0.7rem", opacity: 0.6 }}>
                {g.archivedAt ? `archived ${fmtISTClock(g.archivedAt)} IST` : "archived"}
              </span>
              <DeskButton
                variant="outlined"
                disabled={busy || working}
                onClick={() => void restore(g.generation)}
                style={{ minHeight: 30, padding: "0 12px", fontSize: "0.72rem", marginLeft: "auto" }}
              >
                Restore
              </DeskButton>
            </div>
          ))}
        </div>
      </Panel>
    </div>
  );
}

/** "3h" / "2d 4h" — for saying how stale a chart is without a date. */
function formatAge(ms: number): string {
  const mins = Math.round(ms / 60_000);
  if (mins < 60) return `${mins}m`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  const rem = hours % 24;
  return rem > 0 ? `${days}d ${rem}h` : `${days}d`;
}
