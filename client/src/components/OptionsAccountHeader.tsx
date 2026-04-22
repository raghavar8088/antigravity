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
  const resolvedOnlineLabel = onlineLabel ?? `Options engine live · ${marketLabel} strategies`;
  const resolvedDetailLabel =
    detailLabel ??
    `${openPositions} open ${marketLabel} option positions`;

  return (
    <header className="cockpit-header">
      <div
        style={{
          maxWidth: 1680,
          margin: "0 auto",
          display: "flex",
          alignItems: "center",
          gap: 16,
          padding: "0 20px",
          height: "var(--header-height)",
        }}
      >
        {/* ── Left: Market Badge + Title ──────────────────────────── */}
        <div style={{ display: "flex", alignItems: "center", gap: 12, minWidth: 200, flexShrink: 0 }}>
          <div
            style={{
              width: 36,
              height: 36,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              borderRadius: "var(--radius-card)",
              background: "var(--amber-dim)",
              border: "1px solid rgba(227, 116, 0, 0.12)",
            }}
          >
            <span
              style={{
                fontSize: 10,
                fontWeight: 700,
                letterSpacing: "0.1em",
                color: "var(--amber)",
                fontFamily: "var(--font-display)",
              }}
            >
              {marketCode}
            </span>
          </div>
          <div>
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <span
                style={{
                  fontFamily: "var(--font-display)",
                  fontSize: 15,
                  fontWeight: 600,
                  color: "var(--text-primary)",
                  lineHeight: 1.2,
                }}
              >
                {workspaceTitle}
              </span>
              {/* Auto Execute toggle */}
              {onToggleActions && (
                <div
                  title="Locked hides reset/clear only. Paper engines run on the server either way."
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 6,
                    padding: "3px 4px",
                    borderRadius: "var(--radius-chip)",
                    border: "1px solid var(--border)",
                    background: "var(--surface-2)",
                  }}
                >
                  <button
                    type="button"
                    onClick={() => onToggleActions(false)}
                    title="Reset / clear account buttons hidden"
                    style={{
                      padding: "3px 10px",
                      borderRadius: "var(--radius-chip)",
                      border: "none",
                      fontSize: 11,
                      fontWeight: 500,
                      fontFamily: "var(--font-display)",
                      cursor: "pointer",
                      background: !actionsEnabled ? "var(--surface)" : "transparent",
                      color: !actionsEnabled ? "var(--text-primary)" : "var(--text-muted)",
                      boxShadow: !actionsEnabled ? "var(--shadow-xs)" : "none",
                      transition: "all 0.15s ease",
                    }}
                  >
                    Locked
                  </button>
                  <button
                    type="button"
                    onClick={() => onToggleActions(true)}
                    title="Enable reset and clear account"
                    style={{
                      padding: "3px 10px",
                      borderRadius: "var(--radius-chip)",
                      border: "none",
                      fontSize: 11,
                      fontWeight: 500,
                      fontFamily: "var(--font-display)",
                      cursor: "pointer",
                      background: actionsEnabled ? "var(--green)" : "transparent",
                      color: actionsEnabled ? "#fff" : "var(--text-muted)",
                      transition: "all 0.15s ease",
                    }}
                  >
                    Actions
                  </button>
                </div>
              )}
            </div>
            <div style={{ fontSize: 11, color: "var(--text-muted)" }}>
              {accountLabel}
            </div>
          </div>
        </div>

        {/* ── Center: Status ──────────────────────────────────────── */}
        <div
          style={{
            flex: 1,
            display: "flex",
            alignItems: "center",
            gap: 10,
            padding: "0 16px",
            minWidth: 0,
          }}
        >
          <div className={online ? "live-dot" : "live-dot-red"} />
          <div style={{ minWidth: 0 }}>
            <div
              style={{
                fontFamily: "var(--font-display)",
                fontSize: 13,
                fontWeight: 500,
                color: "var(--text-primary)",
              }}
            >
              {online ? resolvedOnlineLabel : offlineLabel}
            </div>
            <div style={{ fontSize: 11, color: "var(--text-muted)" }}>
              {online ? resolvedDetailLabel : "Waiting for options engine data"}
            </div>
          </div>
          {/* Account type badge */}
          <span
            style={{
              flexShrink: 0,
              display: "inline-flex",
              alignItems: "center",
              padding: "3px 10px",
              borderRadius: "var(--radius-chip)",
              background: "var(--amber-dim)",
              color: "var(--amber)",
              fontSize: 11,
              fontWeight: 500,
              fontFamily: "var(--font-display)",
            }}
          >
            {accountBadgeLabel}
          </span>
        </div>

        {/* ── Right: Metrics ──────────────────────────────────────── */}
        <div style={{ display: "flex", alignItems: "center", gap: 10, flexShrink: 0 }}>
          {/* Equity */}
          <div
            style={{
              padding: "6px 14px",
              borderRadius: "var(--radius-card)",
              border: "1px solid var(--border)",
              background: "var(--surface)",
            }}
          >
            <div style={{ fontSize: 10, fontWeight: 500, color: "var(--text-muted)", letterSpacing: "0.03em" }}>
              {equityLabel}
            </div>
            <div
              style={{
                fontFamily: "var(--font-display)",
                fontSize: 16,
                fontWeight: 600,
                color: "var(--text-primary)",
                marginTop: 2,
              }}
            >
              {formatCurrency(equity, currencyCode, locale)}
            </div>
          </div>

          {/* PnL */}
          <div
            style={{
              padding: "6px 14px",
              borderRadius: "var(--radius-card)",
              border: "1px solid var(--border)",
              background: "var(--surface)",
            }}
          >
            <div style={{ fontSize: 10, fontWeight: 500, color: "var(--text-muted)", letterSpacing: "0.03em" }}>
              {pnlLabel}
            </div>
            <div
              style={{
                fontFamily: "var(--font-display)",
                fontSize: 16,
                fontWeight: 600,
                color: positive ? "var(--green)" : "var(--red)",
                marginTop: 2,
              }}
            >
              {formatSignedCurrency(dailyPnL, currencyCode, locale)}
            </div>
            <div style={{ fontSize: 10, color: "var(--text-muted)", marginTop: 1 }}>
              {positive ? "+" : ""}{pnlPct.toFixed(Math.abs(pnlPct) < 0.01 && pnlPct !== 0 ? 4 : 2)}%
            </div>
          </div>

          {/* Open positions */}
          <div
            style={{
              padding: "6px 14px",
              borderRadius: "var(--radius-card)",
              border: "1px solid var(--border)",
              background: "var(--surface)",
              textAlign: "center",
            }}
          >
            <div style={{ fontSize: 10, fontWeight: 500, color: "var(--text-muted)", letterSpacing: "0.03em" }}>
              {openLabel}
            </div>
            <div
              style={{
                fontFamily: "var(--font-display)",
                fontSize: 16,
                fontWeight: 600,
                color: "var(--text-primary)",
                marginTop: 2,
              }}
            >
              {openPositions}
            </div>
          </div>
        </div>
      </div>
    </header>
  );
}
