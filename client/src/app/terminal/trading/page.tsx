"use client";

import { LivePnlFeed } from "@/components/trading/LivePnlFeed";
import { OrderBlotter } from "@/components/trading/OrderBlotter";
import { PositionManager } from "@/components/trading/PositionManager";
import { EventStreamViewer } from "@/components/trading/EventStreamViewer";
import { ReconciliationStatus } from "@/components/trading/ReconciliationStatus";
import { SessionKPIs } from "@/components/trading/SessionKPIs";

export default function TradingDashboardPage() {
  return (
    <div
      className="h-[calc(100vh-96px)] grid gap-2 p-3"
      style={{
        gridTemplateColumns: "1fr 1fr 280px",
        gridTemplateRows: "220px 1fr 220px",
      }}
    >
      {/* Row 1 */}
      <div
        className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] overflow-hidden"
        style={{ gridColumn: "1", gridRow: "1" }}
      >
        <LivePnlFeed />
      </div>

      <div
        className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] overflow-hidden"
        style={{ gridColumn: "2", gridRow: "1" }}
      >
        <PositionManager />
      </div>

      <div
        className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] overflow-hidden"
        style={{ gridColumn: "3", gridRow: "1" }}
      >
        <SessionKPIs />
      </div>

      {/* Row 2 — Order Blotter full width */}
      <div
        className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] overflow-hidden"
        style={{ gridColumn: "1 / 4", gridRow: "2" }}
      >
        <div className="h-full flex flex-col">
          <div className="px-3 py-2 border-b border-[var(--color-border)]">
            <h2 className="text-[12px] font-semibold text-[var(--color-text-primary)]">Order Blotter — OMS v3</h2>
          </div>
          <div className="flex-1 overflow-hidden">
            <OrderBlotter />
          </div>
        </div>
      </div>

      {/* Row 3 */}
      <div
        className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] overflow-hidden"
        style={{ gridColumn: "1 / 3", gridRow: "3" }}
      >
        <EventStreamViewer />
      </div>

      <div
        className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] overflow-hidden"
        style={{ gridColumn: "3", gridRow: "3" }}
      >
        <ReconciliationStatus />
      </div>
    </div>
  );
}
