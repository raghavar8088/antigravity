"use client";

/**
 * Paper OMS Panel — observability for the paper order lifecycle.
 *
 * Shows: signal → risk-checked → accepted → simulated fill → position opened → closed
 * Helps debug why trades are not opening by showing which stage the pipeline stops at.
 *
 * Hard rules:
 * - Paper only. No live orders shown or placed.
 * - No secrets displayed.
 * - Read-only observability.
 */

import { useCallback, useEffect, useState } from "react";
import { DeskButton } from "@/components/desk/ui";
import type { PaperOmsOrder, PaperOmsOrderStatus } from "@/lib/paperOms";

// ─── Types ────────────────────────────────────────────────────────────────────

type OmsSummaryResponse = {
  ok: boolean;
  windowMinutes?: number;
  generatedAt?: string;
  total?: number;
  countsByStatus?: Record<PaperOmsOrderStatus, number>;
  topRejectGates?: { gate: string; count: number }[];
  latestOrders?: PaperOmsOrder[];
  fillRate?: number;
  rejectRate?: number;
};

type OmsOrdersResponse = {
  ok: boolean;
  count?: number;
  orders?: PaperOmsOrder[];
};

// ─── Status colour mapping ────────────────────────────────────────────────────

const STATUS_COLOR: Record<PaperOmsOrderStatus, string> = {
  NEW: "var(--desk-on-surface-muted)",
  RISK_CHECKED: "var(--desk-primary)",
  ACCEPTED: "var(--desk-success)",
  SIMULATED_FILL: "var(--desk-success)",
  POSITION_OPENED: "var(--desk-success)",
  POSITION_CLOSED: "var(--desk-on-surface-muted)",
  REJECTED: "var(--desk-error)",
  CANCELLED: "var(--desk-warning)",
};

// ─── Blocker interpretation ────────────────────────────────────────────────────

function interpretNoOrders(
  countsByStatus: Record<PaperOmsOrderStatus, number> | undefined,
  total: number | undefined,
): string {
  if (!countsByStatus || !total) return "No OMS data yet — signal may not have reached threshold.";
  if (total === 0) {
    return "No OMS orders this window. Signal threshold never reached → check SIGNAL gate in Signal Trace tab.";
  }
  const opened = countsByStatus.POSITION_OPENED + countsByStatus.POSITION_CLOSED;
  const rejected = countsByStatus.REJECTED;
  const filled = countsByStatus.SIMULATED_FILL + countsByStatus.POSITION_OPENED + countsByStatus.POSITION_CLOSED;

  if (opened > 0) return `${opened} position(s) opened this window. Pipeline working.`;
  if (filled > 0) return `${filled} order(s) filled but no positions tracked — possible ledger sync gap.`;
  if (rejected > 0 && filled === 0)
    return `All ${rejected} orders rejected. Check top reject gate below — fix that gate to enable entries.`;
  return `${total} orders created, none filled. May be stuck at ACCEPTED stage — check worker for position open errors.`;
}

// ─── Components ──────────────────────────────────────────────────────────────

function SummaryCard({
  label,
  value,
  color,
}: {
  label: string;
  value: string | number;
  color?: string;
}) {
  return (
    <div
      style={{
        padding: "8px 12px",
        border: "1px solid var(--desk-outline-variant)",
        borderRadius: "var(--desk-radius-card)",
        background: "var(--desk-surface)",
        minWidth: 80,
      }}
    >
      <div style={{ fontSize: "0.625rem", color: "var(--desk-on-surface-muted)", textTransform: "uppercase", letterSpacing: "0.06em" }}>
        {label}
      </div>
      <div style={{ fontSize: "1.125rem", fontWeight: 700, color: color ?? "var(--desk-on-surface)", lineHeight: 1.2, marginTop: 2 }}>
        {value}
      </div>
    </div>
  );
}

function StatusBadge({ status }: { status: PaperOmsOrderStatus }) {
  return (
    <span
      style={{
        fontSize: "0.625rem",
        fontWeight: 600,
        color: STATUS_COLOR[status],
        padding: "1px 5px",
        border: `1px solid ${STATUS_COLOR[status]}`,
        borderRadius: 4,
        whiteSpace: "nowrap",
      }}
    >
      {status}
    </span>
  );
}

// ─── Main panel ───────────────────────────────────────────────────────────────

export function PaperOmsPanel() {
  const [windowMin, setWindowMin] = useState<30 | 60 | 120 | 360>(120);
  const [statusFilter, setStatusFilter] = useState<PaperOmsOrderStatus | "">("");
  const [summary, setSummary] = useState<OmsSummaryResponse | null>(null);
  const [orders, setOrders] = useState<OmsOrdersResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [sumRes, ordRes] = await Promise.all([
        fetch(`/api/paper-oms/summary?window_minutes=${windowMin}`),
        fetch(`/api/paper-oms/orders?limit=100${statusFilter ? `&status=${statusFilter}` : ""}`),
      ]);
      const sumJson = (await sumRes.json()) as OmsSummaryResponse;
      const ordJson = (await ordRes.json()) as OmsOrdersResponse;
      setSummary(sumJson);
      setOrders(ordJson);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Fetch failed");
    } finally {
      setLoading(false);
    }
  }, [windowMin, statusFilter]);

  useEffect(() => {
    void fetchData();
  }, [fetchData]);

  const copyDebugContext = useCallback(() => {
    if (!summary) return;
    const ctx = {
      generatedAt: summary.generatedAt,
      windowMinutes: summary.windowMinutes,
      total: summary.total,
      countsByStatus: summary.countsByStatus,
      topRejectGates: summary.topRejectGates,
      fillRate: summary.fillRate,
      rejectRate: summary.rejectRate,
      // Never include account keys, API keys, or secrets
    };
    void navigator.clipboard.writeText(JSON.stringify(ctx, null, 2));
  }, [summary]);

  const counts = summary?.countsByStatus;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>

      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", flexWrap: "wrap", gap: 8 }}>
        <div>
          <p style={{ margin: 0, fontSize: "0.75rem", fontWeight: 700, color: "var(--desk-on-surface)" }}>
            Paper OMS — Order Lifecycle
          </p>
          <p style={{ margin: 0, fontSize: "0.6875rem", color: "var(--desk-on-surface-muted)" }}>
            signal → risk-checked → accepted → fill → position opened/closed
          </p>
        </div>
        <div style={{ display: "flex", gap: 6, flexWrap: "wrap", alignItems: "center" }}>
          {([30, 60, 120, 360] as const).map((w) => (
            <DeskButton
              key={w}
              variant={windowMin === w ? "filled" : "outlined"}
              onClick={() => setWindowMin(w)}
              style={{ minHeight: 28, minWidth: 36, padding: "0 10px", fontSize: "0.6875rem" }}
            >
              {w < 60 ? `${w}m` : `${w / 60}h`}
            </DeskButton>
          ))}
          <DeskButton variant="outlined" onClick={() => void fetchData()} style={{ minHeight: 28, padding: "0 10px", fontSize: "0.6875rem" }}>
            {loading ? "…" : "Refresh"}
          </DeskButton>
          <DeskButton variant="outlined" onClick={copyDebugContext} style={{ minHeight: 28, padding: "0 10px", fontSize: "0.6875rem" }}>
            Copy debug
          </DeskButton>
        </div>
      </div>

      {error && (
        <p style={{ margin: 0, fontSize: "0.6875rem", color: "var(--desk-error)" }}>
          Error: {error}
        </p>
      )}

      {/* Summary cards */}
      {counts && (
        <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
          <SummaryCard label="Total intents" value={summary?.total ?? 0} />
          <SummaryCard label="Rejected" value={counts.REJECTED} color="var(--desk-error)" />
          <SummaryCard label="Accepted" value={counts.ACCEPTED} color="var(--desk-success)" />
          <SummaryCard label="Filled" value={counts.SIMULATED_FILL + counts.POSITION_OPENED} color="var(--desk-success)" />
          <SummaryCard label="Opened" value={counts.POSITION_OPENED} color="var(--desk-success)" />
          <SummaryCard label="Closed" value={counts.POSITION_CLOSED} />
          <SummaryCard
            label="Fill rate"
            value={`${(((summary?.fillRate ?? 0)) * 100).toFixed(1)}%`}
            color={(summary?.fillRate ?? 0) > 0 ? "var(--desk-success)" : "var(--desk-on-surface-muted)"}
          />
          <SummaryCard
            label="Top reject gate"
            value={summary?.topRejectGates?.[0]?.gate ?? "—"}
            color="var(--desk-warning)"
          />
        </div>
      )}

      {/* Blocker interpretation (Part 7) */}
      <div
        style={{
          padding: "10px 14px",
          border: "1px solid var(--desk-outline-variant)",
          borderRadius: "var(--desk-radius-card)",
          background: "var(--desk-surface)",
          fontSize: "0.6875rem",
          color: "var(--desk-on-surface-muted)",
          lineHeight: 1.5,
        }}
      >
        <strong style={{ color: "var(--desk-on-surface)" }}>Diagnosis: </strong>
        {interpretNoOrders(counts, summary?.total)}
      </div>

      {/* Top reject gates */}
      {(summary?.topRejectGates?.length ?? 0) > 0 && (
        <div>
          <p style={{ margin: "0 0 6px", fontSize: "0.6875rem", fontWeight: 600, color: "var(--desk-on-surface)" }}>
            Top Reject Gates
          </p>
          <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
            {summary!.topRejectGates!.map(({ gate, count }) => (
              <div
                key={gate}
                style={{
                  padding: "4px 10px",
                  border: "1px solid var(--desk-error)",
                  borderRadius: 6,
                  fontSize: "0.6875rem",
                  color: "var(--desk-error)",
                }}
              >
                {gate} × {count}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Status filter + table */}
      <div>
        <div style={{ display: "flex", gap: 6, flexWrap: "wrap", marginBottom: 8 }}>
          <p style={{ margin: 0, fontSize: "0.6875rem", fontWeight: 600, color: "var(--desk-on-surface)", alignSelf: "center" }}>
            Filter:
          </p>
          {(["", "REJECTED", "POSITION_OPENED", "POSITION_CLOSED", "SIMULATED_FILL", "ACCEPTED", "RISK_CHECKED"] as const).map((s) => (
            <DeskButton
              key={s || "ALL"}
              variant={statusFilter === s ? "filled" : "outlined"}
              onClick={() => setStatusFilter(s)}
              style={{ minHeight: 26, padding: "0 8px", fontSize: "0.625rem" }}
            >
              {s || "ALL"}
            </DeskButton>
          ))}
        </div>

        {/* Orders table */}
        <div style={{ overflowX: "auto" }}>
          <table
            style={{
              width: "100%",
              borderCollapse: "collapse",
              fontSize: "0.6875rem",
              color: "var(--desk-on-surface)",
            }}
          >
            <thead>
              <tr style={{ borderBottom: "1px solid var(--desk-outline-variant)" }}>
                {["Time", "Strategy", "Side", "Status", "Gate", "Score/Thr", "Notional", "Fill $", "Reject Reason", "Position ID"].map(
                  (h) => (
                    <th
                      key={h}
                      style={{
                        padding: "4px 8px",
                        textAlign: "left",
                        fontWeight: 600,
                        color: "var(--desk-on-surface-muted)",
                        whiteSpace: "nowrap",
                      }}
                    >
                      {h}
                    </th>
                  ),
                )}
              </tr>
            </thead>
            <tbody>
              {(orders?.orders ?? []).length === 0 && (
                <tr>
                  <td colSpan={10} style={{ padding: "12px 8px", color: "var(--desk-on-surface-muted)", textAlign: "center" }}>
                    {loading ? "Loading…" : "No orders"}
                  </td>
                </tr>
              )}
              {(orders?.orders ?? []).map((order) => (
                <tr
                  key={order.orderId}
                  style={{ borderBottom: "1px solid var(--desk-outline-variant)" }}
                >
                  <td style={{ padding: "3px 8px", whiteSpace: "nowrap" }}>
                    {new Date(order.createdAt).toLocaleTimeString()}
                  </td>
                  <td style={{ padding: "3px 8px" }}>{order.strategyName}</td>
                  <td style={{ padding: "3px 8px", color: order.side === "LONG" ? "var(--desk-success)" : "var(--desk-error)" }}>
                    {order.side}
                  </td>
                  <td style={{ padding: "3px 8px" }}>
                    <StatusBadge status={order.status} />
                  </td>
                  <td style={{ padding: "3px 8px", color: "var(--desk-warning)" }}>
                    {order.gate ?? "—"}
                  </td>
                  <td style={{ padding: "3px 8px" }}>
                    {order.signalScore.toFixed(1)}/{order.requiredThreshold}
                  </td>
                  <td style={{ padding: "3px 8px" }}>${order.notional.toFixed(0)}</td>
                  <td style={{ padding: "3px 8px" }}>
                    {order.fillPrice != null ? `$${order.fillPrice.toFixed(2)}` : "—"}
                  </td>
                  <td
                    style={{
                      padding: "3px 8px",
                      color: "var(--desk-error)",
                      maxWidth: 200,
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                    }}
                  >
                    {order.rejectReason ?? "—"}
                  </td>
                  <td style={{ padding: "3px 8px", fontFamily: "monospace", fontSize: "0.625rem" }}>
                    {order.positionId ? order.positionId.slice(0, 8) + "…" : "—"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
