module.exports = {
  apps: [
    {
      name: "mock-trading-executor",
      script: "npx",
      args: "tsx --env-file=.env.local scripts/mock-trading-executor-worker.ts",
      cwd: __dirname,
      instances: 1,
      exec_mode: "fork",
      autorestart: true,
      max_restarts: 10,
      min_uptime: "10s",
      env: {
        NODE_ENV: "production",
        MOCK_TRADING_EXECUTOR_INTERVAL_MS: "5000",
        MOCK_TRADING_ACCOUNT_KEY: "mock_trading_main",
        MOCK_TRADING_SYMBOL: "BTCUSD",
      },
      error_file: "logs/mock-trading-executor-error.log",
      out_file: "logs/mock-trading-executor-out.log",
      merge_logs: true,
      time: true,
    },
  ],
};
