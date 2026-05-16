import { randomUUID } from "crypto";
import { createServiceSupabase } from "@/lib/supabase/server";

export type DeltaAuditAction = "place_order" | "cancel_order" | "ping";

export type DeltaAuditEntry = {
  id: string;
  createdAt: string;
  userId: string;
  action: DeltaAuditAction;
  symbol?: string;
  side?: string;
  size?: number;
  orderId?: string;
  status?: string;
  payload?: unknown;
};

const memoryLog: DeltaAuditEntry[] = [];
const MEMORY_CAP = 500;

export async function appendDeltaAuditLog(entry: Omit<DeltaAuditEntry, "id" | "createdAt">): Promise<void> {
  const row: DeltaAuditEntry = {
    id: randomUUID(),
    createdAt: new Date().toISOString(),
    ...entry,
  };

  memoryLog.unshift(row);
  if (memoryLog.length > MEMORY_CAP) memoryLog.length = MEMORY_CAP;

  const supabase = createServiceSupabase();
  if (!supabase) return;

  const { error } = await supabase.from("delta_audit_log").insert({
    user_id: row.userId,
    action: row.action,
    symbol: row.symbol ?? null,
    side: row.side ?? null,
    size: row.size ?? null,
    order_id: row.orderId ?? null,
    status: row.status ?? null,
    payload: row.payload ?? null,
  });

  if (error) {
    console.warn("[delta_audit_log] insert failed (in-memory copy kept):", error.message);
  }
}

export function getDeltaAuditLogMemorySnapshot(): readonly DeltaAuditEntry[] {
  return memoryLog;
}

export function resetDeltaAuditLogForTests(): void {
  memoryLog.length = 0;
}
