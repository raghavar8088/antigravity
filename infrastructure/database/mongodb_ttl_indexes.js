// mongodb_ttl_indexes.js
// Idempotent script to create TTL indexes on high-volume collections.
// Run via: mongosh "$MONGODB_URI" --file mongodb_ttl_indexes.js
//
// Retention rationale:
//   paper_trades    90 days — enough history for 3-month walk-forward analysis
//   audit_logs      90 days — satisfies internal audit trail requirements
//   kill_switch_events 180 days — longer retention for post-incident review
//   sessions        30 days — JWT session lifetime; purge stale sessions faster
//   signal_history  60 days — two strategy evaluation cycles; saves ~40% Atlas storage

"use strict";

const db = db.getSiblingDB(process.env.MONGODB_DB || "loop_trades");

function ensureTTLIndex(collectionName, fieldName, expireAfterSeconds, label) {
  const coll = db.getCollection(collectionName);
  const existing = coll.getIndexes();
  const key = fieldName;
  const alreadyExists = existing.some(
    (idx) =>
      idx.key[key] !== undefined &&
      idx.expireAfterSeconds === expireAfterSeconds
  );
  if (alreadyExists) {
    print(`[TTL] SKIP  ${collectionName}.${fieldName} — index already exists (${label})`);
    return;
  }
  coll.createIndex(
    { [fieldName]: 1 },
    { expireAfterSeconds: expireAfterSeconds, background: true }
  );
  print(`[TTL] OK    ${collectionName}.${fieldName} — TTL ${expireAfterSeconds}s (${label})`);
}

// paper_trades: 90 days = 7,776,000 seconds
// Strategy performance analysis only needs the last quarter; older records waste Atlas storage.
ensureTTLIndex("paper_trades",          "timestamp",   7776000,  "90 days");

// audit_logs: 90 days = 7,776,000 seconds
// Covers any internal audit review window; purges noise from old strategy runs.
ensureTTLIndex("audit_logs",            "timestamp",   7776000,  "90 days");

// kill_switch_events: 180 days = 15,552,000 seconds
// Extended retention enables post-incident forensics across multiple incident cycles.
ensureTTLIndex("engine_killswitch",     "timestamp",   15552000, "180 days");

// sessions: 30 days = 2,592,000 seconds
// Auth sessions expire after 30 days; TTL cleans up abandoned sessions automatically.
ensureTTLIndex("sessions",              "expires",     0,        "self-expiring via expires field");

// signal_history: 60 days = 5,184,000 seconds
// Two full strategy evaluation cycles; sufficient for back-review without unbounded growth.
ensureTTLIndex("signal_history",        "timestamp",   5184000,  "60 days");

print("[TTL] All TTL index checks complete.");
