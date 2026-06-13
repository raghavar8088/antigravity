"use client";

import { usePathname } from "next/navigation";
import { isMockTradingRoute, isPaperDeskRoute, MOCK_TRADING_PATH, TERMINAL_ROUTES } from "@/lib/utils/navRoutes";

type Regime = string | null | undefined;

type TopBarProps = {
  btcPrice?: number;
  btcChange24h?: number;
  regime?: Regime;
  dailyPnl?: number;
  weeklyPnl?: number;
  monthlyPnl?: number;
  totalPnl?: number;
  equity?: number;
  openPositions?: number;
  activeStrategies?: number;
  connectionStatus?: "live" | "reconnecting" | "offline";
  persistenceStatus?: "mongo" | "hydrating" | "local";
  riskStatus?: "safe" | "warning" | "danger";
  systemStatus?: "nominal" | "degraded" | "error";
  researchStatus?: "active" | "idle" | "paused";
  dataFeedStatus?: "synced" | "delayed" | "stale";
  title?: string;
  onMenuToggle?: () => void;
  menuOpen?: boolean;
};

const PAGE_TITLES: Record<string, string> = {
  "/":                   "Command Center",
  "/terminal":           "Command Center",
  "/mock-trading":       "Mock Trading",
  "/btc-future-trading": "BTC Futures",
};

function fmtPrice(v?: number): string {
  if (!v || !Number.isFinite(v) || v <= 0) return "—";
  return v.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function fmtUsd(v?: number, compact = false): string {
  if (v == null || !Number.isFinite(v)) return "—";
  const abs = Math.abs(v);
  const sign = v < 0 ? "-" : v > 0 ? "+" : "";
  if (compact && abs >= 1_000_000) return `${sign}$${(abs / 1_000_000).toFixed(2)}M`;
  if (compact && abs >= 1_000)    return `${sign}$${(abs / 1_000).toFixed(1)}K`;
  return `${sign}$${abs.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

function regimeClass(regime: Regime): string {
  if (!regime) return "unknown";
  const r = regime.toLowerCase();
  if (r.includes("bull") || r.includes("up") || r.includes("trend_up")) return "bull";
  if (r.includes("bear") || r.includes("down") || r.includes("trend_down")) return "bear";
  if (r.includes("range") || r.includes("neutral") || r.includes("sideways")) return "range";
  return "unknown";
}

function regimeLabel(regime: Regime): string {
  if (!regime) return "Unknown";
  return regime.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

function StatusDot({ status, type }: { status: string, type: "risk" | "system" | "research" | "data" }) {
  const getColors = () => {
    if (type === "risk") {
      if (status === "safe") return { color: "var(--green)", bg: "var(--green-dim)" };
      if (status === "warning") return { color: "var(--amber)", bg: "var(--amber-dim)" };
      return { color: "var(--red)", bg: "var(--red-dim)" };
    }
    if (type === "system") {
      if (status === "nominal") return { color: "var(--green)", bg: "var(--green-dim)" };
      if (status === "degraded") return { color: "var(--amber)", bg: "var(--amber-dim)" };
      return { color: "var(--red)", bg: "var(--red-dim)" };
    }
    if (type === "research") {
      if (status === "active") return { color: "var(--accent)", bg: "var(--accent-dim)" };
      if (status === "idle") return { color: "var(--text-muted)", bg: "var(--surface-3)" };
      return { color: "var(--amber)", bg: "var(--amber-dim)" };
    }
    if (type === "data") {
      if (status === "synced") return { color: "var(--green)", bg: "var(--green-dim)" };
      if (status === "delayed") return { color: "var(--amber)", bg: "var(--amber-dim)" };
      return { color: "var(--red)", bg: "var(--red-dim)" };
    }
    return { color: "var(--text-muted)", bg: "var(--surface-3)" };
  };

  const { color, bg } = getColors();

  return (
    <div className="topbar-status-item" style={{ background: bg, border: `1px solid ${color}33` }}>
      <span style={{ width: 4, height: 4, borderRadius: "50%", background: color }} />
      <span style={{ color, fontSize: 9, fontWeight: 700, textTransform: "uppercase" }}>{status}</span>
    </div>
  );
}

export function TopBar({
  btcPrice,
  btcChange24h,
  regime,
  dailyPnl,
  weeklyPnl,
  monthlyPnl,
  totalPnl,
  equity,
  openPositions,
  activeStrategies,
  connectionStatus = "offline",
  persistenceStatus,
  riskStatus = "safe",
  systemStatus = "nominal",
  researchStatus = "idle",
  dataFeedStatus = "synced",
  title,
  onMenuToggle,
  menuOpen = false,
}: TopBarProps) {
  const pathname = usePathname();
  const pageTitle =
    title ??
    (isPaperDeskRoute(pathname)
      ? "Mock Trading"
      : isMockTradingRoute(pathname)
        ? PAGE_TITLES[MOCK_TRADING_PATH]
        : PAGE_TITLES[pathname]) ??
    "Terminal";
  const isLive = connectionStatus === "live";
  const isReconnecting = connectionStatus === "reconnecting";

  return (
    <header className="terminal-topbar" role="banner">
      {onMenuToggle ? (
        <button
          type="button"
          className="topbar-menu-button"
          aria-label={menuOpen ? "Close navigation menu" : "Open navigation menu"}
          aria-expanded={menuOpen}
          onClick={onMenuToggle}
        >
          <span aria-hidden="true" />
          <span aria-hidden="true" />
          <span aria-hidden="true" />
        </button>
      ) : null}
      {/* Page title */}
      <div className="topbar-section">
        <span style={{
          fontFamily: "var(--font-display)",
          fontWeight: 700,
          fontSize: 13,
          color: "var(--text-primary)",
          letterSpacing: "-0.2px",
        }}>
          {pageTitle}
        </span>
      </div>

      <div className="topbar-divider" />

      {/* BTC Price block */}
      <div className="topbar-price-block">
        <span style={{
          fontSize: 10,
          fontWeight: 600,
          color: "var(--gold)",
          letterSpacing: "0.04em",
        }}>BTC</span>
        <span className="topbar-btc-price">
          ${fmtPrice(btcPrice)}
        </span>
        {btcChange24h != null && (
          <span className={`topbar-btc-change ${btcChange24h >= 0 ? "positive" : "negative"}`}>
            {btcChange24h >= 0 ? "+" : ""}{btcChange24h.toFixed(2)}%
          </span>
        )}
      </div>

      <div className="topbar-divider" />

      {/* Market Regime */}
      <div className="topbar-section">
        <span className={`topbar-regime-badge ${regimeClass(regime)}`}>
          {regimeLabel(regime)}
        </span>
      </div>

      <div className="topbar-divider" />

      {/* Daily PnL */}
      {dailyPnl != null && (
        <div className="topbar-metric">
          <span className="topbar-metric-label">Day PnL</span>
          <span className={`topbar-metric-value ${dailyPnl >= 0 ? "positive" : "negative"}`}>
            {fmtUsd(dailyPnl, true)}
          </span>
        </div>
      )}

      {/* Weekly PnL */}
      {weeklyPnl != null && (
        <div className="topbar-metric">
          <span className="topbar-metric-label">Week PnL</span>
          <span className={`topbar-metric-value ${weeklyPnl >= 0 ? "positive" : "negative"}`}>
            {fmtUsd(weeklyPnl, true)}
          </span>
        </div>
      )}

      {/* Monthly PnL */}
      {monthlyPnl != null && (
        <div className="topbar-metric">
          <span className="topbar-metric-label">Month PnL</span>
          <span className={`topbar-metric-value ${monthlyPnl >= 0 ? "positive" : "negative"}`}>
            {fmtUsd(monthlyPnl, true)}
          </span>
        </div>
      )}

      {/* Open Positions */}
      {openPositions != null && (
        <div className="topbar-metric">
          <span className="topbar-metric-label">Positions</span>
          <span className="topbar-metric-value">
            {openPositions}
          </span>
        </div>
      )}

      {/* Active Strategies */}
      {activeStrategies != null && (
        <div className="topbar-metric">
          <span className="topbar-metric-label">Strats</span>
          <span className="topbar-metric-value">
            {activeStrategies}
          </span>
        </div>
      )}

      {/* Equity */}
      {equity != null && (
        <div className="topbar-metric">
          <span className="topbar-metric-label">Equity</span>
          <span className="topbar-metric-value accent">
            {fmtUsd(equity, true)}
          </span>
        </div>
      )}

      <div className="topbar-spacer" />

      {/* Status Group */}
      <div style={{ display: "flex", gap: 6, alignItems: "center", marginRight: 8 }}>
        <StatusDot status={riskStatus} type="risk" />
        <StatusDot status={systemStatus} type="system" />
        <StatusDot status={researchStatus} type="research" />
        <StatusDot status={dataFeedStatus} type="data" />
      </div>

      {/* Persistence */}
      {persistenceStatus && (
        <span style={{
          fontSize: 10,
          fontWeight: 600,
          color: persistenceStatus === "mongo"
            ? "var(--green)"
            : persistenceStatus === "hydrating"
              ? "var(--info)"
              : "var(--amber)",
          background: persistenceStatus === "mongo"
            ? "var(--green-dim)"
            : persistenceStatus === "hydrating"
              ? "var(--info-dim)"
              : "var(--amber-dim)",
          border: `1px solid ${persistenceStatus === "mongo"
            ? "rgba(38,166,154,0.2)"
            : persistenceStatus === "hydrating"
              ? "rgba(88,166,255,0.2)"
              : "rgba(255,179,0,0.2)"}`,
          padding: "2px 7px",
          borderRadius: 3,
          letterSpacing: "0.03em",
        }}>
          {persistenceStatus === "mongo"
            ? "Mongo"
            : persistenceStatus === "hydrating"
              ? "Syncing"
              : "Local"}
        </span>
      )}

      {/* Connection Status */}
      <div className={`topbar-connection ${isLive ? "live" : "offline"}`}>
        <span style={{
          width: 5,
          height: 5,
          borderRadius: "50%",
          background: isLive ? "var(--green)" : isReconnecting ? "var(--amber)" : "var(--red)",
          display: "inline-block",
          animation: isLive ? "pulse-green 2s infinite" : isReconnecting ? "pulse-red 2s infinite" : "none",
        }} />
        {isLive ? "LIVE" : isReconnecting ? "RECONNECT" : "OFFLINE"}
      </div>
    </header>
  );
}
