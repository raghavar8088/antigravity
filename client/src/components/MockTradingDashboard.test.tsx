/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import {
  applyPriceTickToTrade,
  buildMockTradeFromResearchSignal,
  computeAccountState,
  computeAnalytics,
  DEFAULT_MOCK_TRADING_CONFIG,
  type MockTrade,
} from "@/lib/mockTradingEngine";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const mockUseMockTradingEngine = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

vi.mock("@/hooks/useLiveBTCPrice", () => ({
  default: () => ({
    price: 60_000,
    connected: true,
    change24h: 0,
    high24h: 61_000,
    low24h: 59_000,
    ticksPerSecond: 1,
  }),
}));

vi.mock("@/hooks/useMockCandleBuilder", () => ({
  useMockCandleBuilder: () => ({
    snapshot: [],
    newCandleReady: false,
    closedCount: 0,
  }),
}));

vi.mock("@/hooks/useMockResearchRunner", () => ({
  useMockResearchRunner: () => ({
    config: {
      enabled: false,
      maxSignalsPerMinute: 1,
      minConfidence: 0,
      enabledFamilies: new Set(),
    },
    setConfig: vi.fn(),
    strategies: [],
    familyLabels: {},
    lastEvalCount: 0,
    lastSignalCount: 0,
    lastEvalAt: null,
    hasRun: false,
  }),
}));

vi.mock("@/hooks/useMockTradingEngine", () => ({
  useMockTradingEngine: () => mockUseMockTradingEngine(),
}));

import MockTradingDashboard, { MOCK_TRADE_TABLE_REQUIRED_HEADERS } from "@/components/MockTradingDashboard";

afterEach(() => {
  vi.clearAllMocks();
  document.body.innerHTML = "";
});

function fmtUsd(value: number, digits = 2): string {
  const sign = value < 0 ? "-" : "";
  return `${sign}$${Math.abs(value).toLocaleString("en-US", {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })}`;
}

function mockEngine(trades: MockTrade[] = []) {
  return {
    trades,
    historyTrades: [],
    analytics: computeAnalytics(trades),
    account: computeAccountState(trades, DEFAULT_MOCK_TRADING_CONFIG),
    logs: [],
    config: DEFAULT_MOCK_TRADING_CONFIG,
    setConfig: vi.fn(),
    ingestTraceRows: vi.fn(),
    ingestResearchSignals: vi.fn(),
    closeTrade: vi.fn(),
    reset: vi.fn(),
    loading: false,
    error: null,
    traceAgeSeconds: null,
    mockLimitRejectedSignals: 0,
    persistence: {
      status: "fallback",
      loading: false,
      error: null,
      lastHydratedAt: null,
      lastSavedAt: null,
    },
    history: {
      page: 1,
      limit: 100,
      total: 0,
      totalPages: 1,
      loading: false,
      error: null,
      setPage: vi.fn(),
    },
  };
}

async function renderDashboard(trades: MockTrade[] = []): Promise<{ root: Root; container: HTMLDivElement }> {
  mockUseMockTradingEngine.mockReturnValue(mockEngine(trades));
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => {
    root.render(<MockTradingDashboard />);
  });
  return { root, container };
}

describe("MockTradingDashboard trade table", () => {
  it("includes TP/SL outcome columns without TP/SL price columns", () => {
    expect([...MOCK_TRADE_TABLE_REQUIRED_HEADERS]).toEqual([
      "Current PnL",
      "TP Profit $",
      "SL Loss $",
      "Risk/Reward",
      "Exit Reason",
    ]);
    expect(MOCK_TRADE_TABLE_REQUIRED_HEADERS).not.toContain("TP Price");
    expect(MOCK_TRADE_TABLE_REQUIRED_HEADERS).not.toContain("SL Price");
  });

  it("shows realized PnL for a closed trade", async () => {
    const open = buildMockTradeFromResearchSignal({
      signal: {
        strategyId: 1000,
        strategyName: "Research Trend Long",
        strategyFamily: "TrendFollowing",
        side: "BUY",
        confidenceScore: 80,
        params: { fast: 5, slow: 20 },
        evaluatedAt: 1_700_000_000_000,
      },
      currentPrice: 60_000,
      config: DEFAULT_MOCK_TRADING_CONFIG,
      now: 1_700_000_000_000,
    });
    if (!open) throw new Error("expected open trade");
    const closed = applyPriceTickToTrade({
      trade: open,
      price: open.takeProfitPrice,
      now: 1_700_000_060_000,
      config: DEFAULT_MOCK_TRADING_CONFIG,
    });
    const { root, container } = await renderDashboard([closed]);

    expect(container.textContent).toContain("Current PnL");
    expect(container.textContent).toContain("TP Profit $");
    expect(container.textContent).toContain("SL Loss $");
    expect(container.textContent).not.toContain("TP Price");
    expect(container.textContent).not.toContain("SL Price");
    expect(container.textContent).toContain(fmtUsd(closed.realizedPnl));
    expect(container.textContent).toContain("TAKE_PROFIT");
    await act(async () => {
      root.unmount();
    });
  });

  it("clear filters resets the age filter controls", async () => {
    const { root, container } = await renderDashboard();
    const ageSelect = container.querySelector("select[aria-label='Age filter']") as HTMLSelectElement | null;
    expect(ageSelect).not.toBeNull();

    await act(async () => {
      ageSelect!.value = "less";
      ageSelect!.dispatchEvent(new Event("change", { bubbles: true }));
    });
    const ageInput = container.querySelector("input[aria-label='Age less than minutes']") as HTMLInputElement | null;
    expect(ageInput).not.toBeNull();

    await act(async () => {
      ageInput!.value = "10";
      ageInput!.dispatchEvent(new Event("input", { bubbles: true }));
    });
    const clear = [...container.querySelectorAll("button")].find((button) => button.textContent === "Clear filters");
    expect(clear).not.toBeUndefined();

    await act(async () => {
      clear!.click();
    });

    expect(ageSelect!.value).toBe("");
    expect(container.querySelector("input[aria-label='Age less than minutes']")).toBeNull();
    await act(async () => {
      root.unmount();
    });
  });
});
