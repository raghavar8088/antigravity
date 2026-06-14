#!/usr/bin/env node
/**
 * Phase 31 collection migration — mock_* → paper_* unified architecture.
 *
 * Usage:
 *   cd client
 *   node scripts/migrate-collections-phase31.mjs [--dry-run] [--from=anon_e7da5e39]
 *
 * Copies historical documents into the authoritative account_key
 * (mock_trading_main) and canonical collections:
 *   mock_trades            → paper_trades
 *   mock_account_snapshots → paper_state (latest snapshot fields only)
 *
 * Does NOT delete legacy collections (rollback-safe).
 */

import { MongoClient } from "mongodb";

const TARGET_KEY = process.env.OWNER_ACCOUNT_KEY?.trim() || "mock_trading_main";
const DRY_RUN = process.argv.includes("--dry-run");
const FROM_ARG = process.argv.find((a) => a.startsWith("--from="));
const SOURCE_KEYS = FROM_ARG
  ? [FROM_ARG.split("=")[1]]
  : [TARGET_KEY, process.env.DESK_WORKER_ACCOUNT_KEY?.trim()].filter(Boolean);

const uri = process.env.MONGODB_URI;
const dbName = process.env.MONGODB_DB?.trim() || "loop_trades";

if (!uri) {
  console.error("MONGODB_URI required");
  process.exit(1);
}

function mapMockTradeToPaper(doc, accountKey) {
  return {
    schema_version: 31,
    account_key: accountKey,
    source: "paperpersist-phase31a",
    client_trade_id: doc.trade_id || doc.client_trade_id || doc._id?.toString(),
    strategy_id: String(doc.strategy_id ?? doc.strategyId ?? "unknown"),
    symbol: doc.symbol || "BTC-USD",
    side: doc.side || "LONG",
    entry_price: Number(doc.entry_price ?? doc.entryPrice ?? 0),
    exit_price: Number(doc.exit_price ?? doc.exitPrice ?? 0),
    quantity: Number(doc.quantity ?? doc.contracts ?? doc.size ?? 0),
    gross_pnl: Number(doc.gross_pnl ?? doc.grossPnl ?? 0),
    fees: Number(doc.fees ?? 0),
    net_pnl: Number(doc.net_pnl ?? doc.netPnl ?? 0),
    exit_reason: doc.exit_reason || doc.exitReason || "MIGRATED",
    entry_at: doc.entry_at ? new Date(doc.entry_at) : doc.opened_at ? new Date(doc.opened_at) : new Date(),
    exit_at: doc.exit_at ? new Date(doc.exit_at) : doc.closed_at ? new Date(doc.closed_at) : new Date(),
    closed_at: doc.closed_at ? new Date(doc.closed_at) : new Date(),
    updated_at: new Date(),
    migrated_from: "mock_trades",
  };
}

async function main() {
  const client = new MongoClient(uri);
  await client.connect();
  const db = client.db(dbName);

  console.log(`[migrate] target account_key=${TARGET_KEY} dry_run=${DRY_RUN}`);
  console.log(`[migrate] source account keys: ${[...new Set(SOURCE_KEYS)].join(", ")}`);

  let tradesCopied = 0;
  let snapshotsMerged = 0;

  for (const sourceKey of [...new Set(SOURCE_KEYS)]) {
    const mockTrades = db.collection("mock_trades");
    const paperTrades = db.collection("paper_trades");
    const cursor = mockTrades.find({ account_key: sourceKey });
    for await (const doc of cursor) {
      const mapped = mapMockTradeToPaper(doc, TARGET_KEY);
      if (!mapped.client_trade_id) continue;
      if (!DRY_RUN) {
        await paperTrades.updateOne(
          { client_trade_id: mapped.client_trade_id, account_key: TARGET_KEY },
          { $set: mapped },
          { upsert: true },
        );
      }
      tradesCopied++;
    }

    const latestSnap = await db
      .collection("mock_account_snapshots")
      .find({ account_key: sourceKey })
      .sort({ timestamp: -1, created_at: -1 })
      .limit(1)
      .next();

    if (latestSnap && !DRY_RUN) {
      await db.collection("paper_state").updateOne(
        { account_key: TARGET_KEY },
        {
          $set: {
            account_key: TARGET_KEY,
            source: "paperpersist-phase31a",
            schema_version: 31,
            balance: Number(latestSnap.balance ?? 0),
            equity: Number(latestSnap.equity ?? latestSnap.balance ?? 0),
            updated_at: new Date(),
            snapped_at: new Date(),
            migrated_from: "mock_account_snapshots",
          },
        },
        { upsert: true },
      );
      snapshotsMerged++;
    } else if (latestSnap) {
      snapshotsMerged++;
    }
  }

  console.log(`[migrate] paper_trades upserted: ${tradesCopied}`);
  console.log(`[migrate] paper_state snapshots merged: ${snapshotsMerged}`);
  console.log("[migrate] legacy collections retained for rollback");
  await client.close();
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
