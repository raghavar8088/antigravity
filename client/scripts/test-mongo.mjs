// Quick connectivity test for MongoDB Atlas.
// Run from client/ dir:  node --env-file=.env.local scripts/test-mongo.mjs
import { MongoClient } from "mongodb";

const uri = process.env.MONGODB_URI;
const dbName = process.env.MONGODB_DB || "loop_trades";

if (!uri) {
  console.error("FAIL: MONGODB_URI not set. Check .env.local loaded.");
  process.exit(1);
}

console.log("URI host:", uri.replace(/:\/\/[^@]+@/, "://***:***@"));
console.log("DB name :", dbName);
console.log("Connecting...");

const client = new MongoClient(uri, { serverSelectionTimeoutMS: 8000 });
try {
  await client.connect();
  console.log("OK: connected");
  const db = client.db(dbName);
  const col = db.collection("paper_trades");
  const r = await col.insertOne({
    _probe: true,
    client_trade_id: "probe-" + Date.now(),
    note: "connectivity test — safe to delete",
    created_at: new Date().toISOString(),
  });
  console.log("OK: inserted _id =", r.insertedId.toString());
  const count = await col.countDocuments();
  console.log("Collection doc count:", count);
} catch (e) {
  console.error("FAIL:", e.message);
  if (e.message.includes("IP")) console.error(">> Your current IP isn't in Atlas Network Access allowlist.");
  if (e.message.includes("auth")) console.error(">> Username/password is wrong.");
  process.exit(1);
} finally {
  await client.close();
}
