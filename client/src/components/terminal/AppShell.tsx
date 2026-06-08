"use client";

import { useEffect, useState, type ReactNode } from "react";
import { usePathname } from "next/navigation";
import { Sidebar } from "./Sidebar";
import { TopBar } from "./TopBar";

type AppShellProps = {
  children: ReactNode;
  /** Live BTC price in USD */
  btcPrice?: number;
  /** 24h price change percent */
  btcChange24h?: number;
  /** Current market regime string */
  regime?: string | null;
  /** Daily realized + unrealized PnL */
  dailyPnl?: number;
  /** Weekly PnL */
  weeklyPnl?: number;
  /** Monthly PnL */
  monthlyPnl?: number;
  /** Cumulative total PnL */
  totalPnl?: number;
  /** Account equity */
  equity?: number;
  /** Number of open positions */
  openPositions?: number;
  /** Paper Desk open positions (sidebar badge) */
  paperDeskOpenPositions?: number;
  /** Number of active strategies */
  activeStrategies?: number;
  /** WebSocket connection status */
  connectionStatus?: "live" | "reconnecting" | "offline";
  /** Database persistence status */
  persistenceStatus?: "mongo" | "hydrating" | "local";
  /** Risk status */
  riskStatus?: "safe" | "warning" | "danger";
  /** System status */
  systemStatus?: "nominal" | "degraded" | "error";
  /** Research status */
  researchStatus?: "active" | "idle" | "paused";
  /** Data feed status */
  dataFeedStatus?: "synced" | "delayed" | "stale";
  /** Override the page title shown in topbar */
  pageTitle?: string;
};

export function AppShell({
  children,
  btcPrice,
  btcChange24h,
  regime,
  dailyPnl,
  weeklyPnl,
  monthlyPnl,
  totalPnl,
  equity,
  openPositions,
  paperDeskOpenPositions,
  activeStrategies,
  connectionStatus = "offline",
  persistenceStatus,
  riskStatus,
  systemStatus,
  researchStatus,
  dataFeedStatus,
  pageTitle,
}: AppShellProps) {
  const pathname = usePathname();
  const [mobileNavOpen, setMobileNavOpen] = useState(false);

  useEffect(() => {
    setMobileNavOpen(false);
  }, [pathname]);

  return (
    <div className="terminal-shell">
      {mobileNavOpen ? (
        <button
          type="button"
          className="sidebar-mobile-backdrop"
          aria-label="Close navigation menu"
          onClick={() => setMobileNavOpen(false)}
        />
      ) : null}
      <Sidebar
        liveConnected={connectionStatus === "live"}
        openPositions={openPositions}
        paperDeskOpenPositions={paperDeskOpenPositions}
        mobileOpen={mobileNavOpen}
        onNavigate={() => setMobileNavOpen(false)}
      />

      <div className="terminal-main">
        <TopBar
          btcPrice={btcPrice}
          btcChange24h={btcChange24h}
          regime={regime}
          dailyPnl={dailyPnl}
          weeklyPnl={weeklyPnl}
          monthlyPnl={monthlyPnl}
          totalPnl={totalPnl}
          equity={equity}
          openPositions={openPositions}
          activeStrategies={activeStrategies}
          connectionStatus={connectionStatus}
          persistenceStatus={persistenceStatus}
          riskStatus={riskStatus}
          systemStatus={systemStatus}
          researchStatus={researchStatus}
          dataFeedStatus={dataFeedStatus}
          title={pageTitle}
          onMenuToggle={() => setMobileNavOpen((open) => !open)}
          menuOpen={mobileNavOpen}
        />

        <main className="terminal-content">
          {children}
        </main>
      </div>
    </div>
  );
}
