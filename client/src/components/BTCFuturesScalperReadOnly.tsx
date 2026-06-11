"use client";

import Link from "next/link";
import { usePaperDesk } from "@/hooks/usePaperDesk";
import useLiveBTCPrice from "@/hooks/useLiveBTCPrice";
import { DeskBanner, DeskCard, DeskMetricTile, DeskShell } from "@/components/desk/ui";

type Props = {
  title?: string;
};

export function BTCFuturesScalperReadOnly({ title = "Future Trading" }: Props) {
  const desk = usePaperDesk();
  const live = useLiveBTCPrice();
  const state = desk.state;
  const balance = state?.balance;
  const equity = state?.equity;

  return (
    <DeskShell>
      <DeskBanner variant="warning">
        Paper desk execution disabled — Go engine is sole execution authority. This view is read-only.
      </DeskBanner>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
        <h1 style={{ fontSize: 20, fontWeight: 700, margin: 0 }}>{title}</h1>
        <Link href="/paper-desk" style={{ color: "var(--desk-primary)", fontWeight: 600, fontSize: 13 }}>
          View Paper Desk Dashboard →
        </Link>
      </div>
      {desk.connection !== "live" ? (
        <DeskCard>
          <p style={{ fontFamily: "monospace", color: "var(--desk-error)" }}>
            BACKEND AUTHORITY UNAVAILABLE — cannot load paper desk snapshot
          </p>
        </DeskCard>
      ) : (
        <DeskCard>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))", gap: 12 }}>
            <DeskMetricTile label="BTC Mark" value={live.price > 0 ? `$${live.price.toLocaleString()}` : "—"} />
            <DeskMetricTile label="Balance" value={typeof balance === "number" ? `$${balance.toLocaleString()}` : "—"} />
            <DeskMetricTile label="Equity" value={typeof equity === "number" ? `$${equity.toLocaleString()}` : "—"} />
            <DeskMetricTile label="Open Positions" value={String(desk.openPositions.length)} />
            <DeskMetricTile label="Recent Trades" value={String(desk.recentTrades.length)} />
          </div>
        </DeskCard>
      )}
    </DeskShell>
  );
}
