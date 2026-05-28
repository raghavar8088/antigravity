/**
 * MongoDB persistence for Paper OMS orders.
 *
 * Collection: paper_oms_orders
 * - Append-mostly (status updates via updateOne).
 * - Writes are best-effort and non-fatal.
 * - Never stores secrets, API keys, or MongoDB URI.
 * - TTL: orders auto-expire after 30 days.
 */

import { getDb, isMongoConfigured } from "@/lib/mongoTradesClient";
import type { PaperOmsOrder, PaperOmsOrderStatus } from "@/lib/paperOms";

const COLLECTION = "paper_oms_orders";
const TTL_DAYS = 30;

let indexesEnsured = false;

async function ensureOmsIndexes(): Promise<void> {
  if (indexesEnsured || !isMongoConfigured()) return;
  try {
    const db = await getDb();
    const col = db.collection(COLLECTION);
    await Promise.all([
      col.createIndex({ orderId: 1 }, { unique: true, name: "uniq_order_id" }),
      col.createIndex({ intentId: 1 }, { name: "by_intent_id" }),
      col.createIndex({ strategyId: 1, createdAt: -1 }, { name: "by_strategy_created" }),
      col.createIndex({ status: 1, createdAt: -1 }, { name: "by_status_created" }),
      col.createIndex({ createdAt: 1 }, { expireAfterSeconds: TTL_DAYS * 86400, name: "ttl_oms_30d" }),
    ]);
    indexesEnsured = true;
  } catch {
    // Non-fatal — indexes are best-effort; trading continues.
  }
}

/** Insert a new OMS order (non-fatal). */
export async function insertPaperOmsOrder(order: PaperOmsOrder): Promise<void> {
  if (!isMongoConfigured()) return;
  try {
    await ensureOmsIndexes();
    const db = await getDb();
    const col = db.collection(COLLECTION);
    await col.insertOne({ ...order });
  } catch {
    // Non-fatal — OMS writes must never crash the worker.
  }
}

/** Batch insert OMS orders (non-fatal). */
export async function insertPaperOmsOrders(orders: PaperOmsOrder[]): Promise<void> {
  if (!isMongoConfigured() || orders.length === 0) return;
  try {
    await ensureOmsIndexes();
    const db = await getDb();
    const col = db.collection(COLLECTION);
    await col.insertMany(orders.map((o) => ({ ...o })), { ordered: false });
  } catch {
    // Non-fatal.
  }
}

/** Update status (and optional fields) of an existing OMS order by orderId. Non-fatal. */
export async function updatePaperOmsOrderStatus(
  orderId: string,
  status: PaperOmsOrderStatus,
  extra: Partial<
    Pick<PaperOmsOrder, "tradeId" | "positionId" | "fillPrice" | "entryPrice" | "contracts">
  > = {},
): Promise<void> {
  if (!isMongoConfigured()) return;
  try {
    const db = await getDb();
    const col = db.collection(COLLECTION);
    await col.updateOne(
      { orderId },
      { $set: { status, updatedAt: new Date().toISOString(), ...extra } },
    );
  } catch {
    // Non-fatal.
  }
}

export type ListPaperOmsOrdersOpts = {
  limit?: number;
  status?: PaperOmsOrderStatus;
  strategyId?: number;
  sinceMs?: number;
  gate?: string;
};

/** List OMS orders newest-first with optional filters. */
export async function listPaperOmsOrders(
  opts: ListPaperOmsOrdersOpts = {},
): Promise<PaperOmsOrder[]> {
  if (!isMongoConfigured()) return [];
  try {
    await ensureOmsIndexes();
    const db = await getDb();
    const col = db.collection<PaperOmsOrder>(COLLECTION);
    const query: Record<string, unknown> = {};
    if (opts.status) query.status = opts.status;
    if (opts.strategyId) query.strategyId = opts.strategyId;
    if (opts.gate) query.gate = opts.gate;
    if (opts.sinceMs) query.createdAt = { $gte: new Date(opts.sinceMs).toISOString() };

    return await col
      .find(query)
      .sort({ createdAt: -1 })
      .limit(Math.min(opts.limit ?? 100, 500))
      .toArray()
      .then((docs) => docs.map(({ _id: _ignored, ...rest }) => rest as PaperOmsOrder));
  } catch {
    return [];
  }
}

/** Latest N OMS orders, newest-first. */
export async function getLatestPaperOmsOrders(limit = 50): Promise<PaperOmsOrder[]> {
  return listPaperOmsOrders({ limit });
}
