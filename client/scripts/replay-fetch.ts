/**
 * Fetch Delta 1m candles → fixtures/replay/btcusd_1m_live.json
 *   npm run replay:fetch
 *   npm run replay:fetch -- --symbol=BTCUSD --bars=500
 */
import { mkdirSync, writeFileSync } from "node:fs";
import { fetchDeltaFutures1mCandles } from "../src/lib/futuresKlinesFetch";
import { REPLAY_FIXTURE_DIR, replayFixturePath } from "../src/lib/futuresReplayFixtures";
import { loadEnvLocal, parseArg } from "./replayCliShared";

loadEnvLocal();

const symbol = parseArg("symbol", "BTCUSD");
const bars = Number(parseArg("bars", "500"));

async function main() {
  const result = await fetchDeltaFutures1mCandles(symbol, bars);
  mkdirSync(REPLAY_FIXTURE_DIR, { recursive: true });
  const path = replayFixturePath("live");
  const payload = {
    symbol: result.symbol,
    barMs: 60_000,
    fetchedAt: result.fetchedAt,
    source: result.source,
    fundingRate: result.fundingRate,
    candles: result.candles,
  };
  writeFileSync(path, JSON.stringify(payload, null, 0));
  console.log(JSON.stringify({ ok: true, path, bars: result.candles.length, symbol: result.symbol }, null, 2));
}

main().catch((e) => {
  console.error(e instanceof Error ? e.message : e);
  process.exit(1);
});
