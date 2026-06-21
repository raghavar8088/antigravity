/**
 * Trade Threshold Configuration — shared types.
 *
 * Mirrors the Go engine's DTOs exactly:
 *   - ThresholdConfigDTO  ↔ engine/internal/config/http.go ThresholdConfigDTO
 *   - ConfigChangeEntry   ↔ engine/internal/config/http.go ConfigChangeEntry
 *
 * Field names match the engine's JSON tags 1:1 so no field-mapping layer is
 * needed between the Go response and the React components.
 */

export type ThresholdCategory =
  | "signal_quality"
  | "position_sizing"
  | "risk_management"
  | "directional_caps"
  | "walk_forward"
  | "cooldowns"
  | "regime_classification"
  | "execution";

export type ThresholdDataType = "percentage" | "decimal" | "integer" | "currency" | "duration";

export type ThresholdRiskLevel = "low" | "medium" | "high" | "critical";

export type ThresholdSource = "env" | "default" | "runtime_override";

/** One tunable trading threshold: static metadata merged with its live value. */
export type ThresholdConfigDTO = {
  key: string;
  category: ThresholdCategory;
  label: string;
  description: string;
  dataType: ThresholdDataType;
  unit: string;
  currentValue: number;
  defaultValue: number;
  min: number;
  max: number;
  recommendedMin: number;
  recommendedMax: number;
  riskLevel: ThresholdRiskLevel;
  whatHappensIfIncreased: string;
  whatHappensIfDecreased: string;
  requiresRestart: boolean;
  source: ThresholdSource;
  lastChangedAt?: string;
};

export type ThresholdConfigListResponse = {
  ok: boolean;
  thresholds: ThresholdConfigDTO[];
  error?: string;
};

/** One row of the config_changes audit trail. */
export type ConfigChangeEntry = {
  id?: string;
  key: string;
  oldValue: number;
  newValue: number;
  changedAt: string;
  changedBy: string;
  requiresRestart: boolean;
  reason?: string;
  /** Set only for Master Strictness Dial batch applications. */
  isBatch?: boolean;
  dialFrom?: number;
  dialTo?: number;
  batchDeltas?: ThresholdDelta[];
};

export type ConfigChangeHistoryResponse = {
  ok: boolean;
  history: ConfigChangeEntry[];
  error?: string;
};

export type SetThresholdResponse = {
  ok: boolean;
  threshold?: ThresholdConfigDTO;
  requiresRestart?: boolean;
  error?: string;
  min?: number;
  max?: number;
};

/**
 * Master Strictness Dial — mirrors engine/internal/config/strictness.go.
 * One profile per dial-controlled threshold; CurrentValue is always read
 * live from the registry server-side, never hardcoded.
 */
export type StrictnessDirection = "tightens" | "loosens";

export type StrictnessProfile = {
  thresholdKey: string;
  direction: StrictnessDirection;
  looseValue: number;
  currentValue: number;
  strictValue: number;
};

export type StrictnessGetResponse = {
  ok: boolean;
  currentDialPosition: number | null;
  profiles: StrictnessProfile[];
  affectedThresholdCount: number;
  error?: string;
};

export type ThresholdDelta = {
  key: string;
  oldValue: number;
  newValue: number;
};

export type StrictnessSetResponse = {
  ok: boolean;
  dialPosition?: number;
  deltas?: ThresholdDelta[];
  updated?: Record<string, unknown>[];
  error?: string;
};

/** Linear interpolation matching ApplyStrictnessDial's Go formula exactly:
 * 0-50 interpolates LOOSE→CURRENT, 50-100 interpolates CURRENT→STRICT. */
export function interpolateStrictnessValue(p: StrictnessProfile, dialPosition: number): number {
  const d = Math.max(0, Math.min(100, dialPosition));
  if (d <= 50) {
    const t = d / 50;
    return p.looseValue + (p.currentValue - p.looseValue) * t;
  }
  const t = (d - 50) / 50;
  return p.currentValue + (p.strictValue - p.currentValue) * t;
}

export const THRESHOLD_CATEGORIES: { key: ThresholdCategory; label: string }[] = [
  { key: "signal_quality", label: "Signal Quality" },
  { key: "position_sizing", label: "Position Sizing" },
  { key: "risk_management", label: "Risk Management" },
  { key: "directional_caps", label: "Directional Caps" },
  { key: "walk_forward", label: "Walk-Forward" },
  { key: "cooldowns", label: "Cooldowns" },
  { key: "regime_classification", label: "Regime Classification" },
  { key: "execution", label: "Execution" },
];

export const RISK_LEVEL_LABEL: Record<ThresholdRiskLevel, string> = {
  low: "LOW",
  medium: "MEDIUM",
  high: "HIGH",
  critical: "CRITICAL",
};

export const RISK_LEVEL_COLOR_VAR: Record<ThresholdRiskLevel, string> = {
  low: "var(--text-muted)",
  medium: "var(--amber)",
  high: "var(--red)",
  critical: "var(--red)",
};

/** Formats a threshold's current value for display, respecting its dataType/unit. */
export function formatThresholdValue(t: Pick<ThresholdConfigDTO, "dataType" | "unit">, value: number): string {
  switch (t.dataType) {
    case "percentage":
      return `${(value <= 1 ? value * 100 : value).toFixed(2)}%`;
    case "currency":
      return `$${value.toLocaleString(undefined, { maximumFractionDigits: 2 })}`;
    case "duration":
      return `${value}${t.unit ? ` ${t.unit}` : "s"}`;
    case "integer":
      return `${Math.round(value)}${t.unit ? ` ${t.unit}` : ""}`;
    default:
      return `${value}${t.unit ? ` ${t.unit}` : ""}`;
  }
}
