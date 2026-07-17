"use client";

/**
 * BTC Pre-Live Engine — Phase 2 qualification leaderboard.
 *
 * Renders the whitelist produced by engine/cmd/btc_qualify_25 (a static copy
 * of data/btc_prelive_whitelist.json published to
 * client/public/btc-prelive/qualification.json). The qualification run is an
 * offline analysis, so a static artifact is the honest representation — the
 * Phase 3 paper desk will serve live data through its own API.
 */

import { useEffect, useState } from "react";

type Metrics = {
  trades: number;
  win_rate_pct: number;
  sharpe: number;
  profit_factor: number;
  return_pct: number;
  max_dd_pct: number;
};

type SelectedRow = {
  rank: number;
  strategy: string;
  tier: "A" | "B";
  train: Metrics;
  validate: Metrics;
};

type Qualification = {
  symbol: string;
  data_start: string;
  data_end: string;
  train_window: string[];
  validate_window: string[];
  strict_bar: string;
  ranking_rule: string;
  candidates_tested: number;
  tier_a_count: number;
  tier_b_available: number;
  whitelist: string[];
  selected: SelectedRow[];
  ran_at: string;
};

export default function QualificationBoard() {
  const [data, setData] = useState<Qualification | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "missing">("loading");

  useEffect(() => {
    (async () => {
      try {
        const res = await fetch("/btc-prelive/qualification.json", { cache: "no-store" });
        if (!res.ok) {
          setState("missing");
          return;
        }
        setData((await res.json()) as Qualification);
        setState("ready");
      } catch {
        setState("missing");
      }
    })();
  }, []);

  if (state === "loading") {
    return (
      <div className="glass-panel rounded-2xl px-5 py-8 text-center text-sm text-zinc-400">
        Loading qualification results…
      </div>
    );
  }

  if (state === "missing" || !data) {
    return (
      <div className="glass-panel rounded-2xl px-5 py-8 text-center">
        <div className="text-sm font-semibold text-zinc-600">Qualification run not published yet</div>
        <div className="mt-1 text-[12px] text-zinc-400">
          Run <code className="rounded bg-zinc-100 px-1">engine/cmd/btc_qualify_25</code> and publish
          the whitelist JSON — the Phase 2 leaderboard will appear here.
        </div>
      </div>
    );
  }

  const tierACount = data.selected.filter((r) => r.tier === "A").length;
  const tierBCount = data.selected.filter((r) => r.tier === "B").length;

  return (
    <div className="flex flex-col gap-3">
      {/* Run summary */}
      <div className="glass-panel flex flex-wrap items-center gap-x-6 gap-y-2 rounded-2xl px-5 py-3 text-[12px] text-zinc-600">
        <span>
          <span className="font-semibold text-zinc-800">{data.candidates_tested}</span> strategies tested
        </span>
        <span>
          Train <span className="font-semibold text-zinc-800">{data.train_window[0]} → {data.train_window[1]}</span>
        </span>
        <span>
          Validate (OOS) <span className="font-semibold text-zinc-800">{data.validate_window[0]} → {data.validate_window[1]}</span>
        </span>
        <span>
          Whitelist <span className="font-semibold text-emerald-700">{tierACount} Tier A</span>
          {tierBCount > 0 && <span className="font-semibold text-amber-600"> + {tierBCount} Tier B</span>}
        </span>
        <span className="text-zinc-400">ran {new Date(data.ran_at).toLocaleString()}</span>
      </div>

      <div className="rounded-xl border border-zinc-200 bg-zinc-50 px-4 py-2.5 text-[11px] leading-relaxed text-zinc-500">
        <span className="font-semibold text-zinc-700">Bar (out-of-sample):</span> {data.strict_bar} ·{" "}
        <span className="font-semibold text-zinc-700">Ranking:</span> {data.ranking_rule}
      </div>

      {/* Leaderboard table */}
      <div className="glass-panel overflow-x-auto rounded-2xl">
        <table className="w-full min-w-[760px] text-left text-[12px]">
          <thead>
            <tr className="border-b border-zinc-200 text-[10px] uppercase tracking-wider text-zinc-400">
              <th className="px-4 py-2.5">#</th>
              <th className="px-2 py-2.5">Tier</th>
              <th className="px-2 py-2.5">Strategy</th>
              <th className="px-2 py-2.5 text-right">Win rate (OOS)</th>
              <th className="px-2 py-2.5 text-right">Profit factor</th>
              <th className="px-2 py-2.5 text-right">Sharpe</th>
              <th className="px-2 py-2.5 text-right">Trades</th>
              <th className="px-2 py-2.5 text-right">Return</th>
              <th className="px-2 py-2.5 text-right">Max DD</th>
              <th className="px-4 py-2.5 text-right">Train WR</th>
            </tr>
          </thead>
          <tbody>
            {data.selected.map((r) => (
              <tr key={r.strategy} className="border-b border-zinc-100 last:border-0 hover:bg-zinc-50">
                <td className="px-4 py-2 font-semibold tabular-nums text-zinc-500">{r.rank}</td>
                <td className="px-2 py-2">
                  <span
                    className={`inline-block rounded px-1.5 py-0.5 text-[10px] font-bold ${
                      r.tier === "A" ? "bg-emerald-100 text-emerald-700" : "bg-amber-100 text-amber-700"
                    }`}
                  >
                    {r.tier}
                  </span>
                </td>
                <td className="max-w-[260px] truncate px-2 py-2 font-medium text-zinc-800" title={r.strategy}>
                  {r.strategy}
                </td>
                <td className="px-2 py-2 text-right font-semibold tabular-nums text-zinc-900">
                  {r.validate.win_rate_pct.toFixed(1)}%
                </td>
                <td className="px-2 py-2 text-right tabular-nums">{r.validate.profit_factor.toFixed(2)}</td>
                <td className="px-2 py-2 text-right tabular-nums">{r.validate.sharpe.toFixed(2)}</td>
                <td className="px-2 py-2 text-right tabular-nums">{r.validate.trades}</td>
                <td
                  className={`px-2 py-2 text-right tabular-nums ${
                    r.validate.return_pct >= 0 ? "text-emerald-600" : "text-red-600"
                  }`}
                >
                  {r.validate.return_pct.toFixed(1)}%
                </td>
                <td className="px-2 py-2 text-right tabular-nums text-zinc-500">{r.validate.max_dd_pct.toFixed(1)}%</td>
                <td className="px-4 py-2 text-right tabular-nums text-zinc-400">{r.train.win_rate_pct.toFixed(1)}%</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="text-[11px] text-zinc-400">
        Tier A = passed every OOS criterion. Tier B = 3-of-4 near-miss, used only to fill the 25 — treat with extra
        caution during the paper week. All metrics are from the out-of-sample window on real Delta Exchange data.
      </div>
    </div>
  );
}
