/**
 * MongoDB lease/heartbeat for the 24/7 paper desk worker.
 *
 * One document per operator account in `desk_worker_lease`. The long-running
 * AWS worker acquires the lease on startup and renews it every poll tick.
 * The Vercel cron failover checks `isLeaseFresh()` before running — if the
 * worker is alive and current, the cron is a no-op.
 *
 * Staleness threshold: 3 minutes (worker polls every ~60 s, so 3 missed polls).
 */

import { getDb } from "@/lib/broker/mongoTradesClient";

export const LEASE_COLLECTION = "desk_worker_lease";
export const LEASE_STALE_MS = 3 * 60 * 1000;

export interface WorkerLeaseDoc {
  _id: string;
  owner: string;
  startedAt: string;
  heartbeatAt: string;
  pollCount: number;
  version: string;
  symbol: string;
}

export async function acquireLease(
  accountKey: string,
  owner = "aws-lightsail",
  symbol = "BTCUSD",
): Promise<void> {
  const db = await getDb();
  const now = new Date().toISOString();
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  await db.collection(LEASE_COLLECTION).updateOne(
    { _id: accountKey } as any,
    {
      $set: { owner, heartbeatAt: now, symbol },
      $setOnInsert: { startedAt: now, pollCount: 0, version: "1.0.0" },
    },
    { upsert: true },
  );
}

export async function renewLeaseHeartbeat(accountKey: string): Promise<void> {
  const db = await getDb();
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  await db.collection(LEASE_COLLECTION).updateOne(
    { _id: accountKey } as any,
    { $set: { heartbeatAt: new Date().toISOString() }, $inc: { pollCount: 1 } },
  );
}

export async function releaseLease(accountKey: string): Promise<void> {
  const db = await getDb();
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  await db.collection(LEASE_COLLECTION).deleteOne({ _id: accountKey } as any);
}

/** Returns true when the worker heartbeat is recent enough (worker is alive). */
export async function isLeaseFresh(accountKey: string): Promise<boolean> {
  try {
    const db = await getDb();
    const doc = await db
      .collection<WorkerLeaseDoc>(LEASE_COLLECTION)
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      .findOne({ _id: accountKey } as any);
    if (!doc?.heartbeatAt) return false;
    return Date.now() - new Date(doc.heartbeatAt).getTime() < LEASE_STALE_MS;
  } catch {
    return false;
  }
}

/** Returns the raw lease doc for the health endpoint; null if not found or error. */
export async function getLeaseDoc(accountKey: string): Promise<WorkerLeaseDoc | null> {
  try {
    const db = await getDb();
    return db
      .collection<WorkerLeaseDoc>(LEASE_COLLECTION)
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      .findOne({ _id: accountKey } as any) as Promise<WorkerLeaseDoc | null>;
  } catch {
    return null;
  }
}
