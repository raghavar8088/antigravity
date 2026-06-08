#!/usr/bin/env node
/**
 * Backfill entry_fee, exit_fee, total_fee on paper_trades from canonical model.
 * Usage: node scripts/backfill-paper-trade-fees.mjs
 */
import { MongoClient } from "mongodb";

const TAKER = 0.0005;
const uri = process.env.MONGODB_URI;
const dbName = process.env.MONGODB_DB || "loop_trades";

if (!uri) {
  console.error("MONGODB_URI required");
  process.exit(1);
}

function canonicalFees(entryPrice, exitPrice, quantity) {
  const entryNotional = entryPrice * quantity;
  const exitNotional = exitPrice * quantity;
  const entry_fee = entryNotional * TAKER;
  const exit_fee = exitNotional * TAKER;
  return { entry_fee, exit_fee, total_fee: entry_fee + exit_fee };
}

const client = new MongoClient(uri);
await client.connect();
const col = client.db(dbName).collection("paper_trades");

const cursor = col.find({});
let updated = 0;
while (await cursor.hasNext()) {
  const doc = await cursor.next();
  const entry = Number(doc.entry_price) || 0;
  const exit = Number(doc.exit_price) || 0;
  const qty = Number(doc.quantity) || 0;
  if (entry <= 0 || exit <= 0 || qty <= 0) continue;

  const fees = canonicalFees(entry, exit, qty);
  const gross = Number(doc.gross_pnl) || 0;
  const net = gross - fees.total_fee;

  await col.updateOne(
    { _id: doc._id },
    {
      $set: {
        entry_fee: fees.entry_fee,
        exit_fee: fees.exit_fee,
        total_fee: fees.total_fee,
        fees: fees.total_fee,
        net_pnl: net,
      },
    },
  );
  updated++;
}

console.log(`Backfilled ${updated} paper_trades`);
await client.close();
