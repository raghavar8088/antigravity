"use client";

/**
 * Replay + Walk-Forward Lab — test BTC Future Trading strategies on historical data
 * before recommending them to the paper roster.
 *
 * Hard rules:
 * - Paper/research only. No live Delta order placement.
 * - No forced trades, no threshold lowering, no gate bypassing.
 * - Recommendations only; no auto-apply roster changes.
 * - No guaranteed profit language.
 */

import { useCallback, useState } from "react";
import { DeskButton } from "@/components/desk/ui";
import type { ReplayWalkForwardRank } from "@/lib/replayWalkForwardRanker";

// ─── Types ────────────────────────────────────────────────────────────────────

type ReplaySummary = {
  totalTrades: number;
  sumNet: number;
  expectancy: number;
  finalBalance: number;
  exitReasonCounts: Record<string, number>;
};

type ReplayLabResponse = {
  ok: boolean;
  days?: number;
  barsProcessed?: number;
  candlesLoaded?: number;
  coverageDays?: number;
  requestedDays?: number;
  generatedAt?: string;
  fixturePath?: string;
  summary?: ReplaySummary;
  rankings?: ReplayWalkForwardRank[];
  promoted?: number[];
  canvasNote?: string;
  error?: string;
  fetchCommand?: string;
};

// ─── Recommendation colour mapping ───────────────────────────────────────────

const REC_COLOR: Record<string, string> = {
  PROMOTE: "var(--desk-success)",
  KEEP: "var(--desk-primary)",
  WATCH: "var(--desk-warning)",
  DISABLE: "var(--desk-error)",
  INSUFFICIENT: "var(--desk-on-surface-muted)",
};

// ─── Components ──────────────────────────────────────────────────────────────

function MetricCard({ label, value, color }: { label: string; value: string | number; color?: string }) {
  return (
    <div
      style={{
        padding: "8px 12px",
        border: "1px solid var(--desk-outline-variant)",
        borderRadius: "var(--desk-radius-card)",
        background: "var(--desk-surface)",
        minWidth: 80,
      }}
    >
      <div style={{ fontSize: "0.625rem", color: "var(--desk-on-surface-muted)", textTransform: "uppercase", letterSpacing: "0.06em" }}>
        {label}
      </div>
      <div style={{ fontSize: "1.125rem", fontWeight: 700, color: color ?? "var(--desk-on-surface)", lineHeight: 1.2, marginTop: 2 }}>
        {value}
      </div>
    </div>
  );
}

function RecBadge({ rec }: { rec: string }) {
  return (
    <span
      style={{
        fontSize: "0.625rem",
        fontWeight: 600,
        color: REC_COLOR[rec] ?? "var(--desk-on-surface-muted)",
        padding: "1px 5px",
        border: `1px solid ${REC_COLOR[rec] ?? "var(--desk-outline-variant)"}`,
        borderRadius: 4,
        whiteSpace: "nowrap",
      }}
    >
      {rec}
    </span>
  );
}

// ─── Main panel ───────────────────────────────────────────────────────────────

export function ReplayWalkForwardLab() {
  const [days, setDays] = useState<7 | 14 | 30 | 60 | 90>(30);
  const [strategyIdsInput, setStrategyIdsInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<ReplayLabResponse | null>(null);

  const runReplay = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const idsParam = strategyIdsInput.trim() ? `&strategy_ids=${encodeURIComponent(strategyIdsInput.trim())}` : "";
      const res = await fetch(`/api/replay-walkforward?days=${days}${idsParam}`);
      const json = (await res.json()) as ReplayLabResponse;
      setResult(json);
      if (!json.ok) setError(json.error ?? "Unknown error");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Fetch failed");
    } finally {
      setLoading(false);
    }
  }, [days, strategyIdsInput]);

  const copyEnv = useCallback(() => {
    if (!result?.promoted?.length) return;
    const ids = result.promoted.join(",");
    void navigator.clipboard.writeText(`NEXT_PUBLIC_BTC_FT_STRATEGY_IDS=${ids}`);
  }, [result]);

  const rankings = result?.rankings ?? [];

  // Coverage check
  const coverageDays = result?.coverageDays;
  const requestedDays = result?.requestedDays ?? days;
  const coveragePct = coverageDays != null ? (coverageDays / requestedDays) * 100 : null;
  const coverageLow = coveragePct != null && coveragePct < 80;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>

      {/* Header */}
      <div>
        <p style={{ margin: 0, fontSize: "0.75rem", fontWeight: 700, color: "var(--desk-on-surface)" }}>
          Replay + Walk-Forward Lab
        </p>
        <p style={{ margin: 0, fontSize: "0.6875rem", color: "var(--desk-on-surface-muted)" }}>
          Test strategies on historical BTC data before recommending to the paper roster.
          Recommendations only — operator decides whether to apply.
        </p>
      </div>

      {/* Controls */}
      <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "flex-end" }}>
        {/* Days selector */}
        <div>
          <p style={{ margin: "0 0 4px", fontSize: "0.625rem", color: "var(--desk-on-surface-muted)", textTransform: "uppercase" }}>
            Window
          </p>
          <div style={{ display: "flex", gap: 4 }}>
            {([7, 14, 30, 60, 90] as const).map((d) => (
              <DeskButton
                key={d}
                variant={days === d ? "filled" : "outlined"}
                onClick={() => setDays(d)}
                style={{ minHeight: 28, padding: "0 10px", fontSize: "0.6875rem" }}
              >
                {d}d
              </DeskButton>
            ))}
          </div>
        </div>

        {/* Strategy IDs input */}
        <div style={{ flex: 1, minWidth: 160 }}>
          <p style={{ margin: "0 0 4px", fontSize: "0.625rem", color: "var(--desk-on-surface-muted)", textTransform: "uppercase" }}>
            Strategy IDs (blank = all)
          </p>
          <input
            type="text"
            value={strategyIdsInput}
            onChange={(e) => setStrategyIdsInput(e.target.value)}
            placeholder="e.g. 91,92,95,96"
            style={{
              width: "100%",
              height: 28,
              padding: "0 8px",
              fontSize: "0.6875rem",
              border: "1px solid var(--desk-outline-variant)",
              borderRadius: 4,
              background: "var(--desk-surface)",
              color: "var(--desk-on-surface)",
              boxSizing: "border-box",
            }}
          />
        </div>

        {/* Actions */}
        <DeskButton
          variant="filled"
          onClick={() => void runReplay()}
          style={{ minHeight: 28, padding: "0 16px", fontSize: "0.6875rem" }}
        >
          {loading ? "Running…" : "Run Replay"}
        </DeskButton>
        {(result?.promoted?.length ?? 0) > 0 && (
          <DeskButton
            variant="outlined"
            onClick={copyEnv}
            style={{ minHeight: 28, padding: "0 10px", fontSize: "0.6875rem" }}
          >
            Copy env
          </DeskButton>
        )}
      </div>

      {/* Error / insufficient coverage */}
      {error && (
        <div
          style={{
            padding: "10px 14px",
            border: "1px solid var(--desk-error)",
            borderRadius: "var(--desk-radius-card)",
            background: "var(--desk-surface)",
            fontSize: "0.6875rem",
            color: "var(--desk-error)",
          }}
        >
          {error}
          {result?.fetchCommand && (
            <div style={{ marginTop: 6, color: "var(--desk-on-surface-muted)" }}>
              <strong>Fix: </strong>
              <code style={{ userSelect: "all" }}>{result.fetchCommand}</code>
            </div>
          )}
        </div>
      )}

      {/* Coverage warning (when run succeeded but coverage was marginal) */}
      {!error && coverageLow && coverageDays != null && (
        <div
          style={{
            padding: "10px 14px",
            border: "1px solid var(--desk-warning)",
            borderRadius: "var(--desk-radius-card)",
            background: "var(--desk-surface)",
            fontSize: "0.6875rem",
            color: "var(--desk-warning)",
          }}
        >
          <strong>Replay data is too short.</strong>{" "}
          {`Loaded ${coverageDays.toFixed(1)} days of data (${coveragePct?.toFixed(0)}% of ${requestedDays}d requested).`}
          {" "}Fetch more data before trusting rankings:
          <div style={{ marginTop: 4 }}>
            <code style={{ userSelect: "all" }}>
              {result?.fetchCommand ?? `npm run replay:fetch -- --days=${requestedDays}`}
            </code>
          </div>
        </div>
      )}

      {/* Summary cards */}
      {result?.ok && result.summary && (
        <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
          <MetricCard label="Trades" value={result.summary.totalTrades} />
          <MetricCard
            label="Net PnL"
            value={`${result.summary.sumNet >= 0 ? "+" : ""}$${result.summary.sumNet.toFixed(2)}`}
            color={result.summary.sumNet >= 0 ? "var(--desk-success)" : "var(--desk-error)"}
          />
          <MetricCard
            label="Expectancy"
            value={`${result.summary.expectancy >= 0 ? "+" : ""}$${result.summary.expectancy.toFixed(2)}`}
            color={result.summary.expectancy >= 0 ? "var(--desk-success)" : "var(--desk-error)"}
          />
          <MetricCard
            label="Final balance"
            value={`$${result.summary.finalBalance.toFixed(0)}`}
          />
          <MetricCard
            label="Candles"
            value={(result.candlesLoaded ?? result.barsProcessed ?? 0).toLocaleString()}
            color={coverageLow ? "var(--desk-warning)" : undefined}
          />
          <MetricCard
            label="Coverage"
            value={coverageDays != null ? `${coverageDays.toFixed(1)}d` : `${result.barsProcessed ?? 0} bars`}
            color={coverageLow ? "var(--desk-warning)" : "var(--desk-success)"}
          />
          <MetricCard label="Promoted" value={result.promoted?.length ?? 0} color="var(--desk-success)" />
        </div>
      )}

      {/* Promoted roster */}
      {(result?.promoted?.length ?? 0) > 0 && (
        <div
          style={{
            padding: "10px 14px",
            border: "1px solid var(--desk-success)",
            borderRadius: "var(--desk-radius-card)",
            background: "var(--desk-surface)",
            fontSize: "0.6875rem",
          }}
        >
          <strong style={{ color: "var(--desk-success)" }}>Recommended roster: </strong>
          <span style={{ color: "var(--desk-on-surface)" }}>
            {result!.promoted!.join(", ")}
          </span>
          <span style={{ color: "var(--desk-on-surface-muted)", marginLeft: 8 }}>
            — Copy env to apply (operator confirms)
          </span>
        </div>
      )}

      {/* No result yet */}
      {!result && !loading && !error && (
        <div
          style={{
            padding: "10px 14px",
            border: "1px solid var(--desk-outline-variant)",
            borderRadius: "var(--desk-radius-card)",
            background: "var(--desk-surface)",
            fontSize: "0.6875rem",
            color: "var(--desk-on-surface-muted)",
          }}
        >
          Select window and press Run Replay to test strategies on historical candles.
          <div style={{ marginTop: 6 }}>
            Need data first?{" "}
            <code style={{ userSelect: "all" }}>npm run replay:fetch -- --days={days}</code>
          </div>
        </div>
      )}

      {/* Rankings table */}
      {rankings.length > 0 && (
        <div>
          <p style={{ margin: "0 0 6px", fontSize: "0.6875rem", fontWeight: 600, color: "var(--desk-on-surface)" }}>
            Strategy Rankings
            {coverageDays != null && (
              <span style={{ fontWeight: 400, color: coverageLow ? "var(--desk-warning)" : "var(--desk-on-surface-muted)", marginLeft: 8 }}>
                ({coverageDays.toFixed(1)}d coverage{coverageLow ? " ⚠" : ""})
              </span>
            )}
          </p>
          <div style={{ overflowX: "auto" }}>
            <table
              style={{
                width: "100%",
                borderCollapse: "collapse",
                fontSize: "0.6875rem",
                color: "var(--desk-on-surface)",
              }}
            >
              <thead>
                <tr style={{ borderBottom: "1px solid var(--desk-outline-variant)" }}>
                  {["Strategy", "Trades", "Expectancy", "Win%", "Fee/Gross", "WF status", "WFE", "Recommendation"].map((h) => (
                    <th
                      key={h}
                      style={{
                        padding: "4px 8px",
                        textAlign: "left",
                        fontWeight: 600,
                        color: "var(--desk-on-surface-muted)",
                        whiteSpace: "nowrap",
                      }}
                    >
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {rankings.map((r) => (
                  <tr key={r.strategyId} style={{ borderBottom: "1px solid var(--desk-outline-variant)" }}>
                    <td style={{ padding: "3px 8px" }}>
                      <span style={{ fontFamily: "monospace", fontSize: "0.625rem", color: "var(--desk-on-surface-muted)" }}>
                        {r.strategyId}
                      </span>
                      {" "}
                      {r.strategyName}
                    </td>
                    <td style={{ padding: "3px 8px" }}>{r.replayTrades}</td>
                    <td
                      style={{
                        padding: "3px 8px",
                        color: r.replayExpectancy >= 0 ? "var(--desk-success)" : "var(--desk-error)",
                      }}
                    >
                      {r.replayExpectancy >= 0 ? "+" : ""}${r.replayExpectancy.toFixed(2)}
                    </td>
                    <td style={{ padding: "3px 8px" }}>
                      {(r.replayWinRate * 100).toFixed(1)}%
                    </td>
                    <td style={{ padding: "3px 8px" }}>
                      {r.replayFeePctOfAbsGross.toFixed(0)}%
                    </td>
                    <td style={{ padding: "3px 8px", color: r.walkForward.aggregatePass ? "var(--desk-success)" : "var(--desk-error)" }}>
                      {r.walkForward.status}
                    </td>
                    <td style={{ padding: "3px 8px" }}>
                      {r.walkForward.aggregateWFE.toFixed(2)}
                    </td>
                    <td style={{ padding: "3px 8px" }}>
                      <RecBadge rec={r.recommendation} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Canvas note */}
      {result?.canvasNote && (
        <p style={{ margin: 0, fontSize: "0.625rem", color: "var(--desk-on-surface-muted)", fontStyle: "italic" }}>
          {result.canvasNote}
        </p>
      )}
    </div>
  );
}
