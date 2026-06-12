"use client";

import { useEffect, useState } from "react";
import type { CandidateQueueEntry, CandidateQueueSummary } from "@/lib/strategyAuthority/portfolioTypes";

function GateIcon({ pass }: { pass: boolean }) {
  return pass
    ? <span className="text-[10px] text-emerald-400 font-bold">✓</span>
    : <span className="text-[10px] text-rose-500 font-bold">✗</span>;
}

function GateProgress({ passed, total = 5 }: { passed: number; total?: number }) {
  return (
    <div className="flex gap-0.5">
      {Array.from({ length: total }).map((_, i) => (
        <div
          key={i}
          className={`h-1.5 flex-1 rounded-sm ${i < passed ? "bg-emerald-600" : "bg-zinc-700"}`}
        />
      ))}
    </div>
  );
}

function CandidateRow({ entry, expanded, onToggle }: {
  entry: CandidateQueueEntry;
  expanded: boolean;
  onToggle: () => void;
}) {
  const gates = [
    { label: "Corr", gate: entry.gate_correlation },
    { label: "Family", gate: entry.gate_family_concentration },
    { label: "Regime", gate: entry.gate_regime },
    { label: "Fit", gate: entry.gate_portfolio_fit },
    { label: "Kelly", gate: entry.gate_allocation },
  ];

  return (
    <>
      <tr
        className={`border-t border-zinc-800/50 cursor-pointer ${expanded ? "bg-zinc-800/30" : "hover:bg-zinc-900/30"}`}
        onClick={onToggle}
      >
        <td className="py-1.5 px-2 font-medium text-zinc-200 max-w-44 truncate">{entry.strategy_name}</td>
        <td className="py-1.5 px-2 text-[10px] text-zinc-500">{entry.family}</td>
        <td className="py-1.5 px-2">
          <div className="w-20"><GateProgress passed={entry.gates_passed} /></div>
        </td>
        <td className="py-1.5 px-2 text-center">
          {entry.all_gates_pass
            ? <span className="rounded bg-emerald-900/40 border border-emerald-800/50 px-1.5 py-0.5 text-[9px] font-bold text-emerald-400">ELIGIBLE</span>
            : entry.gates_passed >= 3
            ? <span className="rounded bg-amber-900/30 border border-amber-800/40 px-1.5 py-0.5 text-[9px] font-bold text-amber-400">PARTIAL</span>
            : <span className="rounded bg-rose-900/20 border border-rose-900/40 px-1.5 py-0.5 text-[9px] text-rose-500">BLOCKED</span>
          }
        </td>
        <td className="py-1.5 px-2 text-right tabular-nums text-[10px] text-zinc-400">{entry.admission_score}</td>
        <td className="py-1.5 px-2 text-right tabular-nums text-[10px] text-zinc-400">{entry.authority_score}</td>
        <td className="py-1.5 px-2 text-right tabular-nums text-[10px] text-zinc-400">
          {entry.allocation_weight > 0 ? `${entry.allocation_weight.toFixed(1)}%` : "—"}
        </td>
        <td className="py-1.5 px-2 text-center">
          <div className="flex gap-0.5 justify-center">
            {gates.map((g) => <GateIcon key={g.label} pass={g.gate.pass} />)}
          </div>
        </td>
      </tr>
      {expanded && (
        <tr className="border-t border-zinc-800/30 bg-zinc-900/60">
          <td colSpan={8} className="px-4 py-3">
            <div className="grid grid-cols-1 gap-2 md:grid-cols-5">
              {gates.map((g) => (
                <div key={g.label} className={`rounded border px-2 py-2 ${g.gate.pass ? "border-emerald-800/40 bg-emerald-950/20" : "border-rose-900/40 bg-rose-950/15"}`}>
                  <div className="flex items-center gap-1 mb-1">
                    <GateIcon pass={g.gate.pass} />
                    <span className={`text-[9px] font-bold uppercase ${g.gate.pass ? "text-emerald-400" : "text-rose-400"}`}>{g.label}</span>
                    <span className="ml-auto text-[9px] tabular-nums text-zinc-500">{g.gate.score}/100</span>
                  </div>
                  <div className="text-[9px] text-zinc-600 leading-tight">{g.gate.reason}</div>
                </div>
              ))}
            </div>
          </td>
        </tr>
      )}
    </>
  );
}

export function CandidateQueue() {
  const [summary, setSummary] = useState<CandidateQueueSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [filter, setFilter] = useState<"ALL" | "ELIGIBLE" | "PARTIAL" | "BLOCKED">("ALL");

  useEffect(() => {
    fetch("/api/strategy-authority/candidate-queue")
      .then((r) => r.json())
      .then((d) => {
        if (d.ok) setSummary(d.queue);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  if (loading) return <div className="py-8 text-center text-xs text-zinc-600 animate-pulse">Loading candidate queue…</div>;

  if (!summary || summary.total_candidates === 0) {
    return (
      <div className="py-8 text-center text-xs text-zinc-600">
        No Grade 1 candidates in queue.
        <br /><span className="text-[10px] text-zinc-700">Run Portfolio Intelligence Compute after strategies reach Grade 1.</span>
      </div>
    );
  }

  const filtered = summary.entries.filter((e) => {
    if (filter === "ELIGIBLE") return e.all_gates_pass;
    if (filter === "PARTIAL") return e.gates_passed >= 3 && !e.all_gates_pass;
    if (filter === "BLOCKED") return e.gates_passed < 3;
    return true;
  });

  return (
    <div className="space-y-3">
      {/* KPIs */}
      <div className="grid grid-cols-4 gap-2">
        {[
          { label: "Total Candidates", value: String(summary.total_candidates), color: "text-zinc-300" },
          { label: "Fully Eligible", value: String(summary.fully_eligible), color: "text-emerald-400" },
          { label: "Partial (3–4 gates)", value: String(summary.partial_eligible), color: "text-amber-400" },
          { label: "Blocked (≤2 gates)", value: String(summary.ineligible), color: "text-rose-400" },
        ].map((k) => (
          <div key={k.label} className="rounded border border-zinc-800 bg-zinc-900/40 px-2 py-1.5 text-center">
            <div className="text-[9px] uppercase text-zinc-600">{k.label}</div>
            <div className={`text-xl font-bold tabular-nums ${k.color}`}>{k.value}</div>
          </div>
        ))}
      </div>

      {/* Gate architecture diagram */}
      <div className="rounded border border-zinc-800 bg-zinc-900/20 px-3 py-2">
        <div className="text-[9px] uppercase text-zinc-600 mb-2">Five-Gate Admission Architecture</div>
        <div className="flex items-center gap-1 flex-wrap text-[9px]">
          {[
            { label: "Gate 1: Correlation", desc: "Div score > 35" },
            { label: "Gate 2: Family", desc: "< 30% family in Main" },
            { label: "Gate 3: Regime", desc: "Current regime PF > 1.0" },
            { label: "Gate 4: Portfolio Fit", desc: "Fit score > 50" },
            { label: "Gate 5: Kelly", desc: "Positive Kelly fraction" },
          ].map((g, i) => (
            <div key={g.label} className="flex items-center gap-1">
              {i > 0 && <span className="text-zinc-700">→</span>}
              <div className="rounded border border-zinc-700 px-1.5 py-1 bg-zinc-900/40">
                <div className="text-[8px] font-bold text-zinc-400">{g.label}</div>
                <div className="text-[7px] text-zinc-600">{g.desc}</div>
              </div>
            </div>
          ))}
          <span className="text-zinc-700">→</span>
          <div className="rounded border border-emerald-800 px-1.5 py-1 bg-emerald-950/30">
            <div className="text-[8px] font-bold text-emerald-400">MAIN ENGINE</div>
          </div>
        </div>
      </div>

      {/* Filter */}
      <div className="flex items-center gap-1">
        {(["ALL", "ELIGIBLE", "PARTIAL", "BLOCKED"] as const).map((f) => (
          <button
            key={f}
            onClick={() => setFilter(f)}
            className={`px-2 py-0.5 rounded text-[9px] font-bold uppercase border transition-colors ${
              filter === f
                ? f === "ELIGIBLE" ? "border-emerald-700 bg-emerald-900/30 text-emerald-400"
                : f === "PARTIAL" ? "border-amber-700 bg-amber-900/20 text-amber-400"
                : f === "BLOCKED" ? "border-rose-800 bg-rose-900/20 text-rose-400"
                : "border-zinc-600 bg-zinc-800 text-zinc-300"
                : "border-zinc-800 text-zinc-600 hover:text-zinc-400"
            }`}
          >
            {f}
          </button>
        ))}
      </div>

      {/* Table */}
      <div className="overflow-x-auto">
        <table className="w-full text-xs">
          <thead>
            <tr className="text-left text-[9px] uppercase text-zinc-500 border-b border-zinc-800">
              <th className="py-2 px-2">Strategy</th>
              <th className="py-2 px-2">Family</th>
              <th className="py-2 px-2">Gates</th>
              <th className="py-2 px-2 text-center">Status</th>
              <th className="py-2 px-2 text-right">Admission</th>
              <th className="py-2 px-2 text-right">Authority</th>
              <th className="py-2 px-2 text-right">Alloc Weight</th>
              <th className="py-2 px-2 text-center">C F R P K</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((e: CandidateQueueEntry) => (
              <CandidateRow
                key={e.strategy_id}
                entry={e}
                expanded={expanded === e.strategy_id}
                onToggle={() => setExpanded(expanded === e.strategy_id ? null : e.strategy_id)}
              />
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
