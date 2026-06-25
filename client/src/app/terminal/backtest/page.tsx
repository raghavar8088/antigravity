"use client";

import { useCallback, useEffect, useState } from "react";

// ── Types ─────────────────────────────────────────────────────────────────────

interface Job {
  id: string;
  runId: string;
  symbol: string;
  fromDate: string;
  toDate: string;
  strategies: string[];
  status: "pending" | "running" | "done" | "error";
  error?: string;
  createdAt: string;
  finishedAt?: string;
  progress: number;
}

interface LeaderboardRow {
  id: number;
  strategyName: string;
  symbol: string;
  totalTrades: number;
  winRate: number;
  sharpe: number;
  maxDrawdown: number;
  profitFactor: number;
  totalReturn: number;
}

interface StrategyInfo {
  name: string;
  description: string;
  regimes: string[];
  timeframes: string[];
}

// ── API helpers ───────────────────────────────────────────────────────────────

const ENGINE = "/api/engine";

async function safeJson(r: Response) {
  const text = await r.text();
  try {
    const data = JSON.parse(text);
    if (!r.ok) throw new Error(data?.error ?? data?.message ?? `HTTP ${r.status}`);
    return data;
  } catch (e) {
    if (e instanceof SyntaxError) {
      throw new Error(`Engine returned non-JSON (HTTP ${r.status}): ${text.slice(0, 120)}`);
    }
    throw e;
  }
}

async function apiPost(path: string, body: unknown) {
  const r = await fetch(`${ENGINE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return safeJson(r);
}

async function apiGet(path: string) {
  const r = await fetch(`${ENGINE}${path}`, { cache: "no-store" });
  return safeJson(r);
}

// ── Main page ─────────────────────────────────────────────────────────────────

type Tab = "run" | "jobs" | "leaderboard" | "strategies";

export default function BacktestLabPage() {
  const [tab, setTab] = useState<Tab>("run");

  return (
    <div className="min-h-screen bg-gray-950 text-gray-100 p-6">
      <div className="max-w-7xl mx-auto">
        <h1 className="text-2xl font-bold text-white mb-1">Backtest Lab</h1>
        <p className="text-gray-400 text-sm mb-6">
          Run 5-year multi-strategy backtests via the v3 institutional engine.
        </p>

        {/* Tab bar */}
        <div className="flex gap-1 mb-6 border-b border-gray-800">
          {(["run", "jobs", "leaderboard", "strategies"] as Tab[]).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={[
                "px-4 py-2 text-sm font-medium rounded-t transition-colors",
                tab === t
                  ? "bg-gray-800 text-white border-b-2 border-blue-500"
                  : "text-gray-400 hover:text-white",
              ].join(" ")}
            >
              {t === "run"
                ? "Run Backtest"
                : t === "jobs"
                  ? "Jobs"
                  : t === "leaderboard"
                    ? "Leaderboard"
                    : "All Strategies"}
            </button>
          ))}
        </div>

        {tab === "run" && <RunTab />}
        {tab === "jobs" && <JobsTab />}
        {tab === "leaderboard" && <LeaderboardTab />}
        {tab === "strategies" && <StrategiesTab />}
      </div>
    </div>
  );
}

// ── Run Tab ───────────────────────────────────────────────────────────────────

function RunTab() {
  const [symbol, setSymbol] = useState("BTCUSDT");
  const [from, setFrom] = useState("2020-01-01");
  const [to, setTo] = useState(new Date().toISOString().slice(0, 10));
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const submit = useCallback(async () => {
    setLoading(true);
    setResult(null);
    setError(null);
    try {
      const data = await apiPost("/api/backtest/run", { symbol, from, to });
      setResult(`Job created: ${data.id} (run ID: ${data.runId})`);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, [symbol, from, to]);

  return (
    <div className="max-w-lg">
      <div className="bg-gray-900 rounded-lg p-6 space-y-4">
        <div>
          <label className="block text-xs text-gray-400 mb-1">Symbol</label>
          <input
            value={symbol}
            onChange={(e) => setSymbol(e.target.value)}
            className="w-full bg-gray-800 text-white rounded px-3 py-2 text-sm"
          />
        </div>
        <div className="flex gap-3">
          <div className="flex-1">
            <label className="block text-xs text-gray-400 mb-1">From</label>
            <input
              type="date"
              value={from}
              onChange={(e) => setFrom(e.target.value)}
              className="w-full bg-gray-800 text-white rounded px-3 py-2 text-sm"
            />
          </div>
          <div className="flex-1">
            <label className="block text-xs text-gray-400 mb-1">To</label>
            <input
              type="date"
              value={to}
              onChange={(e) => setTo(e.target.value)}
              className="w-full bg-gray-800 text-white rounded px-3 py-2 text-sm"
            />
          </div>
        </div>
        <p className="text-xs text-gray-500">
          Runs all 50 ported strategies (S101–S150) against the selected date range. Requires
          pre-cached historical data in the engine data directory.
        </p>
        <button
          onClick={submit}
          disabled={loading}
          className="w-full py-2 rounded bg-blue-600 hover:bg-blue-500 disabled:opacity-40 text-sm font-medium transition-colors"
        >
          {loading ? "Submitting…" : "Run All Strategies"}
        </button>
        {result && <p className="text-green-400 text-sm">{result}</p>}
        {error && <p className="text-red-400 text-sm">{error}</p>}
      </div>
    </div>
  );
}

// ── Jobs Tab ──────────────────────────────────────────────────────────────────

function JobsTab() {
  const [jobs, setJobs] = useState<Job[]>([]);

  const refresh = useCallback(async () => {
    try {
      const data = await apiGet("/api/backtest/jobs");
      if (Array.isArray(data)) setJobs(data);
    } catch {}
  }, []);

  useEffect(() => {
    refresh();
    const iv = setInterval(refresh, 5000);
    return () => clearInterval(iv);
  }, [refresh]);

  if (jobs.length === 0) {
    return <p className="text-gray-500 text-sm">No backtest jobs yet. Run one from the Run tab.</p>;
  }

  return (
    <div className="space-y-3">
      {jobs.map((job) => (
        <div key={job.id} className="bg-gray-900 rounded-lg p-4">
          <div className="flex items-center justify-between mb-1">
            <span className="font-mono text-xs text-gray-400">{job.id}</span>
            <StatusBadge status={job.status} />
          </div>
          <div className="flex gap-4 text-sm text-gray-300 mt-1">
            <span>{job.symbol}</span>
            <span>{job.fromDate?.slice(0, 10)} → {job.toDate?.slice(0, 10)}</span>
            <span className="text-gray-500">run: {job.runId}</span>
          </div>
          {job.status === "running" && (
            <div className="mt-2 h-1.5 bg-gray-800 rounded overflow-hidden">
              <div
                className="h-full bg-blue-500 transition-all"
                style={{ width: `${job.progress}%` }}
              />
            </div>
          )}
          {job.error && <p className="text-red-400 text-xs mt-1">{job.error}</p>}
          {job.status === "done" && (
            <a
              href="#"
              onClick={(e) => {
                e.preventDefault();
                navigator.clipboard.writeText(job.runId);
              }}
              className="text-blue-400 text-xs mt-1 inline-block hover:underline"
            >
              Copy run ID for Leaderboard →
            </a>
          )}
        </div>
      ))}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    pending: "bg-yellow-900 text-yellow-300",
    running: "bg-blue-900 text-blue-300",
    done: "bg-green-900 text-green-300",
    error: "bg-red-900 text-red-300",
  };
  return (
    <span className={`px-2 py-0.5 rounded text-xs font-medium ${colors[status] ?? "bg-gray-800 text-gray-400"}`}>
      {status}
    </span>
  );
}

// ── Leaderboard Tab ───────────────────────────────────────────────────────────

function LeaderboardTab() {
  const [runId, setRunId] = useState("");
  const [symbol, setSymbol] = useState("BTCUSDT");
  const [rows, setRows] = useState<LeaderboardRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [promoting, setPromoting] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const query = runId ? `?run_id=${encodeURIComponent(runId)}&symbol=${symbol}` : `?symbol=${symbol}`;
      const data = await apiGet(`/api/backtest/leaderboard${query}`);
      if (Array.isArray(data)) setRows(data);
      else setError(data?.error ?? "unknown error");
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, [runId, symbol]);

  useEffect(() => { fetch(); }, [fetch]);

  const promote = useCallback(async (name: string) => {
    setPromoting(name);
    try {
      await apiPost(`/api/backtest/promote/${encodeURIComponent(name)}`, {});
      alert(`${name} promoted to live trading.`);
    } catch (e) {
      alert(`Promotion failed: ${e}`);
    } finally {
      setPromoting(null);
    }
  }, []);

  return (
    <div>
      <div className="flex gap-3 mb-4">
        <input
          placeholder="Run ID (optional — leave blank for latest)"
          value={runId}
          onChange={(e) => setRunId(e.target.value)}
          className="flex-1 bg-gray-800 text-white rounded px-3 py-2 text-sm"
        />
        <input
          value={symbol}
          onChange={(e) => setSymbol(e.target.value)}
          className="w-32 bg-gray-800 text-white rounded px-3 py-2 text-sm"
        />
        <button
          onClick={fetch}
          className="px-4 py-2 bg-blue-600 hover:bg-blue-500 rounded text-sm font-medium"
        >
          {loading ? "…" : "Load"}
        </button>
      </div>

      {error && <p className="text-red-400 text-sm mb-3">{error}</p>}

      {rows.length === 0 ? (
        <p className="text-gray-500 text-sm">No results. Run a backtest first.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-gray-500 border-b border-gray-800 text-xs">
                <th className="pb-2 pr-4">Strategy</th>
                <th className="pb-2 pr-4 text-right">Trades</th>
                <th className="pb-2 pr-4 text-right">Win%</th>
                <th className="pb-2 pr-4 text-right">Sharpe</th>
                <th className="pb-2 pr-4 text-right">MaxDD%</th>
                <th className="pb-2 pr-4 text-right">PF</th>
                <th className="pb-2 pr-4 text-right">Return%</th>
                <th className="pb-2 text-right">Action</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => {
                const eligible =
                  row.sharpe >= 1.0 &&
                  row.winRate >= 0.45 &&
                  row.maxDrawdown <= 20 &&
                  row.profitFactor >= 1.3 &&
                  row.totalTrades >= 50;
                return (
                  <tr key={row.id} className="border-b border-gray-800/50 hover:bg-gray-900/50">
                    <td className="py-2 pr-4">
                      <span className={eligible ? "text-green-400 font-medium" : "text-gray-300"}>
                        {row.strategyName}
                      </span>
                      {eligible && (
                        <span className="ml-2 text-xs bg-green-900 text-green-300 px-1.5 py-0.5 rounded">
                          eligible
                        </span>
                      )}
                    </td>
                    <td className="py-2 pr-4 text-right text-gray-400">{row.totalTrades}</td>
                    <td className="py-2 pr-4 text-right">{(row.winRate * 100).toFixed(1)}%</td>
                    <td className="py-2 pr-4 text-right">
                      <span className={row.sharpe >= 1 ? "text-green-400" : "text-gray-300"}>
                        {row.sharpe.toFixed(2)}
                      </span>
                    </td>
                    <td className="py-2 pr-4 text-right">
                      <span className={row.maxDrawdown > 20 ? "text-red-400" : "text-gray-300"}>
                        {row.maxDrawdown.toFixed(1)}%
                      </span>
                    </td>
                    <td className="py-2 pr-4 text-right">
                      <span className={row.profitFactor >= 1.3 ? "text-green-400" : "text-gray-300"}>
                        {row.profitFactor.toFixed(2)}
                      </span>
                    </td>
                    <td className="py-2 pr-4 text-right">
                      <span className={row.totalReturn >= 0 ? "text-green-400" : "text-red-400"}>
                        {row.totalReturn >= 0 ? "+" : ""}
                        {row.totalReturn.toFixed(1)}%
                      </span>
                    </td>
                    <td className="py-2 text-right">
                      {eligible && (
                        <button
                          onClick={() => promote(row.strategyName)}
                          disabled={promoting === row.strategyName}
                          className="px-2 py-0.5 text-xs bg-green-700 hover:bg-green-600 disabled:opacity-40 rounded transition-colors"
                        >
                          {promoting === row.strategyName ? "…" : "Promote"}
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

// ── All Strategies Tab ────────────────────────────────────────────────────────

function StrategiesTab() {
  const [strategies, setStrategies] = useState<StrategyInfo[]>([]);
  const [filter, setFilter] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    apiGet("/api/backtest/strategies")
      .then((data) => { if (Array.isArray(data)) setStrategies(data); })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const visible = strategies.filter(
    (s) =>
      filter === "" ||
      s.name.toLowerCase().includes(filter.toLowerCase()) ||
      s.description.toLowerCase().includes(filter.toLowerCase()),
  );

  return (
    <div>
      <div className="flex gap-3 mb-4">
        <input
          placeholder="Filter strategies…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          className="flex-1 bg-gray-800 text-white rounded px-3 py-2 text-sm"
        />
        <span className="text-gray-500 text-sm self-center">
          {visible.length} / {strategies.length}
        </span>
      </div>

      {loading ? (
        <p className="text-gray-500 text-sm">Loading…</p>
      ) : visible.length === 0 ? (
        <p className="text-gray-500 text-sm">No strategies found.</p>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
          {visible.map((s) => (
            <div key={s.name} className="bg-gray-900 rounded-lg p-4">
              <div className="font-medium text-white text-sm mb-1">{s.name}</div>
              <p className="text-gray-400 text-xs mb-2">{s.description}</p>
              <div className="flex flex-wrap gap-1">
                {s.regimes.map((r) => (
                  <span
                    key={r}
                    className="text-xs px-1.5 py-0.5 rounded bg-blue-900/60 text-blue-300"
                  >
                    {r}
                  </span>
                ))}
                {s.timeframes.map((tf) => (
                  <span
                    key={tf}
                    className="text-xs px-1.5 py-0.5 rounded bg-gray-800 text-gray-400"
                  >
                    {tf}
                  </span>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
