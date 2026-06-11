"use client";

import { memo, useCallback, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import {
  usePaperDesk,
  fetchTradesPage,
  fetchOrders,
  fetchEquity,
  fetchStrategyHealth,
  type TradesPage,
} from "@/hooks/usePaperDesk";
import useLiveBTCPrice from "@/hooks/useLiveBTCPrice";
import { AppShell } from "@/components/terminal";
import { isPaperDeskTabKey, paperDeskHref, type PaperDeskTabKey } from "@/lib/navRoutes";
import type {
  PaperTradeDoc,
  PaperPositionDoc,
  PaperOrderDoc,
  EquityCurveDoc,
  DailyPnLDoc,
  StrategyScoreDoc,
  StrategyHealthDoc,
} from "@/lib/paperDeskClient";
import { unrealizedPnlForOpenPosition } from "@/lib/paperDeskPositionMath";
import { DeskLinearProgress } from "@/components/desk/ui/DeskLinearProgress";
import { DeskCard } from "@/components/desk/ui/DeskCard";
import { DeskMetricTile } from "@/components/desk/ui/DeskMetricTile";
import { DeskSectionHeader } from "@/components/desk/ui/DeskSectionHeader";
import { DeskDataTable, type DeskColumn } from "@/components/desk/ui/DeskDataTable";
import { DeskEmptyState } from "@/components/desk/ui/DeskEmptyState";
import { DeskChip } from "@/components/desk/ui/DeskChip";
import { DeskButton } from "@/components/desk/ui/DeskButton";
import { DeskTabs, type DeskTabItem } from "@/components/desk/ui/DeskTabs";
import { StatusBadge, type DeskEngineStatus } from "@/components/desk/ui/StatusBadge";
import { PaperDeskAuthBar } from "@/components/PaperDeskAuthBar";

// ── Formatting helpers ──────────────────────────────────────────────────────

const usd = (n: number | undefined | null, dp = 2): string =>
  typeof n === "number" && Number.isFinite(n)
    ? n.toLocaleString("en-US", { style: "currency", currency: "USD", minimumFractionDigits: dp, maximumFractionDigits: dp })
    : "—";

const num = (n: number | undefined | null, dp = 2): string =>
  typeof n === "number" && Number.isFinite(n) ? n.toFixed(dp) : "—";

const pct = (n: number | undefined | null, dp = 1): string =>
  typeof n === "number" && Number.isFinite(n) ? `${(n * 100).toFixed(dp)}%` : "—";

const signed = (n: number | undefined | null): string => {
  if (typeof n !== "number" || !Number.isFinite(n)) return "—";
  const s = usd(n);
  return n > 0 ? `+${s}` : s;
};

const pnlClass = (n: number | undefined | null): string =>
  typeof n === "number" && n > 0 ? "desk-pnl-positive" : typeof n === "number" && n < 0 ? "desk-pnl-negative" : "";

const pnlColor = (n: number | undefined | null): string =>
  typeof n === "number" && n > 0 ? "var(--desk-success)" : typeof n === "number" && n < 0 ? "var(--desk-error)" : "var(--desk-on-surface)";

const fmtTime = (iso: string | undefined): string => {
  if (!iso) return "—";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "—" : d.toLocaleString();
};

const fmtTimeShort = (iso: string | undefined): string => {
  if (!iso) return "—";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "—" : d.toLocaleTimeString();
};

const sideChip = (side: string) => (
  <DeskChip tone={side === "LONG" ? "success" : side === "SHORT" ? "error" : "default"} style={{ minHeight: 22 }}>
    {side}
  </DeskChip>
);

// ── Equity sparkline (inline SVG, no chart dependency) ──────────────────────

function EquitySparkline({ points }: { points: EquityCurveDoc[] }) {
  if (points.length < 2) {
    return <DeskEmptyState title="No equity history yet" subtitle="The engine records an equity point every 5 minutes." />;
  }
  const W = 800;
  const H = 220;
  const pad = 8;
  const values = points.map((p) => p.equity);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  const stepX = (W - pad * 2) / (points.length - 1);
  const coords = points.map((p, i) => {
    const x = pad + i * stepX;
    const y = pad + (H - pad * 2) * (1 - (p.equity - min) / range);
    return [x, y] as const;
  });
  const path = coords.map(([x, y], i) => `${i === 0 ? "M" : "L"}${x.toFixed(1)},${y.toFixed(1)}`).join(" ");
  const areaPath = `${path} L${coords[coords.length - 1][0].toFixed(1)},${H - pad} L${coords[0][0].toFixed(1)},${H - pad} Z`;
  const first = values[0];
  const last = values[values.length - 1];
  const up = last >= first;
  const stroke = up ? "var(--desk-success)" : "var(--desk-error)";

  return (
    <div style={{ width: "100%", overflowX: "auto" }}>
      <svg viewBox={`0 0 ${W} ${H}`} width="100%" height={H} preserveAspectRatio="none" role="img" aria-label="Equity curve">
        <defs>
          <linearGradient id="eqfill" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={stroke} stopOpacity="0.25" />
            <stop offset="100%" stopColor={stroke} stopOpacity="0" />
          </linearGradient>
        </defs>
        <path d={areaPath} fill="url(#eqfill)" />
        <path d={path} fill="none" stroke={stroke} strokeWidth={2} strokeLinejoin="round" strokeLinecap="round" />
      </svg>
      <div style={{ display: "flex", justifyContent: "space-between", marginTop: 8 }}>
        <span className="desk-label-md">{fmtTimeShort(points[0]?.ts)}</span>
        <span className="desk-label-md" style={{ color: pnlColor(last - first) }}>
          {signed(last - first)} over {points.length} pts
        </span>
        <span className="desk-label-md">{fmtTimeShort(points[points.length - 1]?.ts)}</span>
      </div>
    </div>
  );
}

// ── Tab keys ────────────────────────────────────────────────────────────────

type TabKey = PaperDeskTabKey;

const TABS: DeskTabItem<TabKey>[] = [
  { key: "positions", label: "Open Positions" },
  { key: "trades", label: "Trade History" },
  { key: "orders", label: "OMS Orders" },
  { key: "equity", label: "Equity Curve" },
  { key: "strategies", label: "Strategy Health" },
];

// ── Main component ──────────────────────────────────────────────────────────

export default function PaperDeskDashboard() {
  const live = useLiveBTCPrice();
  const router = useRouter();
  const searchParams = useSearchParams();
  const tabParam = searchParams.get("tab");
  const { state, openPositions, recentTrades, healthSummary, connection, lastUpdated, refresh } = usePaperDesk();
  const [riskMetrics, setRiskMetrics] = useState<{
    sharpeRatio: number | null;
    sortinoRatio: number | null;
    calmarRatio: number | null;
    maxDrawdownPct: number;
  } | null>(null);
  const [tab, setTab] = useState<TabKey>(() => (isPaperDeskTabKey(tabParam) ? tabParam : "positions"));

  useEffect(() => {
    if (connection !== "live") return;
    let active = true;
    fetch("/api/paper-desk/risk-metrics")
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (active && data?.ok) {
          setRiskMetrics({
            sharpeRatio: data.sharpeRatio ?? null,
            sortinoRatio: data.sortinoRatio ?? null,
            calmarRatio: data.calmarRatio ?? null,
            maxDrawdownPct: data.maxDrawdownPct ?? 0,
          });
        }
      })
      .catch(() => {});
    return () => { active = false; };
  }, [connection, lastUpdated]);

  useEffect(() => {
    if (isPaperDeskTabKey(tabParam)) {
      setTab(tabParam);
    }
  }, [tabParam]);

  const handleTabChange = useCallback(
    (next: TabKey) => {
      setTab(next);
      router.replace(paperDeskHref(next), { scroll: false });
    },
    [router],
  );

  const status: DeskEngineStatus =
    connection === "live" ? "live" : connection === "stale" ? "syncing" : connection === "unauthorized" ? "cloud-off" : "degraded";

  const equity = state?.equity ?? 0;
  const balance = state?.balance ?? 0;
  const realized = state?.realized_pnl ?? 0;
  const unrealized = state?.unrealized_pnl ?? 0;
  const openCount = state?.open_position_count ?? openPositions.length;

  const connectionStatus =
    connection === "live"
      ? "live" as const
      : connection === "connecting" || connection === "stale"
        ? "reconnecting" as const
        : "offline" as const;

  const persistenceStatus =
    connection === "live"
      ? "mongo" as const
      : connection === "connecting"
        ? "hydrating" as const
        : "local" as const;

  return (
    <AppShell
      btcPrice={live.price}
      btcChange24h={live.change24h}
      totalPnl={realized + unrealized}
      equity={equity}
      openPositions={openCount}
      paperDeskOpenPositions={openCount}
      activeStrategies={healthSummary?.total}
      connectionStatus={connectionStatus}
      persistenceStatus={persistenceStatus}
      systemStatus={connection === "error" ? "degraded" : "nominal"}
      dataFeedStatus={live.connected ? "synced" : "stale"}
      pageTitle="Paper Desk"
    >
      <DeskLinearProgress visible={connection === "connecting"} />
      <div style={{ display: "flex", flexDirection: "column", gap: "var(--desk-space-5)" }}>
        {/* Header */}
        <div style={{ display: "flex", flexWrap: "wrap", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
          <div>
            <h1 className="desk-display-lg">Paper Desk</h1>
            <p className="desk-label-md" style={{ fontWeight: 400 }}>
              Go Engine · MongoDB authoritative state · all devices identical
            </p>
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <StatusBadge status={status} />
            {lastUpdated ? (
              <span className="desk-label-md" style={{ fontWeight: 400 }}>
                Updated {fmtTimeShort(new Date(lastUpdated).toISOString())}
              </span>
            ) : null}
            <DeskButton variant="outlined" onClick={() => void refresh()} style={{ minHeight: 36, padding: "0 14px" }}>
              Refresh
            </DeskButton>
          </div>
        </div>

        <PaperDeskAuthBar compact />

        {connection === "unauthorized" ? (
          <DeskCard>
            <DeskEmptyState
              title="Sign in to view the Paper Desk"
              subtitle="This dashboard shows the live Go Engine paper-trading account. Sign in above to load it."
            />
          </DeskCard>
        ) : (
          <>
            {/* Account Summary */}
            <DeskCard>
              <DeskSectionHeader
                title="Account Summary"
                subtitle={state ? `Session since ${fmtTime(state.session_start)}` : "Awaiting engine snapshot…"}
              />
              <div
                style={{
                  display: "grid",
                  gridTemplateColumns: "repeat(auto-fit, minmax(150px, 1fr))",
                  gap: "var(--desk-space-3)",
                }}
              >
                <DeskMetricTile label="Equity" value={usd(equity)} valueClassName={pnlClass(equity - balance)} />
                <DeskMetricTile label="Balance (cash)" value={usd(balance)} />
                <DeskMetricTile
                  label="Unrealized P&L"
                  value={signed(unrealized)}
                  valueClassName={pnlClass(unrealized)}
                  sub={state ? `${state.open_position_count} open` : undefined}
                  subColor={unrealized >= 0 ? "profit" : "loss"}
                />
                <DeskMetricTile
                  label="Realized P&L"
                  value={signed(realized)}
                  valueClassName={pnlClass(realized)}
                />
                <DeskMetricTile label="Win Rate" value={pct(state?.win_rate)} sub={state ? `${state.winning_trades}W / ${state.losing_trades}L` : undefined} subColor="neutral" />
                <DeskMetricTile label="Total Trades" value={state ? String(state.total_trades) : "—"} />
                <DeskMetricTile
                  label="Exposure (BTC)"
                  value={num(state?.total_exposure_btc, 4)}
                  sub={state ? `L ${num(state.long_exposure_btc, 3)} / S ${num(state.short_exposure_btc, 3)}` : undefined}
                  subColor="neutral"
                />
                <DeskMetricTile label="Max Drawdown" value={pct(state?.max_drawdown)} valueClassName={state?.max_drawdown ? "desk-pnl-negative" : ""} sub={`Current ${pct(state?.current_drawdown)}`} subColor="neutral" />
                <DeskMetricTile label="Sharpe" value={riskMetrics?.sharpeRatio != null ? num(riskMetrics.sharpeRatio) : "—"} />
                <DeskMetricTile label="Sortino" value={riskMetrics?.sortinoRatio != null ? num(riskMetrics.sortinoRatio) : "—"} />
                <DeskMetricTile label="Calmar" value={riskMetrics?.calmarRatio != null ? num(riskMetrics.calmarRatio) : "—"} />
                <DeskMetricTile label="Risk Max DD" value={riskMetrics ? `${num(riskMetrics.maxDrawdownPct, 1)}%` : "—"} />
                <DeskMetricTile label="Total Fees" value={usd(state?.total_fees)} />
                <DeskMetricTile
                  label="Strategy Health"
                  value={healthSummary ? String(healthSummary.total) : "—"}
                  sub={healthSummary ? `${healthSummary.healthy} healthy · ${healthSummary.critical} critical` : undefined}
                  subColor={healthSummary && healthSummary.critical > 0 ? "loss" : "profit"}
                />
              </div>
            </DeskCard>

            {/* Tabbed detail */}
            <DeskCard>
              <DeskTabs items={TABS} active={tab} onChange={handleTabChange} variant="primary" />
              <div style={{ marginTop: "var(--desk-space-4)" }}>
                {tab === "positions" && <PositionsPanel positions={openPositions} markPrice={live.price} />}
                {tab === "trades" && (
                  <TradesPanel
                    recentTrades={recentTrades}
                    closedTradeTotal={state?.total_trades}
                  />
                )}
                {tab === "orders" && <OrdersPanel />}
                {tab === "equity" && <EquityPanel />}
                {tab === "strategies" && <StrategiesPanel />}
              </div>
            </DeskCard>
          </>
        )}
      </div>
    </AppShell>
  );
}

// ── Positions panel ─────────────────────────────────────────────────────────

const PositionsPanel = memo(function PositionsPanel({
  positions,
  markPrice,
}: {
  positions: PaperPositionDoc[];
  markPrice: number;
}) {
  const columns: DeskColumn<PaperPositionDoc>[] = [
    { id: "strategy", header: "Strategy", cell: (r) => <span title={r.strategy_id}>{r.strategy_id}</span> },
    { id: "symbol", header: "Symbol", cell: (r) => r.symbol },
    { id: "side", header: "Side", cell: (r) => sideChip(r.side) },
    { id: "size", header: "Size", align: "right", cell: (r) => num(r.size, 4) },
    { id: "entry", header: "Entry", align: "right", cell: (r) => usd(r.entry_price) },
    { id: "sl", header: "Stop", align: "right", cell: (r) => (r.stop_loss ? usd(r.stop_loss) : "—") },
    { id: "tp", header: "Target", align: "right", cell: (r) => (r.take_profit ? usd(r.take_profit) : "—") },
    {
      id: "pnl",
      header: "PnL",
      align: "right",
      cell: (r) => {
        const pnl = unrealizedPnlForOpenPosition(r, markPrice);
        return (
          <span style={{ color: pnlColor(pnl), fontWeight: 600 }}>
            {signed(pnl)}
          </span>
        );
      },
    },
    { id: "opened", header: "Opened", align: "right", cell: (r) => fmtTime(r.opened_at) },
  ];
  return (
    <DeskDataTable
      columns={columns}
      rows={positions}
      getRowKey={(r) => r.position_id}
      minWidth={860}
      empty={<DeskEmptyState title="No open positions" subtitle="Positions opened by the engine appear here in real time." />}
    />
  );
});

// ── Trades panel (recent live + paginated history) ──────────────────────────

const TradesPanel = memo(function TradesPanel({
  recentTrades,
  closedTradeTotal,
}: {
  recentTrades: PaperTradeDoc[];
  closedTradeTotal?: number;
}) {
  const [page, setPage] = useState<TradesPage | null>(null);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);
  const [side, setSide] = useState<string>("");
  const limit = 25;

  const load = useCallback(
    async (newOffset: number, newSide: string) => {
      setLoading(true);
      try {
        const p = await fetchTradesPage({ limit, offset: newOffset, side: newSide || undefined, sortBy: "closed_at", sortDir: "desc" });
        setPage(p);
        setOffset(newOffset);
      } finally {
        setLoading(false);
      }
    },
    [],
  );

  useEffect(() => {
    void load(0, side);
  }, [load, side]);

  const rows = page?.trades ?? recentTrades;
  const columns: DeskColumn<PaperTradeDoc>[] = [
    { id: "time", header: "Closed", cell: (r) => fmtTime(r.closed_at) },
    { id: "strategy", header: "Strategy", cell: (r) => <span title={r.strategy_id}>{r.strategy_id}</span> },
    { id: "symbol", header: "Symbol", cell: (r) => r.symbol },
    { id: "side", header: "Side", cell: (r) => sideChip(r.side) },
    { id: "qty", header: "Qty", align: "right", cell: (r) => num(r.quantity, 4) },
    { id: "entry", header: "Entry", align: "right", cell: (r) => usd(r.entry_price) },
    { id: "exit", header: "Exit", align: "right", cell: (r) => usd(r.exit_price) },
    { id: "fees", header: "Fees", align: "right", cell: (r) => usd(r.fees) },
    {
      id: "pnl",
      header: "Net P&L",
      align: "right",
      cell: (r) => <span style={{ color: pnlColor(r.net_pnl), fontWeight: 600 }}>{signed(r.net_pnl)}</span>,
    },
    { id: "reason", header: "Exit", cell: (r) => <DeskChip style={{ minHeight: 22 }}>{r.exit_reason || "—"}</DeskChip> },
  ];

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      <div style={{ display: "flex", flexWrap: "wrap", gap: 8, alignItems: "center" }}>
        <DeskChip tone="primary">
          {page
            ? `${page.pagination.total} total trades`
            : typeof closedTradeTotal === "number"
              ? `${closedTradeTotal} total trades`
              : `${rows.length} recent`}
        </DeskChip>
        {(["", "LONG", "SHORT"] as const).map((s) => (
          <DeskButton
            key={s || "all"}
            variant={side === s ? "tonal" : "outlined"}
            onClick={() => setSide(s)}
            style={{ minHeight: 32, padding: "0 12px" }}
          >
            {s === "" ? "All" : s}
          </DeskButton>
        ))}
      </div>
      <DeskDataTable
        columns={columns}
        rows={rows}
        getRowKey={(r, i) => `${r.client_trade_id}-${i}`}
        minWidth={900}
        empty={<DeskEmptyState title="No closed trades yet" subtitle="Completed trades from the engine appear here." />}
      />
      {page ? (
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8 }}>
          <span className="desk-label-md">
            {offset + 1}–{offset + rows.length} of {page.pagination.total}
          </span>
          <div style={{ display: "flex", gap: 8 }}>
            <DeskButton variant="outlined" disabled={loading || offset === 0} onClick={() => void load(Math.max(0, offset - limit), side)} style={{ minHeight: 36 }}>
              Prev
            </DeskButton>
            <DeskButton variant="outlined" disabled={loading || !page.pagination.has_more} onClick={() => void load(offset + limit, side)} style={{ minHeight: 36 }}>
              Next
            </DeskButton>
          </div>
        </div>
      ) : null}
    </div>
  );
});

// ── OMS Orders panel ────────────────────────────────────────────────────────

const OrdersPanel = memo(function OrdersPanel() {
  const [orders, setOrders] = useState<PaperOrderDoc[] | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let on = true;
    fetchOrders(150)
      .then((o) => on && setOrders(o))
      .finally(() => on && setLoading(false));
    return () => {
      on = false;
    };
  }, []);

  const transitionTone = (to: string): "success" | "error" | "warning" | "primary" | "default" => {
    if (to === "POSITION_OPENED" || to === "SIMULATED_FILL" || to === "ACCEPTED") return "success";
    if (to === "POSITION_CLOSED") return "primary";
    if (to === "REJECTED" || to === "CANCELLED") return "error";
    if (to === "RISK_CHECKED") return "warning";
    return "default";
  };

  const columns: DeskColumn<PaperOrderDoc>[] = [
    { id: "time", header: "Time", cell: (r) => fmtTime(r.recorded_at) },
    { id: "order", header: "Order ID", cell: (r) => <span title={r.order_id}>{r.order_id.slice(0, 14)}…</span> },
    { id: "strategy", header: "Strategy", cell: (r) => r.strategy_id },
    { id: "symbol", header: "Symbol", cell: (r) => r.symbol },
    { id: "side", header: "Side", cell: (r) => sideChip(r.side) },
    {
      id: "transition",
      header: "Transition",
      cell: (r) => (
        <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
          <span className="desk-label-md">{r.transition_from || "—"}</span>
          <span aria-hidden>→</span>
          <DeskChip tone={transitionTone(r.transition_to)} style={{ minHeight: 22 }}>
            {r.transition_to}
          </DeskChip>
        </span>
      ),
    },
    { id: "fill", header: "Fill", align: "right", cell: (r) => (r.fill_price ? usd(r.fill_price) : "—") },
    { id: "pnl", header: "Net P&L", align: "right", cell: (r) => (typeof r.net_pnl === "number" && r.net_pnl !== 0 ? <span style={{ color: pnlColor(r.net_pnl) }}>{signed(r.net_pnl)}</span> : "—") },
    { id: "reason", header: "Reason", cell: (r) => r.reason || "—" },
  ];

  if (loading && !orders) {
    return <DeskEmptyState title="Loading OMS history…" />;
  }

  return (
    <DeskDataTable
      columns={columns}
      rows={orders ?? []}
      getRowKey={(r, i) => `${r.order_id}-${r.transition_to}-${i}`}
      minWidth={980}
      empty={<DeskEmptyState title="No OMS transitions yet" subtitle="The order state machine logs every NEW→…→POSITION_CLOSED step here." />}
    />
  );
});

// ── Equity panel ────────────────────────────────────────────────────────────

const EquityPanel = memo(function EquityPanel() {
  const [curve, setCurve] = useState<EquityCurveDoc[]>([]);
  const [daily, setDaily] = useState<DailyPnLDoc[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let on = true;
    fetchEquity(288, 30)
      .then(({ curve: c, daily: d }) => {
        if (!on) return;
        setCurve(c);
        setDaily(d);
      })
      .finally(() => on && setLoading(false));
    return () => {
      on = false;
    };
  }, []);

  const dailyColumns: DeskColumn<DailyPnLDoc>[] = [
    { id: "date", header: "Date", cell: (r) => r.date },
    { id: "open", header: "Open", align: "right", cell: (r) => usd(r.opening_balance) },
    { id: "close", header: "Close", align: "right", cell: (r) => usd(r.closing_balance) },
    { id: "net", header: "Net P&L", align: "right", cell: (r) => <span style={{ color: pnlColor(r.net_pnl), fontWeight: 600 }}>{signed(r.net_pnl)}</span> },
    { id: "trades", header: "Trades", align: "right", cell: (r) => `${r.trade_count} (${r.win_count}W/${r.loss_count}L)` },
    { id: "dd", header: "Max DD", align: "right", cell: (r) => pct(r.max_drawdown_pct) },
  ];

  if (loading) return <DeskEmptyState title="Loading equity curve…" />;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--desk-space-5)" }}>
      <EquitySparkline points={curve} />
      <div>
        <DeskSectionHeader title="Daily P&L" subtitle="Midnight-sealed daily records (last 30 days)" />
        <DeskDataTable
          columns={dailyColumns}
          rows={daily}
          getRowKey={(r) => r.date}
          minWidth={620}
          empty={<DeskEmptyState title="No sealed days yet" subtitle="Daily P&L is sealed at midnight UTC." />}
        />
      </div>
    </div>
  );
});

// ── Strategies panel ────────────────────────────────────────────────────────

const StrategiesPanel = memo(function StrategiesPanel() {
  const [scores, setScores] = useState<StrategyScoreDoc[]>([]);
  const [health, setHealth] = useState<StrategyHealthDoc[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<string>("");

  useEffect(() => {
    let on = true;
    // setLoading(true) is intentionally inside the async chain (not synchronous in
    // the effect body) to satisfy react-hooks/set-state-in-effect.
    void (async () => {
      setLoading(true);
      try {
        const { scores: s, health: h } = await fetchStrategyHealth(filter || undefined);
        if (!on) return;
        setScores(s);
        setHealth(h);
      } finally {
        if (on) setLoading(false);
      }
    })();
    return () => {
      on = false;
    };
  }, [filter]);

  const healthByStrategy = new Map(health.map((h) => [h.strategy_id, h]));
  const healthTone = (s: string): "success" | "warning" | "error" | "default" =>
    s === "HEALTHY" ? "success" : s === "WARNING" ? "warning" : s === "CRITICAL" ? "error" : "default";

  const columns: DeskColumn<StrategyScoreDoc>[] = [
    { id: "strategy", header: "Strategy", cell: (r) => <span title={r.strategy_id}>{r.strategy_id}</span> },
    {
      id: "health",
      header: "Health",
      cell: (r) => {
        const h = healthByStrategy.get(r.strategy_id);
        return (
          <DeskChip tone={healthTone(h?.health_status ?? "")} title={h?.health_reasons?.join("; ")} style={{ minHeight: 22 }}>
            {h?.health_status ?? "—"}
          </DeskChip>
        );
      },
    },
    { id: "win", header: "Win Rate", align: "right", cell: (r) => pct(r.win_rate) },
    { id: "pf", header: "Profit Factor", align: "right", cell: (r) => num(r.profit_factor, 2) },
    { id: "exp", header: "Expectancy", align: "right", cell: (r) => <span style={{ color: pnlColor(r.expectancy) }}>{usd(r.expectancy, 2)}</span> },
    { id: "pnl", header: "Total P&L", align: "right", cell: (r) => <span style={{ color: pnlColor(r.total_pnl), fontWeight: 600 }}>{signed(r.total_pnl)}</span> },
    { id: "dd", header: "Max DD", align: "right", cell: (r) => pct(r.max_drawdown) },
    { id: "n", header: "Trades", align: "right", cell: (r) => String(r.sample_size) },
  ];

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
        {(["", "HEALTHY", "WARNING", "CRITICAL", "INSUFFICIENT_DATA"] as const).map((f) => (
          <DeskButton key={f || "all"} variant={filter === f ? "tonal" : "outlined"} onClick={() => setFilter(f)} style={{ minHeight: 32, padding: "0 12px" }}>
            {f === "" ? "All" : f.replace("_", " ")}
          </DeskButton>
        ))}
      </div>
      {loading ? (
        <DeskEmptyState title="Loading strategy health…" />
      ) : (
        <DeskDataTable
          columns={columns}
          rows={scores}
          getRowKey={(r) => r.strategy_id}
          minWidth={840}
          empty={<DeskEmptyState title="No strategy scores yet" subtitle="The engine computes strategy health every 15 minutes." />}
        />
      )}
    </div>
  );
});
