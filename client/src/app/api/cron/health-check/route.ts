/**
 * GET /api/cron/health-check
 *
 * Standalone executor staleness watchdog. Deliberately NOT registered in
 * vercel.json crons — Vercel Hobby caps a project at 2 crons and that quota
 * is already spent by mock-trading-tick + policy-snapshot. Instead, point
 * a free external pinger (UptimeRobot, cron-job.org, etc.) or the Lightsail
 * pm2-health-check.sh watchdog at this URL every 5 minutes.
 *
 * Catches the failure mode PM2-only monitoring misses: a process that is
 * still "online" in `pm2 status` but hung/stuck and not actually ticking.
 */

import { NextResponse } from "next/server";
import { getDb, isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { loadExecutorState, executorStateHealth } from "@/lib/mockTradingExecutor/persistExecutorState";
import { sendTelegramAlert } from "@/lib/alerts/telegramAlert";
import { DEFAULT_MOCK_ACCOUNT_KEY } from "@/lib/trading/mockTradingPersistenceTypes";

export const dynamic = "force-dynamic";
export const maxDuration = 15;

const STALE_THRESHOLD_SECONDS = 300; // 5 minutes — well above the 30s in-app threshold, avoids alert noise on transient blips
const ALERT_COOLDOWN_MS = 30 * 60 * 1000; // re-alert at most once per 30 minutes while still stale
const ALERT_STATE_COLLECTION = "executor_alert_state";

function unauthorized() {
  return NextResponse.json({ ok: false, error: "Unauthorized" }, { status: 401 });
}

async function shouldSendAlert(accountKey: string, now: number): Promise<boolean> {
  if (!isMongoConfigured()) return true; // can't throttle without Mongo — fail open so the alert isn't silently dropped
  const db = await getDb();
  const col = db.collection<{ account_key: string; last_alert_at: number }>(ALERT_STATE_COLLECTION);
  const doc = await col.findOne({ account_key: accountKey });
  if (doc && now - doc.last_alert_at < ALERT_COOLDOWN_MS) return false;
  await col.updateOne(
    { account_key: accountKey },
    { $set: { account_key: accountKey, last_alert_at: now } },
    { upsert: true },
  );
  return true;
}

async function clearAlertState(accountKey: string): Promise<void> {
  if (!isMongoConfigured()) return;
  const db = await getDb();
  await db.collection(ALERT_STATE_COLLECTION).deleteOne({ account_key: accountKey });
}

export async function GET(req: Request): Promise<NextResponse> {
  const secret = process.env.CRON_SECRET?.trim();
  if (!secret) {
    return NextResponse.json(
      { ok: false, error: "CRON_SECRET env var is not configured — cron endpoint is locked" },
      { status: 503 },
    );
  }
  const auth = req.headers.get("authorization") ?? "";
  if (auth !== `Bearer ${secret}`) return unauthorized();

  const url = new URL(req.url);
  const accountKey = url.searchParams.get("account_key")?.trim() || DEFAULT_MOCK_ACCOUNT_KEY;
  const now = Date.now();

  const state = await loadExecutorState(accountKey);
  const health = executorStateHealth(state, now);
  const staleSeconds = health.ageSeconds;

  const isCriticallyStale = staleSeconds == null || staleSeconds > STALE_THRESHOLD_SECONDS;

  if (!isCriticallyStale) {
    await clearAlertState(accountKey);
    return NextResponse.json({ status: "healthy", account_key: accountKey, staleSeconds });
  }

  const alertSent = await shouldSendAlert(accountKey, now);
  if (alertSent) {
    const durationLabel =
      staleSeconds == null
        ? "no tick has ever been recorded"
        : `last tick was ${Math.floor(staleSeconds / 3600)}h ${Math.floor((staleSeconds % 3600) / 60)}m ago`;
    await sendTelegramAlert(
      `EXECUTOR STALE [${accountKey}]: ${durationLabel}. Expected a tick every 5s (PM2) / 60s (Vercel cron fallback). ` +
        `Check: pm2 status on Lightsail, and Vercel cron invocation history for /api/cron/mock-trading-tick.`,
    );
  }

  return NextResponse.json(
    { status: "stale", account_key: accountKey, staleSeconds, alertSent },
    { status: 503 },
  );
}
