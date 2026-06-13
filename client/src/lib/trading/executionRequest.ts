/**
 * Institutional execution request client.
 * Frontend may ONLY submit execution intents — never broker orders directly.
 */

export type ExecutionRequestPayload = {
  requestId?: string;
  venue: "paper" | "delta";
  symbol: string;
  side: "BUY" | "SELL" | "buy" | "sell" | "LONG" | "SHORT";
  size?: number;
  contracts?: number;
  strategyName?: string;
  reason?: string;
  confidence?: number;
};

export type ExecutionRequestResponse = {
  ok: boolean;
  status?: string;
  requestId?: string;
  clientOrderId?: string;
  message?: string;
  error?: string;
};

/** Submit an execution intent to the backend authorization layer (never a direct broker order). */
export async function submitExecutionRequest(
  payload: ExecutionRequestPayload,
): Promise<ExecutionRequestResponse> {
  const res = await fetch("/api/execution/request", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(payload),
  });
  const data = (await res.json()) as ExecutionRequestResponse & { message?: string; error?: string };
  if (!res.ok) {
    return { ok: false, status: data.status ?? "REJECTED", message: data.message ?? data.error ?? `HTTP ${res.status}` };
  }
  return { ...data, ok: data.ok ?? true };
}
