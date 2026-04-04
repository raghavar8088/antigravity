"use client";
import useOptions, { OptionPosition, OptionTrade, OptionStrategyStatus, OptionStats } from "@/hooks/useOptions";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

function fmt(n: number, decimals = 2) {
  return n.toLocaleString(undefined, { minimumFractionDigits: decimals, maximumFractionDigits: decimals });
}

function fmtUSD(n: number, signed = false) {
  const prefix = signed ? (n >= 0 ? "+" : "") : "";
  return `${prefix}$${fmt(Math.abs(n))}`;
}

function fmtPct(n: number, signed = false) {
  const prefix = signed ? (n >= 0 ? "+" : "") : "";
  return `${prefix}${fmt(Math.abs(n), 1)}%`;
}

function MetricCard({ label, value, sub, accent }: { label: string; value: string; sub?: string; accent?: string }) {
  return (
    <div className="glass-panel p-5 flex flex-col gap-1">
      <div style={{ fontSize: 10, fontWeight: 700, letterSpacing: "0.18em", color: "var(--text-muted)", textTransform: "uppercase" }}>{label}</div>
      <div className={`text-2xl font-semibold leading-none mt-1 ${accent ?? "text-white"}`}>{value}</div>
      {sub && <div style={{ fontSize: 11, color: "var(--text-secondary)", marginTop: 4 }}>{sub}</div>}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, string> = {
    READY: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
    IN_POSITION: "bg-blue-500/10 text-blue-400 border-blue-500/20",
    COOLING: "bg-amber-500/10 text-amber-400 border-amber-500/20",
  };
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${map[status] ?? "text-zinc-400"}`}>
      {status}
    </span>
  );
}

function TypeBadge({ type }: { type: string }) {
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${
      type === "CALL"
        ? "border-emerald-500/25 bg-emerald-500/10 text-emerald-400"
        : "border-rose-500/25 bg-rose-500/10 text-rose-400"
    }`}>
      {type}
    </span>
  );
}

function ReasonBadge({ reason }: { reason: string }) {
  const map: Record<string, string> = {
    TP: "border-emerald-500/25 bg-emerald-500/10 text-emerald-400",
    SL: "border-rose-500/25 bg-rose-500/10 text-rose-400",
    EXPIRY: "border-zinc-500/25 bg-zinc-500/10 text-zinc-400",
  };
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${map[reason] ?? "text-zinc-400"}`}>
      {reason}
    </span>
  );
}

function PositionsTable({ positions }: { positions: OptionPosition[] }) {
  if (positions.length === 0) {
    return (
      <div className="glass-panel p-6 text-center" style={{ color: "var(--text-muted)", fontSize: 13 }}>
        No open option positions. Strategies are scanning for entry signals.
      </div>
    );
  }
  return (
    <div className="glass-panel p-6">
      <div className="mb-4 text-xs font-bold uppercase tracking-widest" style={{ color: "var(--text-secondary)" }}>
        Open Positions ({positions.length})
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <thead className="border-b text-[10px] uppercase tracking-widest" style={{ color: "var(--text-secondary)", borderColor: "var(--border-subtle)" }}>
            <tr>
              <th className="py-2 px-3">Strategy</th>
              <th className="py-2 px-3">Type</th>
              <th className="py-2 px-3">Strike</th>
              <th className="py-2 px-3">Entry Premium</th>
              <th className="py-2 px-3">Current Premium</th>
              <th className="py-2 px-3">Delta</th>
              <th className="py-2 px-3">IV</th>
              <th className="py-2 px-3 text-right">Unrealized PnL</th>
            </tr>
          </thead>
          <tbody className="divide-y" style={{ divideColor: "var(--border-subtle)" }}>
            {positions.map((pos) => (
              <tr key={pos.id} className="hover:bg-white/[0.02]">
                <td className="py-2 px-3 font-medium text-white">{pos.strategyName}</td>
                <td className="py-2 px-3"><TypeBadge type={pos.optionType} /></td>
                <td className="py-2 px-3">${fmt(pos.strike, 0)}</td>
                <td className="py-2 px-3">${fmt(pos.entryPremium)}</td>
                <td className="py-2 px-3">${fmt(pos.currentPremium)}</td>
                <td className="py-2 px-3">{fmt(pos.delta, 3)}</td>
                <td className="py-2 px-3">{fmtPct(pos.iv * 100)}</td>
                <td className={`py-2 px-3 text-right font-mono font-semibold ${pos.unrealizedPnl >= 0 ? "text-emerald-400" : "text-rose-400"}`}>
                  {fmtUSD(pos.unrealizedPnl, true)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function TradesTable({ trades }: { trades: OptionTrade[] }) {
  if (trades.length === 0) {
    return (
      <div className="glass-panel p-6 text-center" style={{ color: "var(--text-muted)", fontSize: 13 }}>
        No completed option trades yet.
      </div>
    );
  }
  return (
    <div className="glass-panel p-6">
      <div className="mb-4 text-xs font-bold uppercase tracking-widest" style={{ color: "var(--text-secondary)" }}>
        Completed Trades ({trades.length})
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <thead className="border-b text-[10px] uppercase tracking-widest" style={{ color: "var(--text-secondary)", borderColor: "var(--border-subtle)" }}>
            <tr>
              <th className="py-2 px-3">Strategy</th>
              <th className="py-2 px-3">Type</th>
              <th className="py-2 px-3">Strike</th>
              <th className="py-2 px-3">Entry</th>
              <th className="py-2 px-3">Exit</th>
              <th className="py-2 px-3">Return</th>
              <th className="py-2 px-3">Reason</th>
              <th className="py-2 px-3 text-right">Net PnL</th>
            </tr>
          </thead>
          <tbody className="divide-y" style={{ divideColor: "var(--border-subtle)" }}>
            {trades.slice(0, 100).map((t) => (
              <tr key={t.id} className="hover:bg-white/[0.02]">
                <td className="py-2 px-3 font-medium text-white">{t.strategyName}</td>
                <td className="py-2 px-3"><TypeBadge type={t.optionType} /></td>
                <td className="py-2 px-3">${fmt(t.strike, 0)}</td>
                <td className="py-2 px-3">${fmt(t.entryPremium)}</td>
                <td className="py-2 px-3">${fmt(t.exitPremium)}</td>
                <td className={`py-2 px-3 font-mono ${t.returnPct >= 0 ? "text-emerald-400" : "text-rose-400"}`}>
                  {fmtPct(t.returnPct, true)}
                </td>
                <td className="py-2 px-3"><ReasonBadge reason={t.exitReason} /></td>
                <td className={`py-2 px-3 text-right font-mono font-semibold ${t.netPnl >= 0 ? "text-emerald-400" : "text-rose-400"}`}>
                  {fmtUSD(t.netPnl, true)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function StrategiesTable({ strategies }: { strategies: OptionStrategyStatus[] }) {
  return (
    <div className="glass-panel p-6">
      <div className="mb-4 text-xs font-bold uppercase tracking-widest" style={{ color: "var(--text-secondary)" }}>
        All 50 Strategies
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <thead className="border-b text-[10px] uppercase tracking-widest" style={{ color: "var(--text-secondary)", borderColor: "var(--border-subtle)" }}>
            <tr>
              <th className="py-2 px-3">#</th>
              <th className="py-2 px-3">Strategy</th>
              <th className="py-2 px-3">Type</th>
              <th className="py-2 px-3">Status</th>
              <th className="py-2 px-3">Trades</th>
              <th className="py-2 px-3">W / L</th>
              <th className="py-2 px-3">Win Rate</th>
              <th className="py-2 px-3 text-right">Total PnL</th>
            </tr>
          </thead>
          <tbody className="divide-y" style={{ divideColor: "var(--border-subtle)" }}>
            {strategies.map((s, i) => (
              <tr key={s.name} className="hover:bg-white/[0.02]">
                <td className="py-2 px-3" style={{ color: "var(--text-muted)" }}>{i + 1}</td>
                <td className="py-2 px-3 font-medium text-white">{s.name}</td>
                <td className="py-2 px-3"><TypeBadge type={s.optionType} /></td>
                <td className="py-2 px-3"><StatusBadge status={s.status} /></td>
                <td className="py-2 px-3">{s.totalTrades}</td>
                <td className="py-2 px-3">{s.wins}W / {s.losses}L</td>
                <td className="py-2 px-3">{s.totalTrades > 0 ? fmtPct(s.winRate) : "—"}</td>
                <td className={`py-2 px-3 text-right font-mono font-semibold ${s.totalPnl >= 0 ? "text-emerald-400" : "text-rose-400"}`}>
                  {s.totalTrades > 0 ? fmtUSD(s.totalPnl, true) : "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export default function OptionsScalper() {
  const { positions, trades, strategies, stats } = useOptions();

  const handleReset = async () => {
    if (!confirm("Reset the options paper account to $50,000? All history will be cleared.")) return;
    await fetch(`${API_URL}/api/options/reset`, { method: "POST" });
  };

  const netPnL = stats?.totalPnl ?? 0;
  const equity = stats?.equity ?? 50000;
  const winRate = stats?.winRate ?? 0;

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="glass-panel px-6 py-5 flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div>
          <div style={{ fontSize: 10, fontWeight: 800, letterSpacing: "0.2em", color: "var(--text-muted)", textTransform: "uppercase" }}>
            Module
          </div>
          <div className="mt-1 text-xl font-bold text-white">BTC Option Scalper</div>
          <div style={{ fontSize: 12, color: "var(--text-secondary)", marginTop: 4 }}>
            50 autonomous option buying strategies — $50,000 paper account — separate from futures engine
          </div>
        </div>
        <button
          type="button"
          onClick={handleReset}
          className="btn-primary self-start md:self-auto"
        >
          Reset Options Account
        </button>
      </div>

      {/* Key metrics */}
      <div className="grid grid-cols-2 md:grid-cols-4 xl:grid-cols-6 gap-4">
        <MetricCard
          label="Options Equity"
          value={`$${fmt(equity)}`}
          sub={`Cash: $${fmt(stats?.balance ?? 50000)}`}
          accent={equity >= 50000 ? "text-emerald-300" : "text-rose-300"}
        />
        <MetricCard
          label="Net PnL"
          value={fmtUSD(netPnL, true)}
          accent={netPnL >= 0 ? "text-emerald-300" : "text-rose-300"}
        />
        <MetricCard
          label="Unrealized"
          value={fmtUSD(stats?.unrealizedPnl ?? 0, true)}
          accent={(stats?.unrealizedPnl ?? 0) >= 0 ? "text-emerald-300" : "text-rose-300"}
        />
        <MetricCard
          label="Win Rate"
          value={stats?.totalTrades ? fmtPct(winRate) : "—"}
          sub={`${stats?.totalWins ?? 0}W / ${stats?.totalLosses ?? 0}L`}
          accent={winRate >= 50 ? "text-emerald-300" : "text-rose-300"}
        />
        <MetricCard
          label="Open Positions"
          value={`${stats?.openPositions ?? 0}`}
          sub={`of 50 strategies`}
          accent="text-sky-300"
        />
        <MetricCard
          label="Total Trades"
          value={`${stats?.totalTrades ?? 0}`}
          sub={`$${fmt(stats?.totalPremiumSpent ?? 0)} premium spent`}
        />
      </div>

      {/* Open positions */}
      <PositionsTable positions={positions} />

      {/* Strategy table */}
      <StrategiesTable strategies={strategies} />

      {/* Trade history */}
      <TradesTable trades={trades} />
    </div>
  );
}
