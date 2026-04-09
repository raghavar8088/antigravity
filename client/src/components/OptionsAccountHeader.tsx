"use client";

type OptionsAccountHeaderProps = {
  online: boolean;
  equity: number;
  dailyPnL: number;
  openPositions: number;
  marketLabel?: string;
  marketCode?: string;
  accountLabel?: string;
  currencyCode?: string;
  locale?: string;
  workspaceTitle?: string;
  onlineLabel?: string;
  offlineLabel?: string;
  detailLabel?: string;
  accountBadgeLabel?: string;
  equityLabel?: string;
  pnlLabel?: string;
  openLabel?: string;
  actionsEnabled?: boolean;
  onToggleActions?: (enabled: boolean) => void;
};

function formatCurrency(value: number, currencyCode: string, locale: string) {
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency: currencyCode,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value);
}

function formatSignedCurrency(value: number, currencyCode: string, locale: string) {
  return `${value >= 0 ? "+" : "-"}${formatCurrency(Math.abs(value), currencyCode, locale)}`;
}

export default function OptionsAccountHeader({
  online,
  equity,
  dailyPnL,
  openPositions,
  marketLabel = "BTC",
  marketCode = "OPT",
  accountLabel = "BTC options paper account",
  currencyCode = "USD",
  locale = "en-US",
  workspaceTitle = "RAIG Options Workspace",
  onlineLabel,
  offlineLabel = "Options engine offline",
  detailLabel,
  accountBadgeLabel = "Separate Account",
  equityLabel = "Options Equity",
  pnlLabel = "Options PnL Today",
  openLabel = "Open Options",
  actionsEnabled,
  onToggleActions,
}: OptionsAccountHeaderProps) {
  const baseBalance = 1_000_000;
  const pnlPct = baseBalance > 0 ? (dailyPnL / baseBalance) * 100 : 0;
  const positive = dailyPnL >= 0;
  const resolvedOnlineLabel = onlineLabel ?? `Options engine live and monitoring ${marketLabel} option strategies`;
  const resolvedDetailLabel =
    detailLabel ??
    `${openPositions} open ${marketLabel} option positions in the separate options account`;

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
              {marketCode}
            </div>
          </div>
          <div>
            <div className="flex items-center gap-3">
              <span className="text-sm font-medium" style={{ color: "var(--text-primary)" }}>
                {workspaceTitle}
              </span>
              {onToggleActions && (
                <div className="inline-flex items-center gap-2 rounded-full border px-2.5 py-1" style={{ borderColor: "var(--border-subtle)", background: "var(--surface-2)" }}>
                  <span className="text-[10px] font-semibold uppercase tracking-[0.14em]" style={{ color: "var(--text-secondary)" }}>Action</span>
                  <div className="inline-flex items-center rounded-full border p-0.5" style={{ borderColor: "var(--border-subtle)", background: "var(--surface)" }}>
                    <button
                      type="button"
                      onClick={() => onToggleActions(false)}
                      className="rounded-full px-2 py-0.5 text-[10px] font-semibold transition"
                      style={{
                        background: !actionsEnabled ? "var(--surface-3)" : "transparent",
                        color: !actionsEnabled ? "var(--text-primary)" : "var(--text-secondary)",
                      }}
                    >
                      No
                    </button>
                    <button
                      type="button"
                      onClick={() => onToggleActions(true)}
                      className="rounded-full px-2 py-0.5 text-[10px] font-semibold transition"
                      style={{
                        background: actionsEnabled ? "rgba(21, 128, 61, 0.16)" : "transparent",
                        color: actionsEnabled ? "var(--green)" : "var(--text-secondary)",
                      }}
                    >
                      Yes
                    </button>
                  </div>
                </div>
              )}
            </div>
            <div className="text-xs" style={{ color: "var(--text-secondary)" }}>
              {accountLabel}
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
              {online ? resolvedOnlineLabel : offlineLabel}
            </div>
            <div className="text-xs" style={{ color: "var(--text-secondary)" }}>
              {online
                ? resolvedDetailLabel
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
            {accountBadgeLabel}
          </span>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <div className="summary-card min-w-[170px]">
            <div className="summary-label">{equityLabel}</div>
            <div className="summary-value">
              {formatCurrency(equity, currencyCode, locale)}
            </div>
          </div>

          <div className="summary-card min-w-[170px]">
            <div className="summary-label">{pnlLabel}</div>
            <div className={`summary-value ${positive ? "profit-positive" : "profit-negative"}`}>
              {formatSignedCurrency(dailyPnL, currencyCode, locale)}
            </div>
            <div className="mt-2 text-xs" style={{ color: "var(--text-secondary)" }}>
              {positive ? "+" : ""}
              {pnlPct.toFixed(Math.abs(pnlPct) < 0.01 && pnlPct !== 0 ? 4 : 2)}%
            </div>
          </div>

          <div className="metric-card min-w-[140px]">
            <div className="metric-label">{openLabel}</div>
            <div className="metric-value">{openPositions}</div>
          </div>
        </div>
      </div>
    </header>
  );
}
