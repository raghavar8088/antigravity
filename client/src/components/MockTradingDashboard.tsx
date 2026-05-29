"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import useLiveBTCPrice from "@/hooks/useLiveBTCPrice";
import { useMockTradingEngine } from "@/hooks/useMockTradingEngine";
import { WorkspaceNavPanel } from "@/components/desk/WorkspaceNavPanel";
import {
  DeskBanner,
  DeskButton,
  DeskCard,
  DeskChip,
  DeskDataTable,
  DeskEmptyState,
  DeskMetricTile,
  DeskSectionHeader,
  type DeskColumn,
} from "@/components/desk/ui";
import {
  computeAnalytics,
  filterMockTrades,
  MOCK_IGNORED_GATES,
  MOCK_TRADE_SORT_OPTIONS,
  sortMockTrades,
  type MockAccountState,
  type MockSide,
  type MockSizingMode,
  type MockTrade,
  type MockTradeFilter,
  type MockTradeSortKey,
  type MockTradeStatus,
  type MockTradingConfig,
} from "@/lib/mockTradingEngine";
import { workspaceModuleDescription } from "@/lib/workspaceModuleDescription";

function fmtUsd(value: number, digits = 2): string {
  if (!Number.isFinite(value)) return "—";
  const sign = value < 0 ? "-" : "";
  return `${sign}$${Math.abs(value).toLocaleString("en-US", {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })}`;
}

function fmtUsdK(value: number): string {
  if (!Number.isFinite(value)) return "—";
  const abs = Math.abs(value);
  if (abs >= 1_000_000) return `${value < 0 ? "-" : ""}$${(abs / 1_000_000).toFixed(2)}M`;
  if (abs >= 10_000) return `${value < 0 ? "-" : ""}$${(abs / 1_000).toFixed(1)}K`;
  return fmtUsd(value, 0);
}

function fmtPrice(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "—";
  return value.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function pnlClass(value: number): string {
  if (value > 0) return "desk-pnl-positive";
  if (value < 0) return "desk-pnl-negative";
  return "";
}

function fmtPct(value: number, digits = 2): string {
  if (!Number.isFinite(value)) return "—";
  return `${(value * 100).toFixed(digits)}%`;
}

function fmtAge(openedAt: number, ref: number = Date.now()): string {
  const secs = Math.max(0, Math.floor((ref - openedAt) / 1000));
  if (secs < 60) return `${secs}s`;
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m`;
  const hrs = Math.floor(mins / 60);
  return `${hrs}h ${mins % 60}m`;
}

export default function MockTradingDashboard() {
  const router = useRouter();
  const live = useLiveBTCPrice();
  const engine = useMockTradingEngine({ price: live.price });

  const [filter, setFilter] = useState<MockTradeFilter>({});
  const [showOpenOnly, setShowOpenOnly] = useState(false);
  const [sortKey, setSortKey] = useState<MockTradeSortKey>("most_profitable");

  const trades = useMemo(() => {
    const combined: MockTradeFilter = { ...filter };
    if (showOpenOnly) combined.status = "OPEN";
    const filtered = filterMockTrades(engine.trades, combined);
    return sortMockTrades(filtered, sortKey);
  }, [engine.trades, filter, showOpenOnly, sortKey]);

  const strategyOptions = useMemo(() => {
    const seen = new Map<number, string>();
    for (const t of engine.trades) {
      if (!seen.has(t.strategyId)) seen.set(t.strategyId, t.strategyName);
    }
    return [...seen.entries()].sort(([a], [b]) => a - b);
  }, [engine.trades]);

  const blockerOptions = useMemo(() => {
    const set = new Set<string>();
    for (const t of engine.trades) for (const b of t.blockers) set.add(b.gate);
    return [...set].sort();
  }, [engine.trades]);

  const columns = useMemo<DeskColumn<MockTrade>[]>(
    () => [
      { id: "ts", header: "Opened", cell: (t) => new Date(t.openedAt).toLocaleTimeString() },
      { id: "strategy", header: "Strategy", cell: (t) => `#${t.strategyId} ${t.strategyName}` },
      {
        id: "side",
        header: "Side",
        cell: (t) => (
          <span style={{ color: t.side === "BUY" ? "var(--desk-success)" : "var(--desk-error)" }}>
            {t.side}
          </span>
        ),
      },
      { id: "entry", header: "Entry", align: "right", cell: (t) => fmtPrice(t.entryPrice) },
      { id: "qty", header: "Qty", align: "right", cell: (t) => t.quantity.toFixed(5) },
      { id: "notional", header: "Notional", align: "right", cell: (t) => fmtUsdK(t.notional) },
      { id: "margin", header: "Margin", align: "right", cell: (t) => fmtUsdK(t.marginUsed) },
      { id: "lev", header: "Lev", align: "right", cell: (t) => `${t.leverage}×` },
      { id: "mark", header: "Mark", align: "right", cell: (t) => fmtPrice(t.currentPrice) },
      {
        id: "pnl",
        header: "PnL (net)",
        align: "right",
        cell: (t) => {
          const pnl = t.status === "OPEN" ? t.unrealizedPnl : t.realizedPnl;
          return <span className={pnlClass(pnl)}>{fmtUsd(pnl)}</span>;
        },
      },
      {
        id: "status",
        header: "Status",
        cell: (t) => (
          <DeskChip tone={t.status === "OPEN" ? "primary" : "default"}>{t.status}</DeskChip>
        ),
      },
      {
        id: "blockers",
        header: "Blockers (ignored)",
        cell: (t) =>
          t.blockers.length === 0 ? (
            <span style={{ color: "var(--desk-on-surface-variant)" }}>—</span>
          ) : (
            <div style={{ display: "flex", gap: 4, flexWrap: "wrap" }}>
              {t.blockers.map((b, i) => (
                <DeskChip key={`${t.id}-b-${i}`} tone="warning">
                  {b.gate}
                </DeskChip>
              ))}
            </div>
          ),
      },
      { id: "age", header: "Age", align: "right", cell: (t) => fmtAge(t.openedAt) },
      {
        id: "actions",
        header: "",
        cell: (t) =>
          t.status === "OPEN" ? (
            <DeskButton
              variant="outlined"
              onClick={() => engine.closeTrade(t.id)}
              style={{ minHeight: 28, fontSize: "0.75rem", padding: "0 10px" }}
            >
              Close
            </DeskButton>
          ) : (
            <span style={{ color: "var(--desk-on-surface-variant)", fontSize: 11 }}>
              {t.exitReason}
            </span>
          ),
      },
    ],
    [engine],
  );

  const acct = engine.account;
  const totalPnlClass = pnlClass(engine.analytics.totalPnl);
  const realizedClass = pnlClass(engine.analytics.realizedPnl);
  const unrealizedClass = pnlClass(engine.analytics.unrealizedPnl);

  return (
    <main className="trading-shell">
      <div className="trading-shell__inner">
        <header className="trading-landing-header">
          <div>
            <h1 className="trading-landing-header__title">Mock Trading</h1>
            <p className="trading-landing-header__desc">
              Simulated ${acct.startingBalance.toLocaleString("en-US")} paper account — every raised
              strategy signal becomes a mock trade with full sizing, fees and slippage applied, and
              the same TP/SL/MaxHold lifecycle as the production paper desk. Blockers are recorded
              for analysis but never prevent a trade.
            </p>
          </div>
          <div className="trading-landing-header__chips">
            <DeskChip tone="warning">Mock</DeskChip>
            <DeskChip tone="default">${(acct.startingBalance / 1_000_000).toFixed(1)}M paper</DeskChip>
            <DeskChip tone="default">{engine.config.leverage}× leverage</DeskChip>
            <DeskChip tone={live.connected ? "success" : "error"}>
              {live.connected ? "Live feed" : "Disconnected"}
            </DeskChip>
            <DeskChip tone="default">{acct.openCount} open</DeskChip>
          </div>
        </header>

        <div className="workspace-nav--slim">
          <WorkspaceNavPanel
            activeModule="mockTrading"
            onModuleChange={(next) => {
              if (next === "btcFutureTrading") router.push("/btc-future-trading");
            }}
            actionsEnabled={false}
            onActionsEnabledChange={() => {}}
            actionToggleTitle="Controls are not applicable in Mock Trading."
            moduleDescription={workspaceModuleDescription("mockTrading")}
          />
        </div>

        <DeskBanner
          variant="warning"
          title={`Mock Trading uses simulated $${acct.startingBalance.toLocaleString("en-US")} paper balance. No real orders are placed.`}
        >
          The production safety pipeline is untouched. This module subscribes to the same BTC price
          feed and the same strategy signal trace, then opens a mock trade for every raised signal
          regardless of REGIME_BLOCKING, SIGNAL, ATR_FEES, confluence, cooldown, max-open, or
          trend/regime filters. Sizing, leverage, fees and slippage mirror the main paper desk so
          PnL is comparable.
        </DeskBanner>

        <AccountSummaryCard account={acct} />

        <DeskCard padding="md">
          <DeskSectionHeader title="Live ticker" subtitle={`BTCUSD · ${live.connected ? "Binance stream" : "reconnecting"}`} />
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))",
              gap: 12,
            }}
          >
            <DeskMetricTile label="BTC Price" value={fmtPrice(live.price)} compact />
            <DeskMetricTile label="24h Change" value={`${live.change24h.toFixed(2)}%`} valueClassName={pnlClass(live.change24h)} compact />
            <DeskMetricTile label="24h High" value={fmtPrice(live.high24h)} compact />
            <DeskMetricTile label="24h Low" value={fmtPrice(live.low24h)} compact />
            <DeskMetricTile label="Ticks / s" value={live.ticksPerSecond} compact />
            <DeskMetricTile
              label="Trace age"
              value={engine.traceAgeSeconds == null ? "—" : `${engine.traceAgeSeconds}s`}
              compact
            />
          </div>
        </DeskCard>

        <DeskCard padding="md">
          <DeskSectionHeader title="Trade analytics" subtitle="Realized PnL is net of round-trip fees and slippage; unrealized PnL marks every OPEN trade to the live BTC mark and subtracts the round-trip fee debt that would crystallize on close." />
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))",
              gap: 12,
            }}
          >
            <DeskMetricTile label="Total trades" value={engine.analytics.totalTrades} compact />
            <DeskMetricTile label="Open" value={engine.analytics.openTrades} compact />
            <DeskMetricTile label="Closed" value={engine.analytics.closedTrades} compact />
            <DeskMetricTile label="Win rate" value={fmtPct(engine.analytics.winRate, 1)} compact />
            <DeskMetricTile label="Total PnL" value={fmtUsd(engine.analytics.totalPnl)} valueClassName={totalPnlClass} compact />
            <DeskMetricTile label="Realized" value={fmtUsd(engine.analytics.realizedPnl)} valueClassName={realizedClass} compact />
            <DeskMetricTile label="Unrealized" value={fmtUsd(engine.analytics.unrealizedPnl)} valueClassName={unrealizedClass} compact />
            <DeskMetricTile label="Avg PnL" value={fmtUsd(engine.analytics.averagePnl)} compact />
            <DeskMetricTile
              label="Profit factor"
              value={engine.analytics.profitFactor == null ? "—" : engine.analytics.profitFactor.toFixed(2)}
              compact
            />
          </div>
        </DeskCard>

        <MockConfigCard config={engine.config} onChange={engine.setConfig} />

        <DeskCard padding="md">
          <DeskSectionHeader
            title="Mock trades"
            subtitle={`${trades.length} matching · ${engine.trades.length} total`}
          />
          <div
            style={{
              display: "flex",
              flexWrap: "wrap",
              gap: 8,
              alignItems: "center",
              marginBottom: 12,
            }}
          >
            <select
              value={filter.strategyId ?? ""}
              onChange={(e) =>
                setFilter((f) => ({
                  ...f,
                  strategyId: e.target.value ? Number(e.target.value) : null,
                }))
              }
              style={selectStyle}
            >
              <option value="">All strategies</option>
              {strategyOptions.map(([id, name]) => (
                <option key={id} value={id}>
                  #{id} {name}
                </option>
              ))}
            </select>

            <select
              value={filter.side ?? ""}
              onChange={(e) => setFilter((f) => ({ ...f, side: (e.target.value || null) as MockSide | null }))}
              style={selectStyle}
            >
              <option value="">All sides</option>
              <option value="BUY">BUY</option>
              <option value="SELL">SELL</option>
            </select>

            <select
              value={filter.status ?? ""}
              onChange={(e) =>
                setFilter((f) => ({ ...f, status: (e.target.value || null) as MockTradeStatus | null }))
              }
              style={selectStyle}
            >
              <option value="">All statuses</option>
              <option value="OPEN">OPEN</option>
              <option value="CLOSED">CLOSED</option>
            </select>

            <select
              value={filter.blockerGate ?? ""}
              onChange={(e) => setFilter((f) => ({ ...f, blockerGate: e.target.value || null }))}
              style={selectStyle}
            >
              <option value="">All blockers</option>
              {blockerOptions.map((g) => (
                <option key={g} value={g}>
                  {g}
                </option>
              ))}
            </select>

            <select
              value={filter.profitability ?? ""}
              onChange={(e) =>
                setFilter((f) => ({
                  ...f,
                  profitability: (e.target.value || null) as "profit" | "loss" | null,
                }))
              }
              style={selectStyle}
            >
              <option value="">Profit & loss</option>
              <option value="profit">Profitable</option>
              <option value="loss">Unprofitable</option>
            </select>

            <select
              value={sortKey}
              onChange={(e) => setSortKey(e.target.value as MockTradeSortKey)}
              style={selectStyle}
              aria-label="Sort trades"
              title="Sort order"
            >
              {MOCK_TRADE_SORT_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>

            <label style={{ display: "flex", gap: 4, alignItems: "center", fontSize: 12 }}>
              <input
                type="checkbox"
                checked={showOpenOnly}
                onChange={(e) => setShowOpenOnly(e.target.checked)}
              />
              Open only
            </label>

            <div style={{ marginLeft: "auto", display: "flex", gap: 8 }}>
              <DeskButton variant="outlined" onClick={() => setFilter({})}>
                Clear filters
              </DeskButton>
              <DeskButton variant="danger-tonal" onClick={engine.reset}>
                Reset mock state
              </DeskButton>
            </div>
          </div>

          <DeskDataTable
            columns={columns}
            rows={trades}
            getRowKey={(t) => t.id}
            empty={
              <DeskEmptyState
                title="No mock trades yet"
                subtitle={
                  engine.error
                    ? `Trace fetch error: ${engine.error}`
                    : "Waiting for the next strategy signal trace tick. The live BTC feed is connected; mock trades will appear as soon as any strategy raises a signal."
                }
              />
            }
            minWidth={1080}
          />
        </DeskCard>

        <PerStrategyCard analytics={engine.analytics} startingBalance={acct.startingBalance} />
        <PerBlockerCard analytics={engine.analytics} />

        <DeskCard padding="md">
          <DeskSectionHeader title="Mock trade log" subtitle="Most recent events — newest first" />
          {engine.logs.length === 0 ? (
            <DeskEmptyState
              title="No log events"
              subtitle="MOCK_TRADE_CREATED and MOCK_TRADE_CLOSED events will appear here as signals arrive."
            />
          ) : (
            <div
              style={{
                fontFamily: "var(--desk-font-mono, monospace)",
                fontSize: 11,
                lineHeight: 1.5,
                maxHeight: 260,
                overflowY: "auto",
                background: "var(--desk-surface-container)",
                padding: 12,
                borderRadius: 6,
              }}
            >
              {engine.logs.map((log, i) => (
                <div key={i}>
                  <span style={{ color: "var(--desk-on-surface-variant)" }}>
                    {new Date(log.ts).toLocaleTimeString()}
                  </span>{" "}
                  <span
                    style={{
                      color: log.event === "MOCK_TRADE_CREATED" ? "var(--desk-success)" : "var(--desk-primary)",
                    }}
                  >
                    {log.event}
                  </span>{" "}
                  strategy=#{log.strategyId} {log.strategyName} side={log.side} price=
                  {fmtPrice(log.price)}
                  {log.notional != null ? ` notional=${fmtUsdK(log.notional)}` : ""}
                  {log.pnl != null ? ` pnl=${fmtUsd(log.pnl)}` : ""}
                  {log.ignoredBlockers.length > 0
                    ? ` ignoredBlockers=[${log.ignoredBlockers.join(",")}]`
                    : ""}
                </div>
              ))}
            </div>
          )}
          <div style={{ marginTop: 8, fontSize: 11, color: "var(--desk-on-surface-variant)" }}>
            Mock trading ignores these production gates by design:{" "}
            {MOCK_IGNORED_GATES.join(", ")}.
          </div>
        </DeskCard>
      </div>
    </main>
  );
}

const selectStyle: React.CSSProperties = {
  padding: "6px 10px",
  borderRadius: 6,
  border: "1px solid var(--desk-outline)",
  background: "var(--desk-surface)",
  color: "var(--desk-on-surface)",
  fontSize: "0.8125rem",
  minWidth: 130,
};

function AccountSummaryCard({ account }: { account: MockAccountState }) {
  const equityClass = pnlClass(account.equity - account.startingBalance);
  const realizedClass = pnlClass(account.realizedPnl);
  const unrealClass = pnlClass(account.unrealizedPnl);
  const returnClass = pnlClass(account.returnPct);
  return (
    <DeskCard padding="md">
      <DeskSectionHeader
        title="Account summary"
        subtitle="Mirrors the main paper desk: starting balance, equity = balance + realized + unrealized; margin = notional / leverage; available = equity − margin."
      />
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fit, minmax(150px, 1fr))",
          gap: 12,
        }}
      >
        <DeskMetricTile label="Starting Balance" value={fmtUsd(account.startingBalance, 0)} compact />
        <DeskMetricTile label="Cash Balance" value={fmtUsd(account.cashBalance, 0)} compact />
        <DeskMetricTile label="Equity" value={fmtUsd(account.equity, 0)} valueClassName={equityClass} compact />
        <DeskMetricTile label="Realized PnL" value={fmtUsd(account.realizedPnl, 2)} valueClassName={realizedClass} compact />
        <DeskMetricTile label="Unrealized PnL" value={fmtUsd(account.unrealizedPnl, 2)} valueClassName={unrealClass} compact />
        <DeskMetricTile label="Exposure" value={fmtUsd(account.exposure, 0)} compact />
        <DeskMetricTile label="Margin Used" value={fmtUsd(account.marginUsed, 0)} compact />
        <DeskMetricTile label="Available" value={fmtUsd(account.availableBalance, 0)} compact />
        <DeskMetricTile label="Open Trades" value={account.openCount} compact />
        <DeskMetricTile label="Return %" value={fmtPct(account.returnPct, 2)} valueClassName={returnClass} compact />
        <DeskMetricTile label="Max Drawdown" value={fmtPct(account.maxDrawdownPct, 2)} compact />
        <DeskMetricTile label="Peak Equity" value={fmtUsd(account.peakEquity, 0)} compact />
      </div>
    </DeskCard>
  );
}

function MockConfigCard({
  config,
  onChange,
}: {
  config: MockTradingConfig;
  onChange: (next: MockTradingConfig) => void;
}) {
  return (
    <DeskCard padding="md">
      <DeskSectionHeader
        title="Mock account & sizing"
        subtitle="Per-strategy SL/TP/hold from the production roster override these defaults when known. Changes persist in localStorage."
      />
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(170px, 1fr))", gap: 12 }}>
        <ConfigNumber
          label="Starting balance ($)"
          value={config.startingBalanceUsd}
          step={10_000}
          onChange={(v) => onChange({ ...config, startingBalanceUsd: v })}
        />
        <ConfigSelect
          label="Sizing mode"
          value={config.sizingMode}
          options={[
            { value: "fixed_pct_equity", label: "Fixed % of equity" },
            { value: "fixed_notional", label: "Fixed notional ($)" },
            { value: "risk_pct_equity", label: "% risk per trade" },
          ]}
          onChange={(v) => onChange({ ...config, sizingMode: v as MockSizingMode })}
        />
        {config.sizingMode === "fixed_pct_equity" && (
          <ConfigNumber
            label="Fixed % of equity"
            value={config.fixedPctOfEquity}
            step={0.1}
            onChange={(v) => onChange({ ...config, fixedPctOfEquity: v })}
          />
        )}
        {config.sizingMode === "fixed_notional" && (
          <ConfigNumber
            label="Fixed notional ($)"
            value={config.fixedNotionalUsd}
            step={100}
            onChange={(v) => onChange({ ...config, fixedNotionalUsd: v })}
          />
        )}
        {config.sizingMode === "risk_pct_equity" && (
          <ConfigNumber
            label="Risk % per trade"
            value={config.riskPctOfEquity}
            step={0.1}
            onChange={(v) => onChange({ ...config, riskPctOfEquity: v })}
          />
        )}
        <ConfigNumber
          label="Leverage (×)"
          value={config.leverage}
          step={1}
          onChange={(v) => onChange({ ...config, leverage: Math.max(1, Math.min(125, Math.floor(v))) })}
        />
        <ConfigNumber
          label="Take profit %"
          value={config.takeProfitPct}
          step={0.1}
          onChange={(v) => onChange({ ...config, takeProfitPct: v })}
        />
        <ConfigNumber
          label="Stop loss %"
          value={config.stopLossPct}
          step={0.1}
          onChange={(v) => onChange({ ...config, stopLossPct: v })}
        />
        <ConfigNumber
          label="Max hold (min)"
          value={config.maxHoldMinutes}
          step={1}
          onChange={(v) => onChange({ ...config, maxHoldMinutes: v })}
        />
        <ConfigNumber
          label="Taker fee per side (%)"
          value={config.takerFeePct * 100}
          step={0.01}
          onChange={(v) => onChange({ ...config, takerFeePct: Math.max(0, v / 100) })}
        />
        <ConfigNumber
          label="Slippage per side (bps)"
          value={config.slippageBpsPerSide}
          step={1}
          onChange={(v) => onChange({ ...config, slippageBpsPerSide: Math.max(0, v) })}
        />
      </div>
    </DeskCard>
  );
}

function ConfigNumber({
  label,
  value,
  step,
  onChange,
}: {
  label: string;
  value: number;
  step: number;
  onChange: (v: number) => void;
}) {
  return (
    <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
      <span className="desk-label-md">{label}</span>
      <input
        type="number"
        value={value}
        step={step}
        min={0}
        onChange={(e) => {
          const v = Number(e.target.value);
          if (Number.isFinite(v) && v >= 0) onChange(v);
        }}
        style={{
          padding: "6px 10px",
          borderRadius: 6,
          border: "1px solid var(--desk-outline)",
          background: "var(--desk-surface)",
          color: "var(--desk-on-surface)",
          fontSize: "0.875rem",
          fontFamily: "var(--desk-font-mono, monospace)",
        }}
      />
    </label>
  );
}

function ConfigSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (v: string) => void;
}) {
  return (
    <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
      <span className="desk-label-md">{label}</span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        style={{
          padding: "6px 10px",
          borderRadius: 6,
          border: "1px solid var(--desk-outline)",
          background: "var(--desk-surface)",
          color: "var(--desk-on-surface)",
          fontSize: "0.875rem",
        }}
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </label>
  );
}

function PerStrategyCard({
  analytics,
  startingBalance,
}: {
  analytics: ReturnType<typeof computeAnalytics>;
  startingBalance: number;
}) {
  return (
    <DeskCard padding="md">
      <DeskSectionHeader
        title="Per-strategy roll-up"
        subtitle="Sorted by total PnL. Return % is PnL / starting balance, so strategies are comparable in account-level terms. Exposure is summed across this strategy's currently OPEN positions."
      />
      {analytics.perStrategy.length === 0 ? (
        <DeskEmptyState title="No data yet" subtitle="Per-strategy roll-up populates as mock trades are created." />
      ) : (
        <DeskDataTable
          columns={[
            { id: "id", header: "Strategy", cell: (r) => `#${r.strategyId} ${r.strategyName}` },
            { id: "total", header: "Total", align: "right", cell: (r) => r.total },
            { id: "open", header: "Open", align: "right", cell: (r) => r.open },
            { id: "closed", header: "Closed", align: "right", cell: (r) => r.closed },
            { id: "wins", header: "W/L", align: "right", cell: (r) => `${r.wins}/${r.losses}` },
            { id: "win", header: "Win %", align: "right", cell: (r) => fmtPct(r.winRate, 1) },
            { id: "exp", header: "Exposure", align: "right", cell: (r) => fmtUsdK(r.exposure) },
            {
              id: "pnl",
              header: "Total PnL",
              align: "right",
              cell: (r) => <span className={pnlClass(r.totalPnl)}>{fmtUsd(r.totalPnl)}</span>,
            },
            {
              id: "ret",
              header: "Return %",
              align: "right",
              cell: (r) => (
                <span className={pnlClass(r.totalPnl)}>
                  {fmtPct(startingBalance > 0 ? r.totalPnl / startingBalance : 0, 2)}
                </span>
              ),
            },
            {
              id: "realized",
              header: "Realized",
              align: "right",
              cell: (r) => <span className={pnlClass(r.realizedPnl)}>{fmtUsd(r.realizedPnl)}</span>,
            },
          ]}
          rows={analytics.perStrategy}
          getRowKey={(r) => String(r.strategyId)}
          minWidth={760}
        />
      )}
    </DeskCard>
  );
}

function PerBlockerCard({ analytics }: { analytics: ReturnType<typeof computeAnalytics> }) {
  return (
    <DeskCard padding="md">
      <DeskSectionHeader
        title="Blockers by frequency"
        subtitle="How often each production gate would have stopped these mock trades, and whether the trades that hit each gate would have been profitable on this $1M paper account."
      />
      {analytics.perBlocker.length === 0 ? (
        <DeskEmptyState
          title="No blockers recorded yet"
          subtitle="When mock trades are created from signals that the production pipeline would have rejected, the blocker reason appears here."
        />
      ) : (
        <DeskDataTable
          columns={[
            { id: "gate", header: "Blocker gate", cell: (r) => <DeskChip tone="warning">{r.gate}</DeskChip> },
            { id: "total", header: "Mock trades", align: "right", cell: (r) => r.total },
            { id: "wl", header: "W/L", align: "right", cell: (r) => `${r.wins}/${r.losses}` },
            { id: "wr", header: "Win %", align: "right", cell: (r) => fmtPct(r.winRate, 1) },
            {
              id: "pnl",
              header: "Total PnL",
              align: "right",
              cell: (r) => <span className={pnlClass(r.totalPnl)}>{fmtUsd(r.totalPnl)}</span>,
            },
          ]}
          rows={analytics.perBlocker}
          getRowKey={(r) => r.gate}
          minWidth={560}
        />
      )}
    </DeskCard>
  );
}
