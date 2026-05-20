// One-shot migration: pulls every row from Supabase `paper_trades` and upserts
// it into MongoDB Atlas. Idempotent — safe to re-run.
//   node --env-file=.env.local scripts/migrate-supabase-to-mongo.mjs
import { MongoClient } from "mongodb";

const SUPABASE_URL = process.env.NEXT_PUBLIC_SUPABASE_URL;
const SUPABASE_KEY = process.env.SUPABASE_SERVICE_ROLE_KEY;
const MONGODB_URI = process.env.MONGODB_URI;
const MONGODB_DB = process.env.MONGODB_DB || "loop_trades";

for (const [k, v] of Object.entries({ SUPABASE_URL, SUPABASE_KEY, MONGODB_URI })) {
  if (!v) { console.error("Missing env:", k); process.exit(1); }
}

// REST API page size — Supabase default cap is 1000 per request.
const PAGE = 1000;

async function fetchSupabasePage(offset) {
  const url = `${SUPABASE_URL}/rest/v1/paper_trades?select=*&order=closed_at.desc&limit=${PAGE}&offset=${offset}`;
  const res = await fetch(url, {
    headers: {
      apikey: SUPABASE_KEY,
      Authorization: `Bearer ${SUPABASE_KEY}`,
    },
  });
  if (!res.ok) throw new Error(`Supabase HTTP ${res.status}: ${await res.text()}`);
  return res.json();
}

async function fetchAllSupabaseTrades() {
  const all = [];
  let offset = 0;
  while (true) {
    const page = await fetchSupabasePage(offset);
    if (!Array.isArray(page) || page.length === 0) break;
    all.push(...page);
    console.log(`  fetched page offset=${offset} count=${page.length} (cumulative ${all.length})`);
    if (page.length < PAGE) break;
    offset += PAGE;
  }
  return all;
}

function normalize(row) {
  // Supabase row already matches PaperTradeDbRow exactly. Strip the Supabase-only
  // `id` (auto-increment PK) so Mongo can assign its own `_id`. Keep `client_trade_id`
  // as the dedup key.
  const { id: _supabaseId, ...rest } = row;
  return rest;
}

const mongo = new MongoClient(MONGODB_URI, { serverSelectionTimeoutMS: 10_000 });
try {
  console.log("→ Fetching from Supabase…");
  const sbRows = await fetchAllSupabaseTrades();
  console.log(`✓ Got ${sbRows.length} rows from Supabase`);
  if (sbRows.length === 0) {
    console.log("Nothing to migrate. Supabase paper_trades is empty.");
    process.exit(0);
  }

  // Group by account_key for visibility before we write anything
  const byAccount = new Map();
  for (const r of sbRows) {
    byAccount.set(r.account_key, (byAccount.get(r.account_key) ?? 0) + 1);
  }
  console.log("\nRows per account_key in Supabase:");
  for (const [k, n] of [...byAccount.entries()].sort((a, b) => b[1] - a[1])) {
    console.log(`  ${k}: ${n}`);
  }

  console.log("\n→ Connecting to MongoDB…");
  await mongo.connect();
  const col = mongo.db(MONGODB_DB).collection("paper_trades");

  await Promise.all([
    col.createIndex({ client_trade_id: 1 }, { unique: true, name: "uniq_client_trade_id" }),
    col.createIndex({ account_key: 1, closed_at: -1 }, { name: "by_account_closed" }),
    col.createIndex({ account_key: 1, strategy_id: 1, closed_at: -1 }, { name: "by_account_strat_closed" }),
  ]);

  console.log("→ Upserting into MongoDB (idempotent on client_trade_id)…");
  let inserted = 0, skipped = 0, failed = 0;
  for (const raw of sbRows) {
    const row = normalize(raw);
    if (!row.client_trade_id) {
      console.warn("  skip row with no client_trade_id");
      skipped++;
      continue;
    }
    try {
      const r = await col.updateOne(
        { client_trade_id: row.client_trade_id },
        {
          $setOnInsert: {
            ...row,
            migrated_from: "supabase",
            migrated_at: new Date().toISOString(),
          },
        },
        { upsert: true },
      );
      if (r.upsertedCount > 0) inserted++; else skipped++;
    } catch (e) {
      failed++;
      console.warn(`  fail ${row.client_trade_id}: ${e.message}`);
    }
  }

  console.log(`\n✓ Done — inserted: ${inserted}, already-in-mongo: ${skipped}, failed: ${failed}`);

  const grandTotal = await col.countDocuments();
  console.log(`Mongo paper_trades total docs now: ${grandTotal}`);

  console.log("\nMongo rows per account_key:");
  const agg = await col.aggregate([
    { $group: { _id: "$account_key", n: { $sum: 1 } } },
    { $sort: { n: -1 } },
  ]).toArray();
  for (const { _id, n } of agg) console.log(`  ${_id}: ${n}`);
} catch (e) {
  console.error("FATAL:", e.message);
  process.exit(1);
} finally {
  await mongo.close();
}
