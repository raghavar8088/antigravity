/**
 * DeskHealthBadge — read-only display of rolling health check.
 * Drop into existing stats panel. No new state, no new fetches.
 * Receives healthCheck from parent that already has engine stats.
 */
import type { HealthCheckResult } from "@/lib/trading/futuresStrategyDiagnostics";

interface Props {
  health: HealthCheckResult | null;
}

const GRADE_COLOR: Record<string, string> = {
  A: "#3fb950",
  B: "#d29922",
  C: "#ff7c2c",
  F: "#f85149",
};

const CHECK_ICON = (pass: boolean) => (pass ? "✅" : "❌");

export function DeskHealthBadge({ health }: Props) {
  if (!health || health.window < 5) {
    return (
      <div style={{ fontSize: 11, color: "#8b949e", padding: "4px 0" }}>
        Health: awaiting 5+ production trades
      </div>
    );
  }

  const gradeColor = GRADE_COLOR[health.grade] ?? "#8b949e";
  const trailCount =
    health.window - health.slCount - health.timeCount - health.tpHits;

  return (
    <div
      style={{
        border: `1px solid ${gradeColor}`,
        borderRadius: 6,
        padding: "6px 10px",
        fontSize: 11,
        color: "#e6edf3",
        marginTop: 6,
        lineHeight: 1.7,
      }}
    >
      <div style={{ color: gradeColor, fontWeight: 700, marginBottom: 4 }}>
        Desk Health: {health.grade} ({health.window} trades)
      </div>

      <div>
        {CHECK_ICON(health.expectancyPass)} Expectancy: ${health.expectancy.toFixed(2)}
      </div>

      <div>
        {CHECK_ICON(health.winRatePass)} Win Rate: {(health.winRate * 100).toFixed(1)}%
      </div>

      <div>
        {CHECK_ICON(health.feePass)} Fee/Gross: {(health.feePctOfAbsGross * 100).toFixed(1)}%
      </div>

      <div>
        {CHECK_ICON(health.pfPass)} Profit Factor:{" "}
        {health.profitFactor === Infinity ? "∞" : health.profitFactor.toFixed(2)}
      </div>

      <div>
        {CHECK_ICON(health.tpHitPass)} TP Hits: {health.tpHits}/{health.window}
      </div>

      <div style={{ color: "#8b949e", marginTop: 4 }}>
        SL:{health.slCount} TIME:{health.timeCount} TRAIL:{trailCount}
      </div>
    </div>
  );
}
