"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { AppShell } from "@/components/terminal";
import useLiveBTCPrice from "@/hooks/useLiveBTCPrice";
import { useMockCandleBuilder } from "@/hooks/useMockCandleBuilder";
import { fetchOrders, fetchStrategyHealth, usePaperDesk } from "@/hooks/usePaperDesk";
import { useMarketRegime } from "@/hooks/useMarketRegime";
import { paperDeskHref } from "@/lib/navRoutes";
import type { PaperOrderDoc, PaperPositionDoc, StrategyScoreDoc } from "@/lib/paperDeskClient";

const STARTING_BALANCE = 1_000_000;

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
  sub?: React.ReactNode;
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

function PaperDeskLink({
  tab,
  children,
  style,
}: {
  tab?: "positions" | "trades" | "orders" | "equity" | "strategies";
  children: React.ReactNode;
  style?: React.CSSProperties;
}) {
  return (
    <Link
      href={paperDeskHref(tab)}
      style={{
        color: "var(--accent)",
        textDecoration: "none",
        fontSize: 11,
        fontWeight: 600,
        ...style,
      }}
    >
      {children}
    </Link>
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

function unrealizedPnlForPosition(pos: PaperPositionDoc, mark: number): number {
  if (!Number.isFinite(mark) || mark <= 0) return 0;
  const delta = pos.side === "SHORT" ? pos.entry_price - mark : mark - pos.entry_price;
  return delta * pos.size;
}

function orderFeedMessage(order: PaperOrderDoc): string {
  const side = order.side ? `${order.side} ` : "";
  const strat = order.strategy_id ? `${order.strategy_id}: ` : "";
  return `${strat}${side}${order.transition_to.replace(/_/g, " ").toLowerCase()}`;
}

export default function TerminalDashboard() {
  const live = useLiveBTCPrice();
  const desk = usePaperDesk();
  const candles = useMockCandleBuilder(live.price);
  const regime = useMarketRegime({ candles: candles.snapshot, newCandleReady: candles.newCandleReady });
  const [strategyScores, setStrategyScores] = useState<StrategyScoreDoc[]>([]);
  const [recentOrders, setRecentOrders] = useState<PaperOrderDoc[]>([]);

  useEffect(() => {
    if (desk.connection !== "live") return;
    let active = true;
    void Promise.all([fetchStrategyHealth(), fetchOrders(40)])
      .then(([health, orders]) => {
        if (!active) return;
        setStrategyScores(health.scores ?? []);
        setRecentOrders(orders ?? []);
      })
      .catch(() => {
        if (!active) return;
      });
    return () => {
      active = false;
    };
  }, [desk.connection, desk.lastUpdated]);

  const state = desk.state;
  const stateBalance = (state as { balance?: number } | null)?.balance;
  const equity = state?.equity ?? stateBalance ?? STARTING_BALANCE;
  const realizedPnl = state?.realized_pnl ?? (stateBalance != null ? stateBalance - STARTING_BALANCE : 0);
  const unrealizedPnl = state?.unrealized_pnl ?? 0;
  const totalPnl = state?.realized_pnl != null || state?.unrealized_pnl != null
    ? realizedPnl + unrealizedPnl
    : equity - STARTING_BALANCE;
  const winRate = state?.win_rate ?? 0;
  const totalTrades = state?.total_trades ?? desk.recentTrades.length;
  const openCount = state?.open_position_count ?? desk.openPositions.length;

  const profitFactor = useMemo(() => {
    const scored = strategyScores.filter((s) => s.sample_size > 0 && Number.isFinite(s.profit_factor));
    if (scored.length === 0) return null;
    return scored.reduce((sum, s) => sum + s.profit_factor, 0) / scored.length;
  }, [strategyScores]);

  const sharpeRatio = useMemo(() => {
    const scored = strategyScores.filter((s) => s.sample_size > 0 && Number.isFinite(s.expectancy));
    if (scored.length === 0) return null;
    const mean = scored.reduce((sum, s) => sum + s.expectancy, 0) / scored.length;
    const variance = scored.reduce((sum, s) => sum + (s.expectancy - mean) ** 2, 0) / scored.length;
    const std = Math.sqrt(variance);
    return std > 0 ? mean / std : null;
  }, [strategyScores]);

  const connectionStatus = live.connected ? "live" as const : "offline" as const;
  const persistenceStatus =
    desk.connection === "live"
      ? "mongo" as const
      : desk.connection === "connecting"
        ? "hydrating" as const
        : "local" as const;

  /* Strategy leaderboard — top 5 by realized PnL from engine scores */
  const strategyLeaderboard = useMemo(() => {
    if (strategyScores.length > 0) {
      return strategyScores
        .filter((s) => s.sample_size > 0)
        .slice(0, 5)
        .map((s) => ({
          id: s.strategy_id,
          name: s.strategy_id,
          pnl: s.total_pnl,
          winRate: s.win_rate,
          sharpe: s.expectancy,
        }));
    }

    const byStrategy = new Map<string, { trades: number; wins: number; totalPnl: number }>();
    for (const t of desk.recentTrades) {
      const existing = byStrategy.get(t.strategy_id) ?? { trades: 0, wins: 0, totalPnl: 0 };
      existing.trades++;
      existing.totalPnl += t.net_pnl;
      if (t.net_pnl > 0) existing.wins++;
      byStrategy.set(t.strategy_id, existing);
    }

    return [...byStrategy.entries()]
      .map(([id, d]) => ({
        id,
        name: id,
        pnl: d.totalPnl,
        winRate: d.trades > 0 ? d.wins / d.trades : 0,
        sharpe: null as number | null,
      }))
      .sort((a, b) => b.pnl - a.pnl)
      .slice(0, 5);
  }, [desk.recentTrades, strategyScores]);

  const recentSignals = useMemo(() => {
    if (recentOrders.length > 0) {
      return recentOrders.slice(0, 8).map((order) => ({
        ts: order.transition_at || order.recorded_at,
        message: orderFeedMessage(order),
        transition: order.transition_to,
      }));
    }
    return desk.recentTrades.slice(0, 8).map((trade) => ({
      ts: trade.closed_at || trade.exit_at,
      message: `${trade.strategy_id} ${trade.side} closed ${trade.exit_reason} ${fmtUsd(trade.net_pnl)}`,
      transition: trade.net_pnl >= 0 ? "POSITION_CLOSED_WIN" : "POSITION_CLOSED_LOSS",
    }));
  }, [desk.recentTrades, recentOrders]);

  const openPositions = useMemo(
    () => desk.openPositions.slice(0, 5),
    [desk.openPositions],
  );

  return (
    <AppShell
      btcPrice={live.price}
      btcChange24h={live.change24h}
      regime={regime.regime}
      dailyPnl={totalPnl}
      totalPnl={totalPnl}
      equity={equity}
      openPositions={openCount}
      paperDeskOpenPositions={openCount}
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
            value={`$${(equity / 1_000_000).toFixed(3)}M`}
            variant="accent"
            sub={`Started: $${(STARTING_BALANCE / 1_000_000).toFixed(1)}M`}
          />
          <KpiCard
            label="Total PnL"
            value={fmtUsd(totalPnl, true)}
            variant={totalPnl >= 0 ? "positive" : "negative"}
            change={fmtPct(totalPnl / STARTING_BALANCE)}
          />
          <KpiCard
            label="Realized PnL"
            value={fmtUsd(realizedPnl, true)}
            variant={realizedPnl >= 0 ? "positive" : "negative"}
          />
          <KpiCard
            label="Unrealized PnL"
            value={fmtUsd(unrealizedPnl, true)}
            variant={unrealizedPnl >= 0 ? "positive" : "negative"}
          />
          <KpiCard
            label="Win Rate"
            value={fmtPct(winRate)}
            variant={winRate >= 0.5 ? "positive" : "negative"}
            sub={`${totalTrades} total trades`}
          />
          <KpiCard
            label="Profit Factor"
            value={profitFactor != null ? profitFactor.toFixed(2) : "—"}
            variant={profitFactor != null && profitFactor >= 1.2 ? "positive" : "default"}
          />
          <KpiCard
            label="Sharpe Ratio"
            value={sharpeRatio != null ? sharpeRatio.toFixed(2) : "—"}
            variant={sharpeRatio != null && sharpeRatio >= 1 ? "positive" : "default"}
          />
          <KpiCard
            label="Open Positions"
            value={`${openCount}`}
            variant={openCount > 0 ? "info" : "default"}
            sub={
              desk.connection === "unauthorized"
                ? "Sign in to load desk data"
                : (
                  <PaperDeskLink tab="positions">Paper Desk live feed →</PaperDeskLink>
                )
            }
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
            actions={desk.connection !== "unauthorized" ? (
              <PaperDeskLink tab="trades">View trade history →</PaperDeskLink>
            ) : undefined}
          />
          {desk.connection === "unauthorized" ? (
            <div style={{ color: "var(--text-muted)", fontSize: 12, padding: "20px 0", textAlign: "center" }}>
              Sign in to load Paper Desk trade history
            </div>
          ) : strategyLeaderboard.length === 0 ? (
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
                  key={String(s.id)}
                  rank={i + 1}
                  name={s.name}
                  pnl={s.pnl}
                  winRate={s.winRate}
                  sharpe={s.sharpe ?? null}
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
            actions={
              desk.connection !== "unauthorized" ? (
                <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                  <PaperDeskLink tab="orders">OMS log →</PaperDeskLink>
                  <LiveBadge />
                </div>
              ) : (
                <LiveBadge />
              )
            }
          />
          <div style={{ display: "flex", flexDirection: "column", gap: 0 }}>
            {desk.connection === "unauthorized" ? (
              <div style={{ color: "var(--text-muted)", fontSize: 12, padding: "20px 0", textAlign: "center" }}>
                Sign in to load Paper Desk OMS events
              </div>
            ) : recentSignals.length === 0 ? (
              <div style={{ color: "var(--text-muted)", fontSize: 12, padding: "20px 0", textAlign: "center" }}>
                No engine events yet — waiting for Paper Desk activity
              </div>
            ) : (
              recentSignals.map((entry, i) => {
                const transition = entry.transition.toUpperCase();
                const isPositive =
                  transition.includes("OPENED") ||
                  transition.includes("FILL") ||
                  transition.includes("ACCEPTED") ||
                  transition.includes("WIN");
                const isNegative =
                  transition.includes("REJECTED") ||
                  transition.includes("CANCELLED") ||
                  transition.includes("LOSS");

                const dotColor = isPositive
                  ? "var(--green)"
                  : isNegative
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
              value={persistenceStatus === "mongo" ? "Mongo" : persistenceStatus === "hydrating" ? "Syncing" : "Local"}
              status={persistenceStatus === "mongo" ? "ok" : persistenceStatus === "hydrating" ? "info" : "warn"}
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
              value={`${desk.healthSummary?.total ?? strategyScores.length}`}
              status={(desk.healthSummary?.total ?? strategyScores.length) > 0 ? "ok" : "warn"}
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
            <StatusRow label="Total Trades" value={`${totalTrades}`} />
            <StatusRow label="Open" value={`${openCount}`} status={openCount > 0 ? "info" : "ok"} />
            <StatusRow label="Closed" value={`${Math.max(0, totalTrades - openCount)}`} />
            <StatusRow label="Wins" value={`${state?.winning_trades ?? 0}`} status="ok" />
            <StatusRow label="Losses" value={`${state?.losing_trades ?? 0}`} status={(state?.losing_trades ?? 0) > 0 ? "warn" : "ok"} />
            <StatusRow
              label="Total Fees"
              value={fmtUsd(state?.total_fees ?? 0, true)}
              status="warn"
            />
            <StatusRow
              label="Max Drawdown"
              value={fmtPct(state?.max_drawdown ?? 0)}
              status={(state?.max_drawdown ?? 0) > 0 ? "error" : "ok"}
            />
          </div>
        </div>
      </div>

      {/* ── Open Positions ────────────────────────────────────────────────── */}
      {openPositions.length > 0 && (
        <div className="terminal-card" style={{ padding: "16px 18px", marginTop: 12 }}>
          <SectionHeader
            title="Open Positions"
            subtitle={`${openCount} active · ${fmtUsd(unrealizedPnl, true)} unrealized`}
            actions={<PaperDeskLink tab="positions">View all in Paper Desk →</PaperDeskLink>}
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
                  const markPnl = unrealizedPnlForPosition(t, live.price);
                  const pnlColor = markPnl > 0
                    ? "var(--green)"
                    : markPnl < 0
                      ? "var(--red)"
                      : "var(--text-muted)";
                  const openedAt = new Date(t.opened_at).getTime();
                  const ageMs = Number.isFinite(openedAt) ? Date.now() - openedAt : 0;
                  const ageMin = Math.floor(ageMs / 60_000);
                  const ageStr = ageMin < 60
                    ? `${ageMin}m`
                    : `${Math.floor(ageMin / 60)}h ${ageMin % 60}m`;

                  return (
                    <tr key={t.position_id}>
                      <td>{t.strategy_id}</td>
                      <td>
                        <span style={{
                          color: t.side === "LONG" ? "var(--green)" : "var(--red)",
                          fontWeight: 700,
                          fontSize: 11,
                          letterSpacing: "0.04em",
                        }}>
                          {t.side}
                        </span>
                      </td>
                      <td style={{ fontFamily: "var(--font-mono)", fontVariantNumeric: "tabular-nums" }}>
                        ${t.entry_price.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                      </td>
                      <td style={{ fontFamily: "var(--font-mono)", fontVariantNumeric: "tabular-nums", color: pnlColor, fontWeight: 700 }}>
                        {markPnl >= 0 ? "+" : ""}${Math.abs(markPnl).toFixed(2)}
                      </td>
                      <td style={{ fontFamily: "var(--font-mono)", color: "var(--green)", fontSize: 11 }}>
                        {t.take_profit ? `$${t.take_profit.toLocaleString("en-US", { maximumFractionDigits: 0 })}` : "—"}
                      </td>
                      <td style={{ fontFamily: "var(--font-mono)", color: "var(--red)", fontSize: 11 }}>
                        {t.stop_loss ? `$${t.stop_loss.toLocaleString("en-US", { maximumFractionDigits: 0 })}` : "—"}
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
