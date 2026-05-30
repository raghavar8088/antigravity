/**
 * OMS integration tests for runPaperDeskPollTick.
 *
 * Verifies that the tick function creates OMS orders at the correct lifecycle
 * stages without changing any trading logic. Tests use a mocked fetch to avoid
 * real Delta API calls.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { PaperDeskWorkerContext, WorkerPosition } from "./runPaperDeskPollTick";

// ── Mock fetch (Delta Exchange calls) ─────────────────────────────────────────

const MARK_PRICE = 65000;

function makeDeltaBar(close = MARK_PRICE) {
  return { time: Math.floor(Date.now() / 1000), open: close, high: close * 1.001, low: close * 0.999, close, volume: 100 };
}

function mockSuccessfulFetch(params: { numBars?: number; fundingRate?: number } = {}) {
  const bars = Array.from({ length: params.numBars ?? 60 }, (_, i) =>
    makeDeltaBar(MARK_PRICE + i * 0.1),
  );
  const candlesResult = { success: true, result: bars };
  const tickerResult = {
    success: true,
    result: { mark_price: MARK_PRICE, funding_rate: params.fundingRate ?? 0, next_funding_time: 0 },
  };

  let call = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn().mockImplementation(() => {
      const data = call++ === 0 ? candlesResult : tickerResult;
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(data),
      });
    }),
  );
}

// ── Base context ──────────────────────────────────────────────────────────────

function makeCtx(overrides: Partial<PaperDeskWorkerContext> = {}): PaperDeskWorkerContext {
  return {
    accountKey: "test-account",
    storageNamespace: "test",
    baseUrl: "http://localhost:3000",
    balance: 10000,
    positions: [],
    trades: [],
    pauseEntries: false,
    clearedAt: 0,
    peakEquity: 10000,
    forceProbeOpened: false,
    mountedAt: Date.now(),
    stratCooldowns: {},
    signalThreshold: 999, // very high → no trades open by default
    initialBalance: 10000,
    moduleKey: "test",
    relaxConfirm: false,
    symbol: "BTCUSD",
    ...overrides,
  };
}

function makePosition(overrides: Partial<WorkerPosition> = {}): WorkerPosition {
  return {
    id: "test-pos",
    symbol: "BTCUSD",
    strategyId: 91,
    strategyName: "test-strat",
    templateFamily: "test",
    side: "LONG",
    entryPrice: 60000,
    contracts: 5,
    notional: 300,
    marginUsed: 12,
    slPrice: 59000,
    tpPrice: 61000,
    holdDeadlineMs: Date.now() + 99999,
    openedAt: new Date().toISOString(),
    liquidationPrice: 57600,
    fundingCosts: 0,
    lastFundingAppliedAt: Date.now(),
    ...overrides,
  };
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe("runPaperDeskPollTick — OMS order generation", () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
    vi.unstubAllEnvs();
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_PAPER_MIN_OPENS_PER_10M", "0");
  });

  it("returns empty omsOrders when no strategies pass signal threshold", async () => {
    mockSuccessfulFetch();
    const { runPaperDeskPollTick } = await import("./runPaperDeskPollTick");
    const result = await runPaperDeskPollTick(makeCtx({ signalThreshold: 999 }));
    // No signals fired → no OMS orders
    expect(result.omsOrders).toHaveLength(0);
    expect(result.omsPositionsClosed).toHaveLength(0);
  });

  it("returns empty omsOrders and omsPositionsClosed on data error", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("network error")));
    const { runPaperDeskPollTick } = await import("./runPaperDeskPollTick");
    const result = await runPaperDeskPollTick(makeCtx());
    expect(result.omsOrders).toHaveLength(0);
    expect(result.omsPositionsClosed).toHaveLength(0);
  });

  it("returns empty omsOrders when activeStrats is empty (no strategies)", async () => {
    const { runPaperDeskPollTick } = await import("./runPaperDeskPollTick");
    const result = await runPaperDeskPollTick(
      makeCtx({ strategyIdsExplicit: true, strategyIds: [999999] }), // non-existent ID
    );
    expect(result.omsOrders).toHaveLength(0);
    expect(result.omsPositionsClosed).toHaveLength(0);
  });

  it("omsPositionsClosed has entry when position with omsOrderId closes", async () => {
    mockSuccessfulFetch({ fundingRate: 0 });

    // Simulate a position that is already beyond TP price → will close this tick
    const { runPaperDeskPollTick } = await import("./runPaperDeskPollTick");
    const closingPos = makePosition({
      id: "pos-to-close",
      tpPrice: 61000, // mark price is 65000 → above TP → closes
      omsOrderId: "oms-order-abc", // linked OMS order
    });

    const result = await runPaperDeskPollTick(
      makeCtx({ positions: [closingPos], signalThreshold: 999 }),
    );

    const closeEvent = result.omsPositionsClosed.find((e) => e.orderId === "oms-order-abc");
    expect(closeEvent).toBeDefined();
    expect(closeEvent?.tradeId).toBe("pos-to-close");
    expect(closeEvent?.exitReason).toBe("TP");
  });

  it("positions without omsOrderId do not produce omsPositionsClosed entries", async () => {
    mockSuccessfulFetch({ fundingRate: 0 });

    const { runPaperDeskPollTick } = await import("./runPaperDeskPollTick");
    const legacyPos = makePosition({
      id: "legacy-pos",
      tpPrice: 61000, // closes at 65000 mark
      // No omsOrderId — legacy position
    });

    const result = await runPaperDeskPollTick(
      makeCtx({ positions: [legacyPos], signalThreshold: 999 }),
    );

    // Position closes but no OMS close event
    expect(result.closedTrades).toHaveLength(1);
    expect(result.omsPositionsClosed).toHaveLength(0);
  });

  it("releases reserved margin when a worker position closes", async () => {
    mockSuccessfulFetch({ fundingRate: 0 });

    const { runPaperDeskPollTick } = await import("./runPaperDeskPollTick");
    const closingPos = makePosition({
      id: "margin-release-pos",
      tpPrice: 61000,
    });

    const result = await runPaperDeskPollTick(
      makeCtx({ balance: 9988, positions: [closingPos], signalThreshold: 999 }),
    );

    const closed = result.closedTrades[0];
    expect(closed).toBeDefined();
    expect(result.balance).toBeCloseTo(9988 + closingPos.marginUsed + closed.netPnl, 9);
  });

  it("credits positive funding to shorts instead of charging both sides", async () => {
    mockSuccessfulFetch({ fundingRate: 0.0001 });

    const { runPaperDeskPollTick } = await import("./runPaperDeskPollTick");
    const openShort = makePosition({
      id: "short-funding-pos",
      side: "SHORT",
      slPrice: 70000,
      tpPrice: 50000,
      liquidationPrice: 80000,
      lastFundingAppliedAt: Date.now() - 8 * 60 * 60 * 1000,
    });

    const result = await runPaperDeskPollTick(
      makeCtx({ positions: [openShort], signalThreshold: 999 }),
    );

    expect(result.closedTrades).toHaveLength(0);
    expect(result.positions[0].fundingCosts).toBeLessThan(0);
    expect(result.positions[0].fundingCosts).toBeCloseTo(-openShort.notional * 0.0001, 6);
  });

  it("uses liquidation before SL for worker hard exits", async () => {
    mockSuccessfulFetch({ fundingRate: 0 });

    const { runPaperDeskPollTick } = await import("./runPaperDeskPollTick");
    const shortAtLiquidation = makePosition({
      id: "short-liq-pos",
      side: "SHORT",
      slPrice: 61000,
      tpPrice: 50000,
      liquidationPrice: 64000,
    });

    const result = await runPaperDeskPollTick(
      makeCtx({ positions: [shortAtLiquidation], signalThreshold: 999 }),
    );

    expect(result.closedTrades).toHaveLength(1);
    expect(result.closedTrades[0].exitReason).toBe("LIQUIDATION_RISK");
    expect(result.closedTrades[0].exitPrice).toBe(shortAtLiquidation.liquidationPrice);
  });

  it("does not create new TIME exits after hold deadline", async () => {
    mockSuccessfulFetch({ fundingRate: 0 });

    const { runPaperDeskPollTick } = await import("./runPaperDeskPollTick");
    const expiredButSafe = makePosition({
      id: "expired-safe-pos",
      slPrice: 50000,
      tpPrice: 80000,
      liquidationPrice: 45000,
      holdDeadlineMs: Date.now() - 60_000,
    });

    const result = await runPaperDeskPollTick(
      makeCtx({ positions: [expiredButSafe], signalThreshold: 999 }),
    );

    expect(result.closedTrades).toHaveLength(0);
    expect(result.positions).toHaveLength(1);
    expect(result.positions[0].id).toBe(expiredButSafe.id);
  });
});

// ── OMS order shape validation ────────────────────────────────────────────────

describe("runPaperDeskPollTick — OMS order shape", () => {
  it("tick result always includes omsOrders and omsPositionsClosed arrays", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("fail")));
    const { runPaperDeskPollTick } = await import("./runPaperDeskPollTick");
    const result = await runPaperDeskPollTick(makeCtx());
    expect(Array.isArray(result.omsOrders)).toBe(true);
    expect(Array.isArray(result.omsPositionsClosed)).toBe(true);
  });
});
