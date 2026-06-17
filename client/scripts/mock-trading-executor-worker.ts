/**
 * Long-running mock-trading executor worker.
 * Run on AWS Lightsail via PM2: pm2 start ecosystem.config.js --only mock-trading-executor
 */
import { runMockTradingExecutorCycle } from "../src/lib/mockTradingExecutor/runMockTradingExecutorCycle";
import { DEFAULT_MOCK_ACCOUNT_KEY } from "../src/lib/trading/mockTradingPersistenceTypes";

const accountKey = process.env.MOCK_TRADING_ACCOUNT_KEY?.trim() || DEFAULT_MOCK_ACCOUNT_KEY;
const intervalMs = Math.max(2_000, Number(process.env.MOCK_TRADING_EXECUTOR_INTERVAL_MS ?? 5_000));
const symbol = process.env.MOCK_TRADING_SYMBOL?.trim() || "BTCUSD";

let running = true;

process.on("SIGINT", () => {
  running = false;
  console.log("[mock-trading-executor] SIGINT — stopping");
});
process.on("SIGTERM", () => {
  running = false;
  console.log("[mock-trading-executor] SIGTERM — stopping");
});

async function tick(): Promise<void> {
  const result = await runMockTradingExecutorCycle({
    accountKey,
    symbol,
    mode: "worker",
  });
  console.log(
    `[mock-trading-executor] tick ok=${result.ok} opened=${result.opened.length} closed=${result.closed.length} candidates=${result.diagnostics.candidateCount} blocker=${result.diagnostics.dominantBlocker} duration=${result.diagnostics.durationMs}ms`,
  );
  if (result.diagnostics.errors.length > 0) {
    console.warn("[mock-trading-executor] errors:", result.diagnostics.errors.join(" | "));
  }
}

async function main(): Promise<void> {
  console.log(
    `[mock-trading-executor] starting account=${accountKey} symbol=${symbol} intervalMs=${intervalMs}`,
  );
  while (running) {
    const started = Date.now();
    try {
      await tick();
    } catch (err) {
      console.error("[mock-trading-executor] fatal tick error:", err);
    }
    const elapsed = Date.now() - started;
    const sleep = Math.max(0, intervalMs - elapsed);
    if (sleep > 0) await new Promise((r) => setTimeout(r, sleep));
  }
}

main().catch((err) => {
  console.error("[mock-trading-executor] startup failed:", err);
  process.exit(1);
});
