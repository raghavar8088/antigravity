/**
 * Promotion gate — all 6 criteria must pass before a strategy enters the recommended roster.
 *
 * Pure function — no I/O, no threshold lowering, no gate bypassing.
 * Recommendations only; operator decides whether to apply.
 */

export interface PromotionGateInput {
  paperTrades: number;
  paperExpectancy: number;
  replayTrades: number;
  replayExpectancy: number;
  walkForwardPass: boolean;
  feePctOfAbsGross: number;
}

export interface PromotionCriterion {
  name: string;
  pass: boolean;
  detail: string;
}

export interface PromotionGateResult {
  pass: boolean;
  reason: string;
  criteria: PromotionCriterion[];
}

const MIN_PAPER_TRADES = 20;
const MIN_REPLAY_TRADES = 20;
const MAX_FEE_PCT = 50;

export function canPromoteStrategy(input: PromotionGateInput): PromotionGateResult {
  const criteria: PromotionCriterion[] = [
    {
      name: "paperTrades ≥ 20",
      pass: input.paperTrades >= MIN_PAPER_TRADES,
      detail: `${input.paperTrades}/${MIN_PAPER_TRADES} paper trades`,
    },
    {
      name: "paperExpectancy > 0",
      pass: input.paperExpectancy > 0,
      detail: `$${input.paperExpectancy.toFixed(2)}/trade paper`,
    },
    {
      name: "replayTrades ≥ 20",
      pass: input.replayTrades >= MIN_REPLAY_TRADES,
      detail: `${input.replayTrades}/${MIN_REPLAY_TRADES} replay trades`,
    },
    {
      name: "replayExpectancy > 0",
      pass: input.replayExpectancy > 0,
      detail: `$${input.replayExpectancy.toFixed(2)}/trade replay`,
    },
    {
      name: "walkForwardPass",
      pass: input.walkForwardPass,
      detail: input.walkForwardPass ? "WFE ≥ 50% — PASS" : "WFE < 50% — FAIL",
    },
    {
      name: "feePctOfAbsGross ≤ 50%",
      pass: input.feePctOfAbsGross <= MAX_FEE_PCT,
      detail: `fee/gross ${input.feePctOfAbsGross.toFixed(0)}% (cap ${MAX_FEE_PCT}%)`,
    },
  ];

  const failing = criteria.filter((c) => !c.pass);
  const pass = failing.length === 0;
  const reason = pass
    ? "All 6 promotion criteria met."
    : `Blocked by: ${failing.map((c) => c.name).join("; ")}.`;

  return { pass, reason, criteria };
}
