/**
 * Futures strategy definitions.
 *
 * FUTURES_STRAT_DEFS contains:
 *   - CORE 20 (IDs 91–152): always active production winners
 *   - Premium (IDs 500–503): active when in promotedStrategyIds set
 *   - CATEGORY_POOL_160 (IDs 600–759): researchOnly: true — NOT auto-active;
 *     enter live roster only via winners promotion or research mode flag
 *
 * Never add researchOnly strategies to CORE_BTC_FT_STRATEGY_IDS.
 */

import type { FuturesStratDef } from "@/lib/trading/futuresStratTypes";
import { BTC_FT_PREMIUM_DEFS } from "@/lib/trading/btcFtPremiumStrategies";
import { CATEGORY_POOL_160 } from "@/lib/trading/futuresCategoryStrategies";

export type { BtcFtTemplateId, FuturesStratDef, RegimeTag } from "@/lib/trading/futuresStratTypes";

const BASE_FUTURES_STRAT_DEFS: FuturesStratDef[] = [
  // ── Scalp-only: pure 1m trend/breakout, short hold ────────────────────────
  { id: 91,  name: "Trend_Continuation_Long",   category: "Trend",       signalKey: "TREND_CONT_LONG",        slPct: 0.50, tpPct: 1.50, cooldownMin: 8,  holdMinutes: 40, confluenceMin: 5, styles: ["scalp"],        playbooks: ["trend"],               templateFamily: "TREND_CONT"      },
  { id: 92,  name: "Trend_Continuation_Short",  category: "Trend",       signalKey: "TREND_CONT_SHORT",       slPct: 0.50, tpPct: 1.50, cooldownMin: 8,  holdMinutes: 40, confluenceMin: 5, styles: ["scalp"],        playbooks: ["trend"],               templateFamily: "TREND_CONT"      },
  { id: 95,  name: "Breakout_Long",             category: "Breakout",    signalKey: "BREAKOUT_LONG",          slPct: 0.55, tpPct: 1.65, cooldownMin: 7,  holdMinutes: 32, confluenceMin: 5, styles: ["scalp", "day"], playbooks: ["breakout"],            templateFamily: "BREAKOUT"        },
  { id: 96,  name: "Breakout_Short",            category: "Breakout",    signalKey: "BREAKOUT_SHORT",         slPct: 0.55, tpPct: 1.65, cooldownMin: 7,  holdMinutes: 32, confluenceMin: 5, styles: ["scalp", "day"], playbooks: ["breakout"],            templateFamily: "BREAKOUT"        },
  // ── Multi-timeframe: scalp + day ──────────────────────────────────────────
  { id: 111, name: "MTF_Trend_Align_Long",      category: "MTF Trend",   signalKey: "MTF_TREND_ALIGN_LONG",   slPct: 0.50, tpPct: 1.55, cooldownMin: 8,  holdMinutes: 50, confluenceMin: 5, requiresHtf: true, styles: ["scalp", "day"],        playbooks: ["trend"],               templateFamily: "MTF_TREND_ALIGN" },
  { id: 112, name: "MTF_Trend_Align_Short",     category: "MTF Trend",   signalKey: "MTF_TREND_ALIGN_SHORT",  slPct: 0.50, tpPct: 1.55, cooldownMin: 8,  holdMinutes: 50, confluenceMin: 5, requiresHtf: true, styles: ["scalp", "day"],        playbooks: ["trend"],               templateFamily: "MTF_TREND_ALIGN" },
  { id: 117, name: "MTF_MACD_Align_Long",       category: "MTF MACD",    signalKey: "MTF_MACD_ALIGN_LONG",    slPct: 0.52, tpPct: 1.56, cooldownMin: 8,  holdMinutes: 45, confluenceMin: 5, requiresHtf: true, styles: ["scalp", "day"],        playbooks: ["trend", "momentum"],   templateFamily: "MTF_MACD_ALIGN"  },
  { id: 118, name: "MTF_MACD_Align_Short",      category: "MTF MACD",    signalKey: "MTF_MACD_ALIGN_SHORT",   slPct: 0.52, tpPct: 1.56, cooldownMin: 8,  holdMinutes: 45, confluenceMin: 5, requiresHtf: true, styles: ["scalp", "day"],        playbooks: ["trend", "momentum"],   templateFamily: "MTF_MACD_ALIGN"  },
  // ── Multi-timeframe: day + swing ──────────────────────────────────────────
  { id: 123, name: "MTF_ADX_Power_Long",        category: "MTF ADX",     signalKey: "MTF_ADX_POWER_LONG",     slPct: 0.52, tpPct: 1.56, cooldownMin: 9,  holdMinutes: 52, confluenceMin: 5, requiresHtf: true, styles: ["day", "swing"],        playbooks: ["trend"],               templateFamily: "MTF_ADX_POWER"   },
  { id: 124, name: "MTF_ADX_Power_Short",       category: "MTF ADX",     signalKey: "MTF_ADX_POWER_SHORT",    slPct: 0.52, tpPct: 1.56, cooldownMin: 9,  holdMinutes: 52, confluenceMin: 5, requiresHtf: true, styles: ["day", "swing"],        playbooks: ["trend"],               templateFamily: "MTF_ADX_POWER"   },
  { id: 125, name: "MTF_Breakout_Long",         category: "MTF Break",   signalKey: "MTF_BREAKOUT_LONG",      slPct: 0.55, tpPct: 1.65, cooldownMin: 8,  holdMinutes: 45, confluenceMin: 5, requiresHtf: true, styles: ["day"],                 playbooks: ["breakout"],            templateFamily: "MTF_BREAKOUT"    },
  { id: 126, name: "MTF_Breakout_Short",        category: "MTF Break",   signalKey: "MTF_BREAKOUT_SHORT",     slPct: 0.55, tpPct: 1.65, cooldownMin: 8,  holdMinutes: 45, confluenceMin: 5, requiresHtf: true, styles: ["day"],                 playbooks: ["breakout"],            templateFamily: "MTF_BREAKOUT"    },
  // ── Smart Money / Order Flow: scalp + day ────────────────────────────────
  { id: 131, name: "SmartMoney_Accum_Long",     category: "Smart Money", signalKey: "SM_ACCUM_LONG",          slPct: 0.50, tpPct: 1.55, cooldownMin: 8,  holdMinutes: 50, confluenceMin: 6, styles: ["scalp", "day"],        playbooks: ["momentum"],            templateFamily: "SM_ACCUM"        },
  { id: 132, name: "SmartMoney_Distrib_Short",  category: "Smart Money", signalKey: "SM_DISTRIB_SHORT",       slPct: 0.50, tpPct: 1.55, cooldownMin: 8,  holdMinutes: 50, confluenceMin: 6, styles: ["scalp", "day"],        playbooks: ["momentum"],            templateFamily: "SM_DISTRIB"      },
  { id: 133, name: "OrderFlow_Break_Long",      category: "Order Flow",  signalKey: "OF_BREAK_LONG",          slPct: 0.52, tpPct: 1.56, cooldownMin: 7,  holdMinutes: 38, confluenceMin: 5, styles: ["scalp"],               playbooks: ["breakout", "momentum"],templateFamily: "OF_BREAK"        },
  { id: 134, name: "OrderFlow_Break_Short",     category: "Order Flow",  signalKey: "OF_BREAK_SHORT",         slPct: 0.52, tpPct: 1.56, cooldownMin: 7,  holdMinutes: 38, confluenceMin: 5, styles: ["scalp"],               playbooks: ["breakout", "momentum"],templateFamily: "OF_BREAK"        },
  // ── Wyckoff: day + swing ─────────────────────────────────────────────────
  { id: 139, name: "Wyckoff_Spring_Long",       category: "Wyckoff",     signalKey: "WYCKOFF_SPRING_LONG",    slPct: 0.55, tpPct: 1.65, cooldownMin: 10, holdMinutes: 60, confluenceMin: 6, styles: ["day", "swing"],        playbooks: ["range"],               templateFamily: "WYCKOFF"         },
  { id: 140, name: "Wyckoff_Upthrust_Short",    category: "Wyckoff",     signalKey: "WYCKOFF_UPTHRUST_SHORT", slPct: 0.55, tpPct: 1.65, cooldownMin: 10, holdMinutes: 60, confluenceMin: 6, styles: ["day", "swing"],        playbooks: ["range"],               templateFamily: "WYCKOFF"         },
  // ── Session / Opening Drive: scalp ───────────────────────────────────────
  { id: 151, name: "OpeningDrive_Long",         category: "Session",     signalKey: "OPEN_DRIVE_LONG",        slPct: 0.50, tpPct: 1.50, cooldownMin: 5,  holdMinutes: 30, confluenceMin: 5, styles: ["scalp"],               playbooks: ["breakout"],            templateFamily: "OPENING_DRIVE"   },
  { id: 152, name: "OpeningDrive_Short",        category: "Session",     signalKey: "OPEN_DRIVE_SHORT",       slPct: 0.50, tpPct: 1.50, cooldownMin: 5,  holdMinutes: 30, confluenceMin: 5, styles: ["scalp"],               playbooks: ["breakout"],            templateFamily: "OPENING_DRIVE"   },
];

export const FUTURES_STRAT_DEFS: readonly FuturesStratDef[] = [
  ...BASE_FUTURES_STRAT_DEFS,
  ...BTC_FT_PREMIUM_DEFS,
  ...CATEGORY_POOL_160,
];
