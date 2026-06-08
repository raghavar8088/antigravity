// Phase 3 — Try paper desk API endpoints on localhost (if dev server running)
// and deep probe the anon VPS account vs mock_trading_default account
import { MongoClient } from 'mongodb';

const URI = 'mongodb+srv://raghavar8088_db_user:4e4qCOEoIPSF9Q6a@loop-trades.92ocjf9.mongodb.net/?retryWrites=true&w=majority&appName=LOOP-trades';
const DB_NAME = 'loop_trades';
const NOW = new Date();

// Try to hit Next.js API locally
async function tryLocalAPI(path) {
  try {
    const resp = await fetch(`http://localhost:3000${path}`, {
      signal: AbortSignal.timeout(5000),
      headers: { 'Content-Type': 'application/json' }
    });
    return { status: resp.status, ok: resp.ok, body: await resp.text() };
  } catch (e) {
    return { status: 0, ok: false, body: e.message };
  }
}

async function main() {
  const client = new MongoClient(URI, { serverSelectionTimeoutMS: 10000 });
  await client.connect();
  const db = client.db(DB_NAME);

  console.log('='.repeat(70));
  console.log('PHASE 3 — API ENDPOINT TESTS (localhost:3000)');
  console.log('='.repeat(70));

  const endpoints = [
    '/api/paper-desk/snapshot',
    '/api/paper-desk/positions',
    '/api/paper-desk/trades',
    '/api/paper-desk/orders',
    '/api/paper-desk/equity',
    '/api/paper-desk/strategy-health',
    '/api/paper-desk/diagnostics',
    '/api/mock-trading/account',
  ];

  for (const ep of endpoints) {
    const r = await tryLocalAPI(ep);
    const preview = r.body?.substring(0, 120).replace(/\n/g, ' ');
    console.log(`  ${r.status.toString().padStart(3)} ${ep}`);
    if (r.status !== 0) console.log(`    → ${preview}`);
  }

  console.log('\n' + '='.repeat(70));
  console.log('VPS WORKER ANALYSIS — anon vs mock_trading_default');
  console.log('='.repeat(70));

  const stateCol = db.collection('paper_state');

  // anon account — active worker
  const anonState = await stateCol.findOne({ account_key: { $regex: /^anon_/ } });
  if (anonState) {
    const workerPollAge = Math.round((NOW - new Date(anonState.worker_last_poll_at)) / 1000);
    const updatedAge = Math.round((NOW - new Date(anonState.updated_at)) / 1000);
    console.log('\nANON ACCOUNT (active VPS worker):');
    console.log(`  account_key: ${anonState.account_key}`);
    console.log(`  worker_id: ${anonState.worker_id}`);
    console.log(`  worker_owner: ${anonState.worker_owner}`);
    console.log(`  worker_last_poll_at age: ${workerPollAge}s`);
    console.log(`  updated_at age: ${updatedAge}s`);
    console.log(`  balance: ${anonState.balance}`);
    console.log(`  positions count: ${anonState.positions?.length}`);

    // Is this the PAPER THROUGHPUT PROBE or a real anon user?
    const sigTrace = anonState.signal_trace_latest;
    if (sigTrace) {
      console.log(`  signal_trace_latest: ${JSON.stringify(sigTrace).substring(0, 200)}`);
    }
    const entryFunnel = anonState.entry_funnel_snapshot;
    if (entryFunnel) {
      console.log(`  entry_funnel_snapshot: ${JSON.stringify(entryFunnel).substring(0, 200)}`);
    }
  }

  // mock_trading_default — stale
  const defaultState = await stateCol.findOne({ account_key: 'mock_trading_default' });
  if (defaultState) {
    const updatedAge = Math.round((NOW - new Date(defaultState.updated_at)) / 1000);
    const workerPollAge = defaultState.worker_last_poll_at
      ? Math.round((NOW - new Date(defaultState.worker_last_poll_at)) / 1000)
      : 'N/A';
    console.log('\nMOCK_TRADING_DEFAULT (stale / cron-backup):');
    console.log(`  worker_id: ${defaultState.worker_id}`);
    console.log(`  worker_owner: ${defaultState.worker_owner}`);
    console.log(`  worker_last_poll_at age: ${workerPollAge}s`);
    console.log(`  updated_at age: ${updatedAge}s`);
    console.log(`  balance: ${defaultState.balance}`);
    console.log(`  positions count: ${defaultState.positions?.length}`);
    const openPos = defaultState.positions?.filter(p => p.status === 'OPEN' || !p.closedAt);
    console.log(`  open positions: ${openPos?.length || 0}`);
    if (openPos?.length > 0) {
      console.log(`  first open pos: ${JSON.stringify(openPos[0]).substring(0, 300)}`);
    }
  }

  // Check equity_curve for anon account — is it logging separately?
  const eqCol = db.collection('equity_curve');
  const anonEq = await eqCol.countDocuments({ account_key: { $regex: /^anon_/ } });
  const defaultEq = await eqCol.countDocuments({ account_key: 'mock_trading_default' });
  console.log(`\nequity_curve: anon_* = ${anonEq} docs, mock_trading_default = ${defaultEq} docs`);

  // Check mock_trades — what kind of trades are in there?
  const mtCol = db.collection('mock_trades');
  const openTrades = await mtCol.find({ status: 'OPEN' }).toArray();
  console.log('\n[mock_trades] Open trades:');
  openTrades.forEach((t, i) => {
    const age = t.created_at ? Math.round((NOW - new Date(t.created_at)) / 1000) : 'N/A';
    console.log(`  [${i}] id=${t._id} | strat=${t.strategy_id || t.strategyId} | side=${t.side} | entry=${t.entry_price || t.entryPrice} | age=${age}s`);
    console.log(`       account_key=${t.account_key} | sl=${t.stop_loss || t.stopLoss} | tp=${t.take_profit || t.takeProfit}`);
  });

  const closedSample = await mtCol.find({ status: 'CLOSED' }).sort({ _id: -1 }).limit(3).toArray();
  console.log('\n[mock_trades] Recent closed trades:');
  closedSample.forEach((t, i) => {
    const ts = t.updated_at || t.closed_at || t.closedAt;
    const age = ts ? Math.round((NOW - new Date(ts)) / 1000) : 'N/A';
    console.log(`  [${i}] id=${t._id} | strat=${t.strategy_id || t.strategyId} | closed_age=${age}s | exit_reason=${t.exit_reason || t.exitReason}`);
  });

  // paper_oms_orders newest full doc to understand schema
  const omsCol = db.collection('paper_oms_orders');
  const omsNewest = await omsCol.findOne({}, { sort: { _id: -1 } });
  console.log('\n[paper_oms_orders] Newest full doc:');
  if (omsNewest) {
    console.log(JSON.stringify(omsNewest, null, 2).substring(0, 800));
  }

  // What are strategy_signals writing?
  const sigCol = db.collection('strategy_signals');
  const sigNewest = await sigCol.findOne({}, { sort: { _id: -1 } });
  console.log('\n[strategy_signals] Newest full doc:');
  if (sigNewest) {
    console.log(JSON.stringify(sigNewest, null, 2).substring(0, 600));
  }

  await client.close();
  console.log('\n[DONE]');
}

main().catch(err => { console.error('[ERROR]', err.message); process.exit(1); });
