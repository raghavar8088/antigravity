// Phase 7 MongoDB analytics cache hardening.
// Run with:
//   mongosh "$MONGODB_URI" infrastructure/database/mongo/analytics-indexes.js
//
// MongoDB is intentionally a cache/read-model layer only. PostgreSQL/TimescaleDB
// remain the source of truth for trading, market, audit, and research records.

const dbName = process.env.MONGODB_DB || "loop_trades";
const database = db.getSiblingDB(dbName);

const ttl30d = 30 * 24 * 60 * 60;
const ttl90d = 90 * 24 * 60 * 60;

database.paper_trades.createIndex({ strategy_id: 1, closed_at: -1 }, { name: "paper_trades_strategy_timestamp_desc" });
database.paper_trades.createIndex({ strategy_id: 1, status: 1, closed_at: -1 }, { name: "paper_trades_strategy_status_timestamp_desc" });
database.paper_trades.createIndex({ closed_at: -1 }, { name: "paper_trades_timestamp_desc" });
database.paper_trades.createIndex({ account_key: 1, closed_at: -1 }, { name: "paper_trades_account_timestamp_desc" });

database.dashboard_metrics.createIndex({ account_key: 1, metric_at: -1 }, { name: "dashboard_metrics_account_time_desc" });
database.dashboard_metrics.createIndex({ metric_at: 1 }, { name: "dashboard_metrics_ttl_30d", expireAfterSeconds: ttl30d });

database.strategy_leaderboards.createIndex({ account_key: 1, leaderboard_date: -1, rank: 1 }, { name: "strategy_leaderboards_account_date_rank" });
database.strategy_leaderboards.createIndex({ leaderboard_date: 1 }, { name: "strategy_leaderboards_ttl_90d", expireAfterSeconds: ttl90d });

database.research_summaries.createIndex({ strategy_id: 1, generated_at: -1 }, { name: "research_summaries_strategy_time_desc" });
database.research_summaries.createIndex({ generated_at: 1 }, { name: "research_summaries_ttl_90d", expireAfterSeconds: ttl90d });

database.analytics_views.createIndex({ view_name: 1, account_key: 1, generated_at: -1 }, { name: "analytics_views_name_account_time" });
database.analytics_views.createIndex({ generated_at: 1 }, { name: "analytics_views_ttl_30d", expireAfterSeconds: ttl30d });

database.ai_summaries.createIndex({ provider: 1, model: 1, generated_at: -1 }, { name: "ai_summaries_provider_model_time" });
database.ai_summaries.createIndex({ generated_at: 1 }, { name: "ai_summaries_ttl_90d", expireAfterSeconds: ttl90d });

database.precomputed_reports.createIndex({ report_type: 1, account_key: 1, generated_at: -1 }, { name: "precomputed_reports_type_account_time" });
database.precomputed_reports.createIndex({ generated_at: 1 }, { name: "precomputed_reports_ttl_90d", expireAfterSeconds: ttl90d });

database.logs.createIndex({ created_at: 1 }, { name: "logs_ttl_30d", expireAfterSeconds: ttl30d });
database.account_snapshots.createIndex({ account_key: 1, snapshot_at: -1 }, { name: "account_snapshots_account_time_desc" });

print(`[phase7] Mongo analytics indexes and TTL policies applied to ${dbName}`);
