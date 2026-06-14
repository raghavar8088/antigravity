"use client";

import { useState } from "react";
import type { StrategyStatus } from "@/lib/strategyAuthority/types";
import { MockStageTradingSuite } from "./MockStageTradingSuite";

type TradeEngineTabKey = "main" | StrategyStatus;

const TRADE_ENGINE_TABS: { key: TradeEngineTabKey; label: string }[] = [
  { key: "main", label: "Main Engine" },
  { key: "GRADE_5", label: "Grade 5" },
  { key: "GRADE_4", label: "Grade 4" },
  { key: "GRADE_3", label: "Grade 3" },
  { key: "GRADE_2", label: "Grade 2" },
  { key: "GRADE_1", label: "Grade 1" },
];

export function TradeEngineCenter() {
  const [activeTab, setActiveTab] = useState<TradeEngineTabKey>("main");
  const activeStatus: StrategyStatus = activeTab === "main" ? "MAIN_ENGINE" : activeTab;

  return (
    <div
      style={{
        display: "grid",
        gridTemplateRows: "auto auto 1fr",
        gridTemplateColumns: "1fr",
        height: "calc(100vh - var(--topbar-h, var(--topbar-height, 48px)) - var(--risk-ribbon-h, 0px))",
        minHeight: 0,
        overflow: "hidden",
      }}
    >
      <div
        role="tablist"
        aria-label="Trade engine stages"
        style={{
          display: "flex",
          flexWrap: "nowrap",
          overflowX: "auto",
          background: "var(--surface-2)",
          borderBottom: "1px solid var(--border)",
          minHeight: 44,
        }}
      >
        {TRADE_ENGINE_TABS.map((tab) => {
          const active = tab.key === activeTab;
          return (
            <button
              key={tab.key}
              type="button"
              className="icc-tab"
              role="tab"
              aria-selected={active}
              onClick={() => setActiveTab(tab.key)}
              style={{
                border: 0,
                borderBottom: active ? "2px solid var(--accent)" : "2px solid transparent",
                background: "transparent",
                color: active ? "var(--text-primary)" : "var(--text-secondary)",
                cursor: "pointer",
                fontSize: 13,
                fontWeight: active ? 700 : 600,
                padding: "12px 16px 10px",
              }}
              onMouseEnter={(event) => {
                if (!active) event.currentTarget.style.color = "var(--text-primary)";
              }}
              onMouseLeave={(event) => {
                if (!active) event.currentTarget.style.color = "var(--text-secondary)";
              }}
            >
              {tab.label}
            </button>
          );
        })}
      </div>

      <MockStageTradingSuite
        status={activeStatus}
        showPipeline
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
