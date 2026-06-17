import { getDb, isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { DEFAULT_MOCK_ACCOUNT_KEY } from "@/lib/trading/mockTradingPersistenceTypes";
import {
  clampMinSignalScore,
  clampSignalThreshold,
  defaultExecutorThresholdConfig,
  EXECUTOR_CONFIG_CACHE_TTL_MS,
  toExecutorConfigView,
  validateExecutorThresholdInput,
  type ExecutorConfigChangeDoc,
  type ExecutorConfigChangeView,
  type ExecutorConfigSource,
  type ExecutorConfigView,
  type ExecutorThresholdConfigDoc,
} from "@/lib/mockTradingExecutor/executorConfigConstants";

export const MOCK_EXECUTOR_CONFIG_COLLECTION = "mock_executor_config";
export const MOCK_CONFIG_CHANGES_COLLECTION = "mock_config_changes";

export type {
  ExecutorConfigChangeView,
  ExecutorConfigSource,
  ExecutorConfigView,
  ExecutorThresholdConfigDoc,
} from "@/lib/mockTradingExecutor/executorConfigConstants";

export {
  clampMinSignalScore,
  clampSignalThreshold,
  MIN_SIGNAL_SCORE_MAX,
  MIN_SIGNAL_SCORE_MIN,
  SIGNAL_THRESHOLD_MAX,
  SIGNAL_THRESHOLD_MIN,
  validateExecutorThresholdInput,
} from "@/lib/mockTradingExecutor/executorConfigConstants";

const configCache = new Map<string, { config: ExecutorThresholdConfigDoc; expiresAt: number }>();

export function invalidateExecutorConfigCache(accountKey: string): void {
  configCache.delete(accountKey);
}

export async function ensureExecutorConfigIndexes(): Promise<void> {
  if (!isMongoConfigured()) return;
  const db = await getDb();
  await Promise.all([
    db
      .collection(MOCK_EXECUTOR_CONFIG_COLLECTION)
      .createIndex({ account_key: 1 }, { unique: true }),
    db.collection(MOCK_EXECUTOR_CONFIG_COLLECTION).createIndex({ updated_at: -1 }),
    db
      .collection(MOCK_CONFIG_CHANGES_COLLECTION)
      .createIndex({ account_key: 1, timestamp: -1 }),
    db.collection(MOCK_CONFIG_CHANGES_COLLECTION).createIndex({ timestamp: -1 }),
  ]);
}

export async function loadExecutorThresholdConfig(
  accountKey: string,
  opts?: { bypassCache?: boolean },
): Promise<{ doc: ExecutorThresholdConfigDoc; source: ExecutorConfigSource }> {
  const key = accountKey.trim() || DEFAULT_MOCK_ACCOUNT_KEY;
  const cached = configCache.get(key);
  if (!opts?.bypassCache && cached && cached.expiresAt > Date.now()) {
    return { doc: cached.config, source: "mongodb" };
  }

  if (!isMongoConfigured()) {
    const doc = defaultExecutorThresholdConfig(key);
    const source: ExecutorConfigSource =
      process.env.NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD ? "env" : "fallback";
    return { doc, source };
  }

  await ensureExecutorConfigIndexes();
  const db = await getDb();
  const stored = await db
    .collection<ExecutorThresholdConfigDoc>(MOCK_EXECUTOR_CONFIG_COLLECTION)
    .findOne({ account_key: key });

  if (stored) {
    const doc: ExecutorThresholdConfigDoc = {
      account_key: key,
      signal_threshold: clampSignalThreshold(stored.signal_threshold),
      min_signal_score: clampMinSignalScore(stored.min_signal_score),
      updated_at: stored.updated_at ?? Date.now(),
      updated_by: stored.updated_by,
      notes: stored.notes,
    };
    configCache.set(key, { config: doc, expiresAt: Date.now() + EXECUTOR_CONFIG_CACHE_TTL_MS });
    return { doc, source: "mongodb" };
  }

  const doc = defaultExecutorThresholdConfig(key);
  const source: ExecutorConfigSource =
    process.env.NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD ? "env" : "fallback";
  configCache.set(key, { config: doc, expiresAt: Date.now() + EXECUTOR_CONFIG_CACHE_TTL_MS });
  return { doc, source };
}

export async function getExecutorConfigView(
  accountKey: string,
  opts?: { bypassCache?: boolean },
): Promise<ExecutorConfigView> {
  const { doc, source } = await loadExecutorThresholdConfig(accountKey, opts);
  return toExecutorConfigView(doc, source);
}

export async function saveExecutorThresholdConfig(args: {
  accountKey: string;
  signalThreshold: number;
  minSignalScore: number;
  reason?: string;
  userId?: string;
}): Promise<{ config: ExecutorConfigView; changeId: string | null }> {
  const accountKey = args.accountKey.trim() || DEFAULT_MOCK_ACCOUNT_KEY;
  const validationError = validateExecutorThresholdInput(args.signalThreshold, args.minSignalScore);
  if (validationError) {
    throw new Error(validationError);
  }
  const signalThreshold = clampSignalThreshold(args.signalThreshold);
  const minSignalScore = clampMinSignalScore(args.minSignalScore);

  const { doc: prior } = await loadExecutorThresholdConfig(accountKey, { bypassCache: true });
  const now = Date.now();
  const nextDoc: ExecutorThresholdConfigDoc = {
    account_key: accountKey,
    signal_threshold: signalThreshold,
    min_signal_score: minSignalScore,
    updated_at: now,
    updated_by: args.userId ?? "api",
    notes: args.reason,
  };

  if (!isMongoConfigured()) {
    invalidateExecutorConfigCache(accountKey);
    configCache.set(accountKey, {
      config: nextDoc,
      expiresAt: Date.now() + EXECUTOR_CONFIG_CACHE_TTL_MS,
    });
    return { config: toExecutorConfigView(nextDoc, "fallback"), changeId: null };
  }

  await ensureExecutorConfigIndexes();
  const db = await getDb();

  await db.collection(MOCK_EXECUTOR_CONFIG_COLLECTION).updateOne(
    { account_key: accountKey },
    { $set: nextDoc },
    { upsert: true },
  );

  const changes: Record<string, { old: number; new: number }> = {};
  if (prior.signal_threshold !== signalThreshold) {
    changes.signalThreshold = { old: prior.signal_threshold, new: signalThreshold };
  }
  if (prior.min_signal_score !== minSignalScore) {
    changes.minSignalScore = { old: prior.min_signal_score, new: minSignalScore };
  }

  let changeId: string | null = null;
  if (Object.keys(changes).length > 0) {
    const insert = await db.collection(MOCK_CONFIG_CHANGES_COLLECTION).insertOne({
      account_key: accountKey,
      timestamp: now,
      changes,
      reason: args.reason,
      user_id: args.userId ?? "api",
    } satisfies ExecutorConfigChangeDoc);
    changeId = insert.insertedId.toString();
  }

  invalidateExecutorConfigCache(accountKey);
  configCache.set(accountKey, {
    config: nextDoc,
    expiresAt: Date.now() + EXECUTOR_CONFIG_CACHE_TTL_MS,
  });

  return { config: toExecutorConfigView(nextDoc, "mongodb"), changeId };
}

export async function listExecutorConfigHistory(
  accountKey: string,
  limit = 10,
): Promise<ExecutorConfigChangeView[]> {
  if (!isMongoConfigured()) return [];
  await ensureExecutorConfigIndexes();
  const db = await getDb();
  const docs = await db
    .collection<ExecutorConfigChangeDoc & { _id?: { toString(): string } }>(
      MOCK_CONFIG_CHANGES_COLLECTION,
    )
    .find({ account_key: accountKey })
    .sort({ timestamp: -1 })
    .limit(Math.max(1, Math.min(50, limit)))
    .toArray();

  return docs.map((doc) => ({
    id: doc._id?.toString() ?? "",
    accountKey: doc.account_key,
    timestamp: doc.timestamp,
    changes: doc.changes,
    reason: doc.reason,
    userId: doc.user_id,
  }));
}
