// Purge Mock Trading telemetry older than KEEP_DAYS.
//
// Why this exists: these three collections grew 442 MB in 14 days (~31 MB/day)
// and filled the Atlas 512 MB quota, which BLOCKED WRITES and silently killed 22
// real live orders — the ledger write failed, so the order aborted. The trading
// logic was never involved.
//
// Only the three hard-coded mock_* log collections are touched. The execution
// ledger (ledger_events / ledger_sequences), paper state, equity curve and every
// real-money record are never referenced here. No pattern matching, so nothing
// can be swept up by accident.
//
// Note on disk: deleting documents does NOT return space to the OS on WiredTiger,
// and `compact` is unavailable on Atlas free/shared tiers. Running daily keeps
// each collection to ~2 days of data, so its files stop growing and settle at a
// bounded high-water mark. To actually shrink the files, drop the collection —
// the app recreates it on next write.
var d = db.getSiblingDB("loop_trades");

var KEEP_DAYS = 2;
var cutoff = Date.now() - KEEP_DAYS * 86400 * 1000;

var targets = [
  { c: "mock_execution_logs", f: "timestamp" },
  { c: "mock_trade_logs", f: "ts" },
  { c: "mock_account_snapshots", f: "timestamp" }
];

function stamp() { return new Date().toISOString(); }

var s = d.stats();
print("[" + stamp() + "] purge start — dataMB=" + (s.dataSize / 1048576).toFixed(1) +
  " diskMB=" + ((s.storageSize + s.indexSize) / 1048576).toFixed(1));

targets.forEach(function (t) {
  var q = {}; q[t.f] = { $lt: cutoff };
  var total = 0;
  // Batch so a single op never runs unbounded against the quota.
  while (true) {
    var ids = d[t.c].find(q, { _id: 1 }).limit(50000).toArray().map(function (x) { return x._id; });
    if (!ids.length) break;
    total += d[t.c].deleteMany({ _id: { $in: ids } }).deletedCount;
  }
  print("[" + stamp() + "]   " + t.c.padEnd(24) + " removed " + String(total).padStart(8) +
    "  remaining " + d[t.c].countDocuments({}));
});

var a = d.stats();
print("[" + stamp() + "] purge done  — dataMB=" + (a.dataSize / 1048576).toFixed(1) +
  " diskMB=" + ((a.storageSize + a.indexSize) / 1048576).toFixed(1) +
  "  ledger_events=" + d.ledger_events.countDocuments({}) +
  " ledger_sequences=" + d.ledger_sequences.countDocuments({}));
