"use client";
import { useState } from "react";
import useDeltaLive, {
  type DeltaLiveTrade,
  type DeltaLiveStats,
  type DeltaRuntimeStatus,
  type WalletEntry,
  type LivePosition,
  type OpenOrder,
} from "@/hooks/useDeltaLive";

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
            Enable this to mirror BTC Option Selling paper positions to Delta.
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

function TestOrderTab({
  actionsEnabled,
  positions,
  onOrderPlaced,
}: {
  actionsEnabled: boolean;
  positions: LivePosition[];
  onOrderPlaced: () => void;
}) {
  const [optionType, setOptionType] = useState<"CALL" | "PUT">("CALL");
  const [strike, setStrike] = useState("120000");
  const [premiumUsd, setPremiumUsd] = useState("100");
  const [closeProductId, setCloseProductId] = useState("");
  const [closeContracts, setCloseContracts] = useState("1");
  const [submitting, setSubmitting] = useState<"open" | "close" | null>(null);
  const [feedback, setFeedback] = useState<{ tone: "success" | "error"; text: string } | null>(null);
  const [lastResult, setLastResult] = useState<TestOrderResponse | null>(null);

  const submitOpen = async () => {
    setSubmitting("open");
    setFeedback(null);
    try {
      const response = await fetch("/api/delta/mirror", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "open",
          optionType,
          strike: Number(strike),
          premiumUsd: Number(premiumUsd),
        }),
      });
      const data = await response.json() as TestOrderResponse;
      setLastResult(data);
      if (response.ok && data.ok) {
        setFeedback({
          tone: "success",
          text: `Open order placed on Delta. Order ID ${data.orderId ?? "-"}${data.symbol ? ` | ${data.symbol}` : ""}`,
        });
        onOrderPlaced();
      } else {
        setFeedback({ tone: "error", text: data.error ?? "Failed to place Delta open order." });
      }
    } catch (error) {
      setFeedback({ tone: "error", text: String(error) });
    } finally {
      setSubmitting(null);
    }
  };

  const submitClose = async () => {
    setSubmitting("close");
    setFeedback(null);
    try {
      const response = await fetch("/api/delta/mirror", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "close",
          productId: Number(closeProductId),
          contracts: Number(closeContracts),
        }),
      });
      const data = await response.json() as TestOrderResponse;
      setLastResult(data);
      if (response.ok && data.ok) {
        setFeedback({
          tone: "success",
          text: `Close order placed on Delta. Order ID ${data.closeOrderId ?? "-"} | ${Number(closeContracts) || 0} contract(s)`,
        });
        onOrderPlaced();
      } else {
        setFeedback({ tone: "error", text: data.error ?? "Failed to place Delta close order." });
      }
    } catch (error) {
      setFeedback({ tone: "error", text: String(error) });
    } finally {
      setSubmitting(null);
    }
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      {/* Warning banner */}
      <div
        style={{
          padding: "10px 16px",
          borderRadius: "var(--radius-card)",
          border: "1px solid rgba(227, 116, 0, 0.2)",
          background: "var(--amber-dim)",
          fontSize: 12,
          color: "var(--amber)",
          fontFamily: "var(--font-display)",
          fontWeight: 500,
        }}
      >
        ⚠ This tab sends live test orders to Delta Exchange. Use small size and prefer testnet first.
      </div>

      {!actionsEnabled && (
        <div
          style={{
            padding: "10px 16px",
            borderRadius: "var(--radius-card)",
            border: "1px solid var(--border)",
            background: "var(--surface-2)",
            fontSize: 12,
            color: "var(--text-muted)",
          }}
        >
          Action buttons are disabled. Turn Action to Yes to use this test order panel.
        </div>
      )}

      {/* Order forms */}
      <div style={{ display: "grid", gap: 16, gridTemplateColumns: "repeat(auto-fit, minmax(380px, 1fr))" }}>
        {/* Open order */}
        <div
          style={{
            padding: 20,
            borderRadius: "var(--radius-card)",
            border: "1px solid var(--border)",
            background: "var(--surface)",
            display: "flex",
            flexDirection: "column",
            gap: 14,
          }}
        >
          <div>
            <div style={{ fontFamily: "var(--font-display)", fontWeight: 600, fontSize: 14, color: "var(--text-primary)" }}>
              Open Test Sell Order
            </div>
            <div style={{ fontSize: 12, color: "var(--text-secondary)", marginTop: 4 }}>
              Finds the nearest Delta option contract and sends a market sell order.
            </div>
          </div>

          <div style={{ display: "grid", gap: 12, gridTemplateColumns: "repeat(3, 1fr)" }}>
            <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
              <span style={{ fontSize: 11, fontWeight: 500, color: "var(--text-secondary)" }}>Option Type</span>
              <select value={optionType} onChange={(e) => setOptionType(e.target.value as "CALL" | "PUT")} style={selectStyle}>
                <option value="CALL">CALL</option>
                <option value="PUT">PUT</option>
              </select>
            </label>
            <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
              <span style={{ fontSize: 11, fontWeight: 500, color: "var(--text-secondary)" }}>Strike</span>
              <input value={strike} onChange={(e) => setStrike(e.target.value)} inputMode="decimal" style={inputStyle} placeholder="120000" />
            </label>
            <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
              <span style={{ fontSize: 11, fontWeight: 500, color: "var(--text-secondary)" }}>Premium USD</span>
              <input value={premiumUsd} onChange={(e) => setPremiumUsd(e.target.value)} inputMode="decimal" style={inputStyle} placeholder="100" />
            </label>
          </div>

          <button
            type="button"
            disabled={!actionsEnabled || submitting !== null}
            onClick={() => void submitOpen()}
            className="btn-danger"
            style={{ alignSelf: "flex-start" }}
          >
            {submitting === "open" ? "Placing…" : "Place Open Test Order"}
          </button>
        </div>

        {/* Close order */}
        <div
          style={{
            padding: 20,
            borderRadius: "var(--radius-card)",
            border: "1px solid var(--border)",
            background: "var(--surface)",
            display: "flex",
            flexDirection: "column",
            gap: 14,
          }}
        >
          <div>
            <div style={{ fontFamily: "var(--font-display)", fontWeight: 600, fontSize: 14, color: "var(--text-primary)" }}>
              Close Test Order
            </div>
            <div style={{ fontSize: 12, color: "var(--text-secondary)", marginTop: 4 }}>
              Sends a market buy order for a Delta option product ID to close a short test position.
            </div>
          </div>

          {positions.length > 0 && (
            <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <span style={{ fontSize: 11, fontWeight: 500, color: "var(--text-secondary)" }}>Quick fill from live positions</span>
              <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
                {positions.slice(0, 6).map((position) => (
                  <button
                    key={`${position.productId}-${position.symbol}`}
                    type="button"
                    onClick={() => {
                      setCloseProductId(String(position.productId));
                      setCloseContracts(String(Math.max(1, Math.round(Math.abs(position.size)))));
                    }}
                    style={{
                      padding: "4px 12px",
                      borderRadius: "var(--radius-chip)",
                      border: "1px solid var(--border)",
                      background: "var(--surface-2)",
                      color: "var(--accent)",
                      fontSize: 11,
                      fontWeight: 500,
                      cursor: "pointer",
                      transition: "border-color 0.15s ease",
                    }}
                  >
                    {position.symbol} | ID {position.productId}
                  </button>
                ))}
              </div>
            </div>
          )}

          <div style={{ display: "grid", gap: 12, gridTemplateColumns: "1fr 1fr" }}>
            <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
              <span style={{ fontSize: 11, fontWeight: 500, color: "var(--text-secondary)" }}>Product ID</span>
              <input value={closeProductId} onChange={(e) => setCloseProductId(e.target.value)} inputMode="numeric" style={inputStyle} placeholder="12345" />
            </label>
            <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
              <span style={{ fontSize: 11, fontWeight: 500, color: "var(--text-secondary)" }}>Contracts</span>
              <input value={closeContracts} onChange={(e) => setCloseContracts(e.target.value)} inputMode="numeric" style={inputStyle} placeholder="1" />
            </label>
          </div>

          <button
            type="button"
            disabled={!actionsEnabled || submitting !== null}
            onClick={() => void submitClose()}
            className="btn-primary"
            style={{ alignSelf: "flex-start" }}
          >
            {submitting === "close" ? "Placing…" : "Place Close Test Order"}
          </button>
        </div>
      </div>

      {/* Feedback */}
      {feedback && (
        <div
          style={{
            padding: "10px 16px",
            borderRadius: "var(--radius-card)",
            border: `1px solid ${feedback.tone === "success" ? "rgba(30, 142, 62, 0.25)" : "rgba(217, 48, 37, 0.25)"}`,
            background: feedback.tone === "success" ? "var(--green-dim)" : "var(--red-dim)",
            color: feedback.tone === "success" ? "var(--green)" : "var(--red)",
            fontSize: 13,
            fontFamily: "var(--font-display)",
          }}
        >
          {feedback.text}
        </div>
      )}

      {/* Last response */}
      {lastResult && (
        <div
          style={{
            padding: 16,
            borderRadius: "var(--radius-card)",
            border: "1px solid var(--border)",
            background: "var(--surface)",
          }}
        >
          <div style={{ fontFamily: "var(--font-display)", fontWeight: 600, fontSize: 13, color: "var(--text-primary)", marginBottom: 8 }}>
            Last Test Response
          </div>
          <pre style={{ overflow: "auto", fontSize: 12, color: "var(--text-secondary)", fontFamily: "var(--font-mono)", lineHeight: 1.6, margin: 0 }}>
            {JSON.stringify(lastResult, null, 2)}
          </pre>
        </div>
      )}
    </div>
  );
}

type MainTab = "test" | "account" | "positions" | "orders" | "mirrored";

export default function DeltaLiveScalper({ actionsEnabled = true }: Props) {
  const [refreshKey, setRefreshKey] = useState(0);
  const { stats, trades, toggling, toggleEnabled, nextStatus } = useDeltaLive(refreshKey);
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

      {/* ── Enable banner ───────────────────────────────────────────── */}
      <EnableBanner
        stats={stats}
        nextStatus={nextStatus}
        toggling={toggling}
        onToggle={actionsEnabled ? toggleEnabled : () => {}}
      />

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
