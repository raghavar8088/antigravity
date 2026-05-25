/**
 * GET /api/health/desk-worker
 *
 * Returns the liveness state of the 24/7 headless paper desk worker.
 * Source of truth: `paper_state.worker_last_poll_at` — the same field the VPS
 * worker writes on every tick and the browser UI reads in monitor mode.
 *
 * Previously read from `desk_worker_lease` (a separate collection) which the
 * production worker never writes to, causing false-stale reports.
 */

import { NextResponse } from "next/server";
import { isMongoConfigured, getAccountState } from "@/lib/mongoTradesClient";
import { isWorkerHeartbeatFresh } from "@/lib/paperDeskWorker/workerLease";

export const dynamic = "force-dynamic";

export async function GET() {
  const accountKey = process.env.DESK_WORKER_ACCOUNT_KEY?.trim();

  if (!accountKey || !isMongoConfigured()) {
    return NextResponse.json({
      workerLastPollAt: null,
      owner: null,
      stale: true,
      source: "paper_state",
    });
  }

  const state = await getAccountState(accountKey);
  const workerLastPollAt = state?.worker_last_poll_at ?? null;
  const stale = !isWorkerHeartbeatFresh(workerLastPollAt);

  return NextResponse.json({
    workerLastPollAt,
    owner: state?.worker_id ?? null,
    stale,
    source: "paper_state",
    buildSha: process.env.NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA?.slice(0, 7) ?? null,
  });
}
