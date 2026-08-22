"use client";

/**
 * Crypto Screener — the whole Delta perpetual universe, described honestly.
 *
 * The equity Stock Screener in the sibling TradingAI app answers seven
 * questions: what is moving, which sector, on what volume, in what chart shape,
 * what is worth trading, how did those signals perform, and is the data even
 * arriving. This page answers all seven against ~220 Delta perpetuals, and then
 * four more that only a derivatives venue can answer at all:
 *
 *   Funding         who is crowded, and what it costs them per 8 hours.
 *   Open Interest   whether a move is new positions or old ones closing —
 *                   the reading the equity module lists as permanently
 *                   unavailable.
 *   Basis           the perp against spot, and when it disagrees with funding.
 *   Microstructure  spread, book imbalance, and whether the contract's own tick
 *                   grid can hold a stop. The equity module explicitly refuses
 *                   to report order-book strength because it has no book.
 *
 * READ-ONLY. Nothing on this page can place, size or cancel an order, and the
 * API behind it has no POST. It describes the market; the desks trade it.
 *
 * WHERE THE NUMBERS COME FROM is never left implicit. Every tab carries the
 * basis of its own measurement, and columns that cannot be sourced honestly —
 * market-cap weighting, news — are reported as unavailable rather than
 * approximated.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  DeskBanner,
  DeskButton,
  DeskCard,
  DeskChip,
  DeskDataTable,
  DeskLinearProgress,
  DeskMetricTile,
  DeskSearchField,
  DeskSectionHeader,
  DeskTabs,
  type DeskColumn,
  type DeskTabItem,
} from "@/components/desk/ui";
import { fmtISTClock } from "@/lib/istTime";

// ── formatting ──────────────────────────────────────────────────────────────

/**
 * Prices on this venue span six orders of magnitude — BTC near $77,000 and
 * 1000SATSUSD near $0.0000012 — so a fixed decimal count is either useless at
 * one end or absurd at the other.
 */
function fmtPx(v: number | null | undefined): string {
  if (v === null || v === undefined || !Number.isFinite(v)) return "—";
  const a = Math.abs(v);
  if (a >= 10_000) return v.toLocaleString(undefined, { maximumFractionDigits: 1 });
  if (a >= 100) return v.toFixed(2);
  if (a >= 1) return v.toFixed(4);
  if (a >= 0.001) return v.toFixed(6);
  return v.toPrecision(4);
}

function fmtUsd(v: number | null | undefined, dp = 2): string {
  if (v === null || v === undefined || !Number.isFinite(v)) return "—";
  return `${v < 0 ? "-" : ""}$${Math.abs(v).toLocaleString(undefined, {
    minimumFractionDigits: dp,
    maximumFractionDigits: dp,
  })}`;
}

/** $1.79bn, $402m, $19.3k — turnover spans a 400,000x range on this venue. */
function fmtCompactUsd(v: number | null | undefined): string {
  if (v === null || v === undefined || !Number.isFinite(v)) return "—";
  const a = Math.abs(v);
  const sign = v < 0 ? "-" : "";
  if (a >= 1e9) return `${sign}$${(a / 1e9).toFixed(2)}bn`;
  if (a >= 1e6) return `${sign}$${(a / 1e6).toFixed(1)}m`;
  if (a >= 1e3) return `${sign}$${(a / 1e3).toFixed(1)}k`;
  return `${sign}$${a.toFixed(0)}`;
}

function fmtPct(v: number | null | undefined, dp = 2): string {
  if (v === null || v === undefined || !Number.isFinite(v)) return "—";
  return `${v >= 0 ? "+" : ""}${v.toFixed(dp)}%`;
}

function fmtNum(v: number | null | undefined, dp = 2): string {
  if (v === null || v === undefined || !Number.isFinite(v)) return "—";
  return v.toFixed(dp);
}

function pnlClass(v: number | null | undefined): string {
  if (v === null || v === undefined || !Number.isFinite(v) || v === 0) return "";
  return v > 0 ? "desk-pnl-positive" : "desk-pnl-negative";
}

function Pct({ v, dp = 2 }: { v: number | null | undefined; dp?: number }) {
  return <span className={pnlClass(v)}>{fmtPct(v, dp)}</span>;
}

/** Cell for a value that may legitimately be absent. Never renders a 0 for a gap. */
function Missing({ why }: { why: string }) {
  return (
    <span title={why} style={{ opacity: 0.45 }}>
      n/a
    </span>
  );
}

// ── types (structural, mirroring the API) ───────────────────────────────────

type Share = { pct: number | null; n: number; of: number };

type Summary = {
  universe: number;
  listed: number;
  advances: number;
  declines: number;
  unchanged: number;
  advanceDeclineRatio: number | null;
  aboveSma20: Share;
  aboveSma50: Share;
  aboveSma200: Share;
  new1yHighs: number;
  new1yLows: number;
  totalTurnoverUsd24h: number;
  totalOiUsd: number;
  btcTurnoverSharePct: number | null;
  funding: {
    medianPct8h: number | null;
    medianAnnualisedPct: number | null;
    longsPaying: number;
    shortsPaying: number;
    neutral: number;
    tilt: string;
  };
  openInterest: { totalUsd: number; byBuildup: Record<string, number>; note: string };
  btcCorrelation: { median: number | null; above85: number; of: number; note: string };
  tradableContracts: { n: number; of: number; note: string };
  benchmark: { symbol: string; available: boolean; returns: Record<string, number | null>; price: number | null };
  coverage: Record<string, { symbols: number; withHistory: number; pct: number; barsNeeded: number }>;
  buildMs: number;
  builtAt: number;
};

type Row = Record<string, unknown>;

type TabKey =
  | "momentum"
  | "sectors"
  | "volume"
  | "funding"
  | "open-interest"
  | "basis"
  | "microstructure"
  | "correlation"
  | "patterns"
  | "setups"
  | "paper"
  | "sources";

const TABS: DeskTabItem<TabKey>[] = [
  { key: "momentum", label: "Momentum" },
  { key: "sectors", label: "Sectors" },
  { key: "volume", label: "Volume" },
  { key: "funding", label: "Funding" },
  { key: "open-interest", label: "Open Interest" },
  { key: "basis", label: "Basis" },
  { key: "microstructure", label: "Microstructure" },
  { key: "correlation", label: "BTC Correlation" },
  { key: "patterns", label: "Chart Patterns" },
  { key: "setups", label: "Setups" },
  { key: "paper", label: "Paper Desk" },
  { key: "sources", label: "Sources" },
];

const HORIZONS = [
  { key: "1d", label: "24 Hours" },
  { key: "1w", label: "7 Days" },
  { key: "1m", label: "30 Days" },
  { key: "6m", label: "6 Months" },
];

async function get<T>(path: string): Promise<T> {
  const r = await fetch(`/api/crypto-screener/${path}`, { cache: "no-store" });
  const body = (await r.json().catch(() => ({}))) as Record<string, unknown>;
  if (!r.ok) {
    const detail = typeof body.error === "string" ? body.error : `HTTP ${r.status}`;
    const hint = typeof body.hint === "string" ? ` ${body.hint}` : "";
    throw new Error(detail + hint);
  }
  return body as T;
}

/** Small pill row used for every in-tab filter. */
function Pills<T extends string>({
  options,
  value,
  onChange,
}: {
  options: { key: T; label: string }[];
  value: T;
  onChange: (v: T) => void;
}) {
  return (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
      {options.map((o) => (
        <button
          key={o.key}
          type="button"
          onClick={() => onChange(o.key)}
          style={{
            padding: "6px 14px",
            minHeight: 36,
            borderRadius: "var(--desk-radius-chip)",
            border: `1px solid ${o.key === value ? "transparent" : "var(--desk-outline)"}`,
            background: o.key === value ? "var(--desk-primary-container)" : "transparent",
            color: o.key === value ? "var(--desk-on-primary-container)" : "var(--desk-on-surface-variant)",
            fontSize: "0.8125rem",
            fontWeight: o.key === value ? 600 : 500,
            cursor: "pointer",
          }}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

/** The basis line every tab carries. Small, always present, never a tooltip. */
function Basis({ children }: { children: React.ReactNode }) {
  return (
    <p
      className="desk-label-md"
      style={{
        fontWeight: 400,
        lineHeight: 1.5,
        marginBottom: "var(--desk-space-4)",
        padding: "10px 14px",
        borderRadius: "var(--desk-radius-chip)",
        background: "var(--desk-surface-container)",
        color: "var(--desk-on-surface-variant)",
      }}
    >
      {children}
    </p>
  );
}

function SymbolCell({ row }: { row: Row }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <span style={{ fontWeight: 700 }}>{String(row.symbol)}</span>
      <span className="desk-label-md" style={{ fontWeight: 400, opacity: 0.7, fontSize: "0.6875rem" }}>
        {String(row.sectorLabel ?? row.name ?? "")}
      </span>
    </div>
  );
}

const BUILDUP_TONE: Record<string, "success" | "error" | "warning" | "default"> = {
  long_buildup: "success",
  short_buildup: "error",
  short_covering: "warning",
  long_unwinding: "warning",
  flat: "default",
  unclassified: "default",
};

// ── page ────────────────────────────────────────────────────────────────────

export default function CryptoScreenerPage() {
  const [tab, setTab] = useState<TabKey>("momentum");
  const [summary, setSummary] = useState<Summary | null>(null);
  const [summaryError, setSummaryError] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [updatedAt, setUpdatedAt] = useState<string>("");

  const loadSummary = useCallback(async (fresh: boolean) => {
    try {
      const s = await get<Summary>(`summary${fresh ? "?fresh=true" : ""}`);
      setSummary(s);
      setSummaryError("");
      setUpdatedAt(fmtISTClock(Date.now()));
    } catch (e) {
      setSummaryError(e instanceof Error ? e.message : "failed to load");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadSummary(false);
  }, [loadSummary]);

  const recompute = useCallback(async () => {
    setRefreshing(true);
    // A full rebuild re-reads ~440 candle series from the venue, so the button
    // says what it is doing instead of appearing to hang.
    await loadSummary(true);
    setRefreshing(false);
    // Force every tab to re-read against the rebuilt snapshot.
    setReloadToken((t) => t + 1);
  }, [loadSummary]);

  const [reloadToken, setReloadToken] = useState(0);

  return (
    <main style={{ padding: "var(--desk-space-5)", maxWidth: 1720, margin: "0 auto" }}>
      <nav className="desk-label-md" style={{ marginBottom: 10, opacity: 0.7 }}>
        Home <span aria-hidden>›</span> Crypto Screener
      </nav>

      <DeskSectionHeader
        title="Crypto Screener"
        subtitle={
          "Momentum across 24 hours, a week, a month and six months over every Delta perpetual — " +
          "with why each contract is moving, which theme the money is rotating into, what funding " +
          "and open interest say about positioning, and which of it can actually be traded after " +
          "real taker fees, funding and the contract's own tick grid."
        }
        actions={
          <>
            <DeskButton variant="outlined" onClick={() => void loadSummary(false)} disabled={refreshing}>
              Refresh
            </DeskButton>
            <DeskButton variant="tonal" onClick={() => void recompute()} disabled={refreshing}>
              {refreshing ? "Rebuilding from venue…" : "Recompute all"}
            </DeskButton>
          </>
        }
      />

      {loading || refreshing ? <DeskLinearProgress /> : null}

      {summaryError ? (
        <DeskBanner variant="error" title="Screener unavailable">
          {summaryError}
        </DeskBanner>
      ) : null}

      {summary ? <Breadth s={summary} updatedAt={updatedAt} /> : null}

      <div style={{ marginTop: "var(--desk-space-5)" }}>
        <DeskTabs items={TABS} active={tab} onChange={setTab} variant="primary" />
      </div>

      <div style={{ marginTop: "var(--desk-space-4)" }}>
        <TabBody tab={tab} reloadToken={reloadToken} />
      </div>
    </main>
  );
}

// ── breadth header ──────────────────────────────────────────────────────────

function Breadth({ s, updatedAt }: { s: Summary; updatedAt: string }) {
  const tiltLabel =
    s.funding.tilt === "longs_crowded"
      ? "Longs crowded"
      : s.funding.tilt === "shorts_crowded"
        ? "Shorts crowded"
        : "Balanced";

  return (
    <>
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fit, minmax(168px, 1fr))",
          gap: "var(--desk-space-3)",
          marginTop: "var(--desk-space-4)",
        }}
      >
        <DeskMetricTile
          label="Advances / Declines"
          value={
            <span>
              <span className="desk-pnl-positive">{s.advances}</span>
              {" / "}
              <span className="desk-pnl-negative">{s.declines}</span>
            </span>
          }
          detail={s.advanceDeclineRatio !== null ? `${s.advanceDeclineRatio.toFixed(2)}x ratio` : "no declines"}
        />
        <DeskMetricTile
          label="Above 20 DMA"
          value={s.aboveSma20.pct !== null ? `${s.aboveSma20.pct}%` : "—"}
          detail={`${s.aboveSma20.n} of ${s.aboveSma20.of}`}
        />
        <DeskMetricTile
          label="Above 50 DMA"
          value={s.aboveSma50.pct !== null ? `${s.aboveSma50.pct}%` : "—"}
          detail={`${s.aboveSma50.n} of ${s.aboveSma50.of}`}
        />
        <DeskMetricTile
          label="Above 200 DMA"
          value={s.aboveSma200.pct !== null ? `${s.aboveSma200.pct}%` : "—"}
          detail={`${s.aboveSma200.n} of ${s.aboveSma200.of}`}
          title="Only contracts with 200 days of history can answer this, which is why the denominator is smaller."
        />
        <DeskMetricTile
          label="1Y Highs / Lows"
          value={
            <span>
              <span className="desk-pnl-positive">{s.new1yHighs}</span>
              {" / "}
              <span className="desk-pnl-negative">{s.new1yLows}</span>
            </span>
          }
          detail="within 0.5% of the extreme"
        />
        <DeskMetricTile
          label={`${s.benchmark.symbol} 24h`}
          value={<Pct v={s.benchmark.returns["1d"]} />}
          detail={s.benchmark.price !== null ? fmtUsd(s.benchmark.price, 1) : "no benchmark bars"}
        />
      </div>

      {/* The crypto-only half of the header. None of these exist on the equity
          screener, and together they say more about the state of the market
          than the moving-average breadth above them does. */}
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fit, minmax(168px, 1fr))",
          gap: "var(--desk-space-3)",
          marginTop: "var(--desk-space-3)",
        }}
      >
        <DeskMetricTile
          label="24h Turnover"
          value={fmtCompactUsd(s.totalTurnoverUsd24h)}
          detail={s.btcTurnoverSharePct !== null ? `BTC is ${s.btcTurnoverSharePct}% of it` : "—"}
        />
        <DeskMetricTile label="Open Interest" value={fmtCompactUsd(s.totalOiUsd)} detail="across all contracts" />
        <DeskMetricTile
          label="Funding Tilt"
          value={tiltLabel}
          detail={`${s.funding.longsPaying} paying long · ${s.funding.shortsPaying} paying short`}
          title={s.openInterest.note}
        />
        <DeskMetricTile
          label="Median Funding"
          value={s.funding.medianPct8h !== null ? `${s.funding.medianPct8h.toFixed(4)}%` : "—"}
          detail={
            s.funding.medianAnnualisedPct !== null ? `${s.funding.medianAnnualisedPct}%/yr per 8h stamp` : "—"
          }
        />
        <DeskMetricTile
          label="Median BTC Correlation"
          value={s.btcCorrelation.median !== null ? s.btcCorrelation.median.toFixed(2) : "—"}
          detail={`${s.btcCorrelation.above85} of ${s.btcCorrelation.of} above 0.85`}
          title={s.btcCorrelation.note}
        />
        <DeskMetricTile
          label="Can Hold A Stop"
          value={`${s.tradableContracts.n} / ${s.tradableContracts.of}`}
          detail="tick grid check"
          title={s.tradableContracts.note}
        />
      </div>

      <p className="desk-label-md" style={{ fontWeight: 400, marginTop: 10, opacity: 0.7 }}>
        Snapshot built in {(s.buildMs / 1000).toFixed(1)}s from Delta public market data
        {updatedAt ? ` · read at ${updatedAt} IST` : ""} · {s.listed} perpetuals listed, {s.universe} with
        enough history to rank. Horizons are calendar days on 00:00 UTC bar boundaries — this market has no
        session and no holiday, so a day is a day.
      </p>
    </>
  );
}

// ── tab dispatch ────────────────────────────────────────────────────────────

function TabBody({ tab, reloadToken }: { tab: TabKey; reloadToken: number }) {
  switch (tab) {
    case "momentum":
      return <MomentumTab key={reloadToken} />;
    case "sectors":
      return <SectorsTab key={reloadToken} />;
    case "volume":
      return <VolumeTab key={reloadToken} />;
    case "funding":
      return <FundingTab key={reloadToken} />;
    case "open-interest":
      return <OiTab key={reloadToken} />;
    case "basis":
      return <BasisTab key={reloadToken} />;
    case "microstructure":
      return <MicroTab key={reloadToken} />;
    case "correlation":
      return <CorrelationTab key={reloadToken} />;
    case "patterns":
      return <PatternsTab key={reloadToken} />;
    case "setups":
      return <SetupsTab key={reloadToken} />;
    case "paper":
      return <PaperTab key={reloadToken} />;
    case "sources":
      return <SourcesTab key={reloadToken} />;
  }
}

/**
 * Shared load-state wrapper so every tab fails and empties the same way.
 *
 * `loading` is DERIVED — "the answer we are holding is not for the path we are
 * asking about" — rather than stored and flipped at the top of the effect.
 * Setting it synchronously in the effect body triggers a cascading render, and
 * more importantly a stored flag can disagree with the data beside it: a filter
 * change would briefly render the PREVIOUS board's rows with loading already
 * false, which on this page means showing one horizon's returns under another
 * horizon's heading. Deriving it makes that state unrepresentable.
 */
function useBoard<T>(path: string): { data: T | null; error: string; loading: boolean } {
  const [result, setResult] = useState<{ path: string; data: T | null; error: string } | null>(null);

  useEffect(() => {
    let live = true;
    get<T>(path)
      .then((d) => {
        if (live) setResult({ path, data: d, error: "" });
      })
      .catch((e: unknown) => {
        if (live) {
          setResult({ path, data: null, error: e instanceof Error ? e.message : "failed to load" });
        }
      });
    return () => {
      live = false;
    };
  }, [path]);

  const fresh = result?.path === path ? result : null;
  return { data: fresh?.data ?? null, error: fresh?.error ?? "", loading: fresh === null };
}

function Board({
  loading,
  error,
  empty,
  children,
}: {
  loading: boolean;
  error: string;
  empty?: boolean;
  children: React.ReactNode;
}) {
  if (error) {
    return (
      <DeskBanner variant="error" title="This board could not be built">
        {error}
      </DeskBanner>
    );
  }
  if (loading) return <DeskLinearProgress />;
  if (empty) {
    return (
      <DeskBanner variant="info" title="Nothing matched">
        No contract met these filters. That is a fact about the market or the filters, not a failure —
        widen the liquidity floor or clear a filter to see more.
      </DeskBanner>
    );
  }
  return <>{children}</>;
}

// ── momentum ────────────────────────────────────────────────────────────────

function MomentumTab() {
  const [horizon, setHorizon] = useState("1d");
  const [assetClass, setAssetClass] = useState("all");
  const [liquid, setLiquid] = useState("default");
  const [q, setQ] = useState("");

  const path = useMemo(() => {
    const p = new URLSearchParams({ horizon, limit: "300" });
    if (assetClass !== "all") p.set("assetClass", assetClass);
    if (liquid === "all") p.set("minTurnover", "0");
    return `momentum?${p.toString()}`;
  }, [horizon, assetClass, liquid]);

  const { data, error, loading } = useBoard<{
    count: number;
    universe: number;
    minTurnover: number;
    rows: Row[];
    benchmark: { symbol: string; returns: Record<string, number | null> };
  }>(path);

  const rows = useMemo(() => {
    const all = data?.rows ?? [];
    if (!q.trim()) return all;
    const needle = q.trim().toUpperCase();
    return all.filter(
      (r) =>
        String(r.symbol).includes(needle) ||
        String(r.name ?? "").toUpperCase().includes(needle) ||
        String(r.sectorLabel ?? "").toUpperCase().includes(needle),
    );
  }, [data, q]);

  const cols: DeskColumn<Row>[] = [
    { id: "rank", header: "#", width: "52px", cell: (r) => String(r.rank), sortValue: (r) => Number(r.rank) },
    { id: "symbol", header: "Contract", cell: (r) => <SymbolCell row={r} />, sortValue: (r) => String(r.symbol) },
    { id: "price", header: "Price", align: "right", cell: (r) => fmtPx(r.price as number) },
    {
      id: "ret",
      header: "Return",
      align: "right",
      cell: (r) => <Pct v={r.returnPct as number} />,
      sortValue: (r) => (r.returnPct as number) ?? null,
    },
    {
      id: "score",
      header: "Score",
      align: "right",
      cell: (r) => fmtNum(r.score as number, 1),
      sortValue: (r) => (r.score as number) ?? null,
    },
    {
      id: "rs",
      header: "vs BTC",
      align: "right",
      cell: (r) =>
        r.rsBenchmark === null ? <Missing why="no benchmark bars for this horizon" /> : <Pct v={r.rsBenchmark as number} dp={1} />,
      sortValue: (r) => (r.rsBenchmark as number) ?? null,
    },
    {
      id: "volx",
      header: "Vol ×",
      align: "right",
      cell: (r) => (r.volumeX === null ? <Missing why="fewer than 21 daily bars" /> : `${fmtNum(r.volumeX as number, 1)}x`),
      sortValue: (r) => (r.volumeX as number) ?? null,
    },
    {
      id: "turnover",
      header: "24h Turnover",
      align: "right",
      cell: (r) => fmtCompactUsd(r.turnoverUsd24h as number),
      sortValue: (r) => (r.turnoverUsd24h as number) ?? null,
    },
    {
      id: "funding",
      header: "Funding",
      align: "right",
      cell: (r) => {
        const f = (r.funding as { ratePct8h: number | null } | undefined)?.ratePct8h ?? null;
        return f === null ? <Missing why="venue published no funding rate" /> : <Pct v={f} dp={4} />;
      },
      sortValue: (r) => (r.funding as { ratePct8h: number | null } | undefined)?.ratePct8h ?? null,
    },
    {
      // The window is in the header, not only the tooltip. This column is
      // always a 6-HOUR reading and it sits beside a return column that may be
      // 24h, 7d, 30d or 6m — a contract up 15% over a day while unwinding over
      // the last six hours is a coherent row, but only if the reader can see
      // that the two numbers describe different windows.
      id: "oi",
      header: "Positioning (6h)",
      cell: (r) => {
        const oi = r.oi as { buildup: string; buildupLabel: string; text: string };
        return (
          <DeskChip tone={BUILDUP_TONE[oi.buildup] ?? "default"} title={oi.text}>
            {oi.buildupLabel}
          </DeskChip>
        );
      },
      sortValue: (r) => (r.oi as { buildupLabel: string }).buildupLabel,
    },
    {
      id: "grid",
      header: "Stop",
      align: "right",
      cell: (r) => {
        const m = r.micro as { stopExpressible: boolean; stopTicks: number | null; gridNote: string };
        return (
          <DeskChip tone={m.stopExpressible ? "success" : "error"} title={m.gridNote}>
            {m.stopTicks !== null ? `${m.stopTicks} ticks` : "no grid"}
          </DeskChip>
        );
      },
      sortValue: (r) => (r.micro as { stopTicks: number | null }).stopTicks ?? null,
    },
    {
      id: "why",
      header: "Why",
      cell: (r) => {
        const chips = (r.why as { label: string; tier: number }[]) ?? [];
        if (chips.length === 0) {
          return (
            <span style={{ opacity: 0.5 }} title={String(r.whySummary)}>
              unexplained
            </span>
          );
        }
        return (
          <div style={{ display: "flex", gap: 4, flexWrap: "wrap" }} title={String(r.whySummary)}>
            {chips.map((c, i) => (
              <DeskChip key={i} tone={c.tier === 1 ? "default" : "primary"}>
                {c.label}
              </DeskChip>
            ))}
          </div>
        );
      },
      sortable: false,
    },
  ];

  return (
    <DeskCard padding="lg">
      <div style={{ display: "flex", flexWrap: "wrap", gap: 12, alignItems: "center", marginBottom: 14 }}>
        <Pills options={HORIZONS.map((h) => ({ key: h.key, label: h.label }))} value={horizon} onChange={setHorizon} />
        <Pills
          options={[
            { key: "all", label: "All" },
            { key: "crypto", label: "Crypto" },
            { key: "tradfi", label: "Tokenised TradFi" },
          ]}
          value={assetClass}
          onChange={setAssetClass}
        />
        <Pills
          options={[
            { key: "default", label: "Liquid only" },
            { key: "all", label: "Include thin" },
          ]}
          value={liquid}
          onChange={setLiquid}
        />
        <div style={{ marginLeft: "auto", minWidth: 200 }}>
          <DeskSearchField
            label="Filter"
            placeholder="symbol or theme"
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
        </div>
      </div>

      <Basis>
        Ranked within the whole perpetual universe, measured to the live traded price from a completed
        00:00 UTC close. <strong>vs BTC</strong> is a return spread in percentage points, not a ratio — a
        ratio explodes whenever the benchmark is near flat, which on a 24-hour horizon is most days.{" "}
        {data ? (
          <>
            Showing <strong>{data.count}</strong> of {data.universe} contracts
            {data.minTurnover > 0 ? (
              <>
                {" "}
                after a {fmtCompactUsd(data.minTurnover)} 24-hour turnover floor. On this venue turnover spans
                from $1.8bn down to about $4,000, so the unfiltered top of a return board is reliably a list of
                contracts nobody can trade. Switch to <em>Include thin</em> to see them anyway.
              </>
            ) : (
              " with no liquidity floor — the thin tail is included and its moves are largely untradable."
            )}
          </>
        ) : null}
      </Basis>

      <Board loading={loading} error={error} empty={rows.length === 0}>
        <DeskDataTable
          columns={cols}
          rows={rows}
          getRowKey={(r) => String(r.symbol)}
          minWidth={1500}
          defaultSort={{ id: "score", dir: "desc" }}
        />
      </Board>
    </DeskCard>
  );
}

// ── sectors ─────────────────────────────────────────────────────────────────

const ROTATION_TONE: Record<string, "success" | "warning" | "error" | "default"> = {
  leading: "success",
  improving: "primary" as never,
  weakening: "warning",
  lagging: "error",
  unknown: "default",
};

function SectorsTab() {
  const { data, error, loading } = useBoard<{
    count: number;
    sectors: Row[];
    basis: string;
    benchmark: Record<string, number | null>;
  }>("sectors");

  const cols: DeskColumn<Row>[] = [
    {
      id: "sector",
      header: "Theme",
      cell: (r) => (
        <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <span style={{ fontWeight: 700 }}>{String(r.label)}</span>
          <span className="desk-label-md" style={{ fontWeight: 400, opacity: 0.7, fontSize: "0.6875rem" }}>
            {String(r.count)} contracts{r.thin ? " · too few to be reliable" : ""}
          </span>
        </div>
      ),
      sortValue: (r) => String(r.label),
    },
    ...HORIZONS.map(
      (h): DeskColumn<Row> => ({
        id: h.key,
        header: h.label,
        align: "right",
        cell: (r) => <Pct v={(r.returns as Record<string, number>)[h.key]} />,
        sortValue: (r) => (r.returns as Record<string, number>)[h.key] ?? null,
      }),
    ),
    {
      id: "breadth",
      header: "Breadth 24h",
      align: "right",
      cell: (r) => {
        const b = (r.breadth as Record<string, number>)["1d"];
        return b === undefined ? "—" : `${b.toFixed(0)}%`;
      },
      sortValue: (r) => (r.breadth as Record<string, number>)["1d"] ?? null,
    },
    {
      id: "rotation",
      header: "Rotation",
      cell: (r) => (
        <DeskChip
          tone={ROTATION_TONE[String(r.rotation)] ?? "default"}
          title={
            r.rankChange !== null
              ? `Rank moved ${Number(r.rankChange) > 0 ? "up" : "down"} ${Math.abs(Number(r.rankChange))} places between its 6-month and 7-day standing`
              : "not enough history on both ends to compare rank"
          }
        >
          {String(r.rotation)}
        </DeskChip>
      ),
      sortValue: (r) => String(r.rotation),
    },
    {
      id: "corr",
      header: "Median BTC corr",
      align: "right",
      cell: (r) =>
        r.medianBtcCorrelation === null ? (
          <Missing why="not enough constituents with 30 days of history" />
        ) : (
          fmtNum(r.medianBtcCorrelation as number, 2)
        ),
      sortValue: (r) => (r.medianBtcCorrelation as number) ?? null,
    },
    {
      id: "fund",
      header: "Median funding",
      align: "right",
      cell: (r) =>
        r.medianFundingPct8h === null ? <Missing why="no funding published" /> : <Pct v={r.medianFundingPct8h as number} dp={4} />,
      sortValue: (r) => (r.medianFundingPct8h as number) ?? null,
    },
    {
      id: "turnover",
      header: "24h Turnover",
      align: "right",
      cell: (r) => fmtCompactUsd(r.turnoverUsd24h as number),
      sortValue: (r) => (r.turnoverUsd24h as number) ?? null,
    },
    {
      id: "leader",
      header: "24h leader / laggard",
      cell: (r) => {
        const lead = (r.leaders as Record<string, { symbol: string; returnPct: number }>)["1d"];
        const lag = (r.laggards as Record<string, { symbol: string; returnPct: number }>)["1d"];
        return (
          <span style={{ fontSize: "0.75rem" }}>
            <span className="desk-pnl-positive">{lead ? `${lead.symbol} ${fmtPct(lead.returnPct, 1)}` : "—"}</span>
            {" · "}
            <span className="desk-pnl-negative">{lag ? `${lag.symbol} ${fmtPct(lag.returnPct, 1)}` : "—"}</span>
          </span>
        );
      },
      sortable: false,
    },
  ];

  return (
    <DeskCard padding="lg">
      <Basis>
        {data?.basis ?? ""} The tags overlap — <code>smart_contracts</code> co-occurs with{" "}
        <code>layer_1</code> on 24 contracts — so each contract is placed in exactly one theme by a fixed
        priority, and 30 contracts (XRPUSD among them) carry no tag at all and stay in{" "}
        <em>Unclassified</em> rather than being filed somewhere convenient. Leader and laggard are
        per-horizon, so a 24-hour leader is never shown beside a six-month sector return.
      </Basis>
      <Board loading={loading} error={error} empty={(data?.sectors.length ?? 0) === 0}>
        <DeskDataTable
          columns={cols}
          rows={data?.sectors ?? []}
          getRowKey={(r) => String(r.sector)}
          minWidth={1400}
        />
      </Board>
    </DeskCard>
  );
}

// ── volume ──────────────────────────────────────────────────────────────────

const VOLUME_TONE: Record<string, "success" | "error" | "warning" | "default"> = {
  accumulation: "success",
  distribution: "error",
  weak_rally: "warning",
  selling_dried: "warning",
  churn: "default",
};

function VolumeTab() {
  const [window, setWindow] = useState("1d");
  const [state, setState] = useState("all");

  const path = useMemo(() => {
    const p = new URLSearchParams({ window, limit: "150" });
    if (state !== "all") p.set("state", state);
    return `volume?${p.toString()}`;
  }, [window, state]);

  const { data, error, loading } = useBoard<{
    count: number;
    rows: Row[];
    byState: Record<string, number>;
    minVolumeRatio: number;
    note: string;
    states: { key: string; label: string; text: string }[];
  }>(path);

  const cols: DeskColumn<Row>[] = [
    { id: "symbol", header: "Contract", cell: (r) => <SymbolCell row={r} />, sortValue: (r) => String(r.symbol) },
    { id: "price", header: "Price", align: "right", cell: (r) => fmtPx(r.price as number) },
    {
      id: "ratio",
      header: "Volume ×",
      align: "right",
      cell: (r) => <strong>{fmtNum(r.volumeRatio as number, 1)}x</strong>,
      sortValue: (r) => (r.volumeRatio as number) ?? null,
    },
    {
      id: "ret",
      header: "Return",
      align: "right",
      cell: (r) => <Pct v={r.returnPct as number} />,
      sortValue: (r) => (r.returnPct as number) ?? null,
    },
    {
      id: "state",
      header: "Price / Volume",
      cell: (r) => (
        <DeskChip tone={VOLUME_TONE[String(r.state)] ?? "default"} title={String(r.stateText)}>
          {String(r.stateLabel)}
        </DeskChip>
      ),
      sortValue: (r) => String(r.stateLabel),
    },
    {
      id: "oi",
      header: "Open Interest says",
      cell: (r) => {
        const conflict = r.oiConflict as string | null;
        return (
          <div style={{ display: "flex", flexDirection: "column", gap: 3 }}>
            <DeskChip tone={BUILDUP_TONE[String(r.oiBuildup)] ?? "default"}>{String(r.oiBuildupLabel)}</DeskChip>
            {conflict ? (
              <span
                className="desk-label-md"
                style={{ fontWeight: 400, fontSize: "0.6875rem", color: "var(--desk-warning)" }}
                title={conflict}
              >
                ⚠ disagrees with the label
              </span>
            ) : null}
          </div>
        );
      },
      sortValue: (r) => String(r.oiBuildupLabel),
    },
    {
      id: "target",
      header: "Next level",
      align: "right",
      cell: (r) => {
        const t = r.target as { target: number | null; upsidePct: number | null; method: string; strength: string; note: string };
        if (t.target === null) return <Missing why={t.note} />;
        return (
          <div style={{ display: "flex", flexDirection: "column", alignItems: "flex-end", gap: 2 }} title={`${t.method} — ${t.note}`}>
            <span>{fmtPx(t.target)}</span>
            <span
              className="desk-label-md"
              style={{
                fontWeight: 400,
                fontSize: "0.6875rem",
                opacity: t.strength === "weak" ? 0.55 : 0.8,
              }}
            >
              {t.upsidePct !== null ? `${fmtPct(t.upsidePct, 1)} · ` : ""}
              {t.method}
            </span>
          </div>
        );
      },
      sortValue: (r) => (r.target as { upsidePct: number | null }).upsidePct ?? null,
    },
    {
      id: "turnover",
      header: "24h Turnover",
      align: "right",
      cell: (r) => fmtCompactUsd(r.turnoverUsd24h as number),
      sortValue: (r) => (r.turnoverUsd24h as number) ?? null,
    },
  ];

  return (
    <DeskCard padding="lg">
      <div style={{ display: "flex", flexWrap: "wrap", gap: 12, marginBottom: 14 }}>
        <Pills
          options={[
            { key: "1d", label: "24 Hours" },
            { key: "1w", label: "7 Days" },
            { key: "1m", label: "30 Days" },
          ]}
          value={window}
          onChange={setWindow}
        />
        <Pills
          options={[
            { key: "all", label: "All states" },
            ...(data?.states ?? []).map((s) => ({
              key: s.key,
              label: `${s.label}${data?.byState[s.key] ? ` (${data.byState[s.key]})` : ""}`,
            })),
          ]}
          value={state}
          onChange={setState}
        />
      </div>

      <Basis>
        The question is not what traded a lot — that is just a list of the largest contracts. It is what
        traded <strong>far more than it usually does</strong>, and whether price went anywhere on it. Volume
        is measured in contracts against this contract&apos;s own 20-day average, never across contracts: one
        BTCUSD contract is 0.001 BTC and one ADAUSD contract is 1 ADA.{" "}
        <strong>Open interest is the tiebreaker</strong> — a rise on heavy volume reads as accumulation by
        definition, but if open interest fell over the same window that volume was shorts closing, which is
        very nearly the opposite. The state stays a pure price-volume fact and the disagreement gets its own
        flag.
      </Basis>

      <Board loading={loading} error={error} empty={(data?.rows.length ?? 0) === 0}>
        <DeskDataTable columns={cols} rows={data?.rows ?? []} getRowKey={(r) => String(r.symbol)} minWidth={1300} />
      </Board>
    </DeskCard>
  );
}

// ── funding ─────────────────────────────────────────────────────────────────

function FundingTab() {
  const [side, setSide] = useState("all");
  const path = useMemo(() => {
    const p = new URLSearchParams({ limit: "250" });
    if (side !== "all") p.set("side", side);
    return `funding?${p.toString()}`;
  }, [side]);

  const { data, error, loading } = useBoard<{ count: number; rows: Row[]; note: string }>(path);

  const cols: DeskColumn<Row>[] = [
    { id: "symbol", header: "Contract", cell: (r) => <SymbolCell row={r} />, sortValue: (r) => String(r.symbol) },
    {
      id: "rate",
      header: "Funding / 8h",
      align: "right",
      cell: (r) => <strong className={pnlClass(-(r.fundingPct8h as number))}>{fmtPct(r.fundingPct8h as number, 4)}</strong>,
      sortValue: (r) => (r.fundingPct8h as number) ?? null,
    },
    {
      id: "ann",
      header: "Annualised",
      align: "right",
      cell: (r) => <Pct v={r.fundingAnnualPct as number} dp={0} />,
      sortValue: (r) => (r.fundingAnnualPct as number) ?? null,
    },
    {
      id: "payer",
      header: "Who pays",
      cell: (r) => {
        const p = String(r.payer);
        return (
          <DeskChip tone={p === "longs" ? "error" : p === "shorts" ? "success" : "default"} title={String(r.fundingText)}>
            {p === "flat" ? "nobody" : p}
          </DeskChip>
        );
      },
      sortValue: (r) => String(r.payer),
    },
    {
      id: "cost1d",
      header: "Cost / day on $1k",
      align: "right",
      cell: (r) => fmtUsd(r.costPerDayPer1kUsd as number, 3),
      sortValue: (r) => (r.costPerDayPer1kUsd as number) ?? null,
    },
    {
      id: "cost5d",
      header: "Cost / 5d on $1k",
      align: "right",
      cell: (r) => (
        <span title="Fifteen funding settlements. This is the number that decides whether a swing hold clears its costs.">
          {fmtUsd(r.costPer5dPer1kUsd as number, 3)}
        </span>
      ),
      sortValue: (r) => (r.costPer5dPer1kUsd as number) ?? null,
    },
    {
      id: "ret",
      header: "24h",
      align: "right",
      cell: (r) => <Pct v={r.returnPct24h as number} />,
      sortValue: (r) => (r.returnPct24h as number) ?? null,
    },
    {
      id: "diverge",
      header: "Positioning vs price",
      cell: (r) => {
        const d = r.divergence as string | null;
        if (!d) return <span style={{ opacity: 0.4 }}>aligned</span>;
        return (
          <DeskChip tone="warning" title={d}>
            crowded side underwater
          </DeskChip>
        );
      },
      sortValue: (r) => (r.divergence ? 1 : 0),
    },
    {
      id: "pctile",
      header: "Percentile",
      align: "right",
      cell: (r) => (r.percentile === null ? "—" : `${fmtNum(r.percentile as number, 0)}`),
      sortValue: (r) => (r.percentile as number) ?? null,
    },
    {
      id: "oi",
      header: "Open Interest",
      align: "right",
      cell: (r) => fmtCompactUsd(r.oiValueUsd as number),
      sortValue: (r) => (r.oiValueUsd as number) ?? null,
    },
  ];

  return (
    <DeskCard padding="lg">
      <div style={{ marginBottom: 14 }}>
        <Pills
          options={[
            { key: "all", label: "All" },
            { key: "longs", label: "Longs paying" },
            { key: "shorts", label: "Shorts paying" },
          ]}
          value={side}
          onChange={setSide}
        />
      </div>
      <Basis>
        <strong>This measurement has no equity analogue at all.</strong> A perpetual has no expiry, so the
        venue pins it to spot by making one side pay the other every eight hours — a continuously published
        price of positioning. Rates are quoted in percent per settlement exactly as Delta publishes them;
        annualised multiplies by 1,095 and is simple, not compounded. Positive means longs pay shorts. The{" "}
        <strong>Positioning vs price</strong> column is the interesting one: funding and price pointing
        opposite ways means the crowded side is underwater and still financing the move against itself.
      </Basis>
      <Board loading={loading} error={error} empty={(data?.rows.length ?? 0) === 0}>
        <DeskDataTable
          columns={cols}
          rows={data?.rows ?? []}
          getRowKey={(r) => String(r.symbol)}
          minWidth={1400}
        />
      </Board>
    </DeskCard>
  );
}

// ── open interest ───────────────────────────────────────────────────────────

function OiTab() {
  const [buildup, setBuildup] = useState("all");
  const path = useMemo(() => {
    const p = new URLSearchParams({ limit: "250" });
    if (buildup !== "all") p.set("buildup", buildup);
    return `open-interest?${p.toString()}`;
  }, [buildup]);

  const { data, error, loading } = useBoard<{
    count: number;
    rows: Row[];
    byBuildup: Record<string, number>;
    totalOiUsd: number;
    unclassified: number;
    note: string;
  }>(path);

  const cols: DeskColumn<Row>[] = [
    { id: "symbol", header: "Contract", cell: (r) => <SymbolCell row={r} />, sortValue: (r) => String(r.symbol) },
    {
      id: "oi",
      header: "Open Interest",
      align: "right",
      cell: (r) => fmtCompactUsd(r.oiValueUsd as number),
      sortValue: (r) => (r.oiValueUsd as number) ?? null,
    },
    {
      id: "oichg",
      header: "OI Δ 6h",
      align: "right",
      cell: (r) =>
        r.oiChangePct6h === null ? <Missing why="venue reported no 6h open-interest change" /> : <Pct v={r.oiChangePct6h as number} dp={1} />,
      sortValue: (r) => (r.oiChangePct6h as number) ?? null,
    },
    {
      id: "oiusd",
      header: "OI Δ 6h ($)",
      align: "right",
      cell: (r) => fmtCompactUsd(r.oiChangeUsd6h as number),
      sortValue: (r) => (r.oiChangeUsd6h as number) ?? null,
    },
    {
      id: "px6h",
      header: "Price Δ 6h",
      align: "right",
      cell: (r) =>
        r.priceChangePct6h === null ? <Missing why="fewer than seven hourly bars — not classified" /> : <Pct v={r.priceChangePct6h as number} />,
      sortValue: (r) => (r.priceChangePct6h as number) ?? null,
    },
    {
      id: "buildup",
      header: "What that means",
      cell: (r) => (
        <DeskChip tone={BUILDUP_TONE[String(r.buildup)] ?? "default"} title={String(r.text)}>
          {String(r.buildupLabel)}
        </DeskChip>
      ),
      sortValue: (r) => String(r.buildupLabel),
    },
    {
      id: "ratio",
      header: "OI / Turnover",
      align: "right",
      cell: (r) =>
        r.oiToTurnover === null ? (
          "—"
        ) : (
          <span title="Open interest against 24h turnover. High means positions are held rather than traded around.">
            {fmtNum(r.oiToTurnover as number, 2)}x
          </span>
        ),
      sortValue: (r) => (r.oiToTurnover as number) ?? null,
    },
    {
      id: "funding",
      header: "Funding / 8h",
      align: "right",
      cell: (r) => <Pct v={r.fundingPct8h as number} dp={4} />,
      sortValue: (r) => (r.fundingPct8h as number) ?? null,
    },
    {
      id: "ret",
      header: "24h",
      align: "right",
      cell: (r) => <Pct v={r.returnPct24h as number} />,
      sortValue: (r) => (r.returnPct24h as number) ?? null,
    },
  ];

  return (
    <DeskCard padding="lg">
      <div style={{ marginBottom: 14 }}>
        <Pills
          options={[
            { key: "all", label: "All" },
            { key: "long_buildup", label: `Long buildup${data?.byBuildup.long_buildup ? ` (${data.byBuildup.long_buildup})` : ""}` },
            { key: "short_buildup", label: `Short buildup${data?.byBuildup.short_buildup ? ` (${data.byBuildup.short_buildup})` : ""}` },
            { key: "short_covering", label: `Short covering${data?.byBuildup.short_covering ? ` (${data.byBuildup.short_covering})` : ""}` },
            { key: "long_unwinding", label: `Long unwinding${data?.byBuildup.long_unwinding ? ` (${data.byBuildup.long_unwinding})` : ""}` },
          ]}
          value={buildup}
          onChange={setBuildup}
        />
      </div>
      <Basis>
        Price up on rising open interest is <strong>new longs</strong>; price up on falling open interest is{" "}
        <strong>shorts closing</strong>. Both are green candles and they mean opposite things about what
        happens next — that distinction is the reason this tab exists, and the equity screener this page
        mirrors lists it as permanently unavailable because NSE stock-level open interest is not in its data
        path. Both axes are measured over the <strong>same six hours</strong>: the venue&apos;s own 6h open
        interest change against a 6h price change from hourly bars. A contract without seven hourly bars
        reports as <em>Not classified</em> rather than being judged against its 24h move.
        {data && data.unclassified > 0 ? ` ${data.unclassified} contract(s) are unclassified right now.` : ""}
      </Basis>
      <Board loading={loading} error={error} empty={(data?.rows.length ?? 0) === 0}>
        <DeskDataTable columns={cols} rows={data?.rows ?? []} getRowKey={(r) => String(r.symbol)} minWidth={1400} />
      </Board>
    </DeskCard>
  );
}

// ── basis ───────────────────────────────────────────────────────────────────

function BasisTab() {
  const [state, setState] = useState("all");
  const path = useMemo(() => {
    const p = new URLSearchParams({ limit: "250" });
    if (state !== "all") p.set("state", state);
    return `basis?${p.toString()}`;
  }, [state]);

  const { data, error, loading } = useBoard<{ count: number; rows: Row[]; note: string }>(path);

  const cols: DeskColumn<Row>[] = [
    { id: "symbol", header: "Contract", cell: (r) => <SymbolCell row={r} />, sortValue: (r) => String(r.symbol) },
    { id: "mark", header: "Mark", align: "right", cell: (r) => fmtPx(r.markPrice as number) },
    { id: "spot", header: "Spot", align: "right", cell: (r) => fmtPx(r.spotPrice as number) },
    {
      id: "basis",
      header: "Basis (bps)",
      align: "right",
      cell: (r) => <strong className={pnlClass(r.basisBps as number)}>{fmtNum(r.basisBps as number, 1)}</strong>,
      sortValue: (r) => (r.basisBps as number) ?? null,
    },
    {
      id: "state",
      header: "State",
      cell: (r) => {
        const s = String(r.state);
        return (
          <DeskChip tone={s === "premium" ? "success" : s === "discount" ? "error" : "default"} title={String(r.text)}>
            {s.replace("_", " ")}
          </DeskChip>
        );
      },
      sortValue: (r) => String(r.state),
    },
    {
      id: "funding",
      header: "Funding / 8h",
      align: "right",
      cell: (r) => <Pct v={r.fundingPct8h as number} dp={4} />,
      sortValue: (r) => (r.fundingPct8h as number) ?? null,
    },
    {
      id: "agree",
      header: "Basis vs funding",
      cell: (r) => {
        const a = r.agreesWithFunding as boolean | null;
        if (a === null) return <Missing why="one of the two is unavailable" />;
        return a ? (
          <span style={{ opacity: 0.5 }}>agree</span>
        ) : (
          <DeskChip tone="warning" title="Basis and funding are the same force measured twice. When they point opposite ways, one of them is about to move.">
            disagree
          </DeskChip>
        );
      },
      sortValue: (r) => (r.agreesWithFunding === false ? 1 : 0),
    },
    {
      id: "ret",
      header: "24h",
      align: "right",
      cell: (r) => <Pct v={r.returnPct24h as number} />,
      sortValue: (r) => (r.returnPct24h as number) ?? null,
    },
    {
      id: "turnover",
      header: "24h Turnover",
      align: "right",
      cell: (r) => fmtCompactUsd(r.turnoverUsd24h as number),
      sortValue: (r) => (r.turnoverUsd24h as number) ?? null,
    },
  ];

  return (
    <DeskCard padding="lg">
      <div style={{ marginBottom: 14 }}>
        <Pills
          options={[
            { key: "all", label: "All" },
            { key: "premium", label: "Premium" },
            { key: "discount", label: "Discount" },
            { key: "at_par", label: "At par" },
          ]}
          value={state}
          onChange={setState}
        />
      </div>
      <Basis>
        The perpetual&apos;s mark against the venue&apos;s spot index, in basis points. A perp has no expiry,
        so nothing forces convergence except funding — which makes basis and funding the same force measured
        twice. Premium or discount is only claimed beyond 5 bps: mark and spot are computed from different
        inputs and are never exactly equal, so a zero threshold would label every contract on the venue.
      </Basis>
      <Board loading={loading} error={error} empty={(data?.rows.length ?? 0) === 0}>
        <DeskDataTable columns={cols} rows={data?.rows ?? []} getRowKey={(r) => String(r.symbol)} minWidth={1300} />
      </Board>
    </DeskCard>
  );
}

// ── microstructure ──────────────────────────────────────────────────────────

function MicroTab() {
  const [tradable, setTradable] = useState("all");
  const path = useMemo(
    () => `microstructure?limit=300${tradable === "tradable" ? "&tradableOnly=true" : ""}`,
    [tradable],
  );
  const { data, error, loading } = useBoard<{
    count: number;
    rows: Row[];
    blockedCount: number;
    note: string;
  }>(path);

  const cols: DeskColumn<Row>[] = [
    { id: "symbol", header: "Contract", cell: (r) => <SymbolCell row={r} />, sortValue: (r) => String(r.symbol) },
    {
      id: "spread",
      header: "Spread (bps)",
      align: "right",
      cell: (r) => (r.spreadBps === null ? <Missing why="no top of book quoted" /> : fmtNum(r.spreadBps as number, 2)),
      sortValue: (r) => (r.spreadBps as number) ?? null,
    },
    {
      id: "imbalance",
      header: "Book",
      cell: (r) => {
        const i = r.bookImbalance as number | null;
        const label = String(r.imbalanceLabel);
        return (
          <DeskChip
            tone={label === "bid heavy" ? "success" : label === "ask heavy" ? "error" : "default"}
            title={
              i === null
                ? "no resting size reported"
                : `Resting size is ${(Math.abs(i) * 100).toFixed(0)}% skewed to the ${i > 0 ? "bid" : "ask"}. This is a snapshot of the top of book, not depth.`
            }
          >
            {label}
          </DeskChip>
        );
      },
      sortValue: (r) => (r.bookImbalance as number) ?? null,
    },
    {
      id: "ticks",
      header: "Stop width (ticks)",
      align: "right",
      cell: (r) => (
        <DeskChip tone={(r.stopExpressible as boolean) ? "success" : "error"} title={String(r.gridNote)}>
          {r.stopTicks !== null ? fmtNum(r.stopTicks as number, 1) : "—"}
        </DeskChip>
      ),
      sortValue: (r) => (r.stopTicks as number) ?? null,
    },
    {
      id: "tick",
      header: "Tick (bps of price)",
      align: "right",
      cell: (r) => fmtNum(r.tickBps as number, 2),
      sortValue: (r) => (r.tickBps as number) ?? null,
    },
    {
      id: "breakeven",
      header: "Break-even move",
      align: "right",
      cell: (r) =>
        r.breakEvenMovePct === null ? (
          <Missing why="no spread available" />
        ) : (
          <span title="Spread plus both taker fees. Below this a trade is not a trade.">
            {fmtNum(r.breakEvenMovePct as number, 3)}%
          </span>
        ),
      sortValue: (r) => (r.breakEvenMovePct as number) ?? null,
    },
    {
      id: "costshare",
      header: "Cost as % of ATR",
      align: "right",
      cell: (r) =>
        r.costShareOfAtrPct === null ? (
          "—"
        ) : (
          <span
            className={Number(r.costShareOfAtrPct) > 20 ? "desk-pnl-negative" : ""}
            title="How much of a typical day's range is spent just getting in and out. Above 20% the contract is a toll booth."
          >
            {fmtNum(r.costShareOfAtrPct as number, 1)}%
          </span>
        ),
      sortValue: (r) => (r.costShareOfAtrPct as number) ?? null,
    },
    {
      id: "notional",
      header: "Min ticket",
      align: "right",
      cell: (r) => (
        <span title={`One contract is ${fmtNum(r.contractValue as number, 6)} of the underlying.`}>
          {fmtUsd(r.notionalPerContract as number, 4)}
        </span>
      ),
      sortValue: (r) => (r.notionalPerContract as number) ?? null,
    },
    {
      id: "lev",
      header: "Max leverage",
      align: "right",
      cell: (r) => (r.maxLeverage === null ? "—" : `${r.maxLeverage}x`),
      sortValue: (r) => (r.maxLeverage as number) ?? null,
    },
    {
      id: "score",
      header: "Tradability",
      align: "right",
      cell: (r) => {
        const blockers = (r.blockers as string[]) ?? [];
        return (
          <span
            className={Number(r.tradability) >= 70 ? "desk-pnl-positive" : Number(r.tradability) < 40 ? "desk-pnl-negative" : ""}
            title={blockers.length ? blockers.join("; ") : "no structural blockers"}
          >
            {fmtNum(r.tradability as number, 0)}
          </span>
        );
      },
      sortValue: (r) => (r.tradability as number) ?? null,
    },
  ];

  return (
    <DeskCard padding="lg">
      <div style={{ marginBottom: 14 }}>
        <Pills
          options={[
            { key: "all", label: "All contracts" },
            { key: "tradable", label: "Can hold a stop only" },
          ]}
          value={tradable}
          onChange={setTradable}
        />
      </div>
      <Basis>
        <strong>This is the tab the equity screener cannot build.</strong> That module explicitly refuses to
        report order-book strength on the grounds that it has no book and a proxy dressed up as one would be
        a lie. Delta publishes best bid, best ask and both resting sizes for every contract, so the number it
        declined to invent is simply measured here.{" "}
        <strong>Stop width is the column that matters most:</strong> it asks whether a 0.35% stop is at least
        20 ticks wide on this contract&apos;s own price grid. Under that, rounding moves the stop by more
        than 5% of the intended risk, so the order the venue receives is not the order the plan described.
        That is a property of the contract and does not improve when the market calms down — 26 contracts on
        this venue are already banned from the live desks for exactly this.
        {data ? ` ${data.blockedCount} of ${data.count} fail it right now.` : ""}
      </Basis>
      <Board loading={loading} error={error} empty={(data?.rows.length ?? 0) === 0}>
        <DeskDataTable
          columns={cols}
          rows={data?.rows ?? []}
          getRowKey={(r) => String(r.symbol)}
          minWidth={1500}
          defaultSort={{ id: "score", dir: "desc" }}
        />
      </Board>
    </DeskCard>
  );
}

// ── correlation ─────────────────────────────────────────────────────────────

function CorrelationTab() {
  const { data, error, loading } = useBoard<{
    count: number;
    rows: Row[];
    medianCorrelation: number | null;
    proxies: number;
    independent: number;
    warning: string | null;
    benchmark: string;
    note: string;
  }>("correlation?limit=300");

  const cols: DeskColumn<Row>[] = [
    { id: "symbol", header: "Contract", cell: (r) => <SymbolCell row={r} />, sortValue: (r) => String(r.symbol) },
    {
      id: "corr",
      header: "30d correlation",
      align: "right",
      cell: (r) => fmtNum(r.correlation30d as number, 3),
      sortValue: (r) => (r.correlation30d as number) ?? null,
    },
    {
      id: "beta",
      header: "Beta",
      align: "right",
      cell: (r) =>
        r.beta30d === null ? (
          <Missing why="fewer than 30 days of history" />
        ) : (
          <span title="How much this contract moves for a 1% move in BTC. Correlation says whether they move together; beta says by how much.">
            {fmtNum(r.beta30d as number, 2)}x
          </span>
        ),
      sortValue: (r) => (r.beta30d as number) ?? null,
    },
    {
      id: "verdict",
      header: "Reads as",
      cell: (r) => {
        const v = String(r.verdict);
        return (
          <DeskChip tone={v === "BTC proxy" ? "error" : v === "independent" ? "success" : "default"}>{v}</DeskChip>
        );
      },
      sortValue: (r) => String(r.verdict),
    },
    {
      id: "ret",
      header: "30d return",
      align: "right",
      cell: (r) => <Pct v={r.returnPct30d as number} dp={1} />,
      sortValue: (r) => (r.returnPct30d as number) ?? null,
    },
    {
      id: "explained",
      header: "Explained by BTC",
      align: "right",
      cell: (r) =>
        r.explainedByBtcPct === null ? <Missing why="no beta" /> : <Pct v={r.explainedByBtcPct as number} dp={1} />,
      sortValue: (r) => (r.explainedByBtcPct as number) ?? null,
    },
    {
      id: "alpha",
      header: "Alpha",
      align: "right",
      cell: (r) =>
        r.alphaPct === null ? (
          <Missing why="no beta or benchmark return" />
        ) : (
          <strong className={pnlClass(r.alphaPct as number)} title="30-day return minus beta times BTC's. What the contract did that BTC does not explain.">
            {fmtPct(r.alphaPct as number, 1)}
          </strong>
        ),
      sortValue: (r) => (r.alphaPct as number) ?? null,
    },
    {
      id: "vol",
      header: "Realised vol",
      align: "right",
      cell: (r) => (r.realisedVol30d === null ? "—" : `${fmtNum(r.realisedVol30d as number, 0)}%`),
      sortValue: (r) => (r.realisedVol30d as number) ?? null,
    },
  ];

  return (
    <DeskCard padding="lg">
      {data?.warning ? (
        <div style={{ marginBottom: 14 }}>
          <DeskBanner variant="warning" title="Most of this board is one trade">
            {data.warning}
          </DeskBanner>
        </div>
      ) : null}
      <Basis>
        Everything else on this page ranks contracts against each other. This tab asks whether that ranking
        is measuring anything: if most of the venue is highly correlated to {data?.benchmark ?? "BTC"}, then
        sector rotation and single-name momentum are largely the same trade wearing different tickers, and
        diversifying across the leaderboard would not diversify the risk. Correlation and beta are computed
        over 30 days of daily returns and <strong>only</strong> for contracts with a full 30 days of history
        — correlating a two-week listing over whatever sample it happens to share would give every row a
        different denominator.{" "}
        {data ? `${data.proxies} contracts read as BTC proxies; ${data.independent} are genuinely independent.` : ""}
      </Basis>
      <Board loading={loading} error={error} empty={(data?.rows.length ?? 0) === 0}>
        <DeskDataTable columns={cols} rows={data?.rows ?? []} getRowKey={(r) => String(r.symbol)} minWidth={1300} />
      </Board>
    </DeskCard>
  );
}

// ── patterns ────────────────────────────────────────────────────────────────

function PatternsTab() {
  const [timeframe, setTimeframe] = useState("all");
  const [state, setState] = useState("all");
  const [direction, setDirection] = useState("all");
  const [family, setFamily] = useState("all");

  const path = useMemo(() => {
    const p = new URLSearchParams({ limit: "400" });
    if (timeframe !== "all") p.set("timeframe", timeframe);
    if (state !== "all") p.set("state", state);
    if (direction !== "all") p.set("direction", direction);
    if (family !== "all") p.set("family", family);
    return `patterns?${p.toString()}`;
  }, [timeframe, state, direction, family]);

  const { data, error, loading } = useBoard<{
    count: number;
    scanned: number;
    triggered: number;
    forming: number;
    elapsedMs: number;
    rows: Row[];
    weeklyCoverage: { pct: number; note: string };
    note: string;
  }>(path);

  const cols: DeskColumn<Row>[] = [
    { id: "symbol", header: "Contract", cell: (r) => <SymbolCell row={r} />, sortValue: (r) => String(r.symbol) },
    {
      id: "pattern",
      header: "Pattern",
      cell: (r) => (
        <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <span style={{ fontWeight: 600 }}>{String(r.pattern)}</span>
          <span className="desk-label-md" style={{ fontWeight: 400, opacity: 0.7, fontSize: "0.6875rem" }}>
            {String(r.familyLabel)} · {String(r.timeframeLabel)}
          </span>
        </div>
      ),
      sortValue: (r) => String(r.pattern),
    },
    {
      id: "state",
      header: "State",
      cell: (r) => {
        const s = String(r.state);
        return (
          <DeskChip
            tone={s === "TRIGGERED" ? "success" : "warning"}
            title={
              s === "TRIGGERED"
                ? "Price has closed through the pattern's own boundary."
                : `The shape is complete and only the break is missing. It would trigger at ${fmtPx(r.triggerLevel as number)}.`
            }
          >
            {s}
          </DeskChip>
        );
      },
      sortValue: (r) => (r.state === "TRIGGERED" ? 1 : 0),
    },
    {
      id: "dir",
      header: "Direction",
      cell: (r) => (
        <DeskChip tone={r.direction === "bullish" ? "success" : "error"}>{String(r.direction)}</DeskChip>
      ),
      sortValue: (r) => String(r.direction),
    },
    { id: "live", header: "Price", align: "right", cell: (r) => fmtPx(r.livePrice as number) },
    {
      id: "trigger",
      header: "Trigger",
      align: "right",
      cell: (r) =>
        r.triggerLevel === null ? (
          <span style={{ opacity: 0.4 }} title="Already triggered — the break has happened">
            —
          </span>
        ) : (
          fmtPx(r.triggerLevel as number)
        ),
      sortValue: (r) => (r.triggerLevel as number) ?? null,
    },
    { id: "entry", header: "Entry", align: "right", cell: (r) => fmtPx(r.entry as number) },
    { id: "stop", header: "Stop", align: "right", cell: (r) => fmtPx(r.stoploss as number) },
    { id: "target", header: "Target", align: "right", cell: (r) => fmtPx(r.target as number) },
    {
      id: "rr",
      header: "R:R",
      align: "right",
      cell: (r) => (r.rewardRisk === null ? "—" : fmtNum(r.rewardRisk as number, 2)),
      sortValue: (r) => (r.rewardRisk as number) ?? null,
    },
    {
      id: "conf",
      header: "Confidence",
      align: "right",
      cell: (r) => (
        <span title={String(r.rationale)}>{fmtNum(Number(r.confidence) * 100, 0)}%</span>
      ),
      sortValue: (r) => (r.confidence as number) ?? null,
    },
  ];

  return (
    <DeskCard padding="lg">
      <div style={{ display: "flex", flexWrap: "wrap", gap: 12, marginBottom: 14 }}>
        <Pills
          options={[
            { key: "all", label: "Both timeframes" },
            { key: "1d", label: "Daily" },
            { key: "1w", label: "Weekly" },
          ]}
          value={timeframe}
          onChange={setTimeframe}
        />
        <Pills
          options={[
            { key: "all", label: "Any state" },
            { key: "TRIGGERED", label: `Triggered${data ? ` (${data.triggered})` : ""}` },
            { key: "FORMING", label: `Forming${data ? ` (${data.forming})` : ""}` },
          ]}
          value={state}
          onChange={setState}
        />
        <Pills
          options={[
            { key: "all", label: "Any direction" },
            { key: "bullish", label: "Bullish" },
            { key: "bearish", label: "Bearish" },
          ]}
          value={direction}
          onChange={setDirection}
        />
        <Pills
          options={[
            { key: "all", label: "All families" },
            { key: "chart", label: "Chart" },
            { key: "candlestick", label: "Candlestick" },
            { key: "structure", label: "Structure" },
          ]}
          value={family}
          onChange={setFamily}
        />
      </div>

      <Basis>
        <strong>TRIGGERED</strong> means price has closed through the pattern&apos;s own boundary — an
        unbroken shape is not a signal. <strong>FORMING</strong> means the shape is complete and only the
        break is missing, found by appending one synthetic bar just past the structure and re-running the{" "}
        <em>unmodified</em> detector; the trigger level shown is the exact price at which the break would
        happen. Candlestick and structure templates are never probed — there is no forming engulfing candle,
        and probing one would manufacture a signal rather than find one. Wedges and triangles require three
        touches on each boundary <em>and</em> genuine convergence, because any two points define a line.
        {data
          ? ` Scanned ${data.scanned} contracts on daily and weekly bars in ${data.elapsedMs}ms; ${data.weeklyCoverage.pct}% have enough weekly history for the longer shapes.`
          : ""}
      </Basis>

      <Board loading={loading} error={error} empty={(data?.rows.length ?? 0) === 0}>
        <DeskDataTable
          columns={cols}
          rows={data?.rows ?? []}
          getRowKey={(r, i) => `${r.symbol}-${r.template}-${r.timeframe}-${i}`}
          minWidth={1500}
        />
      </Board>
    </DeskCard>
  );
}

// ── setups ──────────────────────────────────────────────────────────────────

function SetupsTab() {
  const [kind, setKind] = useState("scalp");
  const { data, error, loading } = useBoard<{
    kind: string;
    label: string;
    universe: number;
    qualified: number;
    worthTaking: number;
    rejected: number;
    rejectionReasons: { reason: string; n: number }[];
    notionalPerTradeUsd: number;
    holdHours: number;
    note: string;
    rows: Row[];
  }>(`setups?kind=${kind}&limit=60`);

  const cols: DeskColumn<Row>[] = [
    { id: "symbol", header: "Contract", cell: (r) => <SymbolCell row={r} />, sortValue: (r) => String(r.symbol) },
    {
      id: "worth",
      header: "Verdict",
      cell: (r) => {
        const p = r.plan as { worthTaking: boolean; tradable: boolean; blockedReason: string | null };
        if (p.worthTaking) return <DeskChip tone="success">worth taking</DeskChip>;
        return (
          <DeskChip tone="warning" title={p.blockedReason ?? "net reward-to-risk below 1 after costs"}>
            {p.tradable ? "does not clear costs" : "not tradable"}
          </DeskChip>
        );
      },
      sortValue: (r) => ((r.plan as { worthTaking: boolean }).worthTaking ? 1 : 0),
    },
    {
      id: "entry",
      header: "Entry",
      align: "right",
      cell: (r) => fmtPx((r.plan as { entry: number }).entry),
    },
    {
      id: "stop",
      header: "Stop",
      align: "right",
      cell: (r) => {
        const p = r.plan as { stop: number; stopPct: number };
        return (
          <span>
            {fmtPx(p.stop)}{" "}
            <span style={{ opacity: 0.6, fontSize: "0.6875rem" }}>({fmtPct(p.stopPct, 1)})</span>
          </span>
        );
      },
    },
    {
      id: "target",
      header: "Target",
      align: "right",
      cell: (r) => {
        const p = r.plan as { target: number; targetPct: number };
        return (
          <span>
            {fmtPx(p.target)}{" "}
            <span style={{ opacity: 0.6, fontSize: "0.6875rem" }}>({fmtPct(p.targetPct, 1)})</span>
          </span>
        );
      },
    },
    {
      id: "grossrr",
      header: "Gross R:R",
      align: "right",
      cell: (r) => fmtNum((r.plan as { grossRr: number | null }).grossRr, 2),
      sortValue: (r) => (r.plan as { grossRr: number | null }).grossRr,
    },
    {
      id: "netrr",
      header: "Net R:R",
      align: "right",
      cell: (r) => {
        const n = (r.plan as { netRr: number | null }).netRr;
        return <strong className={n !== null && n >= 1 ? "desk-pnl-positive" : "desk-pnl-negative"}>{fmtNum(n, 2)}</strong>;
      },
      sortValue: (r) => (r.plan as { netRr: number | null }).netRr,
    },
    {
      id: "cost",
      header: "Fees + funding",
      align: "right",
      cell: (r) => {
        const c = (r.plan as { costWin: { totalUsd: number; fundingUsd: number; fundingIntervals: number; fundingPct: number | null } | null }).costWin;
        if (!c) return "—";
        return (
          <span
            title={`Taker fee both legs plus ${c.fundingIntervals} funding settlement(s) at ${c.fundingPct === null ? "unknown" : c.fundingPct.toFixed(4)}% — funding alone is ${fmtUsd(c.fundingUsd, 3)}.`}
          >
            {fmtUsd(c.totalUsd, 2)}
          </span>
        );
      },
      sortValue: (r) => (r.plan as { costWin: { totalUsd: number } | null }).costWin?.totalUsd ?? null,
    },
    {
      id: "size",
      header: "Size",
      align: "right",
      cell: (r) => {
        const p = r.plan as { contracts: number; notionalUsd: number; stopTicks: number | null };
        return (
          <span title={`${p.contracts} contracts. Stop is ${p.stopTicks ?? "?"} ticks wide.`}>
            {fmtUsd(p.notionalUsd, 0)}
          </span>
        );
      },
      sortValue: (r) => (r.plan as { notionalUsd: number }).notionalUsd,
    },
    {
      id: "oi",
      header: "Positioning",
      cell: (r) => (
        <DeskChip tone={BUILDUP_TONE[String(r.oiBuildup)] ?? "default"}>{String(r.oiBuildupLabel)}</DeskChip>
      ),
      sortValue: (r) => String(r.oiBuildupLabel),
    },
    {
      id: "why",
      header: "Why",
      cell: (r) => {
        const chips = (r.why as { label: string; tier: number }[]) ?? [];
        return (
          <div style={{ display: "flex", gap: 4, flexWrap: "wrap" }} title={String(r.whySummary)}>
            {chips.length === 0 ? <span style={{ opacity: 0.5 }}>unexplained</span> : null}
            {chips.map((c, i) => (
              <DeskChip key={i} tone={c.tier === 1 ? "default" : "primary"}>
                {c.label}
              </DeskChip>
            ))}
          </div>
        );
      },
      sortable: false,
    },
  ];

  return (
    <DeskCard padding="lg">
      <div style={{ marginBottom: 14 }}>
        <Pills
          options={[
            { key: "scalp", label: "Scalp (hours)" },
            { key: "swing", label: "Swing (days)" },
            { key: "breakout", label: "Breakout" },
          ]}
          value={kind}
          onChange={setKind}
        />
      </div>

      <Basis>
        <strong>Reward-to-risk here is net of Delta&apos;s real taker fee on both legs AND the funding this
        hold would actually pay</strong>, charged in whole 8-hour settlements because funding settles at the
        stamp and not pro-rata. A five-day swing crosses fifteen settlements; on a crowded contract that costs
        more than the fees do, which is why a plan can show a healthy gross R:R and a losing net one. Rows
        that do not clear their costs are <em>shown, not hidden</em> — &ldquo;no setups today&rdquo; and
        &ldquo;today&apos;s setups do not clear their costs&rdquo; are different facts. Every plan is sized to{" "}
        {data ? fmtUsd(data.notionalPerTradeUsd, 0) : "$1,000"}
        {" of notional and gated on the contract’s own tick grid."}
      </Basis>

      {data ? (
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fit, minmax(150px, 1fr))",
            gap: "var(--desk-space-3)",
            marginBottom: "var(--desk-space-4)",
          }}
        >
          <DeskMetricTile label="Qualified" value={String(data.qualified)} detail={`of ${data.universe} contracts`} compact />
          <DeskMetricTile
            label="Worth taking"
            value={String(data.worthTaking)}
            detail="net R:R ≥ 1 and tradable"
            valueClassName={data.worthTaking > 0 ? "desk-pnl-positive" : ""}
            compact
          />
          <DeskMetricTile label="Rejected" value={String(data.rejected)} detail="failed the gate" compact />
          <DeskMetricTile label="Hold" value={`${data.holdHours}h`} detail="funding charged over this" compact />
        </div>
      ) : null}

      {data && data.rejectionReasons.length > 0 ? (
        <details style={{ marginBottom: "var(--desk-space-4)" }}>
          <summary className="desk-label-md" style={{ cursor: "pointer", marginBottom: 8 }}>
            Why {data.rejected} contracts were rejected
          </summary>
          <div style={{ display: "flex", flexWrap: "wrap", gap: 6, marginTop: 8 }}>
            {data.rejectionReasons.map((x, i) => (
              <DeskChip key={i}>
                {x.reason} — {x.n}
              </DeskChip>
            ))}
          </div>
        </details>
      ) : null}

      <Board loading={loading} error={error} empty={(data?.rows.length ?? 0) === 0}>
        <DeskDataTable columns={cols} rows={data?.rows ?? []} getRowKey={(r) => String(r.symbol)} minWidth={1600} />
      </Board>
    </DeskCard>
  );
}

// ── sources ─────────────────────────────────────────────────────────────────

function SourcesTab() {
  const { data, error, loading } = useBoard<{
    feeds: { name: string; role: string; ok: boolean | null; detail: string }[];
    snapshot: { builtAt: number; buildMs: number; cacheAgeMs: number | null; coverage: Record<string, { pct: number; withHistory: number; symbols: number; barsNeeded: number }> } | null;
  }>("sources");

  return (
    <DeskCard padding="lg">
      <Basis>
        This tab exists so a silent data failure reads as a data failure. Without it, a venue outage or a
        rate limit looks identical to a quiet market — the tables just come back thin — and that ambiguity is
        how a screener starts lying without anyone changing a line of code.
      </Basis>

      <Board loading={loading} error={error}>
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          {(data?.feeds ?? []).map((f) => (
            <div
              key={f.name}
              style={{
                display: "flex",
                gap: 14,
                alignItems: "flex-start",
                padding: "12px 14px",
                borderRadius: "var(--desk-radius-chip)",
                border: "1px solid var(--desk-outline-variant)",
                background: "var(--desk-surface)",
              }}
            >
              <DeskChip tone={f.ok === true ? "success" : f.ok === false ? "error" : "default"}>
                {f.ok === true ? "OK" : f.ok === false ? "OFF" : "N/A"}
              </DeskChip>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontWeight: 700, fontSize: "0.875rem" }}>{f.name}</div>
                <div className="desk-label-md" style={{ fontWeight: 400, opacity: 0.7, marginTop: 2 }}>
                  {f.role}
                </div>
                <div className="desk-body-md" style={{ marginTop: 6, lineHeight: 1.5, fontSize: "0.8125rem" }}>
                  {f.detail}
                </div>
              </div>
            </div>
          ))}
        </div>

        {data?.snapshot ? (
          <div style={{ marginTop: "var(--desk-space-5)" }}>
            <DeskSectionHeader
              title="History coverage"
              subtitle="How much of the universe can answer each horizon. A contract without the bars reports null, never a partial-window return dressed as a full one."
            />
            <div
              style={{
                display: "grid",
                gridTemplateColumns: "repeat(auto-fit, minmax(160px, 1fr))",
                gap: "var(--desk-space-3)",
              }}
            >
              {HORIZONS.map((h) => {
                const c = data.snapshot!.coverage[h.key];
                if (!c) return null;
                return (
                  <DeskMetricTile
                    key={h.key}
                    label={h.label}
                    value={`${c.pct}%`}
                    detail={`${c.withHistory} of ${c.symbols} · needs ${c.barsNeeded} bars`}
                    compact
                  />
                );
              })}
            </div>
          </div>
        ) : null}
      </Board>
    </DeskCard>
  );
}

// ── paper desk ──────────────────────────────────────────────────────────────

type PaperView = "books" | "families" | "open" | "closed";

const PAPER_VIEWS: { key: PaperView; label: string }[] = [
  { key: "books", label: "Books (per symbol)" },
  { key: "families", label: "Signal families" },
  { key: "open", label: "Open positions" },
  { key: "closed", label: "Closed trades" },
];

const FAMILY_TONE: Record<string, "success" | "error" | "warning" | "primary" | "default"> = {
  scalp: "primary",
  swing: "success",
  breakout: "warning",
  pattern: "default",
  momentum: "primary",
};

const EXIT_TONE: Record<string, "success" | "error" | "warning" | "default"> = {
  TARGET: "success",
  STOP: "error",
  TIME: "warning",
  LIQUIDATION: "error",
  MANUAL: "default",
};

type PaperSummary = {
  configured: boolean;
  reason?: string;
  books: Row[];
  families: Row[];
  totals: Record<string, number | null> | null;
  perSymbolEquityUsd?: number;
  maxOpenTotal?: number;
  maxOpenPerSymbol?: number;
  integrity?: { sameBarAmbiguities: number; assumedStopFirst: number; gappedFills: number; note: string };
  tick?: {
    lastTickAt: number | null;
    lastTickMs: number | null;
    ticks: number;
    lastOpened: number;
    lastClosed: number;
    lastError: string | null;
    note: string;
    thisRequest: { ran: boolean; opened: number; closed: number; managed: number; skippedReason?: string; refusals?: { reason: string; n: number }[] } | null;
  };
};

function PaperTab() {
  const [view, setView] = useState<PaperView>("books");
  const [nonce, setNonce] = useState(0);
  const [running, setRunning] = useState(false);
  const [runNote, setRunNote] = useState("");

  const { data, error, loading } = useBoard<PaperSummary>(`paper/summary?n=${nonce}`);

  const runCycle = useCallback(async () => {
    setRunning(true);
    setRunNote("");
    try {
      const r = await fetch("/api/crypto-screener/paper/run", { method: "POST", cache: "no-store" });
      const body = (await r.json()) as { ok?: boolean; error?: string; cycle?: { opened: number; closed: number; managed: number } };
      setRunNote(
        r.ok && body.cycle
          ? `Cycle done — managed ${body.cycle.managed}, closed ${body.cycle.closed}, opened ${body.cycle.opened}.`
          : body.error ?? "cycle failed",
      );
      setNonce((n) => n + 1);
    } catch (e) {
      setRunNote(e instanceof Error ? e.message : "cycle failed");
    } finally {
      setRunning(false);
    }
  }, []);

  if (!loading && data && data.configured === false) {
    return (
      <DeskCard padding="lg">
        <DeskBanner variant="warning" title="The paper desk is not configured on this deployment">
          {data.reason}
        </DeskBanner>
      </DeskCard>
    );
  }

  const t = data?.totals ?? null;
  const tick = data?.tick;
  const staleMin = tick?.lastTickAt ? Math.round((Date.now() - tick.lastTickAt) / 60000) : null;

  return (
    <DeskCard padding="lg">
      <div style={{ display: "flex", flexWrap: "wrap", gap: 12, alignItems: "center", marginBottom: 14 }}>
        <Pills options={PAPER_VIEWS} value={view} onChange={setView} />
        <div style={{ marginLeft: "auto", display: "flex", gap: 8, alignItems: "center" }}>
          {runNote ? (
            <span className="desk-label-md" style={{ fontWeight: 400, opacity: 0.75 }}>
              {runNote}
            </span>
          ) : null}
          <DeskButton variant="tonal" onClick={() => void runCycle()} disabled={running}>
            {running ? "Running cycle…" : "Run a cycle now"}
          </DeskButton>
        </div>
      </div>

      <Basis>
        Every signal on this page&apos;s other tabs, taken automatically at the entry, stop and target
        those tabs published, with <strong>${(data?.perSymbolEquityUsd ?? 10000).toLocaleString()} of
        its own capital for every symbol</strong>. Nothing here can reach a broker: the desk holds no
        keys and has no order-routing path. Size comes from risk, not from a fixed ticket — each
        position risks 2% of its own book, so an R multiple means the same thing on a contract with a
        0.5% stop and one with a 30% stop. Costs are charged in full: the taker fee on both legs, half
        the quoted spread as slippage on both legs, and every 8-hour funding settlement the position
        was open across — signed by direction, so a short is CREDITED when longs are paying.
      </Basis>

      {tick ? (
        <div style={{ marginBottom: "var(--desk-space-4)" }}>
          <DeskBanner variant={tick.lastError ? "error" : "info"} title="How this desk keeps time">
            {tick.lastError ? `Last cycle errored: ${tick.lastError}. ` : ""}
            {tick.note}
            {staleMin !== null ? ` Last cycle ran ${staleMin === 0 ? "moments" : `${staleMin} min`} ago (${tick.ticks} total).` : " No cycle has run yet."}
          </DeskBanner>
        </div>
      ) : null}

      {t ? (
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fit, minmax(158px, 1fr))",
            gap: "var(--desk-space-3)",
            marginBottom: "var(--desk-space-4)",
          }}
        >
          <DeskMetricTile
            label="Books"
            value={String(t.booksOpened ?? 0)}
            detail={`${fmtCompactUsd(t.startingEquityUsd)} allocated`}
            compact
          />
          <DeskMetricTile
            label="Mark to market"
            value={fmtCompactUsd(t.markToMarketUsd)}
            detail="equity + open P&L"
            valueClassName={pnlClass((t.markToMarketUsd ?? 0) - (t.startingEquityUsd ?? 0))}
            compact
          />
          <DeskMetricTile
            label="Realised"
            value={fmtUsd(t.realisedPnlUsd, 2)}
            detail={t.roiPct !== null && t.roiPct !== undefined ? `${fmtPct(t.roiPct)} ROI` : "—"}
            valueClassName={pnlClass(t.realisedPnlUsd)}
            compact
          />
          <DeskMetricTile
            label="Unrealised"
            value={fmtUsd(t.unrealisedPnlUsd, 2)}
            detail="net of exit costs"
            valueClassName={pnlClass(t.unrealisedPnlUsd)}
            compact
            title="Marked at what it would cost to close right now — the exit taker fee, the spread, and funding already accrued. An unrealised figure quoted gross is the number that makes a desk look profitable until it closes something."
          />
          <DeskMetricTile
            label="Open"
            value={`${t.openPositions ?? 0} / ${data?.maxOpenTotal ?? 60}`}
            detail={`max ${data?.maxOpenPerSymbol ?? 2} per symbol`}
            compact
          />
          <DeskMetricTile label="Closed trades" value={String(t.trades ?? 0)} detail={t.winRate !== null && t.winRate !== undefined ? `${t.winRate}% won` : "none yet"} compact />
          <DeskMetricTile
            label="Profit factor"
            value={t.profitFactor !== null && t.profitFactor !== undefined ? fmtNum(t.profitFactor, 2) : "—"}
            detail={t.expectancyUsd !== null && t.expectancyUsd !== undefined ? `${fmtUsd(t.expectancyUsd, 2)} / trade` : "—"}
            compact
            title="Gross wins over gross losses. Profit factor and expectancy are the two that rank a strategy; win rate alone ranks a martingale first."
          />
          <DeskMetricTile
            label="Costs paid"
            value={fmtUsd(t.costsUsd, 2)}
            detail={`incl. ${fmtUsd(t.fundingUsd, 2)} funding`}
            compact
          />
        </div>
      ) : null}

      {data?.integrity && (data.integrity.sameBarAmbiguities > 0 || data.integrity.gappedFills > 0) ? (
        <div style={{ marginBottom: "var(--desk-space-4)" }}>
          <DeskBanner variant="warning" title="Fill integrity">
            {data.integrity.sameBarAmbiguities} trade(s) had their stop and target inside one replay bar
            {data.integrity.assumedStopFirst > 0
              ? `, and ${data.integrity.assumedStopFirst} of those could not be resolved even at 1-minute resolution — those assumed the STOP came first`
              : ", all resolved at 1-minute resolution"}
            . {data.integrity.gappedFills} exit(s) filled at a bar&apos;s open because price gapped past
            the level. {data.integrity.note}
          </DeskBanner>
        </div>
      ) : null}

      <Board loading={loading} error={error}>
        {view === "books" ? <PaperBooks rows={data?.books ?? []} /> : null}
        {view === "families" ? <PaperFamilies rows={data?.families ?? []} /> : null}
        {view === "open" ? <PaperPositions status="OPEN" nonce={nonce} /> : null}
        {view === "closed" ? <PaperPositions status="CLOSED" nonce={nonce} /> : null}
      </Board>
    </DeskCard>
  );
}

function PaperBooks({ rows }: { rows: Row[] }) {
  const cols: DeskColumn<Row>[] = [
    { id: "symbol", header: "Book", cell: (r) => <SymbolCell row={r} />, sortValue: (r) => String(r.symbol) },
    {
      id: "mtm",
      header: "Mark to market",
      align: "right",
      cell: (r) => <strong className={pnlClass((r.markToMarketUsd as number) - (r.startingEquityUsd as number))}>{fmtUsd(r.markToMarketUsd as number, 2)}</strong>,
      sortValue: (r) => (r.markToMarketUsd as number) ?? null,
    },
    {
      id: "realised",
      header: "Realised",
      align: "right",
      cell: (r) => <span className={pnlClass(r.realisedPnlUsd as number)}>{fmtUsd(r.realisedPnlUsd as number, 2)}</span>,
      sortValue: (r) => (r.realisedPnlUsd as number) ?? null,
    },
    {
      id: "unrealised",
      header: "Unrealised",
      align: "right",
      cell: (r) => <span className={pnlClass(r.unrealisedPnlUsd as number)}>{fmtUsd(r.unrealisedPnlUsd as number, 2)}</span>,
      sortValue: (r) => (r.unrealisedPnlUsd as number) ?? null,
    },
    {
      id: "roi",
      header: "ROI",
      align: "right",
      cell: (r) => <Pct v={r.roiPct as number} />,
      sortValue: (r) => (r.roiPct as number) ?? null,
    },
    {
      id: "open",
      header: "Open",
      align: "right",
      cell: (r) => (
        <span title={`${fmtUsd(r.marginPostedUsd as number, 0)} of this book is posted as margin`}>
          {String(r.openPositions)}
        </span>
      ),
      sortValue: (r) => (r.openPositions as number) ?? null,
    },
    {
      id: "trades",
      header: "Trades",
      align: "right",
      cell: (r) => (r.trades ? `${r.trades} (${r.wins}W/${r.losses}L)` : "—"),
      sortValue: (r) => (r.trades as number) ?? null,
    },
    {
      id: "wr",
      header: "Win rate",
      align: "right",
      cell: (r) => (r.winRate === null ? <Missing why="no closed trades in this book yet" /> : `${fmtNum(r.winRate as number, 0)}%`),
      sortValue: (r) => (r.winRate as number) ?? null,
    },
    {
      id: "pf",
      header: "Profit factor",
      align: "right",
      cell: (r) => (r.profitFactor === null ? <Missing why="no losing trade yet, so the ratio has no denominator" /> : fmtNum(r.profitFactor as number, 2)),
      sortValue: (r) => (r.profitFactor as number) ?? null,
    },
    {
      id: "avgr",
      header: "Avg R",
      align: "right",
      cell: (r) => (r.avgR === null ? "—" : <span className={pnlClass(r.avgR as number)}>{fmtNum(r.avgR as number, 2)}</span>),
      sortValue: (r) => (r.avgR as number) ?? null,
    },
  ];
  return (
    <>
      <p className="desk-label-md" style={{ fontWeight: 400, marginBottom: 10, opacity: 0.75 }}>
        One book per contract, each spending only from its own $10,000. Per-symbol rather than one
        shared pool because the question is which CONTRACT suits these signals — a shared balance
        would let BTC&apos;s results pay for a loss on DOGE and hide it. A book is created the first
        time a symbol produces a signal the desk will act on, so symbols that never signal never
        appear here.
      </p>
      <DeskDataTable
        columns={cols}
        rows={rows}
        getRowKey={(r) => String(r.symbol)}
        minWidth={1250}
        empty={
          <DeskBanner variant="info" title="No books yet">
            No symbol has produced a signal that cleared the desk&apos;s gates. Books are created on
            first trade, so an empty list means nothing qualified — not that the desk is broken.
          </DeskBanner>
        }
      />
    </>
  );
}

function PaperFamilies({ rows }: { rows: Row[] }) {
  const cols: DeskColumn<Row>[] = [
    {
      id: "family",
      header: "Signal family",
      cell: (r) => (
        <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <DeskChip tone={FAMILY_TONE[String(r.family)] ?? "default"}>{String(r.label)}</DeskChip>
          <span className="desk-label-md" style={{ fontWeight: 400, opacity: 0.65, fontSize: "0.6875rem" }}>
            max hold {String(r.maxHoldHours)}h
          </span>
        </div>
      ),
      sortValue: (r) => String(r.label),
    },
    { id: "open", header: "Open", align: "right", cell: (r) => String(r.openPositions), sortValue: (r) => (r.openPositions as number) ?? null },
    {
      id: "unreal",
      header: "Unrealised",
      align: "right",
      cell: (r) => <span className={pnlClass(r.unrealisedPnlUsd as number)}>{fmtUsd(r.unrealisedPnlUsd as number, 2)}</span>,
      sortValue: (r) => (r.unrealisedPnlUsd as number) ?? null,
    },
    { id: "trades", header: "Trades", align: "right", cell: (r) => (r.trades ? `${r.trades} (${r.wins}W/${r.losses}L)` : "—"), sortValue: (r) => (r.trades as number) ?? null },
    {
      id: "net",
      header: "Realised",
      align: "right",
      cell: (r) => <strong className={pnlClass(r.netPnlUsd as number)}>{fmtUsd(r.netPnlUsd as number, 2)}</strong>,
      sortValue: (r) => (r.netPnlUsd as number) ?? null,
    },
    { id: "wr", header: "Win rate", align: "right", cell: (r) => (r.winRate === null ? <Missing why="no closed trades in this family yet" /> : `${fmtNum(r.winRate as number, 0)}%`), sortValue: (r) => (r.winRate as number) ?? null },
    { id: "pf", header: "Profit factor", align: "right", cell: (r) => (r.profitFactor === null ? <Missing why="no losing trade yet" /> : fmtNum(r.profitFactor as number, 2)), sortValue: (r) => (r.profitFactor as number) ?? null },
    { id: "exp", header: "Expectancy", align: "right", cell: (r) => (r.expectancyUsd === null ? "—" : <span className={pnlClass(r.expectancyUsd as number)}>{fmtUsd(r.expectancyUsd as number, 2)}</span>), sortValue: (r) => (r.expectancyUsd as number) ?? null },
    { id: "avgr", header: "Avg R", align: "right", cell: (r) => (r.avgR === null ? "—" : <span className={pnlClass(r.avgR as number)}>{fmtNum(r.avgR as number, 2)}</span>), sortValue: (r) => (r.avgR as number) ?? null },
    { id: "hold", header: "Avg hold", align: "right", cell: (r) => (r.avgHoldHours === null ? "—" : `${fmtNum(r.avgHoldHours as number, 1)}h`), sortValue: (r) => (r.avgHoldHours as number) ?? null },
    { id: "funding", header: "Funding", align: "right", cell: (r) => <span className={pnlClass(-(r.fundingUsd as number))}>{fmtUsd(r.fundingUsd as number, 2)}</span>, sortValue: (r) => (r.fundingUsd as number) ?? null },
  ];
  return (
    <>
      <p className="desk-label-md" style={{ fontWeight: 400, marginBottom: 10, opacity: 0.75 }}>
        The same trades, attributed to the KIND of signal that produced them rather than to the
        contract. Capital is per symbol, so these five share it — which is why the desk fills its
        openings round-robin across families rather than best-first. A flat sort by reward-to-risk
        handed twelve of twelve slots to Swing on the first run, and a leaderboard comparing twelve
        trades against one would be measuring allocation rather than edge.
      </p>
      <DeskDataTable columns={cols} rows={rows} getRowKey={(r) => String(r.family)} minWidth={1350} />
    </>
  );
}

function PaperPositions({ status, nonce }: { status: "OPEN" | "CLOSED"; nonce: number }) {
  const [family, setFamily] = useState("all");
  const path = useMemo(() => {
    const p = new URLSearchParams({ status, limit: "300", n: String(nonce) });
    if (family !== "all") p.set("family", family);
    return `paper/positions?${p.toString()}`;
  }, [status, family, nonce]);

  const { data, error, loading } = useBoard<{ count: number; rows: Row[] }>(path);

  const openCols: DeskColumn<Row>[] = [
    { id: "symbol", header: "Contract", cell: (r) => <SymbolCell row={r} />, sortValue: (r) => String(r.symbol) },
    {
      id: "family",
      header: "Signal",
      cell: (r) => (
        <div style={{ display: "flex", flexDirection: "column", gap: 3 }}>
          <DeskChip tone={FAMILY_TONE[String(r.family)] ?? "default"} title={String(r.signal_reason ?? "")}>
            {String(r.familyLabel)}
          </DeskChip>
          <span className="desk-label-md" style={{ fontWeight: 400, fontSize: "0.6875rem", opacity: 0.7 }}>
            {String(r.side).toUpperCase()}
            {r.pattern ? ` · ${String(r.pattern)}` : ""}
          </span>
        </div>
      ),
      sortValue: (r) => String(r.familyLabel),
    },
    { id: "entry", header: "Entry", align: "right", cell: (r) => fmtPx(r.entry as number) },
    { id: "mark", header: "Mark", align: "right", cell: (r) => (r.mark === null ? <Missing why="no live price this cycle" /> : fmtPx(r.mark as number)) },
    {
      id: "stop",
      header: "Stop",
      align: "right",
      cell: (r) => (
        <span title={`Venue liquidation sits at ${fmtPx(r.liquidation_price as number)} — further away than the stop, which is checked before every fill.`}>
          {fmtPx(r.stop as number)}{" "}
          <span style={{ opacity: 0.6, fontSize: "0.6875rem" }}>({fmtPct(r.toStopPct as number, 1)})</span>
        </span>
      ),
    },
    {
      id: "target",
      header: "Target",
      align: "right",
      cell: (r) => (
        <span>
          {fmtPx(r.target as number)}{" "}
          <span style={{ opacity: 0.6, fontSize: "0.6875rem" }}>({fmtPct(r.toTargetPct as number, 1)})</span>
        </span>
      ),
    },
    {
      id: "size",
      header: "Size",
      align: "right",
      cell: (r) => (
        <span title={`${r.contracts} contracts at ${r.leverage}x. Margin posted ${fmtUsd(r.margin_usd as number, 2)}; risk to the stop ${fmtUsd(r.risk_usd as number, 2)}.`}>
          {fmtCompactUsd(r.notional_usd as number)} <span style={{ opacity: 0.6, fontSize: "0.6875rem" }}>{String(r.leverage)}x</span>
        </span>
      ),
      sortValue: (r) => (r.notional_usd as number) ?? null,
    },
    {
      id: "unreal",
      header: "Unrealised",
      align: "right",
      cell: (r) =>
        r.unrealisedUsd === null ? (
          <Missing why="no live price" />
        ) : (
          <strong className={pnlClass(r.unrealisedUsd as number)}>
            {fmtUsd(r.unrealisedUsd as number, 2)}{" "}
            <span style={{ opacity: 0.7, fontSize: "0.6875rem" }}>({fmtPct(r.unrealisedPct as number, 1)})</span>
          </strong>
        ),
      sortValue: (r) => (r.unrealisedUsd as number) ?? null,
    },
    {
      id: "funding",
      header: "Funding",
      align: "right",
      cell: (r) => <span className={pnlClass(-(r.funding_usd as number))}>{fmtUsd(r.funding_usd as number, 3)}</span>,
      sortValue: (r) => (r.funding_usd as number) ?? null,
    },
    {
      id: "age",
      header: "Age",
      align: "right",
      cell: (r) => (
        <span title={`Closed on the clock at ${r.max_hold_hours}h if neither level is reached.`}>
          {fmtNum(r.ageHours as number, 1)}h / {String(r.max_hold_hours)}h
        </span>
      ),
      sortValue: (r) => (r.ageHours as number) ?? null,
    },
  ];

  const closedCols: DeskColumn<Row>[] = [
    { id: "symbol", header: "Contract", cell: (r) => <SymbolCell row={r} />, sortValue: (r) => String(r.symbol) },
    {
      id: "family",
      header: "Signal",
      cell: (r) => (
        <DeskChip tone={FAMILY_TONE[String(r.family)] ?? "default"} title={String(r.signal_reason ?? "")}>
          {String(r.familyLabel)} · {String(r.side).toUpperCase()}
        </DeskChip>
      ),
      sortValue: (r) => String(r.familyLabel),
    },
    { id: "entry", header: "Entry", align: "right", cell: (r) => fmtPx(r.entry as number) },
    {
      id: "exit",
      header: "Exit",
      align: "right",
      cell: (r) => {
        const gapped = r.exit !== r.exit_level && r.exit_reason !== "TIME";
        return (
          <span
            title={
              gapped
                ? `Price gapped past the ${fmtPx(r.exit_level as number)} level, so the fill is the bar's open — which is what a stop order actually does.`
                : `Level ${fmtPx(r.exit_level as number)}, filled at ${fmtPx(r.exit as number)} after crossing the spread.`
            }
          >
            {fmtPx(r.exit as number)}
            {gapped ? <span style={{ color: "var(--desk-warning)", fontSize: "0.6875rem" }}> gap</span> : null}
          </span>
        );
      },
    },
    {
      id: "reason",
      header: "Why it closed",
      cell: (r) => (
        <div style={{ display: "flex", flexDirection: "column", gap: 3 }}>
          <DeskChip tone={EXIT_TONE[String(r.exit_reason)] ?? "default"}>{String(r.exit_reason)}</DeskChip>
          {r.same_bar_ambiguity ? (
            <span
              className="desk-label-md"
              style={{ fontWeight: 400, fontSize: "0.6875rem", color: "var(--desk-warning)" }}
              title={
                r.ambiguity_resolved_by === "1m-bars"
                  ? "Stop and target both fell inside one replay bar; 1-minute bars settled which came first."
                  : "Stop and target both fell inside one replay bar and even 1-minute bars could not settle it, so the STOP was assumed — the unfavourable branch."
              }
            >
              {r.ambiguity_resolved_by === "1m-bars" ? "resolved at 1m" : "assumed stop first"}
            </span>
          ) : null}
        </div>
      ),
      sortValue: (r) => String(r.exit_reason),
    },
    {
      id: "net",
      header: "Net P&L",
      align: "right",
      cell: (r) => <strong className={pnlClass(r.net_pnl_usd as number)}>{fmtUsd(r.net_pnl_usd as number, 2)}</strong>,
      sortValue: (r) => (r.net_pnl_usd as number) ?? null,
    },
    {
      id: "r",
      header: "R",
      align: "right",
      cell: (r) => <span className={pnlClass(r.r_multiple as number)} title="Net P&L over the risk the position was sized to. Comparable across contracts because every position risks the same 2% of its book.">{fmtNum(r.r_multiple as number, 2)}</span>,
      sortValue: (r) => (r.r_multiple as number) ?? null,
    },
    {
      id: "costs",
      header: "Costs",
      align: "right",
      cell: (r) => (
        <span title={`Taker fees ${fmtUsd((r.entry_fee_usd as number) + (r.exit_fee_usd as number), 3)}, funding ${fmtUsd(r.funding_usd as number, 3)}, slippage ${fmtUsd((r.entry_slippage_usd as number) + (r.exit_slippage_usd as number), 3)} (already inside the fills).`}>
          {fmtUsd(r.costs_usd as number, 2)}
        </span>
      ),
      sortValue: (r) => (r.costs_usd as number) ?? null,
    },
    { id: "hold", header: "Hold", align: "right", cell: (r) => `${fmtNum(r.hold_hours as number, 1)}h`, sortValue: (r) => (r.hold_hours as number) ?? null },
    {
      id: "closed",
      header: "Closed",
      align: "right",
      cell: (r) => fmtISTClock((r.closed_at as number) * 1000),
      sortValue: (r) => (r.closed_at as number) ?? null,
    },
  ];

  return (
    <>
      <div style={{ marginBottom: 12 }}>
        <Pills
          options={[
            { key: "all", label: "All families" },
            { key: "scalp", label: "Scalp" },
            { key: "swing", label: "Swing" },
            { key: "breakout", label: "Breakout" },
            { key: "pattern", label: "Chart Pattern" },
            { key: "momentum", label: "Momentum" },
          ]}
          value={family}
          onChange={setFamily}
        />
      </div>
      <Board
        loading={loading}
        error={error}
        empty={(data?.rows.length ?? 0) === 0}
      >
        <DeskDataTable
          columns={status === "OPEN" ? openCols : closedCols}
          rows={data?.rows ?? []}
          getRowKey={(r, i) => `${r.position_id ?? r.symbol}-${i}`}
          minWidth={1450}
          defaultSort={status === "OPEN" ? { id: "unreal", dir: "desc" } : { id: "closed", dir: "desc" }}
        />
      </Board>
    </>
  );
}
