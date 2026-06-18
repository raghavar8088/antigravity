"use client";

import { PageHeader } from "@/components/ui/PageHeader";
import { DailyPnLTable } from "@/components/DailyPnLTable";
import { MockStageTradingSuite } from "./MockStageTradingSuite";

export function TradeEngineCenter() {
  return (
    <div className="google-page">
      <PageHeader title="Trade Engine" subtitle="Live BTC paper trading engine" />
      <MockStageTradingSuite
        status="MAIN_ENGINE"
        showPipeline={false}
        renderShell={({ summaryStrip, leftPanel, rightPanel }) => (
          <>
            {summaryStrip}
            <div
              className="icc-trade-engine-grid"
              style={{
                display: "grid",
                gridTemplateColumns: "3fr 2fr",
                gap: 18,
                minHeight: 0,
              }}
            >
              <div
                style={{
                  display: "grid",
                  alignContent: "start",
                  gap: 18,
                  minHeight: 0,
                }}
              >
                {leftPanel}
              </div>
              <div
                style={{
                  display: "grid",
                  alignContent: "start",
                  gap: 18,
                  minHeight: 0,
                }}
              >
                {rightPanel}
              </div>
            </div>
            <DailyPnLTable className="mt-6" />
          </>
        )}
      />
    </div>
  );
}
