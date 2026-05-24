"use client";

import type { ScorecardAction } from "@/lib/futuresScorecardActions";
import { formatScorecardActionEnv } from "@/lib/futuresScorecardActions";
import type { DeskRollingPnLScorecard } from "@/lib/futuresDeskPnLTracker";

const SEV_COLOR: Record<ScorecardAction["severity"], string> = {
  OK: "#3fb950",
  WARN: "#d29922",
  ACT: "#f85149",
};

interface Props {
  scorecard: DeskRollingPnLScorecard | null;
  action: ScorecardAction | null;
}

export function ScorecardActionPanel({ scorecard, action }: Props) {
  if (!scorecard || !action) return null;
  if (scorecard.paperReadyHint === "ON_TRACK" && action.severity === "OK") return null;

  const sevColor = SEV_COLOR[action.severity];
  const envText = formatScorecardActionEnv(action);

  const copyEnv = () => {
    void navigator.clipboard.writeText(envText).then(() => {
      console.info("[ScorecardAction] Env lines copied to clipboard");
    });
  };

  return (
    <div
      style={{
        border: "1px solid #21262d",
        borderRadius: 8,
        padding: "8px 12px",
        marginTop: 8,
        background: "#161b22",
        fontSize: 11,
        color: "#e6edf3",
      }}
    >
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <span style={{ fontWeight: 700, color: sevColor }}>
          Closed-loop fix — {action.severity} · {action.action.replace(/_/g, " ")}
        </span>
      </div>
      <p style={{ margin: "8px 0", color: "#c9d1d9", lineHeight: 1.45 }}>{action.rationale}</p>
      {action.worstStrategyId != null ? (
        <p style={{ margin: "0 0 8px", fontSize: 10, color: "#8b949e" }}>
          Worst rotation candidate:{" "}
          <span style={{ color: "#f85149" }}>
            #{action.worstStrategyId} {action.worstStrategyName ?? ""}
          </span>{" "}
          — use Monitor runtime blocklist (manual).
        </p>
      ) : null}
      {action.suggestedEnv && Object.keys(action.suggestedEnv).length > 0 ? (
        <div style={{ marginTop: 8 }}>
          <pre
            style={{
              fontSize: 10,
              background: "#0d1117",
              padding: 8,
              borderRadius: 4,
              overflow: "auto",
              color: "#8b949e",
            }}
          >
            {envText}
          </pre>
          <button
            type="button"
            onClick={copyEnv}
            style={{
              marginTop: 6,
              fontSize: 11,
              padding: "4px 12px",
              background: "#0d1117",
              border: "1px solid #58a6ff",
              borderRadius: 4,
              cursor: "pointer",
              color: "#58a6ff",
            }}
          >
            Copy env fix
          </button>
          <span style={{ marginLeft: 8, fontSize: 9, color: "#6e7681" }}>
            Recommendation only — restart dev server after editing .env.local
          </span>
        </div>
      ) : null}
    </div>
  );
}
