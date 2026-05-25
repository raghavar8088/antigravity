/**
 * GET /api/health/desk-worker
 *
 * Returns the current state of the 24/7 headless paper desk worker for the
 * DeskRunModePanel UI component.
 *
 * Response shape:
 *   { workerLastPollAt: number | null, owner: string | null, stale: boolean }
 */

import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getLeaseDoc, LEASE_STALE_MS } from "@/lib/deskWorkerLease";

export const dynamic = "force-dynamic";

export async function GET() {
  const accountKey = process.env.DESK_WORKER_ACCOUNT_KEY?.trim();

  if (!accountKey || !isMongoConfigured()) {
    return NextResponse.json({
      workerLastPollAt: null,
      owner: null,
      stale: true,
    });
  }

  const lease = await getLeaseDoc(accountKey);

  if (!lease?.heartbeatAt) {
    return NextResponse.json({
      workerLastPollAt: null,
      owner: null,
      stale: true,
    });
  }

  const heartbeatMs = new Date(lease.heartbeatAt).getTime();
  const stale = Date.now() - heartbeatMs > LEASE_STALE_MS;

  return NextResponse.json({
    workerLastPollAt: heartbeatMs,
    owner: lease.owner ?? null,
    stale,
    pollCount: lease.pollCount ?? 0,
    symbol: lease.symbol ?? null,
  });
}
