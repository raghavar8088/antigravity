// One-shot migration: backfills the trades found in browser localStorage into
// MongoDB Atlas. Run from client/ dir:
//   node --env-file=.env.local scripts/migrate-legacy-trades.mjs
import { MongoClient } from "mongodb";
import { randomUUID } from "node:crypto";

const ACCOUNT_KEY = "anon_migrated_legacy";

const ENGINE_STATE = {"balance":906.9126958933055,"trades":[{"id":"TSTUSD-1-1778608728797-525u","symbol":"TSTUSD","strategyId":1,"strategyName":"EMA_Cross_Long","side":"LONG","entryPrice":0.02081,"exitPrice":0.020751732,"contracts":50,"notional":50,"marginUsed":2,"realizedPnl":-0.1400000000000005,"fees":0.1,"netPnl":-1.403494612970703,"netPnlPct":-70.17473064853516,"fundingCosts":1.1634946129707027,"openedAt":"2026-05-12T17:58:48.797Z","closedAt":"2026-05-12T17:59:08.336Z","exitReason":"SL","liquidationPrice":0.02008165,"liquidationDistancePct":3.22904131568391},{"id":"TSTUSD-91-1778608728797-lcx8","symbol":"TSTUSD","strategyId":91,"strategyName":"Trend_Continuation_Long","side":"LONG","entryPrice":0.02081,"exitPrice":0.020755893999999997,"contracts":50,"notional":50,"marginUsed":2,"realizedPnl":-0.13000000000000406,"fees":0.1,"netPnl":-1.3934946129707066,"netPnlPct":-69.67473064853533,"fundingCosts":1.1634946129707027,"openedAt":"2026-05-12T17:58:48.797Z","closedAt":"2026-05-12T17:59:08.336Z","exitReason":"SL","liquidationPrice":0.02008165,"liquidationDistancePct":3.2484459594946746},{"id":"TSTUSD-111-1778608728797-das4","symbol":"TSTUSD","strategyId":111,"strategyName":"MTF_Trend_Align_Long","side":"LONG","entryPrice":0.02081,"exitPrice":0.020755893999999997,"contracts":50,"notional":50,"marginUsed":2,"realizedPnl":-0.13000000000000406,"fees":0.1,"netPnl":-1.3934946129707066,"netPnlPct":-69.67473064853533,"fundingCosts":1.1634946129707027,"openedAt":"2026-05-12T17:58:48.797Z","closedAt":"2026-05-12T17:59:08.336Z","exitReason":"SL","liquidationPrice":0.02008165,"liquidationDistancePct":3.2484459594946746},{"id":"SKYAIUSD-112-1778608728872-hj11","symbol":"SKYAIUSD","strategyId":112,"strategyName":"MTF_Trend_Align_Short","side":"SHORT","entryPrice":0.54777,"exitPrice":0.5491942019999999,"contracts":50,"notional":50,"marginUsed":2,"realizedPnl":-0.12999999999999362,"fees":0.1,"netPnl":-47.18118659414227,"netPnlPct":-2359.059329707113,"fundingCosts":46.95118659414227,"openedAt":"2026-05-12T17:58:48.872Z","closedAt":"2026-05-12T17:59:16.784Z","exitReason":"SL","liquidationPrice":0.56694195,"liquidationDistancePct":3.231597845601458},{"id":"BIOUSD-111-1778608728791-29ze","symbol":"BIOUSD","strategyId":111,"strategyName":"MTF_Trend_Align_Long","side":"LONG","entryPrice":0.04682,"exitPrice":0.046698268,"contracts":50,"notional":50,"marginUsed":2,"realizedPnl":-0.12999999999999934,"fees":0.1,"netPnl":16.0938824874477,"netPnlPct":804.694124372385,"fundingCosts":-16.3238824874477,"openedAt":"2026-05-12T17:58:48.791Z","closedAt":"2026-05-12T17:59:33.985Z","exitReason":"SL","liquidationPrice":0.0451813,"liquidationDistancePct":3.248445959494687},{"id":"SIRENUSD-61-1778608728850-jvgr","symbol":"SIRENUSD","strategyId":61,"strategyName":"Williams_Momentum_Long","side":"LONG","entryPrice":1.16772,"exitPrice":1.163983296,"contracts":50,"notional":50,"marginUsed":2,"realizedPnl":-0.16000000000000147,"fees":0.1,"netPnl":-81.63462067259418,"netPnlPct":-4081.731033629709,"fundingCosts":81.37462067259418,"openedAt":"2026-05-12T17:58:48.850Z","closedAt":"2026-05-12T18:00:04.676Z","exitReason":"SL","liquidationPrice":1.1268498,"liquidationDistancePct":3.190208667736761},{"id":"AIOTUSD-2-1778608804704-9zbj","symbol":"AIOTUSD","strategyId":2,"strategyName":"EMA_Cross_Short","side":"SHORT","entryPrice":0.1026,"exitPrice":0.10288727999999998,"contracts":50,"notional":50,"marginUsed":2,"realizedPnl":-0.13999999999999366,"fees":0.1,"netPnl":-1.4201918148535495,"netPnlPct":-71.00959074267747,"fundingCosts":1.1801918148535557,"openedAt":"2026-05-12T18:00:04.704Z","closedAt":"2026-05-12T18:01:03.429Z","exitReason":"SL","liquidationPrice":0.10619100000000001,"liquidationDistancePct":3.2110091743119504},{"id":"AIOTUSD-92-1778608804704-820r","symbol":"AIOTUSD","strategyId":92,"strategyName":"Trend_Continuation_Short","side":"SHORT","entryPrice":0.1026,"exitPrice":0.10286675999999999,"contracts":50,"notional":50,"marginUsed":2,"realizedPnl":-0.12999999999999556,"fees":0.1,"netPnl":-1.4101918148535513,"netPnlPct":-70.50959074267756,"fundingCosts":1.1801918148535557,"openedAt":"2026-05-12T18:00:04.704Z","closedAt":"2026-05-12T18:01:03.429Z","exitReason":"SL","liquidationPrice":0.10619100000000001,"liquidationDistancePct":3.2315978456014562},{"id":"AIOTUSD-112-1778608804705-99l8","symbol":"AIOTUSD","strategyId":112,"strategyName":"MTF_Trend_Align_Short","side":"SHORT","entryPrice":0.1026,"exitPrice":0.10286675999999999,"contracts":50,"notional":50,"marginUsed":2,"realizedPnl":-0.12999999999999556,"fees":0.1,"netPnl":-1.4101918148535513,"netPnlPct":-70.50959074267756,"fundingCosts":1.1801918148535557,"openedAt":"2026-05-12T18:00:04.705Z","closedAt":"2026-05-12T18:01:03.429Z","exitReason":"SL","liquidationPrice":0.10619100000000001,"liquidationDistancePct":3.2315978456014562},{"id":"LABUSD-2-1778608804708-m6dr","symbol":"LABUSD","strategyId":2,"strategyName":"EMA_Cross_Short","side":"SHORT","entryPrice":4.674,"exitPrice":4.6450212,"contracts":50,"notional":50,"marginUsed":2,"realizedPnl":0.30999999999999966,"fees":0.1,"netPnl":-0.040000000000000396,"netPnlPct":-2.00000000000002,"fundingCosts":0.25000000000000006,"openedAt":"2026-05-12T18:00:04.708Z","closedAt":"2026-05-12T18:01:03.429Z","exitReason":"TP","liquidationPrice":4.837590000000001,"liquidationDistancePct":4.145703360837212}]};

const uri = process.env.MONGODB_URI;
const dbName = process.env.MONGODB_DB || "loop_trades";
if (!uri) { console.error("MONGODB_URI missing"); process.exit(1); }

function toRow(t) {
  return {
    account_key: ACCOUNT_KEY,
    client_trade_id: t.id || randomUUID(),
    opened_at: t.openedAt,
    closed_at: t.closedAt,
    symbol: t.symbol,
    strategy_id: t.strategyId,
    strategy_name: t.strategyName,
    side: t.side,
    entry_price: t.entryPrice,
    exit_price: t.exitPrice,
    contracts: t.contracts,
    notional: t.notional,
    margin_used: t.marginUsed,
    gross_pnl: t.realizedPnl,
    fees: t.fees,
    funding_costs: t.fundingCosts,
    net_pnl: t.netPnl,
    exit_reason: t.exitReason,
    payload: { ...t, migratedFromLocalStorage: true, migratedAt: new Date().toISOString() },
    created_at: new Date().toISOString(),
  };
}

const client = new MongoClient(uri, { serverSelectionTimeoutMS: 8000 });
try {
  await client.connect();
  const col = client.db(dbName).collection("paper_trades");
  await Promise.all([
    col.createIndex({ client_trade_id: 1 }, { unique: true, name: "uniq_client_trade_id" }),
    col.createIndex({ account_key: 1, closed_at: -1 }, { name: "by_account_closed" }),
    col.createIndex({ account_key: 1, strategy_id: 1, closed_at: -1 }, { name: "by_account_strat_closed" }),
  ]);
  const rows = ENGINE_STATE.trades.map(toRow);
  console.log("Migrating " + rows.length + " trades under account_key=" + ACCOUNT_KEY);
  let inserted = 0, skipped = 0;
  for (const row of rows) {
    const r = await col.updateOne(
      { client_trade_id: row.client_trade_id },
      { $setOnInsert: row },
      { upsert: true },
    );
    if (r.upsertedCount > 0) inserted++; else skipped++;
  }
  console.log("Inserted:", inserted, "| Skipped (already existed):", skipped);
  const totalForAccount = await col.countDocuments({ account_key: ACCOUNT_KEY });
  console.log("Total docs for", ACCOUNT_KEY + ":", totalForAccount);
  const sample = await col.find({ account_key: ACCOUNT_KEY }).sort({ closed_at: -1 }).limit(1).toArray();
  console.log("Most recent migrated row:");
  console.log(JSON.stringify(sample[0], null, 2));
} catch (e) {
  console.error("FAIL:", e.message);
  process.exit(1);
} finally {
  await client.close();
}
