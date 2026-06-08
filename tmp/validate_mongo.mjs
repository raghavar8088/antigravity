// Live MongoDB validation script — Phase 2 of production validation
import { MongoClient } from 'mongodb';

const URI = 'mongodb+srv://raghavar8088_db_user:4e4qCOEoIPSF9Q6a@loop-trades.92ocjf9.mongodb.net/?retryWrites=true&w=majority&appName=LOOP-trades';
const DB_NAME = 'loop_trades';
const ACCOUNT_KEY = 'mock_trading_default';

const NOW = new Date();

const COLLECTIONS = [
  { name: 'paper_state',      maxAgeSeconds: 20,   critical: true },
  { name: 'paper_positions',  maxAgeSeconds: 3600, critical: true },
  { name: 'paper_orders',     maxAgeSeconds: 3600, critical: true },
  { name: 'paper_trades',     maxAgeSeconds: 86400, critical: false },
  { name: 'equity_curve',     maxAgeSeconds: 600,  critical: true },
  { name: 'strategy_health',  maxAgeSeconds: 1200, critical: true },
];

// Legacy collections to audit
const LEGACY_COLLECTIONS = ['mock_trades', 'mock_account_snapshots'];

async function main() {
  console.log('='.repeat(70));
  console.log('PHASE 2 — MONGODB LIVE WRITE VALIDATION');
  console.log(`Timestamp: ${NOW.toISOString()}`);
  console.log(`Account key: ${ACCOUNT_KEY}`);
  console.log('='.repeat(70));

  const client = new MongoClient(URI, { serverSelectionTimeoutMS: 10000, connectTimeoutMS: 10000 });

  try {
    await client.connect();
    console.log('\n[MONGO] Connected to Atlas ✓\n');
    const db = client.db(DB_NAME);

    // List all collections
    const allCols = await db.listCollections().toArray();
    console.log('[MONGO] Collections in loop_trades:');
    allCols.forEach(c => console.log(`  - ${c.name}`));
    console.log('');

    const results = [];

    for (const spec of COLLECTIONS) {
      process.stdout.write(`[CHECKING] ${spec.name}... `);
      const col = db.collection(spec.name);
      const count = await col.countDocuments();

      // Find newest doc — try several timestamp fields
      const newest = await col.findOne(
        {},
        { sort: { updated_at: -1, created_at: -1, timestamp: -1, _id: -1 } }
      );

      if (!newest) {
        console.log(`EMPTY (0 docs)`);
        results.push({ collection: spec.name, count: 0, ageSeconds: Infinity, status: 'EMPTY', critical: spec.critical });
        continue;
      }

      // Determine timestamp
      const tsField = newest.updated_at || newest.created_at || newest.timestamp;
      const tsDate = tsField ? new Date(tsField) : null;
      const ageSeconds = tsDate ? Math.round((NOW - tsDate) / 1000) : null;

      // Check account_key
      const akField = newest.account_key;

      // Sample IDs (last 3)
      const recentDocs = await col.find({}, { sort: { _id: -1 }, limit: 3, projection: { _id: 1, account_key: 1, source: 1, updated_at: 1, created_at: 1, timestamp: 1 } }).toArray();

      const status = ageSeconds === null ? 'NO_TIMESTAMP'
        : ageSeconds <= spec.maxAgeSeconds ? 'FRESH'
        : 'STALE';

      const flag = status === 'FRESH' ? '✓' : status === 'STALE' ? '⚠ STALE' : '? NO_TS';

      console.log(`${count} docs | newest: ${tsDate ? tsDate.toISOString() : 'N/A'} | age: ${ageSeconds}s | account_key: ${akField || 'N/A'} | ${flag}`);

      results.push({
        collection: spec.name,
        count,
        newestTimestamp: tsDate ? tsDate.toISOString() : null,
        ageSeconds,
        accountKey: akField,
        source: newest.source,
        recentIds: recentDocs.map(d => d._id.toString()),
        status,
        maxAgeSeconds: spec.maxAgeSeconds,
        critical: spec.critical,
      });
    }

    console.log('\n' + '='.repeat(70));
    console.log('PHASE 8 — LEGACY SYSTEM AUDIT');
    console.log('='.repeat(70));

    for (const colName of LEGACY_COLLECTIONS) {
      const col = db.collection(colName);
      const count = await col.countDocuments();
      const newest = count > 0 ? await col.findOne({}, { sort: { updated_at: -1, created_at: -1, _id: -1 } }) : null;
      const tsField = newest ? (newest.updated_at || newest.created_at || newest.timestamp) : null;
      const tsDate = tsField ? new Date(tsField) : null;
      const ageSeconds = tsDate ? Math.round((NOW - tsDate) / 1000) : null;

      console.log(`[LEGACY] ${colName}: ${count} docs | newest: ${tsDate ? tsDate.toISOString() : 'N/A'} | age: ${ageSeconds !== null ? ageSeconds + 's' : 'N/A'}`);
    }

    console.log('\n' + '='.repeat(70));
    console.log('SUMMARY');
    console.log('='.repeat(70));

    let score = 0;
    let maxScore = 0;
    for (const r of results) {
      maxScore += r.critical ? 20 : 10;
      if (r.status === 'FRESH') score += r.critical ? 20 : 10;
      else if (r.status === 'NO_TIMESTAMP' && r.count > 0) score += r.critical ? 10 : 5;

      const statusLabel = r.status === 'FRESH' ? '✓ FRESH' : r.status === 'EMPTY' ? '✗ EMPTY' : r.status === 'STALE' ? '⚠ STALE' : '? NO_TIMESTAMP';
      const ageStr = r.ageSeconds === Infinity ? '∞' : r.ageSeconds !== null ? r.ageSeconds + 's' : 'unknown';
      console.log(`  ${statusLabel.padEnd(15)} ${r.collection.padEnd(25)} count=${r.count} age=${ageStr} (threshold=${r.maxAgeSeconds}s)`);
    }

    console.log(`\nMongoDB Score: ${score}/${maxScore}`);
    return { results, score, maxScore };

  } finally {
    await client.close();
  }
}

main().catch(err => {
  console.error('[ERROR]', err.message);
  process.exit(1);
});
