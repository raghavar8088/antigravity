"use client";

import { useState } from "react";
import type { DeskRollingPnLScorecard } from "@/lib/futuresDeskPnLTracker";
import { profitModeExitConfig, type ProfitModeConfig } from "@/lib/futuresProfitMode";

interface Props {
  scorecard: DeskRollingPnLScorecard | null;
  profitMode: ProfitModeConfig;
  profitModeSkipCount: number;
  sessionGateOn: boolean;
  allocationByEdgeOn: boolean;
}

function passChip(pass: boolean, label: string) {
  return (
    <span
      style={{
        fontSize: 10,
        padding: "2px 8px",
        borderRadius: 4,
        background: pass ? "#23863633" : "#f8514933",
        color: pass ? "#3fb950" : "#f85149",
        border: `1px solid ${pass ? "#3fb95055" : "#f8514955"}`,
      }}
    >
      {pass ? "✓" : "✗"} {label}
    </span>
  );
}

export function DeskPnLScorecardPanel({
  scorecard,
  profitMode,
  profitModeSkipCount,
  sessionGateOn,
  allocationByEdgeOn,
}: Props) {
  const [expanded, setExpanded] = useState(true);
  const exitCfg = profitModeExitConfig(profitMode);

  if (!scorecard && !profitMode.enabled) return null;

  const hintColor =
    scorecard?.paperReadyHint === "ON_TRACK"
      ? "#3fb950"
      : scorecard?.paperReadyHint === "REVIEW"
        ? "#d29922"
        : "#8b949e";

  return (
    <div
      style={{
        border: "1px solid #21262d",
        borderRadius: 8,
        padding: "8px 12px",
        marginTop: 8,
        background: "#0d1117",
        fontSize: 11,
        color: "#e6edf3",
      }}
    >
      <div
        style={{ display: "flex", justifyContent: "space-between", cursor: "pointer" }}
        onClick={() => setExpanded((e) => !e)}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            setExpanded((v) => !v);
          }
        }}
      >
        <span style={{ fontWeight: 700, color: hintColor }}>
          📊 48h PnL scorecard
          {scorecard ? ` — ${scorecard.paperReadyHint.replace(/_/g, " ")}` : ""}
        </span>
        <span style={{ color: "#8b949e" }}>{expanded ? "▲" : "▼"}</span>
      </div>

      {profitMode.enabled ? (
        <div style={{ marginTop: 6, display: "flex", flexWrap: "wrap", gap: 6 }}>
          <span style={{ fontSize: 10, color: "#58a6ff" }}>Profit mode ON</span>
          <span style={{ fontSize: 10, color: "#8b949e" }}>entry skips {profitModeSkipCount}</span>
          {sessionGateOn ? (
            <span style={{ fontSize: 10, color: "#3fb950" }}>session gate</span>
          ) : null}
          {allocationByEdgeOn ? (
            <span style={{ fontSize: 10, color: "#3fb950" }}>alloc-by-edge</span>
          ) : null}
          {exitCfg ? (
            <span style={{ fontSize: 10, color: "#d29922" }}>
              exits: lock≥{exitCfg.profitLockMinProgress} net≥${exitCfg.profitLockMinNetUsd} gross≥
              {exitCfg.minGrossMultipleOfFees}×fees
              {exitCfg.disablePaperQuickTp ? " · no quick-TP" : ""}
            </span>
          ) : null}
        </div>
      ) : null}

      {expanded && scorecard ? (
        <div style={{ marginTop: 10 }}>
          <div style={{ fontSize: 10, color: "#8b949e", marginBottom: 8 }}>
            {scorecard.closes48h} closes in 48h · targets: fee/|gross| &lt; {scorecard.targets.feePctMax}% · E
            &gt; $0 · WR ≥ {(scorecard.targets.winRateMin * 100).toFixed(0)}% or PF ≥{" "}
            {scorecard.targets.profitFactorMin}
          </div>

          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12, marginBottom: 10 }}>
            <div>
              <div style={{ fontWeight: 600, marginBottom: 4 }}>Last {scorecard.last20.tradeCount}/20</div>
              <div style={{ fontSize: 10, color: "#8b949e" }}>
                E ${scorecard.last20.expectancy.toFixed(2)} · WR{" "}
                {(scorecard.last20.winRate * 100).toFixed(0)}% · PF{" "}
                {scorecard.last20.profitFactor === Infinity
                  ? "∞"
                  : scorecard.last20.profitFactor.toFixed(2)}{" "}
                · fee/gross {scorecard.last20.feePctOfAbsGross.toFixed(1)}%
              </div>
            </div>
            <div>
              <div style={{ fontWeight: 600, marginBottom: 4 }}>Last {scorecard.last50.tradeCount}/50</div>
              <div style={{ fontSize: 10, color: "#8b949e" }}>
                E ${scorecard.last50.expectancy.toFixed(2)} · WR{" "}
                {(scorecard.last50.winRate * 100).toFixed(0)}% · PF{" "}
                {scorecard.last50.profitFactor === Infinity
                  ? "∞"
                  : scorecard.last50.profitFactor.toFixed(2)}{" "}
                · fee/gross {scorecard.last50.feePctOfAbsGross.toFixed(1)}%
              </div>
            </div>
          </div>

          <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
            {passChip(scorecard.passesFeeTarget50, `fee/gross < ${scorecard.targets.feePctMax}% (50)`)}
            {passChip(scorecard.passesExpectancyTarget50, "E > $0 (50)")}
            {passChip(scorecard.passesWinRateOrPf50, "WR≥35% or PF≥1 (50)")}
          </div>
        </div>
      ) : expanded ? (
        <div style={{ marginTop: 8, fontSize: 10, color: "#8b949e" }}>
          Need ≥5 production closes for rolling scorecard.
        </div>
      ) : null}
    </div>
  );
}
