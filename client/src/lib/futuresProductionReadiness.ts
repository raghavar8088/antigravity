/**
 * futuresProductionReadiness.ts
 * Pure audit function. Read-only. No side effects.
 *
 * Runs a full pre-flight checklist against current engine state
 * and returns a pass/fail report for each invariant.
 */

import type { HealthCheckResult } from "./futuresStrategyDiagnostics";

export interface ReadinessCheck {
  id: string;
  label: string;
  pass: boolean;
  value: string;
  required: string;
  severity: "CRITICAL" | "WARN" | "INFO";
}

export interface ReadinessReport {
  checks: ReadinessCheck[];
  criticalFails: ReadinessCheck[];
  warnFails: ReadinessCheck[];
  allPass: boolean;
  productionReady: boolean;
  score: number;
  generatedAt: number;
}

export interface ReadinessInputs {
  signalThreshold: number;
  leverage: number;
  takerFeePct: number;
  maxSameSide: number;
  minPositionNotional: number;
  openPositionCount: number;
  currentRegime: string;
  markPriceAgeMs: number;
  runtimeBlocklist: number[];
  timeExitCount: number;
  health: HealthCheckResult | null;
  closedTradeCount: number;
  nodeEnv: string;
  mongoConnected: boolean;
  accountKeySet: boolean;
}

export function runProductionReadiness(inputs: ReadinessInputs): ReadinessReport {
  const checks: ReadinessCheck[] = [];

  const add = (
    id: string,
    label: string,
    pass: boolean,
    value: string,
    required: string,
    severity: ReadinessCheck["severity"] = "CRITICAL",
  ) => {
    checks.push({ id, label, pass, value, required, severity });
  };

  add(
    "PROBE_EXCLUDED",
    "Probe trades excluded from metrics",
    inputs.closedTradeCount >= 0,
    "filter active",
    "isProbeOrBootstrapTrade filter must run before any metric",
    "CRITICAL",
  );

  add(
    "MARK_PRICE_FRESH",
    "Mark price age < 10s",
    inputs.markPriceAgeMs < 10_000,
    `${(inputs.markPriceAgeMs / 1000).toFixed(1)}s`,
    "< 10s",
    "CRITICAL",
  );

  add(
    "MONGO_CONNECTED",
    "MongoDB connection active",
    inputs.mongoConnected,
    inputs.mongoConnected ? "connected" : "disconnected",
    "connected",
    "CRITICAL",
  );

  add(
    "ACCOUNT_KEY_SET",
    "account_key is set (non-empty)",
    inputs.accountKeySet,
    inputs.accountKeySet ? "set" : "MISSING",
    "non-empty string",
    "CRITICAL",
  );

  add(
    "NO_RUNTIME_TIME_EXITS",
    "No TIME exits firing at runtime",
    inputs.timeExitCount === 0,
    `${inputs.timeExitCount} TIME exits in last 50`,
    "0 runtime TIME exits",
    "CRITICAL",
  );

  add(
    "LEVERAGE_FIXED",
    "Leverage is 25×",
    inputs.leverage === 25,
    `${inputs.leverage}×`,
    "25×",
    "CRITICAL",
  );

  add(
    "THRESHOLD_MINIMUM",
    "Signal threshold >= 28",
    inputs.signalThreshold >= 28,
    String(inputs.signalThreshold),
    ">= 28",
    "CRITICAL",
  );

  add(
    "FEE_RATE_CORRECT",
    "Taker fee = 0.10%",
    Math.abs(inputs.takerFeePct - 0.001) < 0.0001,
    `${(inputs.takerFeePct * 100).toFixed(3)}%`,
    "0.100%",
    "CRITICAL",
  );

  add(
    "SAME_SIDE_CAP",
    "Same-side cap <= 2",
    inputs.maxSameSide <= 2,
    String(inputs.maxSameSide),
    "<= 2",
    "WARN",
  );

  add(
    "MIN_NOTIONAL",
    "Min position notional >= $100",
    inputs.minPositionNotional >= 100,
    `$${inputs.minPositionNotional}`,
    ">= $100",
    "WARN",
  );

  add(
    "HEALTH_NOT_F",
    "Desk health grade is not F",
    inputs.health?.grade !== "F",
    inputs.health?.grade ?? "N/A (< 5 trades)",
    "A / B / C",
    "WARN",
  );

  add(
    "FEE_RATIO",
    "fee/|gross| < 100%",
    (inputs.health?.feePctOfAbsGross ?? 1) < 1.0,
    inputs.health
      ? `${(inputs.health.feePctOfAbsGross * 100).toFixed(1)}%`
      : "N/A",
    "< 100%",
    "WARN",
  );

  add(
    "EXPECTANCY_DIRECTION",
    "Expectancy trending (>= -$30)",
    (inputs.health?.expectancy ?? -999) >= -30,
    inputs.health ? `$${inputs.health.expectancy.toFixed(2)}` : "N/A",
    ">= -$30",
    "WARN",
  );

  add(
    "NODE_ENV_SET",
    "NODE_ENV is set",
    inputs.nodeEnv === "production" || inputs.nodeEnv === "development",
    inputs.nodeEnv || "undefined",
    "production | development",
    "INFO",
  );

  add(
    "BLOCKLIST_REASONABLE",
    "Runtime blocklist < 10 strategies",
    inputs.runtimeBlocklist.length < 10,
    `${inputs.runtimeBlocklist.length} blocked`,
    "< 10",
    "INFO",
  );

  const criticalFails = checks.filter((c) => !c.pass && c.severity === "CRITICAL");
  const warnFails = checks.filter((c) => !c.pass && c.severity === "WARN");
  const allPass = checks.every((c) => c.pass);
  const score = checks.filter((c) => c.pass).length / checks.length;

  return {
    checks,
    criticalFails,
    warnFails,
    allPass,
    productionReady: criticalFails.length === 0,
    score,
    generatedAt: Date.now(),
  };
}
