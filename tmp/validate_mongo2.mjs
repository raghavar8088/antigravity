// Deep MongoDB inspection — check all active collections in detail
import { MongoClient } from 'mongodb';

const URI = 'mongodb+srv://raghavar8088_db_user:4e4qCOEoIPSF9Q6a@loop-trades.92ocjf9.mongodb.net/?retryWrites=true&w=majority&appName=LOOP-trades';
const DB_NAME = 'loop_trades';
const NOW = new Date();

async function main() {
  const client = new MongoClient(URI, { serverSelectionTimeoutMS: 10000 });
  await client.connect();
  const db = client.db(DB_NAME);

  console.log('='.repeat(70));
  console.log('DEEP INSPECTION — ACTIVE COLLECTIONS');
  console.log(`Time: ${NOW.toISOString()}`);
  console.log('='.repeat(70));

  // 1. paper_state — what account keys? what data?
  console.log('\n[1] paper_state — all documents:');
  const stateCol = db.collection('paper_state');
  const stateDocs = await stateCol.find({}).toArray();
  stateDocs.forEach((d, i) => {
    const ts = d.updated_at || d.created_at || d.timestamp;
    const age = ts ? Math.round((NOW - new Date(ts)) / 1000) : 'N/A';
    console.log(`  [${i}] account_key=${d.account_key} | source=${d.source} | updated=${ts} | age=${age}s`);
    console.log(`       balance=${d.balance} | btc_position=${d.btc_position} | pnl=${d.total_pnl}`);
    console.log(`       open_trades=${d.open_trades?.length ?? 'N/A'} | fields: ${Object.keys(d).join(',')}`);
  });

  // 2. equity_curve — sample recent
  console.log('\n[2] equity_curve — last 5 docs:');
  const eqCol = db.collection('equity_curve');
  const eqDocs = await eqCol.find({}).sort({ _id: -1 }).limit(5).toArray();
  eqDocs.forEach((d, i) => {
    const ts = d.timestamp || d.created_at || d.updated_at;
    const age = ts ? Math.round((NOW - new Date(ts)) / 1000) : 'N/A';
    console.log(`  [${i}] account_key=${d.account_key} | equity=${d.equity} | ts=${ts} | age=${age}s | source=${d.source}`);
  });

  // 3. mock_trades — last 5 docs (is browser writing here?)
  console.log('\n[3] mock_trades — last 5 docs (should be INACTIVE):');
  const mtCol = db.collection('mock_trades');
  const mtDocs = await mtCol.find({}).sort({ _id: -1 }).limit(5).toArray();
  mtDocs.forEach((d, i) => {
    const ts = d.updated_at || d.created_at || d.timestamp || d.closedAt || d.openedAt;
    const age = ts ? Math.round((NOW - new Date(ts)) / 1000) : 'N/A';
    console.log(`  [${i}] _id=${d._id} | account_key=${d.account_key} | status=${d.status} | ts=${ts} | age=${age}s`);
  });

  // 4. mock_account_snapshots — last 5
  console.log('\n[4] mock_account_snapshots — last 5 docs:');
  const masCol = db.collection('mock_account_snapshots');
  const masDocs = await masCol.find({}).sort({ _id: -1 }).limit(5).toArray();
  masDocs.forEach((d, i) => {
    const ts = d.updated_at || d.created_at || d.timestamp;
    const age = ts ? Math.round((NOW - new Date(ts)) / 1000) : 'N/A';
    console.log(`  [${i}] account_key=${d.account_key} | balance=${d.balance} | ts=${ts} | age=${age}s`);
  });

  // 5. strategy_scores — are they fresh?
  console.log('\n[5] strategy_scores — sample:');
  const ssCol = db.collection('strategy_scores');
  const ssCount = await ssCol.countDocuments();
  const ssRecent = await ssCol.find({}).sort({ _id: -1 }).limit(3).toArray();
  console.log(`  Total: ${ssCount}`);
  ssRecent.forEach((d, i) => {
    const ts = d.updated_at || d.created_at;
    const age = ts ? Math.round((NOW - new Date(ts)) / 1000) : 'N/A';
    console.log(`  [${i}] strategy_id=${d.strategy_id || d.id} | wins=${d.wins} | losses=${d.losses} | account_key=${d.account_key} | age=${age}s`);
  });

  // 6. paper_oms_orders — check if Go engine is writing
  console.log('\n[6] paper_oms_orders (Go engine writes):');
  const omsCol = db.collection('paper_oms_orders');
  const omsCount = await omsCol.countDocuments();
  const omsRecent = await omsCol.find({}).sort({ _id: -1 }).limit(5).toArray();
  console.log(`  Total: ${omsCount}`);
  omsRecent.forEach((d, i) => {
    const ts = d.updated_at || d.created_at;
    const age = ts ? Math.round((NOW - new Date(ts)) / 1000) : 'N/A';
    console.log(`  [${i}] order_id=${d.order_id || d._id} | account_key=${d.account_key} | status=${d.status} | source=${d.source} | age=${age}s`);
  });

  // 7. strategy_signals — is Go engine signaling?
  console.log('\n[7] strategy_signals (Go engine):');
  const sigCol = db.collection('strategy_signals');
  const sigCount = await sigCol.countDocuments();
  const sigRecent = await sigCol.find({}).sort({ _id: -1 }).limit(5).toArray();
  console.log(`  Total: ${sigCount}`);
  sigRecent.forEach((d, i) => {
    const ts = d.created_at || d.timestamp;
    const age = ts ? Math.round((NOW - new Date(ts)) / 1000) : 'N/A';
    console.log(`  [${i}] strategy=${d.strategy_id} | signal=${d.signal} | price=${d.price} | age=${age}s`);
  });

  // 8. desk_worker_events — is engine heartbeating?
  console.log('\n[8] desk_worker_events (engine heartbeat):');
  const dweCol = db.collection('desk_worker_events');
  const dweCount = await dweCol.countDocuments();
  const dweRecent = await dweCol.find({}).sort({ _id: -1 }).limit(5).toArray();
  console.log(`  Total: ${dweCount}`);
  dweRecent.forEach((d, i) => {
    const ts = d.created_at || d.timestamp;
    const age = ts ? Math.round((NOW - new Date(ts)) / 1000) : 'N/A';
    console.log(`  [${i}] event=${d.event || d.type} | worker=${d.worker_id || d.source} | age=${age}s`);
  });

  await client.close();
  console.log('\n[DONE]');
}

main().catch(err => { console.error('[ERROR]', err.message); process.exit(1); });
