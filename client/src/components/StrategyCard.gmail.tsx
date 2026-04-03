"use client";

type StrategyCardProps = {
  name: string;
  status: string;
  exposure: number;
  profit: number;
  timeframe?: string;
  wins?: number;
  losses?: number;
};

export default function StrategyCard({
  name,
  status,
  exposure,
  profit,
  timeframe,
  wins = 0,
  losses = 0,
}: StrategyCardProps) {
  const isRunning = status === "RUNNING";
  const totalTrades = wins + losses;
  const winRate = totalTrades > 0 ? (wins / totalTrades) * 100 : null;

  return (
    <div
      className="rounded-[20px] border p-4 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-md"
      style={{
        background: isRunning ? "var(--surface)" : "var(--surface-2)",
        borderColor: isRunning ? "rgba(26, 115, 232, 0.18)" : "var(--border)",
        boxShadow: isRunning ? "var(--shadow-sm)" : "none",
        opacity: isRunning ? 1 : 0.84,
      }}
    >
      <div className="mb-2 flex items-center justify-between gap-2">
        <h3 className="truncate font-mono text-xs font-semibold tracking-wide" style={{ color: "var(--text-primary)" }}>
          {name}
        </h3>
        <div className="flex items-center gap-1.5">
          {winRate !== null && (
            <span
              className="rounded-full border px-2 py-1 text-[10px] font-medium tracking-[0.08em]"
              style={{
                background: winRate >= 50 ? "var(--green-dim)" : "var(--red-dim)",
                color: winRate >= 50 ? "var(--green)" : "var(--red)",
                borderColor: winRate >= 50 ? "rgba(24, 128, 56, 0.16)" : "rgba(217, 48, 37, 0.16)",
              }}
            >
              {winRate.toFixed(0)}%
            </span>
          )}
          <span
            className="rounded-full border px-2 py-1 text-[10px] font-medium uppercase tracking-[0.08em]"
            style={{
              background: isRunning ? "var(--accent-dim)" : "var(--surface-3)",
              color: isRunning ? "var(--accent)" : "var(--text-secondary)",
              borderColor: isRunning ? "rgba(26, 115, 232, 0.16)" : "var(--border)",
            }}
          >
            {status}
          </span>
        </div>
      </div>

      <div className="mt-3 grid grid-cols-4 gap-2 border-t pt-3 text-xs" style={{ borderColor: "var(--border-subtle)" }}>
        <div>
          <p className="mb-0.5 text-[10px] font-medium uppercase tracking-wider" style={{ color: "var(--text-secondary)" }}>TF</p>
          <p className="font-mono" style={{ color: "var(--text-primary)" }}>{timeframe || "1m"}</p>
        </div>
        <div>
          <p className="mb-0.5 text-[10px] font-medium uppercase tracking-wider" style={{ color: "var(--text-secondary)" }}>W/L</p>
          <p className="font-mono" style={{ color: "var(--text-primary)" }}>{totalTrades > 0 ? `${wins}/${losses}` : "-"}</p>
        </div>
        <div>
          <p className="mb-0.5 text-[10px] font-medium uppercase tracking-wider" style={{ color: "var(--text-secondary)" }}>Exp</p>
          <p className="font-mono" style={{ color: "var(--text-primary)" }}>{exposure.toFixed(3)}</p>
        </div>
        <div>
          <p className="mb-0.5 text-[10px] font-medium uppercase tracking-wider" style={{ color: "var(--text-secondary)" }}>PnL</p>
          <p className="font-mono font-semibold" style={{ color: profit >= 0 ? "var(--green)" : "var(--red)" }}>
            {profit >= 0 ? "+" : ""}${profit.toFixed(0)}
          </p>
        </div>
      </div>
    </div>
  );
}
