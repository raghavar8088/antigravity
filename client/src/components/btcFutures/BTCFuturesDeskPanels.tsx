"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { StrategyLeaderboardRow } from "@/lib/paperTradesAnalytics";
import type {
  BTCFuturesEngineStats,
  BTCFuturesPosition,
  BTCFuturesStrategyStatus,
  BTCFuturesTrade,
} from "@/hooks/useBTCFuturesScalperEngine";
import { FUTURES_STRATEGY_PROFILES } from "@/lib/futuresSessionMetrics";
import { paperPriceMovePctOnNotional } from "@/lib/futuresPaperMath";
import { ShadowIntentLogPanel } from "@/components/ShadowIntentLogPanel";
import { TestnetOpsPanel } from "@/components/TestnetOpsPanel";
import {
  DeskButton,
  DeskCard,
  DeskChip,
  type DeskColumn,
  DeskDataTable,
  DeskEmptyState,
  DeskMetricTile,
  DeskSectionHeader,
  DeskSwitch,
} from "@/components/desk/ui";
import { useDeskMounted } from "@/hooks/useDeskMounted";
import { formatDeskPct, formatDeskUsd, pnlToneClass } from "@/lib/deskFormat";
import {
  ExitReasonChip,
  exitReasonChipTone,
  parseExitReasonSummary,
  SideChip,
  StrategyStatusChip,
} from "./deskUiHelpers";

const LEADERBOARD_WINDOW_DAYS = 30;
const LEADERBOARD_TABLE_LIMIT = 10;
const PAPER_EXPORT_WINDOW_DAYS = 30;

type Stats = BTCFuturesEngineStats;

function formatShortTime(iso: string) {
  return new Date(iso).toLocaleTimeString("en-US", { hour: "2-digit", minute: "2-digit", hour12: false });
}

function formatShortDate(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric" });
}

function fmtContracts(n: number) {
  return n.toLocaleString("en-US", { maximumFractionDigits: 0 });
}

function tradePriceMovePct(t: BTCFuturesTrade): number {
  return typeof t.priceMovePct === "number" && Number.isFinite(t.priceMovePct)
    ? t.priceMovePct
    : paperPriceMovePctOnNotional(t.entryPrice, t.exitPrice, t.side);
}

function tradeDurationMins(t: BTCFuturesTrade): string {
  const mins = Math.floor((new Date(t.closedAt).getTime() - new Date(t.openedAt).getTime()) / 60000);
  return mins > 0 ? `${mins}m` : "<1m";
}

// --- Cloud leaderboard (30d) ---
function CloudLeaderboardPanel({
  cloudAccountKey,
  disabledStrategyIds,
  onAddDisabledStrategyIds,
}: {
  cloudAccountKey: string | null;
  disabledStrategyIds: readonly number[];
  onAddDisabledStrategyIds: (ids: number[]) => void;
}) {
  const [open, setOpen] = useState(true);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [data, setData] = useState<{ top: StrategyLeaderboardRow[]; bottom: StrategyLeaderboardRow[] } | null>(null);
  const disabledSet = useMemo(() => new Set(disabledStrategyIds), [disabledStrategyIds]);

  const load = useCallback(async () => {
    if (!cloudAccountKey) return;
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams({
        window_days: String(LEADERBOARD_WINDOW_DAYS),
        limit: String(LEADERBOARD_TABLE_LIMIT),
      });
      const res = await fetch(`/api/paper-trades/leaderboard?${params}`, { credentials: "include", cache: "no-store" });
      const body = (await res.json()) as {
        ok?: boolean;
        error?: string;
        top?: StrategyLeaderboardRow[];
        bottom?: StrategyLeaderboardRow[];
      };
      if (!res.ok || !body.ok) {
        setError(body.error ?? `HTTP ${res.status}`);
        setData(null);
        return;
      }
      setData({ top: body.top ?? [], bottom: body.bottom ?? [] });
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setData(null);
    } finally {
      setLoading(false);
    }
  }, [cloudAccountKey]);

  useEffect(() => {
    void load();
  }, [load]);

  const columns = (showDisable: boolean): DeskColumn<StrategyLeaderboardRow>[] => {
    const cols: DeskColumn<StrategyLeaderboardRow>[] = [
      {
        id: "strat",
        header: "Strategy",
        cell: (r) => (
          <span className="desk-body-md" style={{ fontWeight: 500 }}>
            #{r.strategyId} {r.strategyName}
            {disabledSet.has(r.strategyId) ? <DeskChip tone="default">Disabled</DeskChip> : null}
          </span>
        ),
      },
      { id: "n", header: "n", align: "right", cell: (r) => r.tradeCount },
      {
        id: "sum",
        header: "Sum net",
        align: "right",
        cell: (r) => <span className={pnlToneClass(r.sumNet)}>{formatDeskUsd(r.sumNet, { signed: true })}</span>,
      },
      {
        id: "exp",
        header: "Exp",
        align: "right",
        cell: (r) => <span className={pnlToneClass(r.expectancy)}>{formatDeskUsd(r.expectancy, { signed: true })}</span>,
      },
    ];
    if (showDisable) {
      cols.push({
        id: "act",
        header: "",
        align: "right",
        cell: (r) =>
          disabledSet.has(r.strategyId) ? (
            "—"
          ) : (
            <DeskButton
              variant="danger-tonal"
              style={{ minHeight: 32, padding: "0 10px", fontSize: "0.75rem" }}
              onClick={() => onAddDisabledStrategyIds([r.strategyId])}
            >
              Disable
            </DeskButton>
          ),
      });
    }
    return cols;
  };

  return (
    <div style={{ marginTop: 16 }}>
      <DeskSectionHeader
        title={`Cloud leaderboard (${LEADERBOARD_WINDOW_DAYS}d)`}
        actions={
          <>
            <DeskButton variant="outlined" style={{ minHeight: 36 }} onClick={() => setOpen((v) => !v)}>
              {open ? "Collapse" : "Expand"}
            </DeskButton>
            <DeskButton variant="outlined" style={{ minHeight: 36 }} disabled={loading} onClick={() => void load()}>
              {loading ? "Loading…" : "Refresh"}
            </DeskButton>
          </>
        }
      />
      {open ? (
        !cloudAccountKey ? (
          <p className="desk-label-md">Sign in to load your cloud strategy leaderboard.</p>
        ) : error ? (
          <p className="desk-label-md" style={{ color: "var(--desk-error)" }}>
            {error}
          </p>
        ) : data ? (
          <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            <DeskDataTable columns={columns(false)} rows={data.top} getRowKey={(r) => `top-${r.strategyId}`} minWidth={480} />
            <DeskDataTable
              columns={columns(true)}
              rows={data.bottom}
              getRowKey={(r) => `bot-${r.strategyId}`}
              minWidth={520}
              empty={<DeskEmptyState title="No bottom performers" />}
            />
            {data.bottom.length > 0 ? (
              <DeskButton
                variant="danger-tonal"
                onClick={() => {
                  const ids = data.bottom.map((r) => r.strategyId);
                  if (window.confirm(`Disable all ${ids.length} bottom strategies?`)) onAddDisabledStrategyIds(ids);
                }}
              >
                Disable all bottom {data.bottom.length}
              </DeskButton>
            ) : null}
          </div>
        ) : (
          <p className="desk-label-md">{loading ? "Loading…" : "No data"}</p>
        )
      ) : null}
    </div>
  );
}

export type BTCFuturesDeskPanelsProps = {
  title: string;
  baseBalance: number;
  equity: number;
  sessionPnL: number;
  totalReturn: number;
  pnlPositive: boolean;
  stats: Stats;
  isReady: boolean;
  longCount: number;
  shortCount: number;
  totalUnrealized: number;
  sortedTrades: BTCFuturesTrade[];
  trades: BTCFuturesTrade[];
  strategyStatuses: BTCFuturesStrategyStatus[];
  visibleStrategies: BTCFuturesStrategyStatus[];
  showAllStrategies: boolean;
  setShowAllStrategies: (v: boolean) => void;
  positions: BTCFuturesPosition[];
  visibleTrades: BTCFuturesTrade[];
  showAllTrades: boolean;
  setShowAllTrades: (v: boolean) => void;
  tradesByDay: Record<string, { trades: number; wins: number; losses: number; pnl: number }>;
  cloudAccountKey: string | null;
  exportingCsv: boolean;
  onExportCsv: () => void;
  disabledStrategies: number[];
  setDisabledStrategies: (ids: number[]) => void;
  deskShadowIntentsEnabled: boolean;
  deskTestnetOpsEnabled: boolean;
};

export function BTCFuturesDeskPanels(props: BTCFuturesDeskPanelsProps) {
  const mounted = useDeskMounted();
  const {
    title,
    baseBalance,
    equity,
    sessionPnL,
    totalReturn,
    pnlPositive,
    stats,
    isReady,
    longCount,
    shortCount,
    totalUnrealized,
    sortedTrades,
    trades,
    strategyStatuses,
    visibleStrategies,
    showAllStrategies,
    setShowAllStrategies,
    positions,
    visibleTrades,
    showAllTrades,
    setShowAllTrades,
    tradesByDay,
    cloudAccountKey,
    exportingCsv,
    onExportCsv,
    disabledStrategies,
    setDisabledStrategies,
    deskShadowIntentsEnabled,
    deskTestnetOpsEnabled,
  } = props;

  const [profileOpen, setProfileOpen] = useState(true);
  const exitChips = useMemo(() => parseExitReasonSummary(stats.sessionExitReasonSummary), [stats.sessionExitReasonSummary]);

  const positionColumns: DeskColumn<BTCFuturesPosition>[] = [
    { id: "sym", header: "Symbol", cell: (p) => <span className="desk-mono">{p.symbol}</span> },
    {
      id: "strat",
      header: "Strategy",
      cell: (p) => (
        <div>
          <p className="desk-body-md" style={{ fontWeight: 500 }}>
            {p.strategyName}
          </p>
          <p className="desk-label-md">
            {p.leverage}× · {p.marginMode}
          </p>
        </div>
      ),
    },
    { id: "side", header: "Side", cell: (p) => <SideChip side={p.side} /> },
    { id: "ct", header: "Contracts", align: "right", cell: (p) => fmtContracts(p.contracts) },
    {
      id: "entry",
      header: "Entry",
      align: "right",
      cell: (p) => formatDeskUsd(p.entryPrice, { decimals: 0 }),
    },
    {
      id: "mark",
      header: "Mark",
      align: "right",
      cell: (p) => formatDeskUsd(p.markPrice, { decimals: 0 }),
    },
    {
      id: "pnl",
      header: "PnL",
      align: "right",
      cell: (p) => (
        <span className={pnlToneClass(p.unrealizedPnl)}>
          {formatDeskUsd(p.unrealizedPnl, { signed: true })}
        </span>
      ),
    },
  ];

  const strategyColumns: DeskColumn<BTCFuturesStrategyStatus>[] = [
    { id: "id", header: "#", cell: (s) => s.id },
    {
      id: "name",
      header: "Strategy",
      cell: (s) => (
        <div>
          <p className="desk-body-md" style={{ fontWeight: 500 }}>
            {s.name}
          </p>
          <p className="desk-label-md">{s.category}</p>
        </div>
      ),
    },
    { id: "cat", header: "Category", cell: (s) => s.category },
    {
      id: "status",
      header: "Status",
      cell: (s) => <StrategyStatusChip status={s.status} disabled={s.disabled} />,
    },
    {
      id: "score",
      header: "Score",
      align: "right",
      cell: (s) => <span className="desk-mono">{s.score.toFixed(1)}</span>,
    },
    {
      id: "live",
      header: "Live",
      align: "right",
      cell: (s) => (s.openCount > 0 ? `${s.openCount} pos` : "—"),
    },
    {
      id: "active",
      header: "Active",
      align: "right",
      cell: (s) => (
        <DeskSwitch
          checked={!s.disabled}
          label=""
          ariaLabel={`Toggle strategy ${s.name}`}
          onChange={() => {
            const next = s.disabled
              ? disabledStrategies.filter((id) => id !== s.id)
              : [...disabledStrategies, s.id];
            setDisabledStrategies(next);
          }}
        />
      ),
    },
  ];

  const tradeColumns: DeskColumn<BTCFuturesTrade>[] = [
    {
      id: "time",
      header: "Time",
      cell: (t) => (
        <div>
          <span className="desk-mono">{formatShortTime(t.closedAt)}</span>
          <p className="desk-label-md">{formatShortDate(t.closedAt)}</p>
        </div>
      ),
    },
    { id: "sym", header: "Symbol", cell: (t) => t.symbol },
    { id: "strat", header: "Strategy", cell: (t) => t.strategyName },
    { id: "side", header: "Side", cell: (t) => <SideChip side={t.side} /> },
    {
      id: "exit",
      header: "Exit",
      cell: (t) => <ExitReasonChip reason={t.exitReason} />,
    },
    {
      id: "net",
      header: "Net PnL",
      align: "right",
      cell: (t) => (
        <span className={pnlToneClass(t.netPnl)}>{formatDeskUsd(t.netPnl, { signed: true })}</span>
      ),
    },
  ];

  const timeOffenderColumns: DeskColumn<(typeof stats.sessionWorstTimeOffenders)[number]>[] = [
    {
      id: "strat",
      header: "Strategy",
      cell: (r) => (
        <span>
          <span className="desk-mono">{r.strategyId}</span> {r.strategyName}
        </span>
      ),
    },
    { id: "cat", header: "Category", cell: (r) => r.category },
    {
      id: "tp",
      header: "Desk TP",
      cell: (r) => (r.deskTpWidened ? <DeskChip tone="warning">TP↑</DeskChip> : <DeskChip>Base</DeskChip>),
    },
    { id: "tn", header: "TIME n", align: "right", cell: (r) => r.timeCount },
    {
      id: "tsum",
      header: "TIME Σ",
      align: "right",
      cell: (r) => <span className={pnlToneClass(r.timeSumNet)}>{formatDeskUsd(r.timeSumNet, { signed: true })}</span>,
    },
    {
      id: "tavg",
      header: "TIME avg",
      align: "right",
      cell: (r) => <span className={pnlToneClass(r.timeMeanNet)}>{formatDeskUsd(r.timeMeanNet, { signed: true })}</span>,
    },
  ];

  return (
    <>
      <DeskCard>
        <DeskSectionHeader title="Equity & PnL" subtitle={title} />
        <div className="desk-metrics-row">
          <DeskMetricTile
            label="Account equity"
            value={mounted ? formatDeskUsd(equity) : "—"}
            valueClassName="desk-mono"
          />
          <DeskMetricTile
            label="Session PnL"
            value={mounted ? formatDeskUsd(sessionPnL, { signed: true }) : "—"}
            valueClassName={pnlToneClass(sessionPnL)}
            detail={`${formatDeskPct(totalReturn, { signed: true })} vs base`}
          />
          <DeskMetricTile
            label="Closed PnL"
            value={formatDeskUsd(stats.netPnl, { signed: true })}
            valueClassName={pnlToneClass(stats.netPnl)}
            detail={`${stats.totalTrades} trades`}
          />
          <DeskMetricTile
            label="Unrealized"
            value={formatDeskUsd(totalUnrealized, { signed: true })}
            valueClassName={pnlToneClass(totalUnrealized)}
          />
          <DeskMetricTile label="Win rate" value={formatDeskPct(stats.winRate, { decimals: 1 })} compact />
          <DeskMetricTile
            label="Profit factor"
            value={stats.profitFactor > 100 ? "∞" : stats.profitFactor.toFixed(2)}
            valueClassName={stats.profitFactor >= 1 ? "desk-pnl-positive" : "desk-pnl-negative"}
            compact
          />
          <DeskMetricTile
            label="Drawdown"
            value={formatDeskPct(stats.drawdownPct)}
            detail={stats.isDrawdownLocked ? "Entries locked" : `Base ${formatDeskUsd(baseBalance)}`}
            compact
          />
          <DeskMetricTile
            label="Open exposure"
            value={`${stats.openPositions} positions`}
            detail={`${longCount} long · ${shortCount} short`}
            compact
          />
        </div>
        {trades.length === 0 ? (
          <div style={{ marginTop: 12 }}>
            <DeskEmptyState title="No exits yet" subtitle="Last closed trade PnL will appear after the first close." />
          </div>
        ) : (
          <DeskMetricTile
            compact
            label="Last closed"
            value={formatDeskUsd(sortedTrades[0].netPnl, { signed: true })}
            valueClassName={pnlToneClass(sortedTrades[0].netPnl)}
            detail={sortedTrades[0].strategyName}
          />
        )}
      </DeskCard>

      <DeskCard>
        <DeskSectionHeader
          title="Desk profile & session flow"
          actions={
            <>
              <DeskButton variant="outlined" style={{ minHeight: 36 }} onClick={() => setProfileOpen((v) => !v)}>
                {profileOpen ? "Collapse" : "Expand"}
              </DeskButton>
              <DeskButton
                variant="outlined"
                style={{ minHeight: 36 }}
                disabled={!cloudAccountKey || exportingCsv}
                onClick={onExportCsv}
              >
                {exportingCsv ? "Exporting…" : `Export CSV (${PAPER_EXPORT_WINDOW_DAYS}d)`}
              </DeskButton>
            </>
          }
        />
        {profileOpen ? (
          <>
            <div className="desk-metrics-row" style={{ marginBottom: 12 }}>
              <DeskChip tone="primary">{FUTURES_STRATEGY_PROFILES[stats.strategyProfile].label}</DeskChip>
              <DeskChip tone="primary">Regime {stats.deskLastRegimeTag}</DeskChip>
              <DeskChip>Signal {stats.effectiveSignalThreshold}</DeskChip>
              {stats.deskVolSizedNotionalEnabled ? <DeskChip>Vol-sized notional</DeskChip> : null}
            </div>
            <div className="desk-metrics-row">
              <DeskMetricTile label="Trades / hr" value={stats.sessionTradesPerHour.toFixed(2)} compact />
              <DeskMetricTile
                label="Expectancy"
                value={formatDeskUsd(stats.sessionExpectancyPerTrade, { signed: true })}
                valueClassName={pnlToneClass(stats.sessionExpectancyPerTrade)}
                compact
              />
              <DeskMetricTile label="Fee / |gross|" value={formatDeskPct(stats.sessionFeePctOfAbsGross)} compact />
              <DeskMetricTile label="Hold avg" value={`${stats.sessionAvgHoldMinutes.toFixed(1)}m`} compact />
              <DeskMetricTile label="Hold median" value={`${stats.sessionMedianHoldMinutes.toFixed(1)}m`} compact />
              <DeskMetricTile label="Hold P95" value={`${stats.sessionHoldP95Minutes.toFixed(1)}m`} compact />
              <DeskMetricTile
                label="Desk RR"
                value={`${stats.deskTpWidenedStratCount} TP↑`}
                detail={`LowRR ${stats.deskLowRrSkippedStratCount}`}
                compact
              />
            </div>
            <div className="desk-metrics-row" style={{ marginTop: 8 }}>
              <DeskMetricTile label="Skip ATR/fees" value={String(stats.deskSkippedMinExpectedMove)} compact />
              <DeskMetricTile label="Skip same-dir" value={String(stats.deskSkippedSameDirCap)} compact />
              <DeskMetricTile label="Skip regime" value={String(stats.deskSkippedByRegime)} compact />
              <DeskMetricTile label="Skip priority" value={String(stats.deskSkippedLowPriorityEntry)} compact />
              <DeskMetricTile label="Skip spread" value={String(stats.deskSkippedSpread)} compact />
              <DeskMetricTile label="Skip category" value={String(stats.deskSkippedCategoryCap)} compact />
              <DeskMetricTile label="Skip session" value={String(stats.deskSkippedOutsideSession)} compact />
            </div>
            <p className="desk-label-md" style={{ marginTop: 16, marginBottom: 8 }}>
              Exit reasons (last 400)
            </p>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
              {exitChips.length === 0 ? (
                <DeskChip>—</DeskChip>
              ) : (
                exitChips.map((c) => (
                  <DeskChip key={c.reason} tone={exitReasonChipTone(c.reason)}>
                    {c.reason}×{c.count} {c.avg}
                  </DeskChip>
                ))
              )}
            </div>
            <p className="desk-label-md" style={{ marginTop: 12 }}>
              Widened-hold opens: <span className="desk-mono">{stats.deskProfileAdjustedHoldAppliedCount}</span>
            </p>
            <div style={{ marginTop: 16 }}>
              <DeskSectionHeader title="Worst TIME contributors" subtitle="Last 400 closed" />
              {stats.sessionWorstTimeOffenders.length === 0 ? (
                <DeskEmptyState title="No TIME exits yet" />
              ) : (
                <DeskDataTable
                  columns={timeOffenderColumns}
                  rows={stats.sessionWorstTimeOffenders}
                  getRowKey={(r) => `${r.strategyId}-${r.deskTpWidened}`}
                  minWidth={640}
                />
              )}
            </div>
            <ShadowIntentLogPanel enabled={deskShadowIntentsEnabled} signedIn={Boolean(cloudAccountKey)} />
            <CloudLeaderboardPanel
              cloudAccountKey={cloudAccountKey}
              disabledStrategyIds={disabledStrategies}
              onAddDisabledStrategyIds={(ids) => {
                const merged = [...new Set([...disabledStrategies, ...ids])];
                setDisabledStrategies(merged);
              }}
            />
          </>
        ) : null}
      </DeskCard>

      <DeskCard>
        <DeskSectionHeader
          title="Open positions"
          subtitle={`${positions.length} active · engine ${isReady ? "live" : "syncing"}`}
          actions={
            <DeskChip tone={totalUnrealized > 0 ? "success" : totalUnrealized < 0 ? "error" : "default"}>
              Unrealized {formatDeskUsd(totalUnrealized, { signed: true })}
            </DeskChip>
          }
        />
        {positions.length === 0 ? (
          <DeskEmptyState
            title="No open positions"
            subtitle="The engine runs the full strategy library on each symbol when market data is ready."
          />
        ) : (
          <DeskDataTable columns={positionColumns} rows={positions} getRowKey={(p) => p.id} minWidth={720} />
        )}
      </DeskCard>

      {trades.length > 0 && Object.keys(tradesByDay).length > 0 ? (
        <DeskCard>
          <DeskSectionHeader title="Daily PnL ledger" subtitle={`${Object.keys(tradesByDay).length} days`} />
          <DeskDataTable
            minWidth={520}
            columns={[
              { id: "d", header: "Date", cell: ([day]) => formatDate(day) },
              { id: "t", header: "Trades", align: "right", cell: ([, d]) => d.trades },
              { id: "wl", header: "W / L", align: "right", cell: ([, d]) => `${d.wins}W / ${d.losses}L` },
              {
                id: "pnl",
                header: "Daily PnL",
                align: "right",
                cell: ([, d]) => <span className={pnlToneClass(d.pnl)}>{formatDeskUsd(d.pnl, { signed: true })}</span>,
              },
            ]}
            rows={Object.entries(tradesByDay).sort((a, b) => b[0].localeCompare(a[0]))}
            getRowKey={([day]) => day}
          />
        </DeskCard>
      ) : null}

      <DeskCard>
        <DeskSectionHeader
          title="Strategy leaderboard"
          subtitle={`${strategyStatuses.length} strategies · live roster`}
          actions={
            <DeskButton variant="outlined" style={{ minHeight: 36 }} onClick={() => setShowAllStrategies(!showAllStrategies)}>
              {showAllStrategies ? "Top 12" : `All ${strategyStatuses.length}`}
            </DeskButton>
          }
        />
        <DeskDataTable
          columns={strategyColumns}
          rows={visibleStrategies}
          getRowKey={(s) => String(s.id)}
          minWidth={720}
        />
      </DeskCard>

      {trades.length > 0 ? (
        <DeskCard>
          <DeskSectionHeader
            title="Closed trades"
            subtitle={`${stats.totalTrades} total · ${stats.winCount}W / ${stats.lossCount}L`}
            actions={
              <DeskButton variant="outlined" style={{ minHeight: 36 }} onClick={() => setShowAllTrades(!showAllTrades)}>
                {showAllTrades ? "Recent 10" : `All ${trades.length}`}
              </DeskButton>
            }
          />
          <DeskDataTable columns={tradeColumns} rows={visibleTrades} getRowKey={(t) => t.id} minWidth={640} />
        </DeskCard>
      ) : null}

      {deskTestnetOpsEnabled ? (
        <DeskCard variant="outlined">
          <TestnetOpsPanel />
        </DeskCard>
      ) : null}
    </>
  );
}
