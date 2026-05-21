#!/usr/bin/env node
/**
 * End-to-end smoke test for the MongoDB paper-trades pipeline.
 *
 * Run from `client/`:
 *   npm run test:mongo
 * Or directly:
 *   node --env-file=.env.local scripts/test-mongo.mjs
 *
 * Checks (exit 0 only when ALL pass):
 *   1. MONGODB_URI is set and parseable
 *   2. Atlas connects within 8s
 *   3. `loop_trades` database admin ping succeeds
 *   4. `paper_trades` collection accepts an upsert with a fresh client_trade_id
 *   5. The just-inserted document is readable by client_trade_id
 *   6. Cleanup deletes the test document
 */

import { MongoClient } from "mongodb";
import { randomUUID } from "node:crypto";

const uri = process.env.MONGODB_URI;
const dbName = process.env.MONGODB_DB || "loop_trades";
const COLLECTION = "paper_trades";

if (!uri) {
  console.error("FAIL: MONGODB_URI is not set.");
  console.error("Run from client/ with: node --env-file=.env.local scripts/test-mongo.mjs");
  process.exit(1);
}

if (!uri.startsWith("mongodb+srv://") && !uri.startsWith("mongodb://")) {
  console.error("FAIL: MONGODB_URI is not a valid mongodb:// or mongodb+srv:// URI.");
  process.exit(1);
}

const redacted = uri.replace(/\/\/[^@]+@/, "//***:***@");
console.log("URI       :", redacted);
console.log("DB name   :", dbName);
console.log("Collection:", COLLECTION);
console.log();

const clientTradeId = randomUUID();
const accountKey = `__test_${randomUUID().slice(0, 8)}`;
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
  module_key: "btc_future_trading",
};

const client = new MongoClient(uri, {
  serverSelectionTimeoutMS: 8_000,
  connectTimeoutMS: 8_000,
});

let step = "init";
try {
  step = "connect";
  process.stdout.write("[1/6] Connecting to MongoDB Atlas... ");
  await client.connect();
  console.log("OK");

  step = "ping";
  process.stdout.write("[2/6] Ping admin... ");
  const db = client.db(dbName);
  await db.command({ ping: 1 });
  console.log("OK");

  const col = db.collection(COLLECTION);

  step = "upsert";
  process.stdout.write("[3/6] Upserting test trade... ");
  const upsert = await col.updateOne(
    { client_trade_id: clientTradeId },
    { $set: testRow, $setOnInsert: { created_at: now } },
    { upsert: true },
  );
  if (!upsert.upsertedCount && !upsert.modifiedCount && !upsert.matchedCount) {
    throw new Error("updateOne returned no upserted/modified/matched count");
  }
  console.log(`OK (upserted=${upsert.upsertedCount}, matched=${upsert.matchedCount})`);

  step = "read";
  process.stdout.write("[4/6] Reading back by client_trade_id... ");
  const found = await col.findOne({ client_trade_id: clientTradeId });
  if (!found) throw new Error("Document not found after upsert");
  if (found.net_pnl !== testRow.net_pnl) throw new Error("net_pnl mismatch");
  if (found.strategy_name !== testRow.strategy_name) throw new Error("strategy_name mismatch");
  if (found.account_key !== accountKey) throw new Error("account_key mismatch");
  console.log("OK");

  step = "count";
  process.stdout.write("[5/6] Counting docs in collection... ");
  const count = await col.countDocuments();
  console.log(`OK (${count} total)`);

  step = "cleanup";
  process.stdout.write("[6/6] Cleaning up test document... ");
  const del = await col.deleteOne({ client_trade_id: clientTradeId });
  if (del.deletedCount !== 1) throw new Error("Cleanup deletion failed");
  console.log("OK");

  console.log();
  console.log("PASS: MongoDB Atlas is reachable and paper_trades collection works.");
  console.log(`Next: open /api/health/storage in the browser to confirm Next.js sees the same.`);
  process.exit(0);
} catch (err) {
  const message = err instanceof Error ? err.message : String(err);
  console.log();
  console.error(`FAIL at step "${step}": ${message}`);
  if (/IP|whitelist|not\s+allowed/i.test(message)) {
    console.error("HINT: Your current IP is not in Atlas Network Access allow-list.");
  } else if (/authentication|auth/i.test(message)) {
    console.error("HINT: Username/password in MONGODB_URI is wrong, or DB user has no readWrite role.");
  } else if (/timed out|ENOTFOUND|ECONNREFUSED/i.test(message)) {
    console.error("HINT: Network/DNS — check internet, firewall, and that the cluster host resolves.");
  }
  process.exit(1);
} finally {
  await client.close().catch(() => undefined);
}
