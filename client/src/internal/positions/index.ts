import type { Order } from "@/internal/oms";

export interface Position {
  positionId: string;
  symbol: string;
  side: "LONG" | "SHORT";
  entryPrice: number;
  size: number;
  fees: number;
  funding: number;
  exposure: number;
  risk: number;
  unrealizedPnl: number;
  realizedPnl: number;
  openedAt: number;
  updatedAt: number;
  orderId?: string;
}

export interface PositionUpdate {
  markPrice: number;
  feeDelta?: number;
  fundingDelta?: number;
  now?: number;
}

function nextPositionId(): string {
  const random = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return `pos_${random}`;
}

function pnl(side: Position["side"], entry: number, mark: number, size: number): number {
  return side === "LONG" ? (mark - entry) * size : (entry - mark) * size;
}

export class PositionManagerV2 {
  private readonly positions = new Map<string, Position>();

  openFromOrder(order: Order, now = Date.now()): Position {
    if (order.state !== "FILLED") throw new Error(`cannot open position from ${order.state} order`);
    const entryPrice = order.averageFillPrice ?? order.price;
    if (!entryPrice || entryPrice <= 0) throw new Error("filled order missing entry price");

    const position: Position = {
      positionId: nextPositionId(),
      symbol: order.symbol,
      side: order.side === "BUY" ? "LONG" : "SHORT",
      entryPrice,
      size: order.filledQuantity,
      fees: 0,
      funding: 0,
      exposure: entryPrice * order.filledQuantity,
      risk: Math.abs((order.signal?.Entry ?? entryPrice) - (order.signal?.StopLoss ?? entryPrice)) * order.filledQuantity,
      unrealizedPnl: 0,
      realizedPnl: 0,
      openedAt: now,
      updatedAt: now,
      orderId: order.orderId,
    };
    this.positions.set(position.positionId, position);
    return position;
  }

  update(positionId: string, update: PositionUpdate): Position {
    const current = this.positions.get(positionId);
    if (!current) throw new Error(`unknown position ${positionId}`);
    const next: Position = {
      ...current,
      fees: current.fees + (update.feeDelta ?? 0),
      funding: current.funding + (update.fundingDelta ?? 0),
      unrealizedPnl: pnl(current.side, current.entryPrice, update.markPrice, current.size) - current.fees - current.funding,
      updatedAt: update.now ?? Date.now(),
    };
    this.positions.set(positionId, next);
    return next;
  }

  close(positionId: string, exitPrice: number, now = Date.now()): Position {
    const current = this.positions.get(positionId);
    if (!current) throw new Error(`unknown position ${positionId}`);
    const realizedPnl = pnl(current.side, current.entryPrice, exitPrice, current.size) - current.fees - current.funding;
    const closed = { ...current, unrealizedPnl: 0, realizedPnl, updatedAt: now };
    this.positions.delete(positionId);
    return closed;
  }

  list(): Position[] {
    return [...this.positions.values()];
  }
}
