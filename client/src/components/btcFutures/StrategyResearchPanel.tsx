"use client";

import { useCallback, useEffect, useState } from "react";
import {
  DeskBanner,
  DeskButton,
  DeskCard,
  DeskChip,
  DeskDataTable,
  DeskEmptyState,
  DeskSectionHeader,
  type DeskColumn,
} from "@/components/desk/ui";
import { computeVerdict, type ResearchVerdict } from "@/lib/btcFtResearch";
import type { ResearchAggRow } from "@/lib/paperTradesAnalytics";

type ResearchRow = ResearchAggRow & { verdict: ResearchVerdict };

type StrategyResearchPanelProps = {
  cloudAccountKey: string | null;
  storageNamespace: string;
  winners: number[];
  retiredIds: ReadonlySet<number>;
  onPromote: (id: number) => void;
  onRetire: (id: number) => void;
  onUnretire: (id: number) => void;
  onAutoRetireLosers?: (ids: number[]) => void;
  poolIds?: number[];
};

function VerdictChip({ verdict }: { verdict: ResearchVerdict }) {
  const map: Record<ResearchVerdict, string> = {
    WINNER: "bg-emerald-100 text-emerald-700 border-emerald-200",
    LOSER: "bg-rose-100 text-rose-700 border-rose-200",
    CANDIDATE: "bg-amber-100 text-amber-700 border-amber-200",
    INSUFFICIENT_DATA: "bg-zinc-100 text-zinc-500 border-zinc-200",
  };
  return (
    <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide ${map[verdict]}`}>
      {verdict === "INSUFFICIENT_DATA" ? "INSUF." : verdict}
    </span>
  );
}

function fmtUsd(n: number) {
  return `${n >= 0 ? "+" : ""}$${n.toFixed(2)}`;
}
function fmtPct(n: number | null) {
  if (n === null) return "—";
  return `${n.toFixed(1)}%`;
}

export function StrategyResearchPanel({
  cloudAccountKey,
  storageNamespace,
  winners,
  retiredIds,
  onPromote,
  onRetire,
  onUnretire,
  onAutoRetireLosers,
  poolIds,
}: StrategyResearchPanelProps) {
  const [rows, setRows] = useState<ResearchRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [windowDays, setWindowDays] = useState(60);
  const [exportMessage, setExportMessage] = useState<string | null>(null);

  const fetchStats = useCallback(async () => {
    if (!cloudAccountKey) return;
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams({
        account_key: cloudAccountKey,
        window_days: String(windowDays),
      });
      if (poolIds && poolIds.length > 0) {
        params.set("pool_ids", poolIds.join(","));
      }
      const res = await fetch(`/api/paper-trades/strategy-research?${params.toString()}`, {
        credentials: "include",
        cache: "no-store",
      });
      const json = (await res.json()) as { ok: boolean; stats?: ResearchAggRow[]; error?: string };
      if (!json.ok || !json.stats) {
        setError(json.error ?? "Failed to load research stats");
        return;
      }
      const withVerdict: ResearchRow[] = json.stats.map((s) => ({
        ...s,
        verdict: computeVerdict(s),
      }));
      setRows(withVerdict);
      const loserIds = withVerdict
        .filter((r) => r.verdict === "LOSER" && !retiredIds.has(r.strategyId))
        .map((r) => r.strategyId);
      if (loserIds.length > 0) onAutoRetireLosers?.(loserIds);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Network error");
    } finally {
      setLoading(false);
    }
  }, [cloudAccountKey, windowDays, poolIds, retiredIds, onAutoRetireLosers]);

  useEffect(() => {
    void fetchStats();
  }, [fetchStats]);

  const isWinner = (id: number) => winners.includes(id);
  const isRetired = (id: number) => retiredIds.has(id);
  const winnerIds = [...new Set(winners)].sort((a, b) => a - b);
  const winnersEnvLine = `NEXT_PUBLIC_BTC_FT_STRATEGY_IDS=${winnerIds.join(",")}`;
  const downloadWinnersJson = useCallback(() => {
    setExportMessage(null);
    const payload = {
      storageNamespace,
      exportedAt: new Date().toISOString(),
      strategyIds: winnerIds,
      env: winnersEnvLine,
    };
    const blob = new Blob([JSON.stringify(payload, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "btc-ft-winners.json";
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
    setExportMessage(`Exported ${winnerIds.length} winners.`);
  }, [storageNamespace, winnerIds, winnersEnvLine]);

  const copyWinnersEnvLine = useCallback(async () => {
    setExportMessage(null);
    try {
      await navigator.clipboard.writeText(winnersEnvLine);
      setExportMessage("Copied NEXT_PUBLIC_BTC_FT_STRATEGY_IDS line.");
    } catch {
      setExportMessage(winnersEnvLine);
    }
  }, [winnersEnvLine]);

  const persistPromotion = useCallback(async (strategyId: number, status: "winner" | "retired") => {
    if (!cloudAccountKey) return;
    try {
      await fetch("/api/paper-trades/strategy-research", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ strategyId, status, note: "BTC FT research tournament" }),
      });
    } catch {
      // Optional cloud promotion mirror only; localStorage remains source of truth for UI.
    }
  }, [cloudAccountKey]);

  const columns: DeskColumn<ResearchRow>[] = [
    {
      id: "id",
      header: "ID",
      cell: (r) => <span className="desk-mono text-xs">{r.strategyId}</span>,
    },
    {
      id: "name",
      header: "Strategy",
      cell: (r) => (
        <div>
          <div className="desk-body-md font-medium" style={{ fontSize: 12 }}>{r.strategyName}</div>
          {isWinner(r.strategyId) && (
            <span className="text-[10px] font-bold text-emerald-600 uppercase tracking-wide">★ Winner</span>
          )}
          {isRetired(r.strategyId) && (
            <span className="text-[10px] font-bold text-zinc-400 uppercase tracking-wide">⏸ Retired</span>
          )}
        </div>
      ),
    },
    {
      id: "trades",
      header: "Trades",
      align: "right",
      cell: (r) => <span className="desk-mono">{r.tradeCount}</span>,
    },
    {
      id: "sumNet",
      header: "Sum net",
      align: "right",
      cell: (r) => (
        <span className={`desk-mono font-semibold ${r.sumNet >= 0 ? "desk-pnl-positive" : "desk-pnl-negative"}`}>
          {fmtUsd(r.sumNet)}
        </span>
      ),
    },
    {
      id: "exp",
      header: "Expectancy",
      align: "right",
      cell: (r) => (
        <span className={`desk-mono ${r.expectancy >= 0 ? "desk-pnl-positive" : "desk-pnl-negative"}`}>
          {fmtUsd(r.expectancy)}
        </span>
      ),
    },
    {
      id: "wr",
      header: "Win%",
      align: "right",
      cell: (r) => <span className="desk-mono">{fmtPct(r.winRate * 100)}</span>,
    },
    {
      id: "fee",
      header: "Fee%",
      align: "right",
      cell: (r) => <span className="desk-mono text-zinc-500">{fmtPct(r.feePctOfGross)}</span>,
    },
    {
      id: "hold",
      header: "Avg hold",
      align: "right",
      cell: (r) => (
        <span className="desk-mono text-zinc-500">{r.avgHoldMin !== null ? `${r.avgHoldMin.toFixed(0)}m` : "—"}</span>
      ),
    },
    {
      id: "verdict",
      header: "Verdict",
      cell: (r) => <VerdictChip verdict={r.verdict} />,
    },
    {
      id: "actions",
      header: "Actions",
      cell: (r) => (
        <div style={{ display: "flex", gap: 4 }}>
          {!isWinner(r.strategyId) && r.verdict === "WINNER" && (
            <DeskButton
              variant="tonal"
              onClick={() => {
                onPromote(r.strategyId);
                void persistPromotion(r.strategyId, "winner");
              }}
              style={{ fontSize: 10, padding: "2px 8px" }}
            >
              Promote ★
            </DeskButton>
          )}
          {isWinner(r.strategyId) && (
            <DeskChip tone="success">Winner</DeskChip>
          )}
          {!isRetired(r.strategyId) ? (
            <DeskButton
              variant="outlined"
              onClick={() => {
                onRetire(r.strategyId);
                void persistPromotion(r.strategyId, "retired");
              }}
              style={{ fontSize: 10, padding: "2px 8px" }}
            >
              Retire
            </DeskButton>
          ) : (
            <DeskButton
              variant="outlined"
              onClick={() => onUnretire(r.strategyId)}
              style={{ fontSize: 10, padding: "2px 8px" }}
            >
              Restore
            </DeskButton>
          )}
        </div>
      ),
    },
  ];

  const winnerCount = rows.filter((r) => r.verdict === "WINNER").length;
  const loserCount = rows.filter((r) => r.verdict === "LOSER").length;
  const candidateCount = rows.filter((r) => r.verdict === "CANDIDATE").length;
  const insufficientCount = rows.filter((r) => r.verdict === "INSUFFICIENT_DATA").length;

  return (
    <DeskCard>
      <DeskSectionHeader
        title="Strategy Research / Tournament"
        subtitle={`${windowDays}d window · ${rows.length} strategies · paper only`}
        actions={
          <>
            <DeskButton
              variant="outlined"
              onClick={downloadWinnersJson}
              disabled={winnerIds.length === 0}
              style={{ fontSize: 11 }}
            >
              Export winners JSON
            </DeskButton>
            <DeskButton
              variant="outlined"
              onClick={() => void copyWinnersEnvLine()}
              disabled={winnerIds.length === 0}
              style={{ fontSize: 11 }}
            >
              Copy env line
            </DeskButton>
            {[30, 60, 90].map((d) => (
              <DeskButton
                key={d}
                variant={windowDays === d ? "tonal" : "outlined"}
                onClick={() => setWindowDays(d)}
                style={{ fontSize: 11 }}
              >
                {d}d
              </DeskButton>
            ))}
            <DeskButton variant="tonal" onClick={() => void fetchStats()} disabled={loading}>
              {loading ? "Loading…" : "Refresh"}
            </DeskButton>
          </>
        }
      />

      {/* Summary chips */}
      <div style={{ display: "flex", gap: 8, marginBottom: 12, flexWrap: "wrap" }}>
        <DeskChip tone="success">Winner {winnerCount}</DeskChip>
        <DeskChip tone="error">Loser {loserCount}</DeskChip>
        <DeskChip tone="warning">~ {candidateCount} Candidates</DeskChip>
        <DeskChip tone="default">? {insufficientCount} Insufficient data</DeskChip>
        {winners.length > 0 && (
          <DeskChip tone="success">Promoted {winners.length}</DeskChip>
        )}
      </div>

      {/* Promote hint */}
      {winnerCount > 0 && (
        <div style={{ marginBottom: 12 }}>
          <DeskBanner variant="info" title="Winners ready to promote">
            {winnerCount} strateg{winnerCount === 1 ? "y has" : "ies have"} WINNER verdict.
            Promote them, then switch to production mode (RESEARCH_MODE=0) with WINNERS-only roster.
            See{" "}
            <a href="#btc-ft-research-mode" style={{ textDecoration: "underline", fontWeight: 600 }}>
              PAPER_DESK_RUNBOOK.md#promote-winners
            </a>.
          </DeskBanner>
        </div>
      )}

      {error && (
        <div style={{ marginBottom: 12 }}>
          <DeskBanner variant="warning" title="Load error">{error}</DeskBanner>
        </div>
      )}

      {exportMessage && (
        <div style={{ marginBottom: 12 }}>
          <DeskBanner variant="info" title="Winners export">{exportMessage}</DeskBanner>
        </div>
      )}

      {!cloudAccountKey && (
        <div style={{ marginBottom: 12 }}>
          <DeskBanner variant="warning" title="Sign-in required">
            Strategy research requires a Supabase account to read closed paper trades.
          </DeskBanner>
        </div>
      )}

      <DeskDataTable
        minWidth={900}
        columns={columns}
        rows={rows}
        getRowKey={(r) => String(r.strategyId)}
        empty={
          loading
            ? <DeskEmptyState title="Loading…" subtitle="Fetching research stats from Supabase." />
            : <DeskEmptyState title="No data yet" subtitle="Run research mode for at least 10 trades per strategy." />
        }
      />

      <p className="desk-label-md" style={{ marginTop: 8, color: "var(--desk-on-surface-variant)" }}>
        Verdicts: INSUFFICIENT_DATA (&lt;10 trades) · CANDIDATE (10–19) · WINNER (≥20, exp&gt;0, sum&gt;0) ·
        LOSER (≥15, sum&lt;−$2 or exp&lt;−$0.10). Auto-retire in research mode: LOSER rows skipped next rotation.
      </p>
    </DeskCard>
  );
}
