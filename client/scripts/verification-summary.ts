/**
 * CLI: npm run ai:verification
 * Paste-ready summary for Cursor AI.
 */

import { listVerificationEvents } from "../src/lib/verificationTrack/verificationTrackMongo";

async function main() {
  const events = await listVerificationEvents({ limit: 100, sinceMs: Date.now() - 2 * 3600_000 });

  const root = events.find(e => e.type === "NO_TRADE_ROOT_CAUSE");
  const funnel = events.find(e => e.type === "ENTRY_FUNNEL");
  const worker = events.find(e => e.type === "WORKER_HEALTH");
  const opened = events.filter(e => e.type === "POSITION_OPENED").slice(0, 3);
  const closed = events.filter(e => e.type === "POSITION_CLOSED").slice(0, 3);

  console.log("=== BTC Paper Desk Verification Summary (last 2h) ===");
  console.log(`Latest root cause: ${root?.summary ?? "none"}`);
  console.log(`Latest blocker: ${funnel?.dominant_blocker ?? "none"}`);
  console.log(`Worker: ${worker?.summary ?? "unknown"}`);
  console.log(`Opened: ${opened.length}  Closed: ${closed.length}`);
  console.log("");
  console.log("Recent opens:");
  opened.forEach(e => console.log(`  ${e.strategy_name} ${e.side} score=${e.signal_score?.toFixed(1)}`));
  console.log("Recent closes:");
  closed.forEach(e => console.log(`  ${e.strategy_name} PnL=${e.net_pnl?.toFixed(2)} reason=${e.exit_reason}`));
  console.log("");
  console.log("Recommended files for Cursor:");
  console.log("  - runPaperDeskPollTick.ts");
  console.log("  - useBTCFuturesScalperEngine.ts");
  console.log("  - noTradeRootCause.ts");
  console.log("  - verificationTrack/*");
  console.log("  - /api/verification-track/ai-context");
}

main().catch(console.error);
