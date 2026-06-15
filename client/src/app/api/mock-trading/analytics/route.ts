import { NextResponse } from "next/server";
import { isMongoConfigured, getDb } from "@/lib/broker/mongoTradesClient";
import {
  getMockAnalyticsSummary,
  getLatestMockAccountSnapshot,
  MOCK_STRATEGY_ANALYTICS_COLLECTION,
  MOCK_TRADES_COLLECTION,
  type MockTradeDoc,
  mockTradeFromDoc,
} from "@/lib/trading/mockTradingMongo";
import { mockTradingConfigSchema } from "@/lib/trading/mockTradingPersistenceTypes";
import { DEFAULT_MOCK_TRADING_CONFIG, computeAnalytics } from "@/lib/trading/mockTradingEngine";
import { OWNER_ACCOUNT_KEY } from "@/lib/broker/ownerAuth";
import type { MockTradingConfig } from "@/lib/trading/mockTradingEngine";

export const dynamic = "force-dynamic";

const ANALYTICS_STALE_MS = 5 * 60 * 1000; // 5 minutes

/** PERF 2: Recompute analytics from raw trades and upsert to mock_strategy_analytics. */
export async function triggerAnalyticsRecompute(
  accountKey: string,
  config: MockTradingConfig,
): Promise<void> {
  try {
    const db = await getDb();
    const tradesColl = db.collection<MockTradeDoc>(MOCK_TRADES_COLLECTION);
    const docs = await tradesColl
      .find({ account_key: accountKey })
      .sort({ opened_at: 1 })
      .limit(50_000)
      .toArray();
    const rawTrades = docs.map(mockTradeFromDoc);
    const analytics = computeAnalytics(rawTrades);
    const analyticsColl = db.collection(MOCK_STRATEGY_ANALYTICS_COLLECTION);
    await analyticsColl.updateOne(
      { account_key: accountKey },
      {
        $set: {
          account_key: accountKey,
          generated_at: Date.now(),
          analytics,
          config,
          computed_at: new Date(),
          created_at: new Date().toISOString(),
        },
      },
      { upsert: true },
    );
  } catch (err) {
    console.error("[analytics recompute] error:", err);
  }
}

export async function GET() {
  const accountKey = OWNER_ACCOUNT_KEY;

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED", error: "MongoDB not configured" }, { status: 503 });
  }

  try {
    // PERF 2: Try reading from cached mock_strategy_analytics first.
    const db = await getDb();
    const analyticsColl = db.collection(MOCK_STRATEGY_ANALYTICS_COLLECTION);
    const cached = await analyticsColl
      .findOne({ account_key: accountKey }, { sort: { computed_at: -1 } });

    let config: MockTradingConfig = DEFAULT_MOCK_TRADING_CONFIG;
    try {
      const latest = await getLatestMockAccountSnapshot(accountKey);
      const parsed = mockTradingConfigSchema.safeParse(latest?.config);
      if (parsed.success) config = (latest?.config ?? DEFAULT_MOCK_TRADING_CONFIG) as MockTradingConfig;
    } catch {
      // use default config
    }

    if (cached && cached.computed_at) {
      const age = Date.now() - new Date(cached.computed_at as string | Date).getTime();
      if (age < ANALYTICS_STALE_MS) {
        // Fresh cache: return immediately.
        return NextResponse.json({
          ok: true,
          analytics: cached.analytics,
          source: "cache",
          storage: "mongo",
          cache_age_ms: age,
        });
      }
      // Stale: trigger background recompute but return cached result immediately.
      void triggerAnalyticsRecompute(accountKey, config);
      return NextResponse.json({
        ok: true,
        analytics: cached.analytics,
        source: "stale_cache",
        storage: "mongo",
        cache_age_ms: age,
      });
    }

    // No cache: compute synchronously and return.
    const analytics = await getMockAnalyticsSummary(accountKey, config);
    return NextResponse.json({ ok: true, analytics, source: "computed", storage: "mongo" });
  } catch (err) {
    return NextResponse.json(
      {
        ok: false,
        code: "MONGO_READ_FAILED",
        error: "Mongo read failed",
        detail: err instanceof Error ? err.message : "unknown",
      },
      { status: 500 },
    );
  }
}
