/**
 * Deletes mock_trades rows with invalid opened_at (0 / missing).
 * Run: npx tsx --env-file=.env.local scripts/fix-corrupt-mock-trades.ts
 */
import { getDb, isMongoConfigured } from "../src/lib/broker/mongoTradesClient";
import { MOCK_TRADES_COLLECTION } from "../src/lib/trading/mockTradingMongo";

async function main() {
  if (!isMongoConfigured()) {
    console.error("MONGODB_URI not configured");
    process.exit(1);
  }
  const db = await getDb();
  const col = db.collection(MOCK_TRADES_COLLECTION);
  const corrupt = await col
    .find({
      $or: [
        { opened_at: { $in: [0, null] } },
        { "raw_trade.openedAt": { $in: [0, null] } },
      ],
    })
    .toArray();

  console.log(`Found ${corrupt.length} corrupt mock_trades row(s)`);
  for (const row of corrupt) {
    await col.deleteOne({ _id: row._id });
    console.log(`Deleted trade_id=${row.trade_id ?? row._id}`);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
