"use client";

type OptionsAccountHeaderProps = {
  online: boolean;
  equity: number;
  dailyPnL: number;
  openPositions: number;
};

function formatSignedCurrency(value: number) {
  return `${value >= 0 ? "+" : "-"}$${Math.abs(value).toLocaleString(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`;
}

export default function OptionsAccountHeader({
  online,
  equity,
  dailyPnL,
  openPositions,
}: OptionsAccountHeaderProps) {
  const baseBalance = 1_000_000;
  const pnlPct = baseBalance > 0 ? (dailyPnL / baseBalance) * 100 : 0;
  const positive = dailyPnL >= 0;

  return (
    <header className="cockpit-header">
      <div className="mx-auto flex max-w-[1680px] flex-wrap items-center gap-4 px-5 py-4">
        <div className="flex min-w-[220px] items-center gap-3">
          <div
            className="flex h-12 w-12 items-center justify-center rounded-2xl border"
            style={{
              borderColor: "rgba(176, 96, 0, 0.18)",
              background: "rgba(197, 139, 0, 0.08)",
            }}
          >
            <div
              className="text-[11px] font-bold uppercase tracking-[0.18em]"
              style={{ color: "var(--amber)" }}
            >
              OPT
            </div>
          </div>
          <div>
            <div className="text-sm font-medium" style={{ color: "var(--text-primary)" }}>
              RAIG Options Workspace
            </div>
            <div className="text-xs" style={{ color: "var(--text-secondary)" }}>
              BTC options paper account
            </div>
          </div>
        </div>

        <div
          className="flex min-w-[240px] flex-1 items-center gap-3 rounded-full border px-4 py-3"
          style={{
            borderColor: "var(--border)",
            background: "var(--surface)",
          }}
        >
          <div className={online ? "live-dot" : "live-dot-red"} />
          <div className="flex-1">
            <div className="text-sm font-medium" style={{ color: "var(--text-primary)" }}>
              {online ? "Options engine live and monitoring BTC option strategies" : "Options engine offline"}
            </div>
            <div className="text-xs" style={{ color: "var(--text-secondary)" }}>
              {online
                ? `${openPositions} open BTC option positions in the separate options account`
                : "Waiting for options engine data"}
            </div>
          </div>
          <span
            className="inline-flex items-center rounded-full border px-3 py-2 text-xs font-medium"
            style={{
              borderColor: "rgba(176, 96, 0, 0.18)",
              background: "rgba(197, 139, 0, 0.08)",
              color: "var(--amber)",
            }}
          >
            Separate Account
          </span>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <div className="summary-card min-w-[170px]">
            <div className="summary-label">Options Equity</div>
            <div className="summary-value">
              ${equity.toLocaleString(undefined, {
                minimumFractionDigits: 2,
                maximumFractionDigits: 2,
              })}
            </div>
          </div>

          <div className="summary-card min-w-[170px]">
            <div className="summary-label">Options PnL Today</div>
            <div className={`summary-value ${positive ? "profit-positive" : "profit-negative"}`}>
              {formatSignedCurrency(dailyPnL)}
            </div>
            <div className="mt-2 text-xs" style={{ color: "var(--text-secondary)" }}>
              {positive ? "+" : ""}
              {pnlPct.toFixed(Math.abs(pnlPct) < 0.01 && pnlPct !== 0 ? 4 : 2)}%
            </div>
          </div>

          <div className="metric-card min-w-[140px]">
            <div className="metric-label">Open Options</div>
            <div className="metric-value">{openPositions}</div>
          </div>
        </div>
      </div>
    </header>
  );
}
