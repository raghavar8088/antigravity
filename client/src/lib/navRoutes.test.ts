import { describe, expect, it } from "vitest";
import {
  COMMAND_CENTER_NAV,
  MOCK_TRADING_PATH,
  MONITOR_NAV,
  TERMINAL_ROUTES,
  TRADING_NAV,
  gradeStatusFromPath,
  isMockTradingRoute,
  isPaperDeskRoute,
  isPaperDeskTabKey,
  isTerminalRoute,
  legacyPaperDeskRedirect,
  paperDeskHref,
} from "./navRoutes";

describe("isPaperDeskRoute", () => {
  it("matches retired paper desk paths", () => {
    expect(isPaperDeskRoute("/paper-desk")).toBe(true);
    expect(isPaperDeskRoute("/paperdesk")).toBe(true);
    expect(isPaperDeskRoute("/btc-future-trading")).toBe(true);
  });

  it("does not match mock trading or terminal", () => {
    expect(isPaperDeskRoute("/")).toBe(false);
    expect(isPaperDeskRoute("/mock-trading")).toBe(false);
    expect(isPaperDeskRoute("/terminal")).toBe(false);
  });
});

describe("isMockTradingRoute", () => {
  it("matches mock trading paths", () => {
    expect(isMockTradingRoute("/mock-trading")).toBe(true);
    expect(isMockTradingRoute("/mock-trading/history")).toBe(true);
  });
});

describe("legacyPaperDeskRedirect", () => {
  it("redirects all legacy tabs to mock trading", () => {
    expect(legacyPaperDeskRedirect()).toBe(MOCK_TRADING_PATH);
    expect(legacyPaperDeskRedirect("positions")).toBe(MOCK_TRADING_PATH);
    expect(legacyPaperDeskRedirect("trades")).toBe(MOCK_TRADING_PATH);
    expect(legacyPaperDeskRedirect("invalid")).toBe(MOCK_TRADING_PATH);
  });
});

describe("paperDeskHref", () => {
  it("returns mock trading path", () => {
    expect(paperDeskHref()).toBe(MOCK_TRADING_PATH);
    expect(paperDeskHref("equity")).toBe(MOCK_TRADING_PATH);
  });
});

describe("ICC-NRP navigation registry", () => {
  it("lists mock engine first in trading section", () => {
    expect(TRADING_NAV[0]?.href).toBe(TERMINAL_ROUTES["mock-engine"]);
    expect(TRADING_NAV[0]?.label).toBe("Mock Trading Engine");
  });

  it("has six trading pipeline items in mandatory order", () => {
    expect(TRADING_NAV.map((i) => i.label)).toEqual([
      "Mock Trading Engine",
      "Mock Trading Grade 1",
      "Mock Trading Grade 2",
      "Mock Trading Grade 3",
      "Mock Trading Grade 4",
      "Mock Trading Grade 5",
    ]);
  });

  it("has thirteen monitor items in mandatory order", () => {
    expect(MONITOR_NAV.map((i) => i.label)).toEqual([
      "Command Center",
      "Execution",
      "Strategy Authority",
      "Portfolio Intelligence",
      "Strategies",
      "Portfolio",
      "Risk",
      "Analytics",
      "Research",
      "Events",
      "Health",
      "Diagnostics",
      "Settings",
    ]);
  });

  it("concatenates trading then monitor in COMMAND_CENTER_NAV", () => {
    expect(COMMAND_CENTER_NAV.length).toBe(TRADING_NAV.length + MONITOR_NAV.length);
    expect(COMMAND_CENTER_NAV[0]?.section).toBe("trading");
    expect(COMMAND_CENTER_NAV[TRADING_NAV.length]?.section).toBe("monitor");
  });

  it("maps grade routes to ISPAP statuses", () => {
    expect(gradeStatusFromPath(TERMINAL_ROUTES["grade-5"])).toBe("GRADE_5");
    expect(gradeStatusFromPath(TERMINAL_ROUTES["mock-engine"])).toBe("MAIN_ENGINE");
  });
});

describe("isPaperDeskTabKey", () => {
  it("validates tab keys", () => {
    expect(isPaperDeskTabKey("positions")).toBe(true);
    expect(isPaperDeskTabKey("invalid")).toBe(false);
  });
});

describe("isTerminalRoute", () => {
  it("matches terminal paths including grade routes", () => {
    expect(isTerminalRoute("/terminal")).toBe(true);
    expect(isTerminalRoute("/terminal/execution")).toBe(true);
    expect(isTerminalRoute("/terminal/grade-3")).toBe(true);
    expect(isTerminalRoute("/terminal/mock-engine")).toBe(true);
    expect(isTerminalRoute("/paper-desk")).toBe(false);
  });
});
