/**
 * Operator health score: single function that tells the operator what state the desk is in
 * and what to do next — without requiring them to read 6 different panels.
 *
 * Priority order (first match wins):
 *   1. NEEDS_REPAIR   — balance drift or probe-dominant trades → PnL untrustworthy
 *   2. WORKER_STALE   — worker enabled but not heartbeating → risk of silent failure
 *   3. PAUSED         — operator paused or drawdown lock → no entries until cleared
 *   4. MARKET_WAIT    — chop + high regime skips → correct behavior, not a bug
 *   5. COLLECT_DATA   — < 50 production closes → rotation has no signal yet
 *   6. READY          — scorecard on track, worker fresh, clean state
 *   7. BLOCKED_STATE  — catch-all for anything else that needs attention
 */

export type DeskOperatorHealthStatus =
  | "READY"
  | "COLLECT_DATA"
  | "BLOCKED_STATE"
  | "WORKER_STALE"
  | "MARKET_WAIT"
  | "PAUSED"
  | "NEEDS_REPAIR";

export type DeskOperatorHealthSeverity = "success" | "info" | "warning" | "danger";

export interface DeskOperatorHealthInput {
  workerEnabled: boolean;
  workerLastPollAt: number | null;
  workerStale: boolean;
  balanceDriftUsd: number;
  totalTrades: number;
  openPositions: number;
  closedTrades48h: number;
  regime: string;
  skipRegime: number;
  skipAtrFees: number;
  rotationSuspended: number;
  rotationPending: number;
  rotationActive: number;
  rotationPromoted: number;
  pauseEntries: boolean;
  drawdownLocked: boolean;
  probeDominant: boolean;
  scorecardState?: "COLLECT_DATA" | "REVIEW" | "ON_TRACK";
}

export interface DeskOperatorHealthResult {
  status: DeskOperatorHealthStatus;
  severity: DeskOperatorHealthSeverity;
  headline: string;
  nextAction: string;
  blockers: string[];
}

export function computeDeskOperatorHealth(
  input: DeskOperatorHealthInput,
): DeskOperatorHealthResult {
  const blockers: string[] = [];

  // ── 1. NEEDS_REPAIR ──────────────────────────────────────────────────────────
  if (input.balanceDriftUsd > 1 || input.probeDominant) {
    if (input.balanceDriftUsd > 1) {
      blockers.push(`Balance drift $${input.balanceDriftUsd.toFixed(2)} — raw balance diverges from production PnL`);
    }
    if (input.probeDominant) {
      blockers.push("Probe/bootstrap trades dominate history — PnL metrics untrustworthy");
    }
    return {
      status: "NEEDS_REPAIR",
      severity: "danger",
      headline: "NEEDS_REPAIR — paper state corrupted by probe/bootstrap trades",
      nextAction: "Click Repair state to reset balance and positions. Historical trades are kept but excluded from metrics by cleared_at.",
      blockers,
    };
  }

  // ── 2. WORKER_STALE ───────────────────────────────────────────────────────────
  if (input.workerEnabled && input.workerStale) {
    blockers.push(
      input.workerLastPollAt
        ? `Worker last polled ${Math.round((Date.now() - input.workerLastPollAt) / 1000)}s ago (TTL: 45s)`
        : "Worker has never polled",
    );
    return {
      status: "WORKER_STALE",
      severity: "warning",
      headline: "WORKER_STALE — VPS worker is not heartbeating",
      nextAction: "SSH to VPS and run: pm2 restart btc-ft-worker. Check pm2 logs for errors.",
      blockers,
    };
  }

  // ── 3. PAUSED ────────────────────────────────────────────────────────────────
  if (input.pauseEntries || input.drawdownLocked) {
    if (input.pauseEntries) blockers.push("Pause entries is ON — operator manually paused");
    if (input.drawdownLocked) blockers.push("Drawdown lock active — equity below session peak threshold");
    return {
      status: "PAUSED",
      severity: "warning",
      headline: "PAUSED — no new entries until resumed",
      nextAction: input.pauseEntries
        ? "Click Resume entries when ready."
        : "Drawdown lock auto-clears after partial recovery. No action needed.",
      blockers,
    };
  }

  // ── 4. MARKET_WAIT ───────────────────────────────────────────────────────────
  const isChop = input.regime.toLowerCase().includes("chop");
  const highRegimeSkips = input.skipRegime > 5;
  const noOpenPositions = input.openPositions === 0;
  if (noOpenPositions && isChop && highRegimeSkips) {
    blockers.push(`Regime: ${input.regime} — trend strategies blocked in chop`);
    if (input.skipAtrFees > 0) {
      blockers.push(`${input.skipAtrFees} entries blocked by ATR/fee gate — expected move too small to cover fees`);
    }
    if (input.rotationSuspended > 0) {
      blockers.push(`${input.rotationSuspended} strategies suspended by rotation`);
    }
    return {
      status: "MARKET_WAIT",
      severity: "info",
      headline: "MARKET_WAIT — no trade is correct: chop + ATR/fee gate blocking bad entries",
      nextAction: "Wait for trend regime. Do NOT lower signal threshold — the gate is working correctly.",
      blockers,
    };
  }

  // ── 5. COLLECT_DATA ───────────────────────────────────────────────────────────
  if (input.totalTrades < 50) {
    const pct = Math.round((input.totalTrades / 50) * 100);
    blockers.push(`${input.totalTrades}/50 production closes collected (${pct}% to rotation signal)`);
    if (input.rotationPending > 0) {
      blockers.push(`${input.rotationPending} strategies need ≥5 trades to be scored`);
    }
    return {
      status: "COLLECT_DATA",
      severity: "info",
      headline: `COLLECT_DATA — ${input.totalTrades}/50 closes collected. Keep worker running.`,
      nextAction: "No action needed. Allow the worker to collect production trades. Rotation will score strategies after 5+ closes each.",
      blockers,
    };
  }

  // ── 6. READY ─────────────────────────────────────────────────────────────────
  const workerOk = !input.workerEnabled || !input.workerStale;
  const cleanBalance = input.balanceDriftUsd <= 1;
  const scorecardOk = input.scorecardState === "ON_TRACK";
  if (scorecardOk && workerOk && cleanBalance) {
    return {
      status: "READY",
      severity: "success",
      headline: "READY — paper desk clean, worker fresh, scorecard on track",
      nextAction: "Continue running. Monitor soak trend and rotation report.",
      blockers: [],
    };
  }

  // ── 7. BLOCKED_STATE ─────────────────────────────────────────────────────────
  if (!scorecardOk) {
    if (input.scorecardState === "REVIEW") {
      blockers.push("Scorecard in REVIEW — last-50 metrics below targets");
    } else {
      blockers.push("Scorecard not yet available — need more production trades");
    }
  }
  if (input.rotationSuspended > 0) {
    blockers.push(`${input.rotationSuspended} strategies suspended — consider disabling via Suggested Safety Actions`);
  }
  if (input.skipAtrFees > 5) {
    blockers.push(`${input.skipAtrFees} entries blocked by ATR/fee gate — expected move small`);
  }

  return {
    status: "BLOCKED_STATE",
    severity: "warning",
    headline: "BLOCKED_STATE — desk running but entries blocked by one or more gates",
    nextAction: "Review blockers below. Check rotation, regime, and scorecard tabs for details.",
    blockers,
  };
}

export const OPERATOR_HEALTH_SEVERITY_COLOR: Record<DeskOperatorHealthSeverity, string> = {
  success: "#3fb950",
  info: "#58a6ff",
  warning: "#d29922",
  danger: "#f85149",
};
