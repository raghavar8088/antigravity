/** Client-safe mark-to-market helpers for Paper Desk open positions (no MongoDB). */

export type PaperPositionMarkInputs = {
  side: string;
  entry_price: number;
  size: number;
};

export function normalizePaperPositionSide(side: string): "LONG" | "SHORT" {
  const s = side.trim().toUpperCase();
  return s === "SHORT" || s === "SELL" ? "SHORT" : "LONG";
}

/** Mark-to-market unrealized PnL for an open position (size in BTC). */
export function unrealizedPnlForOpenPosition(
  pos: PaperPositionMarkInputs,
  markPrice: number,
): number {
  if (!Number.isFinite(markPrice) || markPrice <= 0) return 0;
  if (!Number.isFinite(pos.entry_price) || !Number.isFinite(pos.size)) return 0;
  const side = normalizePaperPositionSide(pos.side);
  const delta = side === "SHORT" ? pos.entry_price - markPrice : markPrice - pos.entry_price;
  return delta * pos.size;
}
