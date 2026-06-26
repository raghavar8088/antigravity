"use client";

import { useCallback, useEffect, useState } from "react";
import { TerminalCard } from "@/components/terminal/institutional/TerminalCard";

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

// ── Status badge ──────────────────────────────────────────────────────────────

function StatusBadge({ status }: { status: string }) {
  const modifier =
    status === "done"
      ? "m3-status-chip--success"
      : status === "running"
      ? "m3-status-chip--info"
      : status === "error"
      ? "m3-status-chip--error"
      : "m3-status-chip--warning";

  return (
    <span className={`m3-status-chip ${modifier}`}>
      <span className="m3-status-chip__dot" />
      {status}
    </span>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────

type Tab = "run" | "jobs" | "leaderboard" | "strategies";

export default function BacktestLabPage() {
  const [tab, setTab] = useState<Tab>("run");

  return (
    <div className="m3-page-stack">
      <div className="m3-tabs__list">
        {(["run", "jobs", "leaderboard", "strategies"] as Tab[]).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className="m3-tabs__trigger"
            data-state={tab === t ? "active" : "inactive"}
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

      <div className="m3-tabs__content">
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
    <TerminalCard
      title="Run Backtest"
      subtitle="Run all ported strategies (S101–S150) against a date range via the v3 institutional engine"
    >
      <div style={{ maxWidth: 480, display: "flex", flexDirection: "column", gap: 16 }}>
        <div className="m3-field">
          <label className="m3-field__label">Symbol</label>
          <input
            value={symbol}
            onChange={(e) => setSymbol(e.target.value)}
            className="m3-field__input"
          />
        </div>
        <div style={{ display: "flex", gap: 12 }}>
          <div className="m3-field" style={{ flex: 1 }}>
            <label className="m3-field__label">From</label>
            <input
              type="date"
              value={from}
              onChange={(e) => setFrom(e.target.value)}
              className="m3-field__input"
            />
          </div>
          <div className="m3-field" style={{ flex: 1 }}>
            <label className="m3-field__label">To</label>
            <input
              type="date"
              value={to}
              onChange={(e) => setTo(e.target.value)}
              className="m3-field__input"
            />
          </div>
        </div>
        <p className="m3-field__hint">
          Requires pre-cached historical data in the engine data directory.
        </p>
        <button
          onClick={submit}
          disabled={loading}
          className="m3-btn m3-btn--filled"
        >
          {loading ? "Submitting…" : "Run All Strategies"}
        </button>
        {result && (
          <p style={{ fontSize: 13, color: "var(--m3-profit)" }}>{result}</p>
        )}
        {error && (
          <p style={{ fontSize: 13, color: "var(--m3-loss)" }}>{error}</p>
        )}
      </div>
    </TerminalCard>
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
    return (
      <TerminalCard title="Jobs">
        <p className="m3-field__hint">No backtest jobs yet. Run one from the Run tab.</p>
      </TerminalCard>
    );
  }

  return (
    <TerminalCard title="Jobs" subtitle="Auto-refreshes every 5 seconds">
      <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        {jobs.map((job) => (
          <div
            key={job.id}
            style={{
              padding: "12px 14px",
              borderRadius: "var(--m3-radius-md)",
              border: "1px solid var(--m3-outline-variant)",
              background: "var(--m3-surface-container)",
            }}
          >
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 4 }}>
              <span style={{ fontFamily: "var(--m3-font-mono)", fontSize: 11, color: "var(--m3-on-surface-variant)" }}>
                {job.id}
              </span>
              <StatusBadge status={job.status} />
            </div>
            <div style={{ display: "flex", gap: 16, fontSize: 13, color: "var(--m3-on-surface)" }}>
              <span>{job.symbol}</span>
              <span style={{ color: "var(--m3-on-surface-variant)" }}>
                {job.fromDate?.slice(0, 10)} → {job.toDate?.slice(0, 10)}
              </span>
              <span style={{ color: "var(--m3-on-surface-variant)" }}>run: {job.runId}</span>
            </div>
            {job.status === "running" && (
              <div style={{ marginTop: 8 }}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 4 }}>
                  <span style={{ fontSize: 11, color: "var(--m3-on-surface-variant)" }}>Progress</span>
                  <span style={{ fontFamily: "var(--m3-font-mono)", fontSize: 12, fontWeight: 600, color: "var(--m3-primary)" }}>
                    {job.progress}%
                  </span>
                </div>
                <div
                  style={{
                    height: 6,
                    borderRadius: 3,
                    background: "var(--m3-outline-variant)",
                    overflow: "hidden",
                  }}
                >
                  <div
                    style={{
                      height: "100%",
                      width: `${job.progress}%`,
                      background: "var(--m3-primary)",
                      transition: "width 0.3s ease",
                      borderRadius: 3,
                    }}
                  />
                </div>
              </div>
            )}
            {job.error && (
              <p style={{ fontSize: 11, color: "var(--m3-loss)", marginTop: 4 }}>{job.error}</p>
            )}
            {job.status === "done" && (
              <button
                onClick={() => navigator.clipboard.writeText(job.runId)}
                className="m3-btn m3-btn--text"
                style={{ fontSize: 11, padding: "2px 0", marginTop: 4 }}
              >
                Copy run ID for Leaderboard →
              </button>
            )}
          </div>
        ))}
      </div>
    </TerminalCard>
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

  const loadLeaderboard = useCallback(async () => {
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

  useEffect(() => { loadLeaderboard(); }, [loadLeaderboard]);

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

  const eligibleCount = rows.filter(
    (r) => r.sharpe >= 1.0 && r.winRate >= 0.45 && r.maxDrawdown <= 20 && r.profitFactor >= 1.3 && r.totalTrades >= 50,
  ).length;

  return (
    <TerminalCard
      title="Leaderboard"
      subtitle={rows.length > 0 ? `${rows.length} strategies · ${eligibleCount} eligible for promotion` : undefined}
    >
      <div style={{ display: "flex", gap: 10, marginBottom: 16 }}>
        <input
          placeholder="Run ID (optional — leave blank for latest)"
          value={runId}
          onChange={(e) => setRunId(e.target.value)}
          className="m3-field__input"
          style={{ flex: 1 }}
        />
        <input
          value={symbol}
          onChange={(e) => setSymbol(e.target.value)}
          className="m3-field__input"
          style={{ width: 120 }}
        />
        <button
          onClick={loadLeaderboard}
          disabled={loading}
          className="m3-btn m3-btn--outlined"
        >
          {loading ? "…" : "Load"}
        </button>
      </div>

      {error && (
        <p style={{ fontSize: 13, color: "var(--m3-loss)", marginBottom: 12 }}>{error}</p>
      )}

      {rows.length === 0 ? (
        <p className="m3-field__hint">No results. Run a backtest first.</p>
      ) : (
        <div className="m3-data-table">
          <div className="m3-data-table__scroll">
            <table>
              <thead className="m3-data-table__head--sticky">
                <tr>
                  <th>Strategy</th>
                  <th style={{ textAlign: "right" }}>Trades</th>
                  <th style={{ textAlign: "right" }}>Win%</th>
                  <th style={{ textAlign: "right" }}>Sharpe</th>
                  <th style={{ textAlign: "right" }}>MaxDD%</th>
                  <th style={{ textAlign: "right" }}>PF</th>
                  <th style={{ textAlign: "right" }}>Return%</th>
                  <th style={{ textAlign: "right" }}>Action</th>
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
                    <tr key={row.id} className="m3-data-table__row">
                      <td>
                        <span
                          style={{
                            color: eligible ? "var(--m3-profit)" : "var(--m3-on-surface)",
                            fontWeight: eligible ? 600 : 400,
                          }}
                        >
                          {row.strategyName}
                        </span>
                        {eligible && (
                          <span
                            className="m3-status-chip m3-status-chip--success"
                            style={{ marginLeft: 8, fontSize: 10 }}
                          >
                            eligible
                          </span>
                        )}
                      </td>
                      <td className="m3-data-table__cell--tabular" style={{ textAlign: "right", color: "var(--m3-on-surface-variant)" }}>
                        {row.totalTrades}
                      </td>
                      <td className="m3-data-table__cell--tabular" style={{ textAlign: "right" }}>
                        {(row.winRate * 100).toFixed(1)}%
                      </td>
                      <td className="m3-data-table__cell--tabular" style={{ textAlign: "right" }}>
                        <span style={{ color: row.sharpe >= 1 ? "var(--m3-profit)" : "var(--m3-on-surface)" }}>
                          {row.sharpe.toFixed(2)}
                        </span>
                      </td>
                      <td className="m3-data-table__cell--tabular" style={{ textAlign: "right" }}>
                        <span style={{ color: row.maxDrawdown > 20 ? "var(--m3-loss)" : "var(--m3-on-surface)" }}>
                          {row.maxDrawdown.toFixed(1)}%
                        </span>
                      </td>
                      <td className="m3-data-table__cell--tabular" style={{ textAlign: "right" }}>
                        <span style={{ color: row.profitFactor >= 1.3 ? "var(--m3-profit)" : "var(--m3-on-surface)" }}>
                          {row.profitFactor.toFixed(2)}
                        </span>
                      </td>
                      <td className="m3-data-table__cell--tabular" style={{ textAlign: "right" }}>
                        <span style={{ color: row.totalReturn >= 0 ? "var(--m3-profit)" : "var(--m3-loss)" }}>
                          {row.totalReturn >= 0 ? "+" : ""}{row.totalReturn.toFixed(1)}%
                        </span>
                      </td>
                      <td style={{ textAlign: "right" }}>
                        {eligible && (
                          <button
                            onClick={() => promote(row.strategyName)}
                            disabled={promoting === row.strategyName}
                            className="m3-btn m3-btn--tonal"
                            style={{ fontSize: 11, padding: "3px 10px" }}
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
        </div>
      )}
    </TerminalCard>
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
    <TerminalCard title="All Strategies" subtitle={`${visible.length} / ${strategies.length} strategies`}>
      <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 16 }}>
        <input
          placeholder="Filter strategies…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          className="m3-field__input"
          style={{ maxWidth: 360 }}
        />
        <span className="m3-data-table__count">{visible.length} / {strategies.length}</span>
      </div>

      {loading ? (
        <p className="m3-field__hint">Loading…</p>
      ) : visible.length === 0 ? (
        <p className="m3-field__hint">No strategies found.</p>
      ) : (
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fill, minmax(260px, 1fr))",
            gap: 10,
          }}
        >
          {visible.map((s) => (
            <div
              key={s.name}
              style={{
                padding: "12px 14px",
                borderRadius: "var(--m3-radius-md)",
                border: "1px solid var(--m3-outline-variant)",
                background: "var(--m3-surface-container)",
              }}
            >
              <div style={{ fontWeight: 600, fontSize: 13, color: "var(--m3-on-surface)", marginBottom: 4 }}>
                {s.name}
              </div>
              <p style={{ fontSize: 11, color: "var(--m3-on-surface-variant)", marginBottom: 8, lineHeight: 1.5 }}>
                {s.description}
              </p>
              <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
                {s.regimes.map((r) => (
                  <span key={r} className="m3-chip m3-chip--selected" style={{ fontSize: 10 }}>
                    {r}
                  </span>
                ))}
                {s.timeframes.map((tf) => (
                  <span key={tf} className="m3-chip" style={{ fontSize: 10 }}>
                    {tf}
                  </span>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </TerminalCard>
  );
}
