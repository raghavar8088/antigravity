import type { ScorecardAction } from "@/lib/futuresScorecardActions";

/** Whether hero "Copy env fix" should show (UI-20). */
export function scorecardShowsEnvFixCopy(action: ScorecardAction | null | undefined): boolean {
  if (!action) return false;
  if (action.severity !== "ACT" && action.severity !== "WARN") return false;
  if (!action.suggestedEnv) return false;
  return Object.keys(action.suggestedEnv).length > 0;
}
