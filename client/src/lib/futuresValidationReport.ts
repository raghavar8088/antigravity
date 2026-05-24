/**
 * futuresValidationReport.ts
 * Human-readable validation export for paper-to-live readiness.
 */

import type { AttributionReport } from "./futuresAttribution";
import type { HealthCheckResult } from "./futuresStrategyDiagnostics";
import type { GoLiveGateReport } from "./futuresGoLiveGates";
import type { ReadinessReport } from "./futuresProductionReadiness";
import type { SoakDaySnapshot } from "./futuresSoakTracker";
import { soakTrendSummary } from "./futuresSoakTracker";
import type { UnifiedReadiness } from "./futuresUnifiedReadiness";
import { unifiedReadinessLabel } from "./futuresUnifiedReadiness";

export function generateValidationReport(inputs: {
  gates: GoLiveGateReport;
  health: HealthCheckResult | null;
  readiness: ReadinessReport | null;
  attribution: AttributionReport | null;
  accountKey: string;
  generatedAt: number;
  unifiedReadiness?: UnifiedReadiness | null;
  unifiedBlockers?: string[];
  unifiedNextStep?: string;
  soakHistory?: SoakDaySnapshot[];
  replaySignFlipRate?: number | null;
}): string {
  const {
    gates,
    health,
    readiness,
    attribution,
    accountKey,
    generatedAt,
    unifiedReadiness,
    unifiedBlockers = [],
    unifiedNextStep,
    soakHistory = [],
    replaySignFlipRate,
  } = inputs;
  const lines: string[] = [];
  const sep = "─".repeat(50);

  lines.push("BTC FUTURES PAPER DESK — GO-LIVE VALIDATION REPORT");
  lines.push(new Date(generatedAt).toISOString());
  lines.push(`Account: ${accountKey}`);
  lines.push("READ-ONLY — does not enable live trading");
  lines.push(sep);

  lines.push("GO-LIVE GATE SUMMARY");
  lines.push(`  Recommendation: ${gates.recommendation}`);
  lines.push(`  Score:          ${(gates.score * 100).toFixed(0)}% (${gates.gates.filter((g) => g.pass).length}/${gates.gates.length} gates)`);
  lines.push(`  Production trades: ${gates.totalProduction}`);
  lines.push(`  Days of data:      ${gates.daysOfData.toFixed(1)}`);
  lines.push(`  Blockers failing:  ${gates.blockers.filter((g) => !g.pass).length}`);
  lines.push(`  Warnings failing: ${gates.warnings.filter((g) => !g.pass).length}`);
  lines.push(sep);

  const failedBlockers = gates.blockers.filter((g) => !g.pass);
  lines.push("BLOCKERS (must fix)");
  if (!failedBlockers.length) {
    lines.push("  ✓ All blocker gates pass");
  } else {
    for (const g of failedBlockers) {
      lines.push(`  ✗ [${g.id}] ${g.label}`);
      lines.push(`      value: ${g.value}  required: ${g.required}`);
    }
  }
  lines.push(sep);

  const failedWarnings = gates.warnings.filter((g) => !g.pass);
  lines.push("WARNINGS (review)");
  if (!failedWarnings.length) {
    lines.push("  ✓ No warning failures");
  } else {
    for (const g of failedWarnings) {
      lines.push(`  ⚠ [${g.id}] ${g.label}`);
      lines.push(`      value: ${g.value}  required: ${g.required}`);
    }
  }
  lines.push(sep);

  lines.push("PERFORMANCE SNAPSHOT");
  if (!health) {
    lines.push("  Awaiting health check (need closed trades)");
  } else {
    lines.push(`  Grade:       ${health.grade} (${health.window} trade window)`);
    lines.push(`  Expectancy:  $${health.expectancy.toFixed(2)}`);
    lines.push(`  Win rate:    ${(health.winRate * 100).toFixed(1)}%`);
    lines.push(`  Profit factor: ${health.profitFactor === Infinity ? "∞" : health.profitFactor.toFixed(2)}`);
    lines.push(`  Fee/gross:   ${(health.feePctOfAbsGross * 100).toFixed(1)}%`);
    lines.push(`  TIME exits:  ${health.timeCount}`);
  }
  lines.push(sep);

  lines.push("ATTRIBUTION SUMMARY");
  if (!attribution || attribution.totalAnalyzed < 10) {
    lines.push("  Insufficient trades for attribution");
  } else {
    lines.push(`  Trades analyzed: ${attribution.totalAnalyzed}`);
    lines.push(`  Best hold bucket: ${attribution.bestHoldBucket ?? "N/A"}`);
    lines.push(`  Worst hold bucket: ${attribution.worstHoldBucket ?? "N/A"}`);
    lines.push(
      `  Best UTC hour: ${attribution.bestHour != null ? `${attribution.bestHour}:00` : "N/A"}`,
    );
    const longE = attribution.bySide.find((s) => s.label === "LONG");
    const shortE = attribution.bySide.find((s) => s.label === "SHORT");
    lines.push(
      `  LONG avg: $${longE?.avgNetPnl.toFixed(2) ?? "N/A"}  SHORT avg: $${shortE?.avgNetPnl.toFixed(2) ?? "N/A"}`,
    );
  }
  lines.push(sep);

  lines.push("READINESS CHECK (runtime invariants)");
  if (!readiness) {
    lines.push("  Not computed");
  } else {
    lines.push(`  Production ready: ${readiness.productionReady ? "YES" : "NO"}`);
    lines.push(`  Score: ${(readiness.score * 100).toFixed(0)}%`);
    for (const c of readiness.criticalFails) {
      lines.push(`  ✗ ${c.label}: ${c.value} (need ${c.required})`);
    }
  }
  lines.push(sep);

  if (unifiedReadiness) {
    lines.push("UNIFIED READINESS");
    lines.push(`  State: ${unifiedReadinessLabel(unifiedReadiness)}`);
    if (unifiedBlockers.length) {
      for (const b of unifiedBlockers) {
        lines.push(`  ✗ ${b}`);
      }
    } else {
      lines.push("  ✓ No unified blockers");
    }
    if (unifiedNextStep) {
      lines.push(`  Next: ${unifiedNextStep}`);
    }
    lines.push(sep);
  }

  if (soakHistory.length) {
    const soak = soakTrendSummary(soakHistory);
    lines.push("SOAK 7-DAY SUMMARY");
    lines.push(`  Days tracked: ${soak.daysTracked}`);
    lines.push(`  Green days:   ${soak.greenDays}/7`);
    lines.push(`  Avg E (7d):   $${soak.avgExpectancy7d.toFixed(2)}`);
    lines.push(`  Improving:    ${soak.improving ? "yes" : "no"}`);
    for (const day of soakHistory.slice(-7)) {
      lines.push(
        `  ${day.dateUtc}  closes=${day.closes}  E=$${day.expectancy.toFixed(2)}  ` +
          `fee/gross=${day.feePctOfAbsGross.toFixed(1)}%  ${day.grade}`,
      );
    }
    lines.push(sep);
  }

  if (replaySignFlipRate != null) {
    lines.push("REPLAY SIGN-FLIP");
    lines.push(`  Rate: ${(replaySignFlipRate * 100).toFixed(1)}% (target ≤ 15%)`);
    lines.push(sep);
  } else {
    lines.push("REPLAY SIGN-FLIP");
    lines.push("  Not available — run replay:compare or set NEXT_PUBLIC_DESK_REPLAY_GATE=1");
    lines.push(sep);
  }

  lines.push("NEXT STEPS");
  const next: string[] = [];
  if (gates.recommendation === "COLLECT_MORE_DATA") {
    next.push(`Collect more closed trades (target >= 50 paper-ready, 200 full live). Currently ${gates.totalProduction}.`);
  }
  if (gates.recommendation === "NOT_READY") {
    for (const g of failedBlockers.slice(0, 5)) {
      next.push(`Fix blocker: ${g.label} (${g.value} vs ${g.required})`);
    }
  }
  if (gates.recommendation === "REVIEW_WARNINGS") {
    for (const g of failedWarnings.slice(0, 5)) {
      next.push(`Review warning: ${g.label}`);
    }
    next.push("Run npm run replay:compare on sample UTC days before mainnet.");
  }
  if (gates.recommendation === "PAPER_READY") {
    next.push("Paper gates pass — proceed to testnet soak (LIVE_TRADING_PHASE.md §2) before mainnet.");
    next.push("Document sign-off with Export CSV (30d) and this validation report.");
  }
  if (!next.length) {
    next.push("Continue monitoring desk metrics weekly.");
  }
  for (const n of next) {
    lines.push(`  • ${n}`);
  }
  lines.push(sep);
  lines.push("END OF VALIDATION REPORT");

  return lines.join("\n");
}
