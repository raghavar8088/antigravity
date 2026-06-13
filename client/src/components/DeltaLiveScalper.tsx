"use client";
import { useState } from "react";
import { submitExecutionRequest } from "@/lib/trading/executionRequest";
import useDeltaLive, {
  type DeltaLiveTrade,
  type DeltaLiveStats,
  type DeltaRuntimeStatus,
  type DeltaLocalConfig,
  type WalletEntry,
  type LivePosition,
  type OpenOrder,
} from "@/hooks/useDeltaLive";
import useDeltaStrikes from "@/hooks/useDeltaStrikes";

const CONFIG_LS_KEY = "delta_api_config_v1";

type Props = { actionsEnabled?: boolean };

function fmt(n: number, dp = 2) {
  return n.toLocaleString("en-US", { minimumFractionDigits: dp, maximumFractionDigits: dp });
}
function fmtTime(iso: string) {
  try { return new Date(iso).toLocaleTimeString(); } catch { return iso; }
}
function fmtDate(iso: string) {
  try { return new Date(iso).toLocaleString(); } catch { return iso; }
}

function pnlColor(v: number) { return v >= 0 ? "profit-positive" : "profit-negative"; }
function pnlSign(v: number) { return v >= 0 ? "+" : ""; }

// ─── Status badge ───────────────────────────────────────────────────────────
function StatusBadge({ status }: { status: DeltaLiveTrade["status"] }) {
  const colors: Record<string, { bg: string; color: string }> = {
    OPEN:      { bg: "var(--accent-dim)", color: "var(--accent)" },
    CLOSED:    { bg: "var(--green-dim)", color: "var(--green)" },
    FAILED:    { bg: "var(--red-dim)", color: "var(--red)" },
    CANCELLED: { bg: "var(--surface-3)", color: "var(--text-muted)" },
  };
  const c = colors[status] ?? colors.CANCELLED;
  return (
    <span
      style={{
        display: "inline-flex",
        alignItems: "center",
        padding: "2px 8px",
        borderRadius: "var(--radius-chip)",
        background: c.bg,
        color: c.color,
        fontSize: 11,
        fontWeight: 600,
        fontFamily: "var(--font-display)",
      }}
    >
      {status}
    </span>
  );
}

// ─── Enable/Disable banner ───────────────────────────────────────────────────
function RuntimeStatusPill({ label, configured, detail }: { label: string; configured: boolean; detail: string }) {
  return (
    <div
      style={{
        padding: "10px 14px",
        borderRadius: "var(--radius-card)",
        border: `1px solid ${configured ? "rgba(30, 142, 62, 0.2)" : "rgba(217, 48, 37, 0.2)"}`,
        background: configured ? "var(--green-dim)" : "var(--red-dim)",
        fontSize: 12,
      }}
    >
      <div style={{ fontWeight: 600, color: configured ? "var(--green)" : "var(--red)", fontFamily: "var(--font-display)" }}>
        {label}: {configured ? "Configured" : "Not Configured"}
      </div>
      <div style={{ marginTop: 4, color: "var(--text-secondary)", fontSize: 11 }}>{detail}</div>
    </div>
  );
}

function EnableBanner({
  stats,
  nextStatus,
  toggling,
  onToggle,
}: {
  stats: DeltaLiveStats;
  nextStatus: DeltaRuntimeStatus;
  toggling: boolean;
  onToggle: (v: boolean) => void;
}) {
  if (!stats.configured || !nextStatus.configured) {
    return (
      <div
        style={{
          padding: 20,
          borderRadius: "var(--radius-card)",
          border: "1px solid var(--amber)",
          background: "var(--amber-dim)",
        }}
      >
        <div style={{ display: "flex", gap: 14, alignItems: "flex-start" }}>
          <span style={{ fontSize: 24, lineHeight: 1 }}>⚠️</span>
          <div style={{ flex: 1 }}>
            <div style={{ fontFamily: "var(--font-display)", fontWeight: 600, fontSize: 14, color: "var(--text-primary)", marginBottom: 4 }}>
              Delta Runtime Configuration Check
            </div>
            <div style={{ fontSize: 12, color: "var(--text-secondary)", marginBottom: 12, lineHeight: 1.5 }}>
              This screen depends on two different runtimes. The Go engine powers live mirroring, while Vercel / Next.js powers the test-order routes.
            </div>
            <div style={{ display: "grid", gap: 12, gridTemplateColumns: "repeat(auto-fit, minmax(260px, 1fr))" }}>
              <RuntimeStatusPill
                label="Go Engine"
                configured={stats.configured}
                detail={stats.configured ? `Ready${stats.testnet ? " · testnet" : " · production"}` : "Set DELTA_API_KEY and DELTA_API_SECRET on the Go backend server"}
              />
              <RuntimeStatusPill
                label="Vercel / Next.js"
                configured={nextStatus.configured}
                detail={nextStatus.configured ? `Ready${nextStatus.testnet ? " · testnet" : " · production"}` : "Set DELTA_API_KEY and DELTA_API_SECRET in Vercel project env vars"}
              />
            </div>
            <div
              style={{
                marginTop: 12,
                padding: 14,
                borderRadius: "var(--radius-input)",
                background: "var(--surface)",
                border: "1px solid var(--border)",
                fontFamily: "var(--font-mono)",
                fontSize: 12,
                lineHeight: 1.8,
              }}
            >
              <div style={{ color: "var(--green)" }}>DELTA_API_KEY=<span style={{ color: "var(--text-muted)" }}>your_api_key</span></div>
              <div style={{ color: "var(--green)" }}>DELTA_API_SECRET=<span style={{ color: "var(--text-muted)" }}>your_api_secret</span></div>
              <div style={{ color: "var(--text-muted)" }}># Optional testnet</div>
              <div style={{ color: "var(--text-secondary)" }}>DELTA_TESTNET=true</div>
            </div>
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 8 }}>
              API keys: <span style={{ color: "var(--accent)" }}>india.delta.exchange → Settings → API Keys</span>
            </div>
            {nextStatus.error && (
              <div style={{ fontSize: 11, color: "var(--amber)", marginTop: 8 }}>
                Vercel / Next.js message: {nextStatus.error}
              </div>
            )}
          </div>
        </div>
      </div>
    );
  }
  return (
    <div
      style={{
        padding: 16,
        borderRadius: "var(--radius-card)",
        border: `1px solid ${stats.enabled ? "var(--green)" : "var(--border)"}`,
        background: stats.enabled ? "var(--green-dim)" : "var(--surface)",
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: 16,
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
        <div
          style={{
            width: 10,
            height: 10,
            borderRadius: "var(--radius-chip)",
            background: stats.enabled ? "var(--green)" : "var(--text-muted)",
            boxShadow: stats.enabled ? "0 0 0 3px rgba(30, 142, 62, 0.2)" : "none",
          }}
        />
        <div>
          <div style={{ fontFamily: "var(--font-display)", fontWeight: 600, fontSize: 14, color: "var(--text-primary)" }}>
            Live Order Mirroring {stats.enabled ? "ACTIVE" : "PAUSED"}
          </div>
          <div style={{ fontSize: 12, color: "var(--text-secondary)", marginTop: 2 }}>
            {stats.testnet ? "🧪 Testnet" : "🔴 Production — real money"}&nbsp;·&nbsp;
            Mirrors BTC Option Selling signals to Delta. Turn on <strong>Buying mode</strong> for small balances (long options); selling needs large margin.
          </div>
        </div>
      </div>
      <button
        type="button"
        disabled={toggling}
        onClick={() => onToggle(!stats.enabled)}
        className={stats.enabled ? "btn-danger" : "btn-primary"}
        style={{ minWidth: 100 }}
      >
        {toggling ? "..." : stats.enabled ? "Disable" : "Enable"}
      </button>
    </div>
  );
}

// ─── Wallet cards ────────────────────────────────────────────────────────────
function WalletCards({ wallets }: { wallets: WalletEntry[] }) {
  if (!wallets?.length) return <div style={{ textAlign: "center", padding: 20, color: "var(--text-muted)", fontSize: 13 }}>No wallet data</div>;
  return (
    <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))", gap: 12 }}>
      {wallets.map((w) => (
        <div
          key={w.asset}
          style={{
            padding: 14,
            borderRadius: "var(--radius-card)",
            border: "1px solid var(--border)",
            background: "var(--surface)",
          }}
        >
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 8 }}>
            <span
              style={{
                padding: "2px 8px",
                borderRadius: "var(--radius-chip)",
                background: "var(--accent-dim)",
                color: "var(--accent)",
                fontSize: 11,
                fontWeight: 600,
                fontFamily: "var(--font-display)",
              }}
            >
              {w.asset}
            </span>
            {w.unrealisedPnl !== 0 && (
              <span style={{ fontSize: 11, fontWeight: 500, color: w.unrealisedPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                {pnlSign(w.unrealisedPnl)}{fmt(w.unrealisedPnl)} uPnL
              </span>
            )}
          </div>
          <div style={{ fontSize: 18, fontWeight: 600, color: "var(--text-primary)", fontFamily: "var(--font-display)", marginBottom: 6 }}>
            {fmt(w.balance, 4)}
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 3 }}>
            <div style={{ display: "flex", justifyContent: "space-between", fontSize: 12 }}>
              <span style={{ color: "var(--text-muted)" }}>Available</span>
              <span style={{ color: "var(--green)", fontWeight: 500, fontFamily: "var(--font-mono)" }}>{fmt(w.availableBalance, 4)}</span>
            </div>
            {w.blockedBalance > 0 && (
              <div style={{ display: "flex", justifyContent: "space-between", fontSize: 12 }}>
                <span style={{ color: "var(--text-muted)" }}>In Margin</span>
                <span style={{ color: "var(--amber)", fontFamily: "var(--font-mono)" }}>{fmt(w.blockedBalance, 4)}</span>
              </div>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

// ─── Google-style data table ─────────────────────────────────────────────────
const thStyle: React.CSSProperties = {
  padding: "10px 14px",
  textAlign: "left",
  color: "var(--text-secondary)",
  fontSize: 11,
  fontWeight: 500,
  letterSpacing: "0.02em",
  borderBottom: "1px solid var(--border)",
  whiteSpace: "nowrap",
};
const thRight: React.CSSProperties = { ...thStyle, textAlign: "right" };
const tdStyle: React.CSSProperties = {
  padding: "10px 14px",
  fontSize: 13,
  color: "var(--text-primary)",
  borderBottom: "1px solid var(--border-subtle)",
  verticalAlign: "middle",
};
const tdRight: React.CSSProperties = { ...tdStyle, textAlign: "right" };
const tdMuted: React.CSSProperties = { ...tdStyle, color: "var(--text-muted)" };
const tdMono: React.CSSProperties = { ...tdStyle, fontFamily: "var(--font-mono)" };

// ─── Live positions on Delta ─────────────────────────────────────────────────
function LivePositionsTable({ positions }: { positions: LivePosition[] }) {
  if (!positions?.length) {
    return <div style={{ textAlign: "center", padding: 32, color: "var(--text-muted)", fontSize: 13 }}>No open positions on Delta Exchange</div>;
  }
  return (
    <div style={{ overflowX: "auto" }}>
      <table style={{ width: "100%", borderCollapse: "collapse" }}>
        <thead>
          <tr>
            <th style={thStyle}>Symbol</th>
            <th style={thStyle}>Side</th>
            <th style={thRight}>Size</th>
            <th style={thRight}>Entry</th>
            <th style={thRight}>Mark</th>
            <th style={thRight}>uPnL</th>
            <th style={thRight}>rPnL</th>
            <th style={thRight}>Margin</th>
          </tr>
        </thead>
        <tbody>
          {positions.map((p, i) => (
            <tr key={i} style={{ transition: "background-color 0.1s" }} onMouseEnter={e => (e.currentTarget.style.background = "var(--accent-hover)")} onMouseLeave={e => (e.currentTarget.style.background = "")}>
              <td style={{ ...tdMono, color: "var(--accent)", fontWeight: 600 }}>{p.symbol}</td>
              <td style={tdStyle}>
                <span style={{ fontWeight: 600, color: p.side === "LONG" ? "var(--green)" : "var(--red)" }}>{p.side}</span>
              </td>
              <td style={{ ...tdRight, fontFamily: "var(--font-mono)" }}>{fmt(Math.abs(p.size), 0)}</td>
              <td style={{ ...tdRight, fontFamily: "var(--font-mono)" }}>${fmt(p.entryPrice, 2)}</td>
              <td style={{ ...tdRight, fontFamily: "var(--font-mono)", color: "var(--text-secondary)" }}>${fmt(p.markPrice, 2)}</td>
              <td style={{ ...tdRight, fontWeight: 600, color: p.unrealisedPnl >= 0 ? "var(--green)" : "var(--red)", fontFamily: "var(--font-mono)" }}>
                {pnlSign(p.unrealisedPnl)}${fmt(p.unrealisedPnl)}
              </td>
              <td style={{ ...tdRight, color: p.realisedPnl >= 0 ? "var(--green)" : "var(--red)", fontFamily: "var(--font-mono)" }}>
                {pnlSign(p.realisedPnl)}${fmt(p.realisedPnl)}
              </td>
              <td style={{ ...tdRight, color: "var(--amber)", fontFamily: "var(--font-mono)" }}>${fmt(p.margin)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ─── Open orders on Delta ─────────────────────────────────────────────────────
function OpenOrdersTable({ orders }: { orders: OpenOrder[] }) {
  if (!orders?.length) {
    return <div style={{ textAlign: "center", padding: 20, color: "var(--text-muted)", fontSize: 13 }}>No open orders</div>;
  }
  return (
    <div style={{ overflowX: "auto" }}>
      <table style={{ width: "100%", borderCollapse: "collapse" }}>
        <thead>
          <tr>
            <th style={thStyle}>Order ID</th>
            <th style={thStyle}>Symbol</th>
            <th style={thStyle}>Side</th>
            <th style={thRight}>Size</th>
            <th style={thRight}>Price</th>
            <th style={thStyle}>State</th>
            <th style={thStyle}>Time</th>
          </tr>
        </thead>
        <tbody>
          {orders.map((o) => (
            <tr key={o.orderId} style={{ transition: "background-color 0.1s" }} onMouseEnter={e => (e.currentTarget.style.background = "var(--accent-hover)")} onMouseLeave={e => (e.currentTarget.style.background = "")}>
              <td style={{ ...tdMuted, fontFamily: "var(--font-mono)", fontSize: 11 }}>{o.orderId}</td>
              <td style={{ ...tdMono, color: "var(--accent)", fontWeight: 600 }}>{o.symbol}</td>
              <td style={tdStyle}>
                <span style={{ fontWeight: 600, color: o.side === "buy" ? "var(--green)" : "var(--red)" }}>{o.side.toUpperCase()}</span>
              </td>
              <td style={{ ...tdRight, fontFamily: "var(--font-mono)" }}>{fmt(o.size, 0)}</td>
              <td style={{ ...tdRight, fontFamily: "var(--font-mono)" }}>${fmt(o.price, 2)}</td>
              <td style={{ ...tdStyle, color: "var(--amber)", fontWeight: 500 }}>{o.state}</td>
              <td style={tdMuted}>{fmtTime(o.createdAt)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ─── Mirrored trades table ────────────────────────────────────────────────────
function MirroredTradesTable({ trades }: { trades: DeltaLiveTrade[] }) {
  if (!trades.length) {
    return <div style={{ textAlign: "center", padding: 40, color: "var(--text-muted)", fontSize: 13 }}>No Delta trade records are available on the server.</div>;
  }
  return (
    <div style={{ overflowX: "auto" }}>
      <table style={{ width: "100%", borderCollapse: "collapse" }}>
        <thead>
          <tr>
            <th style={thStyle}>ID</th>
            <th style={thStyle}>Strategy</th>
            <th style={thStyle}>Type</th>
            <th style={thRight}>Strike</th>
            <th style={thStyle}>Delta Symbol</th>
            <th style={thRight}>Qty</th>
            <th style={thRight}>Fill $</th>
            <th style={thRight}>PnL</th>
            <th style={thStyle}>Status</th>
            <th style={thStyle}>Opened</th>
          </tr>
        </thead>
        <tbody>
          {trades.map((t) => (
            <tr key={t.id} style={{ transition: "background-color 0.1s" }} onMouseEnter={e => (e.currentTarget.style.background = "var(--accent-hover)")} onMouseLeave={e => (e.currentTarget.style.background = "")}>
              <td style={{ ...tdMuted, fontFamily: "var(--font-mono)", fontSize: 11 }}>{t.id}</td>
              <td style={{ ...tdStyle, maxWidth: 120, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{t.strategyName}</td>
              <td style={tdStyle}>
                <span style={{ fontWeight: 600, color: t.optionType === "CALL" ? "var(--green)" : "var(--red)" }}>{t.optionType}</span>
              </td>
              <td style={{ ...tdRight, fontFamily: "var(--font-mono)" }}>${fmt(t.strike, 0)}</td>
              <td style={{ ...tdMono, color: "var(--accent)" }}>{t.deltaSymbol || "—"}</td>
              <td style={{ ...tdRight, fontFamily: "var(--font-mono)" }}>{t.contracts}</td>
              <td style={{ ...tdRight, fontFamily: "var(--font-mono)" }}>${fmt(t.fillPrice, 4)}</td>
              <td style={tdRight}>
                {t.status === "CLOSED" && t.realizedPnl != null ? (
                  <span style={{ fontWeight: 600, color: t.realizedPnl >= 0 ? "var(--green)" : "var(--red)", fontFamily: "var(--font-mono)" }}>
                    {pnlSign(t.realizedPnl)}${fmt(t.realizedPnl)}
                  </span>
                ) : t.status === "FAILED" ? (
                  <span style={{ color: "var(--red)", fontSize: 11 }} title={t.failureReason}>ERR</span>
                ) : <span style={{ color: "var(--text-muted)" }}>—</span>}
              </td>
              <td style={tdStyle}><StatusBadge status={t.status} /></td>
              <td style={tdMuted}>{fmtTime(t.openedAt)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ─── Main component ───────────────────────────────────────────────────────────
type TestOrderResponse = {
  ok: boolean;
  error?: string;
  orderId?: string;
  closeOrderId?: string;
  symbol?: string;
  productId?: number;
  contracts?: number;
  fillPrice?: number;
  closeFillPrice?: number;
  state?: string;
};

const inputStyle: React.CSSProperties = {
  width: "100%",
  padding: "8px 12px",
  borderRadius: "var(--radius-input)",
  border: "1px solid var(--border)",
  background: "var(--surface)",
  color: "var(--text-primary)",
  fontSize: 13,
  outline: "none",
  fontFamily: "var(--font-body)",
  transition: "border-color 0.15s ease",
};

const selectStyle: React.CSSProperties = {
  ...inputStyle,
  cursor: "pointer",
};

// ─── Delta API Config Panel ──────────────────────────────────────────────────
function DeltaConfigPanel({
  config,
  onSave,
}: {
  config: DeltaLocalConfig | null;
  onSave: (c: DeltaLocalConfig) => void;
}) {
  const [open, setOpen] = useState(!config?.apiKey);
  const [key, setKey] = useState(config?.apiKey ?? "");
  const [secret, setSecret] = useState(config?.apiSecret ?? "");
  const [testnet, setTestnet] = useState(config?.testnet ?? false);
  const [showSecret, setShowSecret] = useState(false);
  const [saved, setSaved] = useState(false);

  const handleSave = () => {
    onSave({ apiKey: key.trim(), apiSecret: secret.trim(), testnet });
    setSaved(true);
    setOpen(false);
    setTimeout(() => setSaved(false), 2000);
  };
  const handleClear = () => {
    setKey(""); setSecret(""); setTestnet(false);
    onSave({ apiKey: "", apiSecret: "", testnet: false });
    setOpen(true);
  };
  const hasKeys = !!(config?.apiKey && config?.apiSecret);

  return (
    <div
      style={{
        borderRadius: "var(--radius-card)",
        border: "1px solid var(--border)",
        background: "var(--surface)",
        overflow: "hidden",
      }}
    >
      {/* Header row */}
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        style={{
          width: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "12px 16px",
          background: "none",
          border: "none",
          cursor: "pointer",
          gap: 12,
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <span style={{ fontSize: 14 }}>🔑</span>
          <span style={{ fontFamily: "var(--font-display)", fontWeight: 600, fontSize: 13, color: "var(--text-primary)" }}>
            API Key Configuration
          </span>
          {hasKeys && (
            <span
              style={{
                padding: "2px 8px",
                borderRadius: "var(--radius-chip)",
                background: "var(--green-dim)",
                color: "var(--green)",
                fontSize: 11,
                fontWeight: 600,
              }}
            >
              {config?.testnet ? "Testnet" : "Production"}
            </span>
          )}
          {!hasKeys && (
            <span
              style={{
                padding: "2px 8px",
                borderRadius: "var(--radius-chip)",
                background: "var(--amber-dim)",
                color: "var(--amber)",
                fontSize: 11,
                fontWeight: 600,
              }}
            >
              Not Set
            </span>
          )}
          {saved && (
            <span style={{ fontSize: 11, color: "var(--green)", fontWeight: 500 }}>Saved</span>
          )}
        </div>
        <span style={{ color: "var(--text-muted)", fontSize: 11 }}>{open ? "▲" : "▼"}</span>
      </button>

      {/* Form */}
      {open && (
        <div style={{ padding: "0 16px 16px", display: "flex", flexDirection: "column", gap: 12 }}>
          <div style={{ fontSize: 11, color: "var(--text-muted)", lineHeight: 1.5 }}>
            Keys are stored in your browser only (localStorage) and sent directly to Delta Exchange via Vercel Edge.
            Get your key at <span style={{ color: "var(--accent)" }}>india.delta.exchange → Settings → API Keys</span>.
          </div>

          {/* API Key */}
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <label style={{ fontSize: 11, fontWeight: 600, color: "var(--text-secondary)" }}>API Key</label>
            <input
              type="text"
              value={key}
              onChange={(e) => setKey(e.target.value)}
              placeholder="Paste your API key here"
              autoComplete="off"
              style={inputStyle}
            />
          </div>

          {/* API Secret */}
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
              <label style={{ fontSize: 11, fontWeight: 600, color: "var(--text-secondary)" }}>API Secret</label>
              <button
                type="button"
                onClick={() => setShowSecret((v) => !v)}
                style={{ fontSize: 10, color: "var(--accent)", background: "none", border: "none", cursor: "pointer" }}
              >
                {showSecret ? "Hide" : "Show"}
              </button>
            </div>
            <input
              type={showSecret ? "text" : "password"}
              value={secret}
              onChange={(e) => setSecret(e.target.value)}
              placeholder="Paste your API secret here"
              autoComplete="off"
              style={inputStyle}
            />
          </div>

          {/* Testnet toggle */}
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <div>
              <div style={{ fontSize: 12, fontWeight: 600, color: "var(--text-primary)" }}>Use Testnet</div>
              <div style={{ fontSize: 11, color: "var(--text-muted)" }}>
                {testnet ? "cdn-ind.testnet.deltaex.org — paper money" : "cdn.india.deltaex.org — real money"}
              </div>
            </div>
            <button
              type="button"
              aria-label={testnet ? "Disable testnet" : "Enable testnet"}
              onClick={() => setTestnet((v) => !v)}
              style={{
                width: 44,
                height: 24,
                borderRadius: 12,
                border: "none",
                cursor: "pointer",
                background: testnet ? "var(--accent)" : "var(--surface-3)",
                position: "relative",
                transition: "background 0.2s",
                flexShrink: 0,
              }}
            >
              <span
                style={{
                  position: "absolute",
                  top: 3,
                  left: testnet ? 22 : 3,
                  width: 18,
                  height: 18,
                  borderRadius: "50%",
                  background: "#fff",
                  transition: "left 0.2s",
                }}
              />
            </button>
          </div>

          {/* Actions */}
          <div style={{ display: "flex", gap: 8 }}>
            <button
              type="button"
              onClick={handleSave}
              disabled={!key.trim() || !secret.trim()}
              className="btn-primary"
              style={{ flex: 1, opacity: (!key.trim() || !secret.trim()) ? 0.5 : 1 }}
            >
              Save Keys
            </button>
            {hasKeys && (
              <button
                type="button"
                onClick={handleClear}
                className="btn-danger"
                style={{ flex: "0 0 auto" }}
              >
                Clear
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function TestOrderTab({
  actionsEnabled,
  positions,
  onOrderPlaced,
  walletUsdt,
  config,
}: {
  actionsEnabled: boolean;
  positions: LivePosition[];
  onOrderPlaced: () => void;
  walletUsdt: number;
  config: DeltaLocalConfig | null;
}) {
  const [side, setSide] = useState<"buy" | "sell">("sell"); // sell = open, buy = close/reduce
  const [orderType, setOrderType] = useState<"market" | "limit">("market");
  const [strike, setStrike] = useState("");
  const [premiumUsd, setPremiumUsd] = useState("10");
  const [contracts, setContracts] = useState("1");
  const [leverage] = useState("10x");
  const [reduceOnly, setReduceOnly] = useState(false);
  
  // Custom Hook for strikes
  const { strikes } = useDeltaStrikes();
  
  const [submitting, setSubmitting] = useState(false);
  const [feedback, setFeedback] = useState<{ tone: "success" | "error"; text: string } | null>(null);
  const [lastResult, setLastResult] = useState<TestOrderResponse | null>(null);

  // Auto-switch side if reduceOnly is toggled
  const handleReduceChange = (v: boolean) => {
    setReduceOnly(v);
    if (v) setSide("buy");
  };

  const handleAction = async () => {
    setSubmitting(true);
    setFeedback(null);
    try {
      const isClose = side === "buy" || reduceOnly;
      const payload = isClose 
        ? {
            action: "close",
            productId: Number(strike), // In 'close' mode, user can treat strike field as product ID or we use a dedicated logic
            contracts: Number(contracts),
          }
        : {
            action: "open",
            optionType: "CALL", // default to CALL, can be improved to detect from symbol
            strike: Number(strike),
            premiumUsd: Number(premiumUsd),
          };

      const mirrorHeaders: Record<string, string> = { "Content-Type": "application/json" };
      if (config?.apiKey) mirrorHeaders["x-delta-api-key"] = config.apiKey;
      if (config?.apiSecret) mirrorHeaders["x-delta-api-secret"] = config.apiSecret;
      if (config?.testnet !== undefined) mirrorHeaders["x-delta-testnet"] = String(config.testnet);

      const result = await submitExecutionRequest({
        venue: "delta",
        symbol: isClose ? String(strike) : `OPT-${strike}`,
        side: isClose ? "buy" : "sell",
        contracts: Number(contracts) || 1,
        strategyName: "DELTA_MIRROR_UI",
        reason: isClose ? "mirror_close" : "mirror_open",
      });
      const data: TestOrderResponse = {
        ok: result.ok,
        error: result.ok ? undefined : result.message,
        orderId: result.clientOrderId,
      };
      setLastResult(data);
      if (result.ok && data.ok) {
        setFeedback({
          tone: "success",
          text: `${isClose ? "Close" : "Open"} order placed successfully.`,
        });
        onOrderPlaced();
      } else {
        setFeedback({ tone: "error", text: data.error ?? "Order execution failed." });
      }
    } catch (error) {
      setFeedback({ tone: "error", text: String(error) });
    } finally {
      setSubmitting(false);
    }
  };

  const sideColor = side === "buy" ? "var(--green)" : "var(--red)";
  const orderTypeOptions: Array<{ label: string; value: "limit" | "market" }> = [
    { label: "Limit", value: "limit" },
    { label: "Market", value: "market" },
  ];

  return (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 24, alignItems: "flex-start" }}>
      {/* ── Left Side: Order Panel ────────────────────────────────── */}
      <div
        style={{
          flex: "0 0 340px",
          display: "flex",
          flexDirection: "column",
          gap: 16,
          padding: 20,
          borderRadius: "var(--radius-card)",
          border: "1px solid var(--border)",
          background: "var(--surface)",
        }}
      >
        {/* Buy/Sell Side Toggle */}
        <div style={{ display: "flex", height: 36, background: "var(--surface-2)", borderRadius: 6, padding: 3, gap: 4 }}>
          <button
            onClick={() => { setSide("buy"); setReduceOnly(true); }}
            style={{
              flex: 1, border: "none", borderRadius: 4, cursor: "pointer", fontSize: 13, fontWeight: 600,
              background: side === "buy" ? "var(--green)" : "transparent",
              color: side === "buy" ? "#fff" : "var(--text-secondary)",
              transition: "all 0.2s"
            }}
          >
            Buy | Long
          </button>
          <button
            onClick={() => { setSide("sell"); setReduceOnly(false); }}
            style={{
              flex: 1, border: "none", borderRadius: 4, cursor: "pointer", fontSize: 13, fontWeight: 600,
              background: side === "sell" ? "var(--red)" : "transparent",
              color: side === "sell" ? "#fff" : "var(--text-secondary)",
              transition: "all 0.2s"
            }}
          >
            Sell | Short
          </button>
        </div>

        {/* Leverage Select */}
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", fontSize: 13 }}>
          <span style={{ color: "var(--text-secondary)" }}>Leverage</span>
          <div style={{ display: "flex", alignItems: "center", gap: 4, color: "var(--amber)", fontWeight: 600, cursor: "pointer" }}>
            {leverage} <span style={{ fontSize: 10 }}>▼</span>
          </div>
        </div>

        {/* Order Type Tabs */}
        <div style={{ display: "flex", borderBottom: "1px solid var(--border)", gap: 16 }}>
          {orderTypeOptions.map((t) => (
            <button
              key={t.value}
              onClick={() => setOrderType(t.value)}
              style={{
                background: "none", border: "none", padding: "8px 0", cursor: "pointer", fontSize: 12, fontWeight: 500,
                color: orderType === t.value ? "var(--accent)" : "var(--text-muted)",
                borderBottom: orderType === t.value ? "2px solid var(--accent)" : "2px solid transparent",
                transition: "all 0.1s"
              }}
            >
              {t.label}
            </button>
          ))}
        </div>

        {/* Strike Selection */}
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <span style={{ fontSize: 11, fontWeight: 500, color: "var(--text-secondary)" }}>{side === "sell" ? "Select Strike" : "Product ID"}</span>
            {side === "sell" && (
              <span style={{ fontSize: 10, color: "var(--accent)", cursor: "pointer" }} onClick={() => setStrike("")}>Manual?</span>
            )}
          </div>
          {side === "sell" && strikes.length > 0 ? (
            <select
              value={strike}
              onChange={(e) => setStrike(e.target.value)}
              style={{ ...selectStyle, border: "1px solid var(--border-subtle)" }}
            >
              <option value="">Select Strike...</option>
              {strikes.slice(0, 20).map(s => (
                <option key={s.strike} value={s.strike}>${s.strike.toLocaleString()} (IV: {(s.callIv * 100).toFixed(1)}%)</option>
              ))}
            </select>
          ) : (
            <input
              value={strike}
              onChange={(e) => setStrike(e.target.value)}
              placeholder={side === "sell" ? "e.g. 96000" : "Delta Product ID"}
              style={inputStyle}
            />
          )}
        </div>

        {/* Quantity / Size */}
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <span style={{ fontSize: 11, fontWeight: 500, color: "var(--text-secondary)" }}>Quantity</span>
            <div style={{ fontSize: 11, color: "var(--text-muted)" }}>{side === "sell" ? "USD" : "Lots"} <span style={{ fontSize: 8 }}>▼</span></div>
          </div>
          <input
            value={side === "sell" ? premiumUsd : contracts}
            onChange={(e) => side === "sell" ? setPremiumUsd(e.target.value) : setContracts(e.target.value)}
            style={inputStyle}
            placeholder={side === "sell" ? "100" : "1"}
          />
        </div>

        {/* Percentage Pills */}
        <div style={{ display: "grid", gridTemplateColumns: "repeat(5, 1fr)", gap: 4 }}>
          {["10%", "25%", "50%", "75%", "100%"].map(p => (
            <button
              key={p}
              onClick={() => {
                if (side === "sell") setPremiumUsd(String(Math.floor(walletUsdt * parseFloat(p) / 100)));
              }}
              style={{
                padding: "4px 0", border: "1px solid var(--border)", borderRadius: 4, background: "var(--surface-2)",
                fontSize: 10, color: "var(--text-secondary)", cursor: "pointer", transition: "all 0.1s"
              }}
              onMouseEnter={e => e.currentTarget.style.borderColor = "var(--accent)"}
              onMouseLeave={e => e.currentTarget.style.borderColor = "var(--border)"}
            >
              {p}
            </button>
          ))}
        </div>

        {/* Metrics Section */}
        <div style={{ display: "flex", flexDirection: "column", gap: 8, marginTop: 8 }}>
          <div style={{ display: "flex", justifyContent: "space-between", fontSize: 12 }}>
            <span style={{ color: "var(--text-muted)" }}>Available Margin</span>
            <span style={{ color: "var(--text-primary)", fontWeight: 500 }}>{walletUsdt.toFixed(2)} USD</span>
          </div>
          <div style={{ display: "flex", justifyContent: "space-between", fontSize: 12 }}>
            <span style={{ color: "var(--text-muted)" }}>{side === "buy" ? "Est. Fill" : "Funds Required"}</span>
            <span style={{ color: "var(--text-primary)", fontWeight: 500 }}>~{side === "sell" ? premiumUsd : "--"} USD</span>
          </div>
        </div>

        {/* Big Action Button */}
        <button
          onClick={handleAction}
          disabled={!actionsEnabled || submitting || !strike}
          style={{
            width: "100%", height: 44, border: "none", borderRadius: 8, fontSize: 15, fontWeight: 600, cursor: "pointer",
            background: side === "buy" ? "var(--green)" : "var(--red)",
            color: "#fff", marginTop: 8, transition: "opacity 0.2s",
            opacity: (!actionsEnabled || submitting || !strike) ? 0.5 : 1
          }}
        >
          {submitting ? "Processing..." : side === "buy" ? "Buy / Long" : "Sell / Short"}
        </button>

        {/* Reduce Only Checkbox */}
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <input
            id="reduce-only"
            type="checkbox"
            checked={reduceOnly}
            onChange={(e) => handleReduceChange(e.target.checked)}
            style={{ width: 16, height: 16, cursor: "pointer" }}
          />
          <label htmlFor="reduce-only" style={{ fontSize: 13, color: "var(--text-secondary)", cursor: "pointer" }}>Reduce Only</label>
        </div>
      </div>

      {/* ── Right Side: Info & Feed ──────────────────────────────── */}
      <div style={{ flex: 1, display: "flex", flexDirection: "column", gap: 16 }}>
        <div
          style={{
            padding: "12px 16px", borderRadius: "var(--radius-card)", background: "var(--surface-2)",
            border: "1px solid var(--border)", borderLeft: `4px solid ${sideColor}`,
          }}
        >
          <div style={{ fontSize: 14, fontWeight: 600, color: sideColor, marginBottom: 4 }}>
            {side === "sell" ? "Mirror Open Simulation" : "Position Exit Flow"}
          </div>
          <p style={{ fontSize: 12, color: "var(--text-secondary)", lineHeight: 1.5, margin: 0 }}>
            {side === "sell" 
              ? "This panel allows you to manually trigger the Delta mirroring engine. It will find the closest expiration for the strike you select and place a market sell order."
              : "Use the Buy side (or Reduce Only) to exit an existing position. You can click a position button below to auto-fill the Product ID."}
          </p>
        </div>

        {positions.length > 0 && side === "buy" && (
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            <span style={{ fontSize: 11, fontWeight: 600, color: "var(--text-muted)", textTransform: "uppercase", letterSpacing: "0.05em" }}>Quick Exit Buttons</span>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
              {positions.map(p => (
                <button
                  key={p.symbol}
                  onClick={() => { setStrike(String(p.productId)); setContracts(String(Math.abs(p.size))); }}
                  style={{
                    padding: "6px 14px", borderRadius: "var(--radius-input)", border: "1px solid var(--border)",
                    background: "var(--surface)", color: "var(--accent)", fontSize: 12, fontWeight: 500, cursor: "pointer"
                  }}
                >
                  Close {p.symbol}
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Status/Feedback */}
        {feedback && (
          <div style={{
            padding: 16, borderRadius: "var(--radius-card)", background: feedback.tone === "success" ? "var(--green-dim)" : "var(--red-dim)",
            color: feedback.tone === "success" ? "var(--green)" : "var(--red)", border: `1px solid ${feedback.tone === "success" ? "rgba(30,142,62,0.2)" : "rgba(217,48,37,0.2)"}`,
            fontSize: 13, fontWeight: 500
          }}>
            {feedback.text}
          </div>
        )}

        {/* Response JSON */}
        {lastResult && (
          <div style={{ padding: 16, borderRadius: "var(--radius-card)", background: "var(--surface-3)", border: "1px solid var(--border)" }}>
            <div style={{ fontSize: 11, fontWeight: 600, color: "var(--text-muted)", marginBottom: 8, textTransform: "uppercase" }}>Response Data</div>
            <pre style={{ margin: 0, fontSize: 11, color: "var(--text-secondary)", overflowX: "auto", fontFamily: "var(--font-mono)" }}>
              {JSON.stringify(lastResult, null, 2)}
            </pre>
          </div>
        )}
      </div>
    </div>
  );
}

type MainTab = "test" | "account" | "positions" | "orders" | "mirrored";

export default function DeltaLiveScalper({ actionsEnabled = true }: Props) {
  const [refreshKey, setRefreshKey] = useState(0);
  // Lazy initializer reads localStorage synchronously on first render so the
  // config panel receives the saved keys before DeltaConfigPanel mounts.
  const [config, setConfig] = useState<DeltaLocalConfig | null>(() => {
    if (typeof window === "undefined") return null;
    try {
      const raw = localStorage.getItem(CONFIG_LS_KEY);
      if (raw) return JSON.parse(raw) as DeltaLocalConfig;
    } catch { /* ignore */ }
    return null;
  });

  const handleConfigSave = (c: DeltaLocalConfig) => {
    setConfig(c);
    try { localStorage.setItem(CONFIG_LS_KEY, JSON.stringify(c)); } catch { /* ignore */ }
    setRefreshKey((v) => v + 1);
  };

  const { stats, trades, toggling, toggleEnabled, nextStatus } = useDeltaLive(refreshKey, config);
  const [tab, setTab] = useState<MainTab>("test");
  const [mirroredFilter, setMirroredFilter] = useState<"open" | "all">("open");

  const account = stats.account;
  const openTrades = trades.filter((t) => t.status === "OPEN");
  const displayTrades = mirroredFilter === "open" ? openTrades : trades;
  const failedTrades = trades.filter((t) => t.status === "FAILED");
  const refreshDeltaState = () => setRefreshKey((value) => value + 1);

  // Total unrealised PnL from Delta positions
  const totalUPnl = (account?.positions ?? []).reduce((s, p) => s + p.unrealisedPnl, 0);
  const winRate = stats.wins + stats.losses > 0 ? (stats.wins / (stats.wins + stats.losses)) * 100 : 0;

  const tabItems: { key: MainTab; label: string }[] = [
    { key: "test",      label: "Test Orders" },
    { key: "account",   label: "Wallet & Balances" },
    { key: "positions", label: `Positions (${account?.positions?.length ?? 0})` },
    { key: "orders",    label: `Open Orders (${account?.openOrders?.length ?? 0})` },
    { key: "mirrored",  label: `Mirrored Trades (${trades.length})` },
  ];

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>

      {/* ── Header ──────────────────────────────────────────────────── */}
      <div
        style={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
          padding: "16px 20px",
          borderRadius: "var(--radius-card)",
          border: "1px solid var(--border)",
          background: "var(--surface)",
        }}
      >
        <div>
          <h2
            style={{
              fontFamily: "var(--font-display)",
              fontSize: 18,
              fontWeight: 600,
              color: "var(--text-primary)",
              display: "flex",
              alignItems: "center",
              gap: 8,
              margin: 0,
            }}
          >
            <span style={{ width: 10, height: 10, borderRadius: "var(--radius-chip)", background: "var(--red)", display: "inline-block" }} />
            Delta Exchange Live Trading
          </h2>
          <p style={{ fontSize: 12, color: "var(--text-secondary)", marginTop: 4 }}>
            When enabled, BTC Option Selling paper positions are mirrored to Delta Exchange.
          </p>
        </div>
        <div style={{ textAlign: "right", flexShrink: 0 }}>
          <div
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 6,
              padding: "4px 10px",
              borderRadius: "var(--radius-chip)",
              background: stats.testnet ? "var(--accent-dim)" : "var(--red-dim)",
              color: stats.testnet ? "var(--accent)" : "var(--red)",
              fontSize: 11,
              fontWeight: 500,
              fontFamily: "var(--font-display)",
            }}
          >
            {stats.testnet ? "🧪 Testnet" : "🔴 Live"} · india.delta.exchange
          </div>
          {account?.fetchedAt && (
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 4 }}>Updated {fmtDate(account.fetchedAt)}</div>
          )}
        </div>
      </div>

      {/* ── API Key Config Panel ────────────────────────────────────── */}
      <DeltaConfigPanel config={config} onSave={handleConfigSave} />

      {/* ── Enable banner ───────────────────────────────────────────── */}
      <EnableBanner
        stats={stats}
        nextStatus={nextStatus}
        toggling={toggling}
        onToggle={actionsEnabled ? toggleEnabled : () => {}}
      />

      {stats.configured && stats.buyingMode && stats.lowBalanceProfile && stats.walletUsdt > 0 && (
        <div
          style={{
            padding: "12px 16px",
            borderRadius: "var(--radius-card)",
            border: "1px solid var(--accent)",
            background: "var(--accent-dim)",
            fontSize: 12,
            color: "var(--text-secondary)",
            lineHeight: 1.55,
          }}
        >
          <span style={{ fontFamily: "var(--font-display)", fontWeight: 600, color: "var(--accent)" }}>Low balance live sizing</span>
          {" — "}
          Wallet ~${fmt(stats.walletUsdt)}: engine caps long-option mirror at ~{(stats.buyRiskPct ?? 0) * 100}% of wallet per signal and max{" "}
          {stats.buyMaxContracts ?? "—"} contracts (est. ${fmt(stats.buyEstPremiumUsd ?? 35)} premium/contract). Min wallet ${fmt(stats.minWalletUsd ?? 5)}.
          Override with <span style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>DELTA_BUY_RISK_PCT</span>,{" "}
          <span style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>DELTA_BUY_MAX_CONTRACTS</span>,{" "}
          <span style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>DELTA_BUY_EST_PREMIUM_USD</span>.
        </div>
      )}

      {/* ── Top KPI row ─────────────────────────────────────────────── */}
      {stats.configured && (
        <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 12 }}>
          <div className="metric-card" style={{ padding: "14px 16px" }}>
            <div className="metric-label">USDT Available</div>
            <div className="metric-value" style={{ fontFamily: "var(--font-display)" }}>${fmt(stats.walletUsdt)}</div>
          </div>
          <div className="metric-card" style={{ padding: "14px 16px" }}>
            <div className="metric-label">Open Positions</div>
            <div className="metric-value" style={{ color: "var(--accent)", fontFamily: "var(--font-display)" }}>{account?.positions?.length ?? 0}</div>
          </div>
          <div className="metric-card" style={{ padding: "14px 16px" }}>
            <div className="metric-label">Unrealised PnL</div>
            <div className={`metric-value ${pnlColor(totalUPnl)}`} style={{ fontFamily: "var(--font-display)" }}>
              {pnlSign(totalUPnl)}${fmt(totalUPnl)}
            </div>
          </div>
          <div className="metric-card" style={{ padding: "14px 16px" }}>
            <div className="metric-label">Mirror Win Rate</div>
            <div className={`metric-value ${winRate >= 50 ? "profit-positive" : "profit-negative"}`} style={{ fontFamily: "var(--font-display)" }}>
              {fmt(winRate, 1)}%
            </div>
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 2 }}>{stats.wins}W / {stats.losses}L</div>
          </div>
        </div>
      )}

      {/* ── Google-style Tabs ────────────────────────────────────────── */}
      <div
        style={{
          borderRadius: "var(--radius-card)",
          border: "1px solid var(--border)",
          background: "var(--surface)",
          overflow: "hidden",
        }}
      >
        {/* Tab bar */}
        <div style={{ display: "flex", borderBottom: "1px solid var(--border)", overflowX: "auto" }}>
          {tabItems.map((t) => (
            <button
              type="button"
              key={t.key}
              onClick={() => setTab(t.key)}
              className={`groww-tab${tab === t.key ? " active" : ""}`}
              style={{ padding: "12px 18px" }}
            >
              {t.label}
            </button>
          ))}
        </div>

        {/* Tab content */}
        <div style={{ padding: 20 }}>
          {tab === "account" && (
            <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
              <div style={{ fontFamily: "var(--font-display)", fontWeight: 600, fontSize: 14, color: "var(--text-primary)" }}>Account Balances</div>
              <WalletCards wallets={account?.wallets ?? []} />
              {account?.error && (
                <div style={{ padding: "8px 12px", borderRadius: "var(--radius-input)", background: "var(--red-dim)", color: "var(--red)", fontSize: 12 }}>
                  {account.error}
                </div>
              )}
            </div>
          )}

          {tab === "positions" && (
            <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
              <div style={{ fontFamily: "var(--font-display)", fontWeight: 600, fontSize: 14, color: "var(--text-primary)" }}>Live Positions on Delta Exchange</div>
              <LivePositionsTable positions={account?.positions ?? []} />
            </div>
          )}

          {tab === "orders" && (
            <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
              <div style={{ fontFamily: "var(--font-display)", fontWeight: 600, fontSize: 14, color: "var(--text-primary)" }}>Open Orders on Delta Exchange</div>
              <OpenOrdersTable orders={account?.openOrders ?? []} />
            </div>
          )}

          {tab === "mirrored" && (
            <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                <div style={{ fontFamily: "var(--font-display)", fontWeight: 600, fontSize: 14, color: "var(--text-primary)" }}>Mirrored Orders</div>
                <div style={{ display: "flex", gap: 4 }}>
                  {(["open", "all"] as const).map((f) => (
                    <button
                      type="button"
                      key={f}
                      onClick={() => setMirroredFilter(f)}
                      style={{
                        padding: "4px 12px",
                        borderRadius: "var(--radius-chip)",
                        border: "none",
                        fontSize: 12,
                        fontWeight: 500,
                        fontFamily: "var(--font-display)",
                        cursor: "pointer",
                        background: mirroredFilter === f ? "var(--accent)" : "var(--surface-2)",
                        color: mirroredFilter === f ? "#fff" : "var(--text-secondary)",
                        transition: "all 0.15s ease",
                      }}
                    >
                      {f === "open" ? `Open (${openTrades.length})` : `All (${trades.length})`}
                    </button>
                  ))}
                </div>
              </div>
              <MirroredTradesTable trades={displayTrades} />

              {failedTrades.length > 0 && (
                <div
                  style={{
                    padding: 14,
                    borderRadius: "var(--radius-card)",
                    border: "1px solid rgba(217, 48, 37, 0.2)",
                    background: "var(--red-dim)",
                  }}
                >
                  <div style={{ fontFamily: "var(--font-display)", fontWeight: 600, fontSize: 12, color: "var(--red)", marginBottom: 8 }}>
                    ⚠ Failed Orders ({failedTrades.length})
                  </div>
                  {failedTrades.slice(0, 5).map((t) => (
                    <div key={t.id} style={{ fontSize: 12, color: "var(--text-secondary)", marginBottom: 4 }}>
                      <span style={{ color: "var(--red)", fontFamily: "var(--font-mono)" }}>{t.id}</span> — {t.failureReason}
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {tab === "test" && (
            <TestOrderTab
              actionsEnabled={actionsEnabled}
              positions={account?.positions ?? []}
              onOrderPlaced={refreshDeltaState}
              walletUsdt={stats.walletUsdt}
              config={config}
            />
          )}
        </div>
      </div>

      {/* Mirrored trade stats bar */}
      {stats.configured && trades.length > 0 && (
        <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 12 }}>
          <div className="metric-card" style={{ padding: "14px 16px", textAlign: "center" }}>
            <div style={{ fontSize: 22, fontWeight: 600, color: "var(--accent)", fontFamily: "var(--font-display)" }}>{stats.openTrades}</div>
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 4 }}>Live Open</div>
          </div>
          <div className="metric-card" style={{ padding: "14px 16px", textAlign: "center" }}>
            <div style={{ fontSize: 22, fontWeight: 600, color: stats.totalPnl >= 0 ? "var(--green)" : "var(--red)", fontFamily: "var(--font-display)" }}>
              {pnlSign(stats.totalPnl)}${fmt(stats.totalPnl)}
            </div>
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 4 }}>Realised PnL</div>
          </div>
          <div className="metric-card" style={{ padding: "14px 16px", textAlign: "center" }}>
            <div style={{ fontSize: 22, fontWeight: 600, color: "var(--text-primary)", fontFamily: "var(--font-display)" }}>{stats.totalTrades}</div>
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 4 }}>Total Mirrored</div>
          </div>
        </div>
      )}
    </div>
  );
}
