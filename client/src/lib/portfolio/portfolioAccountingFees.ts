/**
 * Canonical Paper Desk fee model — mirrors engine BinanceFuturesTakerFeePct (0.05%).
 * NetPnL = GrossPnL - EntryFee - ExitFee
 */

export const BINANCE_FUTURES_TAKER_FEE_PCT = 0.0005;

export type CanonicalTradeFees = {
  entry_fee: number;
  exit_fee: number;
  total_fee: number;
};

export function canonicalTradeFees(
  entryPrice: number,
  exitPrice: number,
  quantity: number,
  takerFeePct = BINANCE_FUTURES_TAKER_FEE_PCT,
): CanonicalTradeFees {
  if (
    !Number.isFinite(entryPrice) || entryPrice <= 0 ||
    !Number.isFinite(exitPrice) || exitPrice <= 0 ||
    !Number.isFinite(quantity) || quantity <= 0
  ) {
    return { entry_fee: 0, exit_fee: 0, total_fee: 0 };
  }
  const entryNotional = entryPrice * quantity;
  const exitNotional = exitPrice * quantity;
  const entry_fee = entryNotional * takerFeePct;
  const exit_fee = exitNotional * takerFeePct;
  return { entry_fee, exit_fee, total_fee: entry_fee + exit_fee };
}

export function canonicalNetPnl(grossPnl: number, fees: CanonicalTradeFees): number {
  return grossPnl - fees.entry_fee - fees.exit_fee;
}

/** Backfill fee legs when only legacy `fees` (round-trip) is stored. */
export function resolveTradeFeeLegs(trade: {
  entry_price: number;
  exit_price: number;
  quantity: number;
  fees?: number;
  entry_fee?: number;
  exit_fee?: number;
  total_fee?: number;
}): CanonicalTradeFees {
  const entry = Number(trade.entry_fee);
  const exit = Number(trade.exit_fee);
  const total = Number(trade.total_fee);
  if (Number.isFinite(entry) && Number.isFinite(exit)) {
    return {
      entry_fee: entry,
      exit_fee: exit,
      total_fee: Number.isFinite(total) ? total : entry + exit,
    };
  }
  const canonical = canonicalTradeFees(trade.entry_price, trade.exit_price, trade.quantity);
  const legacy = Number(trade.fees);
  if (Number.isFinite(legacy) && legacy > 0) {
    const half = legacy / 2;
    return { entry_fee: half, exit_fee: half, total_fee: legacy };
  }
  return canonical;
}
