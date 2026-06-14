"use client";

import { MockStageTradingSuite } from "./MockStageTradingSuite";

export function TradeEngineCenter() {
  return (
    <div
      style={{
        display: "grid",
        gridTemplateRows: "auto 1fr",
        gridTemplateColumns: "1fr",
        height: "calc(100vh - var(--topbar-h, 48px) - var(--risk-ribbon-h, 0px))",
        minHeight: 0,
        overflow: "hidden",
      }}
    >
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
                gap: 12,
                overflow: "hidden",
                minHeight: 0,
                padding: 12,
              }}
            >
              <div
                style={{
                  display: "grid",
                  alignContent: "start",
                  gap: 12,
                  minHeight: 0,
                  overflowY: "auto",
                  paddingRight: 2,
                }}
              >
                {leftPanel}
              </div>
              <div
                style={{
                  display: "grid",
                  alignContent: "start",
                  gap: 12,
                  minHeight: 0,
                  overflowY: "auto",
                  paddingRight: 2,
                }}
              >
                {rightPanel}
              </div>
            </div>
          </>
        )}
      />
    </div>
  );
}
