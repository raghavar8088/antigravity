// Phase 3 — Paper Desk API validation via local Next.js (if running) + direct MongoDB proof
// Also Phase 4-8 via MongoDB evidence
import { MongoClient, ObjectId } from 'mongodb';

const URI = 'mongodb+srv://raghavar8088_db_user:4e4qCOEoIPSF9Q6a@loop-trades.92ocjf9.mongodb.net/?retryWrites=true&w=majority&appName=LOOP-trades';
const DB_NAME = 'loop_trades';
const NOW = new Date();

async function main() {
  const client = new MongoClient(URI, { serverSelectionTimeoutMS: 10000 });
  await client.connect();
  const db = client.db(DB_NAME);

  // ============================================================
  // 3A. Identify what execution source is writing data
  // ============================================================
  console.log('='.repeat(70));
  console.log('PHASE 3 — EXECUTION SOURCE IDENTIFICATION');
  console.log('='.repeat(70));

  // Check paper_state worker_owner — who is claiming ownership?
  const stateCol = db.collection('paper_state');
  const stateDocs = await stateCol.find({}).toArray();

  console.log('\n[paper_state] Worker ownership:');
  for (const d of stateDocs) {
    const age = Math.round((NOW - new Date(d.updated_at)) / 1000);
    console.log(`  account_key: ${d.account_key}`);
    console.log(`  worker_id: ${d.worker_id}`);
    console.log(`  worker_owner: ${d.worker_owner}`);
    console.log(`  worker_last_poll_at: ${d.worker_last_poll_at}`);
    console.log(`  last_updated: ${d.updated_at} (${age}s ago)`);
    console.log(`  balance: ${d.balance}`);
    console.log(`  positions: ${JSON.stringify(d.positions)?.substring(0, 200)}`);
    console.log(`  pause_entries: ${d.pause_entries}`);
    console.log('');
  }

  // ============================================================
  // 3B. Check paper_oms_orders — are these Go engine orders?
  // ============================================================
  console.log('='.repeat(70));
  console.log('PHASE 3B — paper_oms_orders DEEP INSPECT');
  console.log('='.repeat(70));

  const omsCol = db.collection('paper_oms_orders');
  const omsCount = await omsCol.countDocuments();

  // Get oldest and newest
  const oldest = await omsCol.findOne({}, { sort: { _id: 1 } });
  const newest = await omsCol.findOne({}, { sort: { _id: -1 } });

  console.log(`\nTotal orders: ${omsCount}`);
  if (oldest) {
    const oldestTs = oldest.created_at || oldest.updated_at || oldest.timestamp;
    console.log(`Oldest order: ${oldest._id} | ts=${oldestTs} | status=${oldest.status}`);
  }
  if (newest) {
    const newestTs = newest.created_at || newest.updated_at || newest.timestamp;
    const age = newestTs ? Math.round((NOW - new Date(newestTs)) / 1000) : 'N/A';
    console.log(`Newest order: ${newest._id} | ts=${newestTs} | age=${age}s | status=${newest.status}`);
    console.log(`  Full newest doc: ${JSON.stringify(newest).substring(0, 500)}`);
  }

  // Count by status
  const statuses = await omsCol.aggregate([{ $group: { _id: '$status', count: { $sum: 1 } } }]).toArray();
  console.log('\nOrder status breakdown:');
  statuses.forEach(s => console.log(`  ${s._id || 'null'}: ${s.count}`));

  // Count by source
  const sources = await omsCol.aggregate([{ $group: { _id: '$source', count: { $sum: 1 } } }]).toArray();
  console.log('\nOrder source breakdown:');
  sources.forEach(s => console.log(`  ${s._id || 'null'}: ${s.count}`));

  // ============================================================
  // Phase 4/5 — Recovery evidence: check if Go engine is restoring state
  // ============================================================
  console.log('\n' + '='.repeat(70));
  console.log('PHASE 4/5 — RECOVERY EVIDENCE (MongoDB time series)');
  console.log('='.repeat(70));

  // equity_curve — is it writing at expected intervals?
  const eqCol = db.collection('equity_curve');
  const eqAll = await eqCol.find({}).sort({ _id: -1 }).limit(20).toArray();
  console.log('\nequity_curve — last 20 entries (time gaps):');
  let prev = null;
  for (const d of eqAll) {
    const ts = typeof d.timestamp === 'number' ? new Date(d.timestamp) : new Date(d.timestamp || d.created_at);
    const age = Math.round((NOW - ts) / 1000);
    const gap = prev ? Math.round((prev - ts) / 1000) : 0;
    console.log(`  ${ts.toISOString()} | age=${age}s | gap=${gap}s | equity=${d.equity?.toFixed(2)} | acct=${d.account_key}`);
    prev = ts;
  }

  // mock_account_snapshots — are these still being written (browser active)?
  const masCol = db.collection('mock_account_snapshots');
  const masAll = await masCol.find({}).sort({ _id: -1 }).limit(10).toArray();
  console.log('\nmock_account_snapshots — last 10 (60s interval check):');
  let prevMas = null;
  for (const d of masAll) {
    const ts = new Date(d.created_at || d.updated_at || d.timestamp);
    const age = Math.round((NOW - ts) / 1000);
    const gap = prevMas ? Math.round((prevMas - ts) / 1000) : 0;
    console.log(`  ${ts.toISOString()} | age=${age}s | gap=${gap}s | acct=${d.account_key}`);
    prevMas = ts;
  }

  // ============================================================
  // Phase 7 — Browser independence: time-span analysis
  // ============================================================
  console.log('\n' + '='.repeat(70));
  console.log('PHASE 7 — BROWSER INDEPENDENCE (time span analysis)');
  console.log('='.repeat(70));

  // How far back does equity_curve go?
  const eqOldest = await eqCol.findOne({}, { sort: { _id: 1 } });
  const eqNewest = await eqCol.findOne({}, { sort: { _id: -1 } });
  const eqOldTs = eqOldest ? (typeof eqOldest.timestamp === 'number' ? new Date(eqOldest.timestamp) : new Date(eqOldest.timestamp || eqOldest.created_at)) : null;
  const eqNewTs = eqNewest ? (typeof eqNewest.timestamp === 'number' ? new Date(eqNewest.timestamp) : new Date(eqNewest.timestamp || eqNewest.created_at)) : null;
  if (eqOldTs && eqNewTs) {
    const spanHours = ((eqNewTs - eqOldTs) / 3600000).toFixed(1);
    console.log(`\nequity_curve spans ${spanHours} hours: ${eqOldTs.toISOString()} → ${eqNewTs.toISOString()}`);
    console.log(`Total equity_curve docs: ${await eqCol.countDocuments()}`);
    console.log(`Expected rate: 1/60s → expected docs for ${spanHours}h = ${Math.round(parseFloat(spanHours) * 60)}`);
  }

  // mock_account_snapshots oldest
  const masOldest = await masCol.findOne({}, { sort: { _id: 1 } });
  const masNewest = await masCol.findOne({}, { sort: { _id: -1 } });
  if (masOldest && masNewest) {
    const masOldTs = new Date(masOldest.created_at || masOldest.updated_at || masOldest.timestamp);
    const masNewTs = new Date(masNewest.created_at || masNewest.updated_at || masNewest.timestamp);
    const spanHours = ((masNewTs - masOldTs) / 3600000).toFixed(1);
    console.log(`\nmock_account_snapshots spans ${spanHours}h: ${masOldTs.toISOString()} → ${masNewTs.toISOString()}`);
    console.log(`Total docs: ${await masCol.countDocuments()}`);
  }

  // strategy_signals — when were they generated?
  const sigCol = db.collection('strategy_signals');
  const sigNewest = await sigCol.findOne({}, { sort: { _id: -1 } });
  const sigOldest = await sigCol.findOne({}, { sort: { _id: 1 } });
  if (sigNewest) {
    const ts = new Date(sigNewest.created_at || sigNewest.timestamp);
    const age = Math.round((NOW - ts) / 1000);
    console.log(`\nstrategy_signals newest: age=${age}s | ts=${ts.toISOString()}`);
    console.log(`  Full doc: ${JSON.stringify(sigNewest).substring(0, 300)}`);
  }

  // mock_trades — are new trades being opened/closed still?
  const mtCol = db.collection('mock_trades');
  const mtCount = await mtCol.countDocuments();
  const mtOpen = await mtCol.countDocuments({ status: 'OPEN' });
  const mtClosed = await mtCol.countDocuments({ status: 'CLOSED' });
  const mtNewest = await mtCol.findOne({}, { sort: { _id: -1 } });
  const mtTs = mtNewest ? new Date(mtNewest.updated_at || mtNewest.created_at || mtNewest.timestamp) : null;
  const mtAge = mtTs ? Math.round((NOW - mtTs) / 1000) : 'N/A';
  console.log(`\nmock_trades: total=${mtCount} open=${mtOpen} closed=${mtClosed} newest_age=${mtAge}s`);
  if (mtNewest) {
    console.log(`  newest: ${JSON.stringify(mtNewest).substring(0, 300)}`);
  }

  await client.close();
  console.log('\n[DONE]');
}

main().catch(err => { console.error('[ERROR]', err.message); process.exit(1); });
