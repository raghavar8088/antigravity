/**
 * pm2 ecosystem config for the BTC futures paper desk worker.
 *
 * Usage:
 *   cd client
 *   pm2 start ecosystem.worker.config.cjs --name btc-ft-worker
 *   pm2 logs btc-ft-worker
 *   pm2 save          # persist across VPS reboots
 *   pm2 startup       # install systemd hook
 *
 * The worker runs continuously, polls Delta klines every POLL_MS,
 * executes exit/entry logic, and writes results to MongoDB.
 * The browser UI is optional (monitor mode only when worker is active).
 */

module.exports = {
  apps: [
    {
      name: "btc-ft-worker",
      script: "npx",
      args: "tsx scripts/btc-ft-paper-worker.ts",
      cwd: __dirname,
      interpreter: "none",
      instances: 1,          // single writer; lease prevents duplicates
      autorestart: true,
      watch: false,
      max_memory_restart: "256M",
      restart_delay: 5000,   // 5 s back-off on crash
      env: {
        NODE_ENV: "production",
        POLL_MS: "4000",
        // ── Copy exact values from your Vercel project env vars ──────────────
        // DESK_WORKER_ACCOUNT_KEY: "your-uuid-here",
        // MONGODB_URI: "mongodb+srv://...",
        // MONGODB_DB: "loop_trades",
        // DELTA_BTC_FUTURES_SYMBOL: "BTCUSD",
        // NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD: "26",
        // NEXT_PUBLIC_DESK_INITIAL_BALANCE_USD: "1000",
        // DESK_WORKER_NOTIONAL: "300",
        // DESK_WORKER_STORAGE_NAMESPACE: "btc_future_trading_v4",
        // NEXT_PUBLIC_BTC_FT_RELAX_CONFIRM: "0",
      },
      log_date_format: "YYYY-MM-DD HH:mm:ss Z",
      error_file: "logs/btc-ft-worker-error.log",
      out_file: "logs/btc-ft-worker-out.log",
      merge_logs: true,
    },
  ],
};
