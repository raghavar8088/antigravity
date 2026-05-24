/**
 * futuresHealthReport.ts
 * Pure function. Generates a human-readable text health report.
 * Used in console output, copy-to-clipboard, and future email/Slack.
 */

import type {
  DiagnosticSummary,
  HealthCheckResult,
} from "./futuresStrategyDiagnostics";
import type { RotationReport } from "./futuresStrategyRotation";
import type { TuneRecommendation } from "./futuresParameterTuner";
import type { ReadinessReport } from "./futuresProductionReadiness";

export interface HealthReportInputs {
  health: HealthCheckResult | null;
  diagnostics: DiagnosticSummary | null;
  rotation: RotationReport | null;
  tune: TuneRecommendation | null;
  readiness: ReadinessReport | null;
  accountKey: string;
  generatedAt: number;
}

export function generateHealthReport(inputs: HealthReportInputs): string {
  const { health, diagnostics, rotation, tune, readiness, accountKey, generatedAt } =
    inputs;

  const lines: string[] = [];
  const ts = new Date(generatedAt).toISOString();
  const sep = "─".repeat(50);

  lines.push("BTC FUTURES PAPER DESK — HEALTH REPORT");
  lines.push(ts);
  lines.push(`Account: ${accountKey}`);
  lines.push(sep);

  lines.push("ROLLING HEALTH CHECK");
  if (!health) {
    lines.push("  Awaiting trade data (minimum 5 trades)");
  } else {
    lines.push(`  Grade:         ${health.grade} (${health.window} trades)`);
    lines.push(
      `  Expectancy:    $${health.expectancy.toFixed(2)} ${health.expectancyPass ? "✓" : "✗"}`,
    );
    lines.push(
      `  Win Rate:      ${(health.winRate * 100).toFixed(1)}% ${health.winRatePass ? "✓" : "✗"}`,
    );
    lines.push(
      `  Fee/Gross:     ${(health.feePctOfAbsGross * 100).toFixed(1)}% ${health.feePass ? "✓" : "✗"}`,
    );
    lines.push(
      `  Profit Factor: ${health.profitFactor === Infinity ? "∞" : health.profitFactor.toFixed(2)} ${health.pfPass ? "✓" : "✗"}`,
    );
    lines.push(
      `  TP Hits:       ${health.tpHits}/${health.window} ${health.tpHitPass ? "✓" : "✗"}`,
    );
    lines.push(`  SL Count:      ${health.slCount}`);
    lines.push(
      `  TIME Count:    ${health.timeCount} ${health.timeCount > 0 ? "⚠ SHOULD BE 0" : "✓"}`,
    );
  }
  lines.push(sep);

  lines.push("STRATEGY DIAGNOSTICS");
  if (!diagnostics) {
    lines.push("  No diagnostics available");
  } else {
    lines.push(`  Production Trades: ${diagnostics.totalProduction}`);
    lines.push("");
    lines.push("  Top 5 by Expectancy:");
    diagnostics.topByExpectancy.forEach((r, i) => {
      lines.push(
        `    ${i + 1}. ${r.strategyName.padEnd(30)} ` +
          `E:$${r.avgNetPnl.toFixed(2).padStart(7)} ` +
          `WR:${(r.winRate * 100).toFixed(0).padStart(3)}% ` +
          `(${r.totalTrades}t)`,
      );
    });
    lines.push("");
    lines.push("  Bottom 5 by Expectancy:");
    diagnostics.bottomByExpectancy.forEach((r, i) => {
      lines.push(
        `    ${i + 1}. ${r.strategyName.padEnd(30)} ` +
          `E:$${r.avgNetPnl.toFixed(2).padStart(7)} ` +
          `WR:${(r.winRate * 100).toFixed(0).padStart(3)}% ` +
          `(${r.totalTrades}t)`,
      );
    });
    if (diagnostics.slDominatedStrats.length > 0) {
      lines.push("");
      lines.push("  ⚠ SL-Dominated (>60% SL rate):");
      diagnostics.slDominatedStrats.forEach((r) => {
        lines.push(`    ${r.strategyName} — ${r.slCount}/${r.totalTrades} SL exits`);
      });
    }
    if (diagnostics.highFeeStrategies.length > 0) {
      lines.push("");
      lines.push("  ⚠ High Fee Ratio (>50%):");
      diagnostics.highFeeStrategies.forEach((r) => {
        lines.push(
          `    ${r.strategyName} — ${(r.feePctOfAbsGross * 100).toFixed(1)}% fee/gross`,
        );
      });
    }
  }
  lines.push(sep);

  lines.push("STRATEGY ROTATION");
  if (!rotation) {
    lines.push("  No rotation data available");
  } else {
    lines.push(`  Promoted:    ${rotation.promoted.length}`);
    lines.push(`  Active:      ${rotation.active.length}`);
    lines.push(`  Probation:   ${rotation.probation.length}`);
    lines.push(`  Suspended:   ${rotation.suspended.length}`);
    lines.push(`  Insufficient: ${rotation.insufficient.length}`);
    if (rotation.topStrategyId) {
      const top = rotation.scores.find((s) => s.strategyId === rotation.topStrategyId);
      lines.push(`  Top Performer: ${top?.strategyName ?? rotation.topStrategyId}`);
    }
    if (rotation.suspended.length > 0) {
      lines.push(
        `  Suspended IDs: ${rotation.suspended.map((s) => s.strategyId).join(", ")}`,
      );
    }
  }
  lines.push(sep);

  lines.push("PARAMETER TUNER");
  if (!tune || tune.target === "NO_CHANGE") {
    lines.push(`  ✓ No change needed (${tune?.tradesAnalyzed ?? 0} trades analyzed)`);
  } else {
    lines.push(`  ⚠ Recommendation: ${tune.target}`);
    lines.push(`    Current:   ${tune.currentValue}`);
    lines.push(
      `    Suggested: ${tune.suggestedValue} (${tune.delta > 0 ? "+" : ""}${tune.delta})`,
    );
    lines.push(`    Confidence: ${tune.confidence}`);
    lines.push(`    Rationale: ${tune.rationale.replace(/\s+/g, " ").trim()}`);
    lines.push(`    If ignored: ${tune.doNothing}`);
  }
  lines.push(sep);

  lines.push("PRODUCTION READINESS");
  if (!readiness) {
    lines.push("  Not computed");
  } else {
    lines.push(`  Status: ${readiness.productionReady ? "✓ READY" : "✗ NOT READY"}`);
    lines.push(`  Score:  ${(readiness.score * 100).toFixed(0)}%`);
    if (readiness.criticalFails.length > 0) {
      lines.push("  Critical Failures:");
      readiness.criticalFails.forEach((c) => {
        lines.push(`    ✗ ${c.label}: ${c.value} (need ${c.required})`);
      });
    }
    if (readiness.warnFails.length > 0) {
      lines.push("  Warnings:");
      readiness.warnFails.forEach((c) => {
        lines.push(`    ⚠ ${c.label}: ${c.value} (need ${c.required})`);
      });
    }
  }
  lines.push(sep);
  lines.push("END OF REPORT");

  return lines.join("\n");
}
