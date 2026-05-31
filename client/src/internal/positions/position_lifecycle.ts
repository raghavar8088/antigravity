import type { Position } from "@/internal/positions";

export type LifecycleAction =
  | "HOLD"
  | "SCALE_IN"
  | "SCALE_OUT"
  | "MOVE_TO_BREAKEVEN"
  | "TRAILING_STOP"
  | "TIME_EXIT"
  | "EMERGENCY_EXIT"
  | "LIQUIDATION_RISK";

export interface PositionLifecycleInput {
  position: Position;
  markPrice: number;
  stopLoss: number;
  takeProfit: number;
  openedAt: number;
  maxHoldMs: number;
  liquidationPrice?: number;
  now?: number;
}

export interface PositionLifecycleDecision {
  action: LifecycleAction;
  reason: string;
  exitPrice?: number;
  stopPrice?: number;
}

export class PositionLifecycleEngine {
  evaluate(input: PositionLifecycleInput): PositionLifecycleDecision {
    const now = input.now ?? Date.now();
    const p = input.position;
    const long = p.side === "LONG";
    const mark = input.markPrice;

    if (input.liquidationPrice != null) {
      const distancePct = Math.abs((mark - input.liquidationPrice) / Math.max(mark, 1)) * 100;
      if (distancePct <= 0.75) {
        return { action: "LIQUIDATION_RISK", reason: "mark near modeled liquidation", exitPrice: mark };
      }
    }

    if ((long && mark <= input.stopLoss) || (!long && mark >= input.stopLoss)) {
      return { action: "EMERGENCY_EXIT", reason: "stop loss crossed", exitPrice: mark };
    }

    if ((long && mark >= input.takeProfit) || (!long && mark <= input.takeProfit)) {
      return { action: "SCALE_OUT", reason: "take profit reached", exitPrice: mark };
    }

    if (now - input.openedAt >= input.maxHoldMs) {
      return { action: "TIME_EXIT", reason: "max hold elapsed", exitPrice: mark };
    }

    const unrealizedPct = long
      ? ((mark - p.entryPrice) / p.entryPrice) * 100
      : ((p.entryPrice - mark) / p.entryPrice) * 100;
    if (unrealizedPct >= 0.8) {
      return { action: "TRAILING_STOP", reason: "profit cushion supports trailing stop", stopPrice: p.entryPrice };
    }
    if (unrealizedPct >= 0.4) {
      return { action: "MOVE_TO_BREAKEVEN", reason: "initial risk paid for", stopPrice: p.entryPrice };
    }

    return { action: "HOLD", reason: "no lifecycle trigger" };
  }
}
