import { getDb, isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import type {
  MockExecutionLogDoc,
  MockExecutorCycleResult,
  MockExecutorMode,
  MockExecutorStateDoc,
} from "@/lib/mockTradingExecutor/types";

export const MOCK_EXECUTOR_STATE_COLLECTION = "mock_executor_state";
export const MOCK_EXECUTION_LOGS_COLLECTION = "mock_execution_logs";

const EXECUTOR_STALE_MS = 30_000;

function nowIso(): string {
  return new Date().toISOString();
}

export async function persistExecutorCycle(
  result: MockExecutorCycleResult,
): Promise<void> {
  if (!isMongoConfigured()) return;

  const db = await getDb();
  const stateDoc: MockExecutorStateDoc = {
    account_key: result.accountKey,
    last_tick_at: result.diagnostics.tickAt,
    last_duration_ms: result.diagnostics.durationMs,
    last_mode: result.mode,
    last_opened: result.opened.length,
    last_closed: result.closed.length,
    last_candidate_count: result.diagnostics.candidateCount,
    last_dominant_blocker: result.diagnostics.dominantBlocker,
    last_error: result.diagnostics.errors[0] ?? null,
    entry_funnel_snapshot: result.funnel,
    updated_at: nowIso(),
  };

  const logDoc: MockExecutionLogDoc = {
    account_key: result.accountKey,
    timestamp: result.diagnostics.tickAt,
    mode: result.mode,
    candidate_count: result.diagnostics.candidateCount,
    opened_count: result.opened.length,
    closed_count: result.closed.length,
    diagnostics: result.diagnostics,
    created_at: nowIso(),
  };

  await Promise.all([
    db.collection(MOCK_EXECUTOR_STATE_COLLECTION).updateOne(
      { account_key: result.accountKey },
      { $set: stateDoc },
      { upsert: true },
    ),
    db.collection(MOCK_EXECUTION_LOGS_COLLECTION).insertOne(logDoc),
  ]);
}

export async function loadExecutorState(
  accountKey: string,
): Promise<MockExecutorStateDoc | null> {
  if (!isMongoConfigured()) return null;
  const db = await getDb();
  const doc = await db
    .collection<MockExecutorStateDoc>(MOCK_EXECUTOR_STATE_COLLECTION)
    .findOne({ account_key: accountKey });
  return doc ?? null;
}

export function executorStateHealth(
  state: MockExecutorStateDoc | null,
  now = Date.now(),
): {
  healthy: boolean;
  stale: boolean;
  ageSeconds: number | null;
} {
  if (!state?.last_tick_at) {
    return { healthy: false, stale: true, ageSeconds: null };
  }
  const ageMs = now - state.last_tick_at;
  const stale = ageMs > EXECUTOR_STALE_MS;
  return {
    healthy: !stale,
    stale,
    ageSeconds: Math.floor(ageMs / 1000),
  };
}

export async function ensureExecutorIndexes(): Promise<void> {
  if (!isMongoConfigured()) return;
  const db = await getDb();
  await Promise.all([
    db
      .collection(MOCK_EXECUTOR_STATE_COLLECTION)
      .createIndex({ account_key: 1 }, { unique: true }),
    db.collection(MOCK_EXECUTION_LOGS_COLLECTION).createIndex({ account_key: 1, timestamp: -1 }),
    db.collection(MOCK_EXECUTION_LOGS_COLLECTION).createIndex({ timestamp: -1 }),
  ]);
}

export type { MockExecutorMode };
