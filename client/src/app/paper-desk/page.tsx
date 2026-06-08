import type { Metadata } from "next";
import nextDynamic from "next/dynamic";
import { Suspense } from "react";

export const metadata: Metadata = {
  title: "Paper Desk — Go Engine",
  description: "Live Go Engine paper-trading account: positions, trades, OMS, equity, strategy health.",
};

export const dynamic = "force-dynamic";

const PaperDeskDashboard = nextDynamic(() => import("@/components/PaperDeskDashboard"), {
  loading: () => (
    <div className="terminal-content" style={{ padding: 24, color: "var(--text-muted)" }}>
      Loading Paper Desk…
    </div>
  ),
});

export default function PaperDeskPage() {
  return (
    <Suspense
      fallback={(
        <div className="terminal-content" style={{ padding: 24, color: "var(--text-muted)" }}>
          Loading Paper Desk…
        </div>
      )}
    >
      <PaperDeskDashboard />
    </Suspense>
  );
}
