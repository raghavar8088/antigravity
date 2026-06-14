/**
 * Futures strategy definitions.
 *
 * The executable BTC futures strategy inventory has been removed from the
 * application. Keep this module as the source of truth so callers can handle an
 * intentionally empty roster without import failures.
 */

import type { FuturesStratDef } from "@/lib/trading/futuresStratTypes";

export type { BtcFtTemplateId, FuturesStratDef, RegimeTag } from "@/lib/trading/futuresStratTypes";

export const FUTURES_STRAT_DEFS: readonly FuturesStratDef[] = [];
