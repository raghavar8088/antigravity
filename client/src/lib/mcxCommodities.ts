/**
 * MCX commodity definitions for the Commodity Scalper engine.
 * Tokens are resolved at runtime via Angel One searchScrip API.
 * Lot sizes, IVs, and strike steps are calibrated for typical MCX market conditions.
 */

export type MCXCommodity = {
  id: string;           // engine/routing key
  name: string;         // display name
  searchQuery: string;  // prefix for Angel One searchScrip
  unit: string;         // price unit label
  lotSize: number;      // units per lot (for option cost calculation)
  iv: number;           // typical annualized IV
  strikeStep: number;   // option strike increment
  sector: string;
  positionINR: number;  // target paper-money per position
  dteDays: number;      // assumed DTE for option premium model
  colorClass: string;   // Tailwind text color for UI
  bgClass: string;      // Tailwind bg color for UI
  borderClass: string;  // Tailwind border color for UI
};

export const MCX_COMMODITIES: MCXCommodity[] = [
  {
    id: "CRUDEOIL",
    name: "Crude Oil",
    searchQuery: "CRUDEOIL",
    unit: "₹/bbl",
    lotSize: 100,
    iv: 0.35,
    strikeStep: 50,
    sector: "Energy",
    positionINR: 20000,
    dteDays: 7,
    colorClass: "text-amber-400",
    bgClass: "bg-amber-500/10",
    borderClass: "border-amber-500/30",
  },
  {
    id: "GOLDM",
    name: "Gold Mini",
    searchQuery: "GOLDM",
    unit: "₹/10g",
    lotSize: 1,
    iv: 0.14,
    strikeStep: 100,
    sector: "Precious Metals",
    positionINR: 25000,
    dteDays: 7,
    colorClass: "text-yellow-400",
    bgClass: "bg-yellow-500/10",
    borderClass: "border-yellow-500/30",
  },
  {
    id: "SILVERM",
    name: "Silver Mini",
    searchQuery: "SILVERM",
    unit: "₹/kg",
    lotSize: 5,
    iv: 0.22,
    strikeStep: 500,
    sector: "Precious Metals",
    positionINR: 20000,
    dteDays: 7,
    colorClass: "text-slate-300",
    bgClass: "bg-slate-500/10",
    borderClass: "border-slate-500/30",
  },
  {
    id: "NATURALGAS",
    name: "Natural Gas",
    searchQuery: "NATURALGAS",
    unit: "₹/mmbtu",
    lotSize: 1250,
    iv: 0.55,
    strikeStep: 10,
    sector: "Energy",
    positionINR: 20000,
    dteDays: 7,
    colorClass: "text-blue-400",
    bgClass: "bg-blue-500/10",
    borderClass: "border-blue-500/30",
  },
  {
    id: "COPPER",
    name: "Copper",
    searchQuery: "COPPER",
    unit: "₹/kg",
    lotSize: 2500,
    iv: 0.22,
    strikeStep: 5,
    sector: "Base Metals",
    positionINR: 20000,
    dteDays: 7,
    colorClass: "text-orange-400",
    bgClass: "bg-orange-500/10",
    borderClass: "border-orange-500/30",
  },
];

export const MCX_COMMODITY_MAP = new Map(MCX_COMMODITIES.map((c) => [c.id, c]));
