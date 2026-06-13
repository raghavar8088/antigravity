"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Badge, type BadgeVariant } from "@/components/ui/Badge";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { Sparkline } from "@/components/ui/Sparkline";
import { PnlValue } from "@/components/ui/PnlValue";
import { cn } from "@/components/ui/cn";
import type { AlphaTier, AlphaTierSummary, StrategyRow, WinnersOnlyGate } from "@/types/strategy";

// ── Helpers ──────────────────────────────────────────────────────────────────
const TIER_VARIANT: Record<AlphaTier, BadgeVariant> = {
  A: "profit", B: "info", C: "caution", UNRANKED: "neutral",
};
const STATUS_VARIANT: Record<string, BadgeVariant> = {
  ACTIVE: "profit", PAUSED: "caution", DISQUALIFIED: "neutral",
};

function winRateColor(v: number): string {
  if (v >= 60) return "text-[var(--color-profit)]";
  if (v >= 45) return "text-[var(--color-caution)]";
  return "text-[var(--color-loss)]";
}
function sharpeColor(v: number): string {
  if (v >= 1.5) return "text-[var(--color-profit)]";
  if (v >= 0.5) return "text-[var(--color-caution)]";
  return "text-[var(--color-loss)]";
}
function ddColor(v: number): string {
  if (v <= 10) return "text-[var(--color-profit)]";
  if (v <= 20) return "text-[var(--color-caution)]";
  return "text-[var(--color-loss)]";
}
function relTime(iso: string | null): string {
  if (!iso) return "—";
  const s = (Date.now() - new Date(iso).getTime()) / 1000;
  if (s < 60) return `${Math.floor(s)}s`;
  if (s < 3600) return `${Math.floor(s/60)}m`;
  return `${Math.floor(s/3600)}h`;
}

// ── Main Component ────────────────────────────────────────────────────────────
export function StrategyRegistryDashboard({ onSelectStrategy }: { onSelectStrategy: (id: string) => void }) {
  const [rows, setRows] = useState<StrategyRow[]>([]);
  const [tierSummary, setTierSummary] = useState<AlphaTierSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [filterExchange, setFilterExchange] = useState("ALL");
  const [filterStatus, setFilterStatus] = useState("ALL");
  const [filterTier, setFilterTier] = useState("ALL");
  const [sortBy, setSortBy] = useState<keyof StrategyRow>("dailyPnl");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");

  // Enable/disable modals
  const [enableTarget, setEnableTarget] = useState<StrategyRow | null>(null);
  const [pauseTarget, setPauseTarget] = useState<StrategyRow | null>(null);
  const [overrideTarget, setOverrideTarget] = useState<StrategyRow | null>(null);
  const [overrideTyped, setOverrideTyped] = useState("");
  const [acting, setActing] = useState(false);

  const load = useCallback(async () => {
    try {
      const res = await fetch("/api/strategies", { cache: "no-store" });
      const data = await res.json();
      if (Array.isArray(data.strategies)) setRows(data.strategies);
      if (Array.isArray(data.tierSummary)) setTierSummary(data.tierSummary);
    } catch { /* engine offline */ }
    finally { setLoading(false); }
  }, []);

  useEffect(() => { load(); }, [load]);

  const filtered = useMemo(() => {
    let r = rows;
    if (filterExchange !== "ALL") r = r.filter((s) => s.exchange === filterExchange);
    if (filterStatus !== "ALL")   r = r.filter((s) => s.status === filterStatus);
    if (filterTier !== "ALL")     r = r.filter((s) => s.alphaTier === filterTier);
    if (search.trim()) {
      const q = search.toLowerCase();
      r = r.filter((s) => [s.name, s.id, s.exchange, s.family].join(" ").toLowerCase().includes(q));
    }
    return [...r].sort((a, b) => {
      const av = a[sortBy] as number | string, bv = b[sortBy] as number | string;
      const cmp = typeof av === "number" && typeof bv === "number" ? av - bv : String(av).localeCompare(String(bv));
      return sortDir === "desc" ? -cmp : cmp;
    });
  }, [rows, filterExchange, filterStatus, filterTier, search, sortBy, sortDir]);

  function toggleSort(col: keyof StrategyRow) {
    if (sortBy === col) setSortDir((d) => d === "desc" ? "asc" : "desc");
    else { setSortBy(col); setSortDir("desc"); }
  }

  async function doAction(id: string, action: "enable" | "pause" | "override_winners_only", reason?: string) {
    setActing(true);
    try {
      await fetch(`/api/strategies/${id}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action, reason }),
      });
      await load();
    } finally {
      setActing(false);
      setEnableTarget(null); setPauseTarget(null); setOverrideTarget(null); setOverrideTyped("");
    }
  }

  // Tier summary cards
  const tierOrder: AlphaTier[] = ["A", "B", "C", "UNRANKED"];
  const tierLabel: Record<AlphaTier, string> = { A: "Tier A", B: "Tier B", C: "Tier C", UNRANKED: "Disqualified" };
  const disqualifiedCount = rows.filter((r) => r.winnersOnlyGate === "FAIL").length;

  return (
    <>
      {/* Tier summary cards */}
      <div className="grid grid-cols-4 gap-2 px-4 pt-4 pb-2">
        {tierOrder.map((tier) => {
          const ts = tierSummary.find((t) => t.tier === tier);
          const count = tier === "UNRANKED" ? disqualifiedCount : (ts?.count ?? 0);
          const alloc = ts?.capitalAllocated ?? 0;
          return (
            <div
              key={tier}
              className={cn(
                "rounded-lg border p-3",
                tier === "UNRANKED"
                  ? "border-[var(--color-border)] bg-[var(--color-bg-surface)] opacity-60"
                  : "border-[var(--color-border)] bg-[var(--color-bg-surface)]",
              )}
            >
              <div className="flex items-center justify-between mb-1">
                <Badge variant={TIER_VARIANT[tier]} size="sm">{tierLabel[tier]}</Badge>
                <span className="text-[11px] font-mono tnum text-[var(--color-text-secondary)]">{count} strategies</span>
              </div>
              <div className="text-[12px] font-mono tnum text-[var(--color-text-primary)]">
                {tier === "UNRANKED" ? "₹0 — WINNERS_ONLY GATE" : alloc > 0 ? `₹${(alloc/100000).toFixed(1)}L allocated` : "—"}
              </div>
            </div>
          );
        })}
      </div>

      {/* Search + filters */}
      <div className="flex items-center gap-2 px-4 py-2 border-b border-[var(--color-border)] flex-wrap">
        <input
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search strategies…"
          className="text-[12px] px-3 py-1.5 rounded border border-[var(--color-border)] bg-[var(--color-bg-elevated)] text-[var(--color-text-primary)] outline-none focus:border-[var(--color-border-focus)] w-52"
          aria-label="Search strategies"
        />
        {[
          { label: "Exchange", val: filterExchange, set: setFilterExchange, opts: ["ALL","NSE","BSE","BINANCE","DELTA"] },
          { label: "Status",   val: filterStatus,   set: setFilterStatus,   opts: ["ALL","ACTIVE","PAUSED","DISQUALIFIED"] },
          { label: "Tier",     val: filterTier,     set: setFilterTier,     opts: ["ALL","A","B","C"] },
        ].map(({ label, val, set, opts }) => (
          <select
            key={label}
            value={val}
            onChange={(e) => set(e.target.value)}
            className="text-[12px] px-2 py-1.5 rounded border border-[var(--color-border)] bg-[var(--color-bg-elevated)] text-[var(--color-text-secondary)] outline-none"
            aria-label={`Filter by ${label}`}
          >
            {opts.map((o) => <option key={o} value={o}>{label}: {o}</option>)}
          </select>
        ))}
        <span className="ml-auto text-[11px] text-[var(--color-text-muted)]">
          {filtered.length} of {rows.length} strategies
        </span>
      </div>

      {/* Table */}
      <div className="flex-1 overflow-auto">
        {loading ? (
          <div className="p-4 space-y-2">
            {Array.from({ length: 8 }).map((_, i) => <div key={i} className="skeleton h-9 rounded" />)}
          </div>
        ) : (
          <table className="w-full" role="table">
            <thead className="sticky top-0 bg-[var(--color-bg-surface)]">
              <tr className="text-[10px] text-[var(--color-text-muted)]">
                {([
                  ["Name", "name"], ["Exch", "exchange"], ["Status", "status"],
                  ["Tier", "alphaTier"], ["Session", null], ["Gate", null],
                  ["Win%", "winRatePct"], ["Sharpe", "sharpeRatio"], ["Max DD%", "maxDrawdownPct"],
                  ["Daily P&L", "dailyPnl"], ["Positions", "openPositions"],
                  ["Signals", "signalsToday"], ["Last Signal", "lastSignalAt"],
                  ["7d", null], ["Actions", null],
                ] as [string, keyof StrategyRow | null][]).map(([h, col]) => (
                  <th
                    key={h}
                    className={cn(
                      "px-2 py-1.5 text-left font-medium whitespace-nowrap",
                      col ? "cursor-pointer hover:text-[var(--color-text-primary)]" : "",
                    )}
                    onClick={() => col && toggleSort(col)}
                  >
                    {h}{col && sortBy === col ? (sortDir === "desc" ? " ▼" : " ▲") : ""}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {filtered.map((row) => (
                <StrategyTableRow
                  key={row.id}
                  row={row}
                  onView={() => onSelectStrategy(row.id)}
                  onEnable={() => setEnableTarget(row)}
                  onPause={() => setPauseTarget(row)}
                  onOverride={() => setOverrideTarget(row)}
                />
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Enable confirmation */}
      <ConfirmModal
        open={enableTarget !== null}
        onOpenChange={(v) => { if (!v) setEnableTarget(null); }}
        title={`Enable: ${enableTarget?.name ?? ""}`}
        description={`Capital allocated: ${enableTarget?.exchange === "NSE" || enableTarget?.exchange === "BSE" ? "₹" : "$"}—. This strategy will immediately begin scanning for entry signals.`}
        consequence="Enabling this strategy may immediately place orders. Existing risk gates still apply."
        confirmLabel="Enable Strategy"
        destructive={false}
        onConfirm={() => enableTarget && doAction(enableTarget.id, "enable")}
        loading={acting}
      />

      {/* Pause confirmation */}
      <ConfirmModal
        open={pauseTarget !== null}
        onOpenChange={(v) => { if (!v) setPauseTarget(null); }}
        title={`Pause: ${pauseTarget?.name ?? ""}`}
        description={`Open positions (${pauseTarget?.openPositions ?? 0}) will NOT be auto-closed. Existing positions remain open until strategy logic exits them.`}
        consequence={`Unrealized P&L on ${pauseTarget?.openPositions ?? 0} positions remains at risk. Only new entries are blocked.`}
        confirmLabel="Pause Strategy"
        destructive
        onConfirm={() => pauseTarget && doAction(pauseTarget.id, "pause")}
        loading={acting}
      />

      {/* WINNERS_ONLY Override */}
      <ConfirmModal
        open={overrideTarget !== null}
        onOpenChange={(v) => { if (!v) { setOverrideTarget(null); setOverrideTyped(""); } }}
        title="⚠ WINNERS_ONLY Gate Override"
        description={`${overrideTarget?.name} failed the gate: ${overrideTarget?.winnersOnlyFailReason ?? "performance below threshold"}. Admin override required.`}
        consequence="Overriding the WINNERS_ONLY gate allows a disqualified strategy to trade capital. This creates an audit log entry. Override should only be used for deliberate testing."
        confirmLabel="Override Gate"
        typeToConfirm={overrideTarget?.name ?? ""}
        destructive
        onConfirm={() => overrideTarget && doAction(overrideTarget.id, "override_winners_only", `Manual override by operator`)}
        loading={acting}
      />
    </>
  );
}

function GateIndicator({ gate }: { gate: WinnersOnlyGate }) {
  if (gate === "PASS") return <span className="text-[var(--color-profit)] text-[10px]">✅ PASS</span>;
  if (gate === "FAIL") return <span className="text-[var(--color-loss)] text-[10px]">⛔ FAIL</span>;
  return <span className="text-[var(--color-caution)] text-[10px]">🔒 PENDING</span>;
}

function StrategyTableRow({
  row, onView, onEnable, onPause, onOverride,
}: {
  row: StrategyRow;
  onView: () => void;
  onEnable: () => void;
  onPause: () => void;
  onOverride: () => void;
}) {
  return (
    <tr className="border-b border-[var(--color-border)] hover:bg-[var(--color-bg-elevated)] text-[11px]">
      <td className="px-2 py-1.5 max-w-[140px]">
        <button type="button" onClick={onView} className="text-[var(--color-info)] hover:underline truncate block max-w-full text-left">
          {row.name}
        </button>
      </td>
      <td className="px-2 py-1.5">
        <Badge variant="neutral" size="sm">{row.exchange}</Badge>
      </td>
      <td className="px-2 py-1.5">
        <Badge variant={STATUS_VARIANT[row.status] ?? "neutral"} size="sm" dot={row.status === "ACTIVE"}>
          {row.status}
        </Badge>
      </td>
      <td className="px-2 py-1.5">
        <Badge variant={TIER_VARIANT[row.alphaTier]} size="sm">{row.alphaTier}</Badge>
      </td>
      <td className="px-2 py-1.5">
        <Badge variant={row.sessionType === "SESSION_GATED" ? "caution" : "info"} size="sm">
          {row.sessionType === "SESSION_GATED" ? "9:15–15:30" : "24/7"}
        </Badge>
      </td>
      <td className="px-2 py-1.5"><GateIndicator gate={row.winnersOnlyGate} /></td>
      <td className={cn("px-2 py-1.5 font-mono tnum text-right", winRateColor(row.winRatePct))}>
        {row.winRatePct.toFixed(1)}%
      </td>
      <td className={cn("px-2 py-1.5 font-mono tnum text-right", sharpeColor(row.sharpeRatio))}>
        {row.sharpeRatio.toFixed(2)}
      </td>
      <td className={cn("px-2 py-1.5 font-mono tnum text-right", ddColor(row.maxDrawdownPct))}>
        {row.maxDrawdownPct.toFixed(1)}%
      </td>
      <td className="px-2 py-1.5 text-right">
        <PnlValue value={row.dailyPnl} showSign size="sm" />
      </td>
      <td className="px-2 py-1.5 font-mono tnum text-right text-[var(--color-text-secondary)]">
        {row.openPositions}
      </td>
      <td className="px-2 py-1.5 font-mono tnum text-right text-[var(--color-text-secondary)]">
        {row.signalsToday}
      </td>
      <td className="px-2 py-1.5 font-mono text-[10px] text-[var(--color-text-muted)]">
        {relTime(row.lastSignalAt)}
      </td>
      <td className="px-2 py-1.5">
        <Sparkline data={row.intradayHistory} width={56} height={18} />
      </td>
      <td className="px-2 py-1.5">
        <div className="flex items-center gap-1">
          <button type="button" onClick={onView} className="text-[10px] text-[var(--color-info)] hover:underline">View</button>
          {row.winnersOnlyGate === "FAIL" ? (
            <button
              type="button"
              onClick={onOverride}
              className="text-[10px] text-[var(--color-loss)] hover:underline"
              aria-label={`Request WINNERS_ONLY override for ${row.name}`}
            >
              Override
            </button>
          ) : row.status === "ACTIVE" ? (
            <button type="button" onClick={onPause} className="text-[10px] text-[var(--color-caution)] hover:underline">Pause</button>
          ) : row.status === "PAUSED" ? (
            <button type="button" onClick={onEnable} className="text-[10px] text-[var(--color-profit)] hover:underline">Enable</button>
          ) : null}
        </div>
      </td>
    </tr>
  );
}
