#!/usr/bin/env node
/**
 * Manual smoke-test for the MongoDB paper-trades pipeline.
 *
 * Usage (from project root):
 *   node --env-file=client/.env.local scripts/test-mongo.mjs
 *
 * Or from the client directory:
 *   npm run test:mongo
 *
 * Exit 0 = all checks passed. Exit 1 = failure.
 */

import { MongoClient } from "mongodb";
import { randomUUID } from "crypto";

const MONGODB_URI = process.env.MONGODB_URI;
const MONGODB_DB = process.env.MONGODB_DB || "loop_trades";

if (!MONGODB_URI) {
  console.error("[test-mongo] MONGODB_URI is not set. Load .env.local first:");
  console.error("  node --env-file=client/.env.local scripts/test-mongo.mjs");
  process.exit(1);
}

const client = new MongoClient(MONGODB_URI, {
  serverSelectionTimeoutMS: 8_000,
  connectTimeoutMS: 8_000,
});

const clientTradeId = randomUUID();
const accountKey = `test_${randomUUID().slice(0, 8)}`;
const now = new Date().toISOString();

const testRow = {
  account_key: accountKey,
  client_trade_id: clientTradeId,
  opened_at: now,
  closed_at: now,
  symbol: "BTCUSDT",
  strategy_id: 999,
  strategy_name: "__test_mongo_script__",
  side: "LONG",
  entry_price: 50000,
  exit_price: 50100,
  contracts: 2,
  notional: 100,
  margin_used: 10,
  gross_pnl: 0.19,
  fees: 0.1,
  funding_costs: 0,
  net_pnl: 0.09,
  exit_reason: "TP",
  payload: { _test: true },
  created_at: now,
};

try {
  process.stdout.write("[test-mongo] Connecting to MongoDB Atlas... ");
  await client.connect();
  console.log("OK");

  const db = client.db(MONGODB_DB);

  process.stdout.write("[test-mongo] Ping... ");
  await db.command({ ping: 1 });
  console.log("OK");

  const col = db.collection("paper_trades");

  process.stdout.write("[test-mongo] Inserting test trade... ");
  await col.updateOne(
    { client_trade_id: clientTradeId },
    { $setOnInsert: testRow },
    { upsert: true },
  );
  console.log("OK");

  process.stdout.write("[test-mongo] Reading back... ");
  const found = await col.findOne({ client_trade_id: clientTradeId });
  if (!found) throw new Error("Document not found after upsert");
  if (found.net_pnl !== testRow.net_pnl) throw new Error("net_pnl mismatch");
  if (found.strategy_name !== testRow.strategy_name) throw new Error("strategy_name mismatch");
  console.log("OK");

  process.stdout.write("[test-mongo] Cleaning up test document... ");
  await col.deleteOne({ client_trade_id: clientTradeId });
  console.log("OK");

  console.log("\n[test-mongo] All checks passed. MongoDB is reachable and paper_trades collection works.");
  console.log(`  DB: ${MONGODB_DB}  Collection: paper_trades`);
} catch (err) {
  console.error("\n[test-mongo] FAILED:", err instanceof Error ? err.message : err);
  process.exit(1);
} finally {
  await client.close().catch(() => undefined);
}
