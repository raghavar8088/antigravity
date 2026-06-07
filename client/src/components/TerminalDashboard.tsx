"use client";

import { useMemo } from "react";
import { AppShell } from "@/components/terminal";
import useLiveBTCPrice from "@/hooks/useLiveBTCPrice";
import { useMockCandleBuilder } from "@/hooks/useMockCandleBuilder";
import { useMockTradingEngine } from "@/hooks/useMockTradingEngine";
import type { MockTradeLog } from "@/lib/mockTradingEngine";
import { useMarketRegime } from "@/hooks/useMarketRegime";
import { useStrategyScoring } from "@/hooks/useStrategyScoring";

function fmtUsd(v: number, compact = false): string {
  if (!Number.isFinite(v)) return "—";
  const abs = Math.abs(v);
  const sign = v < 0 ? "-" : v > 0 ? "+" : "";
  if (compact && abs >= 1_000_000) return `${sign}$${(abs / 1_000_000).toFixed(2)}M`;
  if (compact && abs >= 1_000)    return `${sign}$${(abs / 1_000).toFixed(1)}K`;
  return `${sign}$${abs.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

function fmtPct(v: number, decimals = 1): string {
  if (!Number.isFinite(v)) return "—";
  return `${(v * 100).toFixed(decimals)}%`;
}

function fmtPrice(v: number): string {
  if (!Number.isFinite(v) || v <= 0) return "—";
  return v.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

type KpiCardProps = {
  label: string;
  value: string;
  change?: string;
  variant?: "default" | "positive" | "negative" | "accent" | "info";
  sub?: string;
};

function KpiCard({ label, value, change, variant = "default", sub }: KpiCardProps) {
  const variantClass = variant !== "default" ? `kpi-card--${variant}` : "";
  const valueClass = variant !== "default" ? variant : "";

  return (
    <div className={`kpi-card ${variantClass}`}>
      <div className="kpi-label">{label}</div>
      <div className={`kpi-value ${valueClass}`}>{value}</div>
      {change && (
        <div className={`kpi-change ${variant !== "default" ? variant : ""}`}>{change}</div>
      )}
      {sub && (
        <div style={{ fontSize: 10, color: "var(--text-muted)", marginTop: 2 }}>{sub}</div>
      )}
    </div>
  );
}

type SectionHeaderProps = {
  title: string;
  subtitle?: string;
  actions?: React.ReactNode;
};

function SectionHeader({ title, subtitle, actions }: SectionHeaderProps) {
  return (
    <div style={{
      display: "flex",
      alignItems: "flex-start",
      justifyContent: "space-between",
      gap: 12,
      marginBottom: 12,
    }}>
      <div>
        <h2 style={{
          fontFamily: "var(--font-display)",
          fontSize: 13,
          fontWeight: 700,
          color: "var(--text-primary)",
          margin: 0,
          letterSpacing: "-0.1px",
        }}>
          {title}
        </h2>
        {subtitle && (
          <p style={{ fontSize: 11, color: "var(--text-muted)", margin: "2px 0 0", lineHeight: 1.4 }}>
            {subtitle}
          </p>
        )}
      </div>
      {actions}
    </div>
  );
}

function LiveBadge() {
  return (
    <span style={{
      display: "inline-flex",
      alignItems: "center",
      gap: 4,
      fontSize: 10,
      fontWeight: 700,
      letterSpacing: "0.06em",
      color: "var(--green)",
      background: "var(--green-dim)",
      border: "1px solid rgba(38,166,154,0.2)",
      padding: "2px 6px",
      borderRadius: 3,
    }}>
      <span style={{
        width: 5,
        height: 5,
        borderRadius: "50%",
        background: "var(--green)",
        animation: "pulse-green 2s infinite",
        display: "inline-block",
      }} />
      LIVE
    </span>
  );
}

function StatusRow({ label, value, status }: { label: string; value: string; status?: "ok" | "warn" | "error" | "info" }) {
  const color = status === "ok" ? "var(--green)"
    : status === "warn" ? "var(--amber)"
    : status === "error" ? "var(--red)"
    : status === "info" ? "var(--info)"
    : "var(--text-primary)";

  return (
    <div style={{
      display: "flex",
      alignItems: "center",
      justifyContent: "space-between",
      padding: "7px 0",
      borderBottom: "1px solid var(--border-subtle)",
    }}>
      <span style={{ fontSize: 11, color: "var(--text-secondary)", fontWeight: 500 }}>{label}</span>
      <span style={{ fontSize: 12, fontWeight: 700, color, fontFamily: "var(--font-mono)", fontVariantNumeric: "tabular-nums" }}>
        {value}
      </span>
    </div>
  );
}

function StrategyRow({ rank, name, pnl, winRate, sharpe }: {
  rank: number;
  name: string;
  pnl: number;
  winRate: number;
  sharpe?: number | null;
}) {
  const pnlColor = pnl > 0 ? "var(--green)" : pnl < 0 ? "var(--red)" : "var(--text-muted)";

  return (
    <div style={{
      display: "grid",
      gridTemplateColumns: "28px 1fr 90px 70px 60px",
      gap: 8,
      alignItems: "center",
      padding: "8px 0",
      borderBottom: "1px solid var(--border-subtle)",
    }}>
      <span style={{
        fontSize: 10,
        fontWeight: 700,
        color: "var(--text-muted)",
        textAlign: "center",
        fontFamily: "var(--font-mono)",
      }}>
        {rank}
      </span>
      <span style={{
        fontSize: 12,
        color: "var(--text-primary)",
        fontWeight: 500,
        overflow: "hidden",
        textOverflow: "ellipsis",
        whiteSpace: "nowrap",
      }}>
        {name}
      </span>
      <span style={{
        fontSize: 12,
        fontFamily: "var(--font-mono)",
        fontWeight: 700,
        color: pnlColor,
        textAlign: "right",
        fontVariantNumeric: "tabular-nums",
      }}>
        {fmtUsd(pnl, true)}
      </span>
      <span style={{
        fontSize: 11,
        fontFamily: "var(--font-mono)",
        color: winRate >= 0.5 ? "var(--green)" : "var(--text-secondary)",
        textAlign: "right",
        fontVariantNumeric: "tabular-nums",
      }}>
        {fmtPct(winRate)}
      </span>
      <span style={{
        fontSize: 11,
        fontFamily: "var(--font-mono)",
        color: sharpe != null && sharpe > 1 ? "var(--green)" : "var(--text-muted)",
        textAlign: "right",
        fontVariantNumeric: "tabular-nums",
      }}>
        {sharpe != null && Number.isFinite(sharpe) ? sharpe.toFixed(2) : "—"}
      </span>
    </div>
  );
}

export default function TerminalDashboard() {
  const live = useLiveBTCPrice();
  const engine = useMockTradingEngine({ price: live.price, disablePolling: true });
  const candles = useMockCandleBuilder(live.price);
  const regime = useMarketRegime({ candles: candles.snapshot, newCandleReady: candles.newCandleReady });
  const scoring = useStrategyScoring({
    trades: engine.trades,
    currentRegime: regime.regime,
    newCandleReady: candles.newCandleReady,
    topNCount: 10,
  });

  const acct = engine.account;
  const analytics = engine.analytics;

  const connectionStatus = live.connected ? "live" as const : "offline" as const;
  const persistenceStatus = engine.persistence.status === "mongo"
    ? "mongo" as const
    : engine.persistence.status === "hydrating"
      ? "hydrating" as const
      : "local" as const;

  /* Strategy leaderboard — top 5 by PnL */
  const strategyLeaderboard = useMemo(() => {
    const byStrategy = new Map<number, {
      name: string;
      trades: number;
      wins: number;
      totalPnl: number;
    }>();

    for (const t of engine.trades) {
      if (t.status !== "CLOSED") continue;
      const existing = byStrategy.get(t.strategyId) ?? {
        name: t.strategyName,
        trades: 0,
        wins: 0,
        totalPnl: 0,
      };
      existing.trades++;
      existing.totalPnl += t.realizedPnl;
      if (t.realizedPnl > 0) existing.wins++;
      byStrategy.set(t.strategyId, existing);
    }

    return [...byStrategy.entries()]
      .map(([id, d]) => ({
        id,
        name: d.name,
        pnl: d.totalPnl,
        winRate: d.trades > 0 ? d.wins / d.trades : 0,
        trades: d.trades,
      }))
      .sort((a, b) => b.pnl - a.pnl)
      .slice(0, 5);
  }, [engine.trades]);

  /* Recent signals */
  const recentSignals = useMemo(
    () => engine.logs.slice(0, 8),
    [engine.logs],
  );

  /* Open positions */
  const openPositions = useMemo(
    () => engine.trades.filter((t) => t.status === "OPEN").slice(0, 5),
    [engine.trades],
  );

  return (
    <AppShell
      btcPrice={live.price}
      btcChange24h={live.change24h}
      regime={regime.regime}
      dailyPnl={analytics.totalPnl}
      totalPnl={analytics.totalPnl}
      equity={acct.equity}
      openPositions={acct.openCount}
      connectionStatus={connectionStatus}
      persistenceStatus={persistenceStatus}
      pageTitle="Dashboard"
    >
      {/* ── KPI Row ──────────────────────────────────────────────────────── */}
      <section style={{ marginBottom: 20 }}>
        <div style={{
          display: "flex",
          alignItems: "center",
          gap: 8,
          marginBottom: 12,
        }}>
          <h1 style={{
            fontFamily: "var(--font-display)",
            fontSize: 16,
            fontWeight: 700,
            color: "var(--text-primary)",
            margin: 0,
            letterSpacing: "-0.3px",
          }}>
            Overview
          </h1>
          <LiveBadge />
        </div>

        <div className="kpi-grid">
          <KpiCard
            label="Account Equity"
            value={`$${(acct.equity / 1_000_000).toFixed(3)}M`}
            variant="accent"
            sub={`Started: $${(acct.startingBalance / 1_000_000).toFixed(1)}M`}
          />
          <KpiCard
            label="Total PnL"
            value={fmtUsd(analytics.totalPnl, true)}
            variant={analytics.totalPnl >= 0 ? "positive" : "negative"}
            change={fmtPct(acct.equity > 0 ? analytics.totalPnl / acct.startingBalance : 0)}
          />
          <KpiCard
            label="Realized PnL"
            value={fmtUsd(analytics.realizedPnl, true)}
            variant={analytics.realizedPnl >= 0 ? "positive" : "negative"}
          />
          <KpiCard
            label="Unrealized PnL"
            value={fmtUsd(analytics.unrealizedPnl, true)}
            variant={analytics.unrealizedPnl >= 0 ? "positive" : "negative"}
          />
          <KpiCard
            label="Win Rate"
            value={fmtPct(analytics.winRate)}
            variant={analytics.winRate >= 0.5 ? "positive" : "negative"}
            sub={`${analytics.totalTrades} total trades`}
          />
          <KpiCard
            label="Profit Factor"
            value={analytics.profitFactor != null ? analytics.profitFactor.toFixed(2) : "—"}
            variant={analytics.profitFactor != null && analytics.profitFactor >= 1.2 ? "positive" : "default"}
          />
          <KpiCard
            label="Sharpe Ratio"
            value={analytics.sharpeRatio != null ? analytics.sharpeRatio.toFixed(2) : "—"}
            variant={analytics.sharpeRatio != null && analytics.sharpeRatio >= 1 ? "positive" : "default"}
          />
          <KpiCard
            label="Open Positions"
            value={`${acct.openCount}`}
            variant={acct.openCount > 0 ? "info" : "default"}
            sub={`Max: ${engine.config.maxOpenMockTrades}`}
          />
          <KpiCard
            label="BTC Price"
            value={`$${fmtPrice(live.price)}`}
            variant="accent"
            change={`${live.change24h >= 0 ? "+" : ""}${live.change24h.toFixed(2)}% 24h`}
          />
        </div>
      </section>

      {/* ── Main content grid ─────────────────────────────────────────────── */}
      <div style={{
        display: "grid",
        gridTemplateColumns: "1fr 1fr 300px",
        gap: 12,
        alignItems: "start",
      }}>
        {/* Strategy Leaderboard */}
        <div className="terminal-card" style={{ padding: "16px 18px" }}>
          <SectionHeader
            title="Strategy Leaderboard"
            subtitle="Top performers by realized PnL"
          />
          {strategyLeaderboard.length === 0 ? (
            <div style={{ color: "var(--text-muted)", fontSize: 12, padding: "20px 0", textAlign: "center" }}>
              No closed trades yet
            </div>
          ) : (
            <>
              <div style={{
                display: "grid",
                gridTemplateColumns: "28px 1fr 90px 70px 60px",
                gap: 8,
                marginBottom: 4,
              }}>
                {["#", "Strategy", "PnL", "Win %", "Sharpe"].map((h) => (
                  <span key={h} style={{
                    fontSize: 10,
                    fontWeight: 700,
                    letterSpacing: "0.06em",
                    textTransform: "uppercase",
                    color: "var(--text-muted)",
                    textAlign: h === "#" ? "center" : h === "Strategy" ? "left" : "right",
                  }}>{h}</span>
                ))}
              </div>
              {strategyLeaderboard.map((s, i) => (
                <StrategyRow
                  key={s.id}
                  rank={i + 1}
                  name={s.name}
                  pnl={s.pnl}
                  winRate={s.winRate}
                  sharpe={null}
                />
              ))}
            </>
          )}
        </div>

        {/* Recent Signal Feed */}
        <div className="terminal-card" style={{ padding: "16px 18px" }}>
          <SectionHeader
            title="Signal Feed"
            subtitle="Latest engine events"
            actions={<LiveBadge />}
          />
          <div style={{ display: "flex", flexDirection: "column", gap: 0 }}>
            {recentSignals.length === 0 ? (
              <div style={{ color: "var(--text-muted)", fontSize: 12, padding: "20px 0", textAlign: "center" }}>
                No signals yet — waiting for price feed
              </div>
            ) : (
              recentSignals.map((entry: MockTradeLog, i: number) => {
                const isCreated = entry.event === "MOCK_TRADE_CREATED";
                const isTP = entry.event === "MOCK_TRADE_TP_HIT";
                const isSL = entry.event === "MOCK_TRADE_SL_HIT";
                const isRejected = entry.event === "MOCK_TRADE_REJECTED" || entry.event === "MOCK_TRADE_LIMIT_REACHED";

                const dotColor = isCreated || isTP
                  ? "var(--green)"
                  : isSL || isRejected
                    ? "var(--red)"
                    : "var(--text-muted)";

                return (
                  <div key={i} style={{
                    display: "flex",
                    gap: 10,
                    padding: "7px 0",
                    borderBottom: i < recentSignals.length - 1 ? "1px solid var(--border-subtle)" : "none",
                    alignItems: "flex-start",
                  }}>
                    <div style={{
                      width: 6,
                      height: 6,
                      borderRadius: "50%",
                      background: dotColor,
                      flexShrink: 0,
                      marginTop: 4,
                    }} />
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{
                        fontSize: 11,
                        color: "var(--text-primary)",
                        fontWeight: 500,
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        whiteSpace: "nowrap",
                      }}>
                        {entry.message}
                      </div>
                      <div style={{
                        fontSize: 10,
                        color: "var(--text-muted)",
                        fontFamily: "var(--font-mono)",
                        marginTop: 1,
                      }}>
                        {new Date(entry.ts).toLocaleTimeString()}
                      </div>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </div>

        {/* Right column: Status + Market */}
        <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          {/* System Status */}
          <div className="terminal-card" style={{ padding: "16px 18px" }}>
            <SectionHeader title="System Status" />
            <StatusRow
              label="BTC Feed"
              value={live.connected ? "Connected" : "Offline"}
              status={live.connected ? "ok" : "error"}
            />
            <StatusRow
              label="Database"
              value={engine.persistence.status === "mongo" ? "Mongo" : engine.persistence.status === "hydrating" ? "Syncing" : "Local"}
              status={engine.persistence.status === "mongo" ? "ok" : engine.persistence.status === "hydrating" ? "info" : "warn"}
            />
            <StatusRow
              label="Market Regime"
              value={regime.regime ?? "Unknown"}
              status="info"
            />
            <StatusRow
              label="Candles Built"
              value={`${candles.closedCount}`}
              status="ok"
            />
            <StatusRow
              label="Active Strategies"
              value={`${scoring.scores.length}`}
              status={scoring.scores.length > 0 ? "ok" : "warn"}
            />
          </div>

          {/* Market Stats */}
          <div className="terminal-card" style={{ padding: "16px 18px" }}>
            <SectionHeader title="Market" />
            <StatusRow label="BTC Price" value={`$${fmtPrice(live.price)}`} />
            <StatusRow
              label="24h Change"
              value={`${live.change24h >= 0 ? "+" : ""}${live.change24h.toFixed(2)}%`}
              status={live.change24h >= 0 ? "ok" : "error"}
            />
            <StatusRow label="24h High" value={`$${fmtPrice(live.high24h)}`} />
            <StatusRow label="24h Low"  value={`$${fmtPrice(live.low24h)}`} />
            <StatusRow label="Ticks/s"  value={`${live.ticksPerSecond}`} status="ok" />
          </div>

          {/* Trade Analytics */}
          <div className="terminal-card" style={{ padding: "16px 18px" }}>
            <SectionHeader title="Trade Analytics" />
            <StatusRow label="Total Trades" value={`${analytics.totalTrades}`} />
            <StatusRow label="Open"   value={`${analytics.openTrades}`}   status={analytics.openTrades > 0 ? "info" : "ok"} />
            <StatusRow label="Closed" value={`${analytics.closedTrades}`} />
            <StatusRow label="TP Wins"  value={`${analytics.takeProfitWins}`}  status="ok" />
            <StatusRow label="SL Losses" value={`${analytics.stopLossLosses}`} status={analytics.stopLossLosses > 0 ? "warn" : "ok"} />
            <StatusRow
              label="Avg Win"
              value={fmtUsd(analytics.averageWin, true)}
              status="ok"
            />
            <StatusRow
              label="Avg Loss"
              value={fmtUsd(analytics.averageLoss, true)}
              status="error"
            />
          </div>
        </div>
      </div>

      {/* ── Open Positions ────────────────────────────────────────────────── */}
      {openPositions.length > 0 && (
        <div className="terminal-card" style={{ padding: "16px 18px", marginTop: 12 }}>
          <SectionHeader
            title="Open Positions"
            subtitle={`${acct.openCount} active · ${fmtUsd(analytics.unrealizedPnl, true)} unrealized`}
          />
          <div style={{ overflowX: "auto" }}>
            <table className="raig-table">
              <thead>
                <tr>
                  {["Strategy", "Side", "Entry", "Unrealized PnL", "TP", "SL", "Age"].map((h) => (
                    <th key={h}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {openPositions.map((t) => {
                  const pnlColor = t.unrealizedPnl > 0
                    ? "var(--green)"
                    : t.unrealizedPnl < 0
                      ? "var(--red)"
                      : "var(--text-muted)";
                  const ageMs = Date.now() - t.openedAt;
                  const ageMin = Math.floor(ageMs / 60_000);
                  const ageStr = ageMin < 60
                    ? `${ageMin}m`
                    : `${Math.floor(ageMin / 60)}h ${ageMin % 60}m`;

                  return (
                    <tr key={t.id}>
                      <td>{t.strategyName}</td>
                      <td>
                        <span style={{
                          color: t.side === "BUY" ? "var(--green)" : "var(--red)",
                          fontWeight: 700,
                          fontSize: 11,
                          letterSpacing: "0.04em",
                        }}>
                          {t.side}
                        </span>
                      </td>
                      <td style={{ fontFamily: "var(--font-mono)", fontVariantNumeric: "tabular-nums" }}>
                        ${t.entryPrice.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                      </td>
                      <td style={{ fontFamily: "var(--font-mono)", fontVariantNumeric: "tabular-nums", color: pnlColor, fontWeight: 700 }}>
                        {t.unrealizedPnl >= 0 ? "+" : ""}${Math.abs(t.unrealizedPnl).toFixed(2)}
                      </td>
                      <td style={{ fontFamily: "var(--font-mono)", color: "var(--green)", fontSize: 11 }}>
                        +${t.takeProfitUsd.toFixed(2)}
                      </td>
                      <td style={{ fontFamily: "var(--font-mono)", color: "var(--red)", fontSize: 11 }}>
                        -${t.stopLossUsd.toFixed(2)}
                      </td>
                      <td style={{ fontFamily: "var(--font-mono)", color: "var(--text-muted)", fontSize: 11 }}>
                        {ageStr}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </AppShell>
  );
}
