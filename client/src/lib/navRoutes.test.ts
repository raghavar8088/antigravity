import { describe, expect, it } from "vitest";
import {
  COMMAND_CENTER_NAV,
  MOCK_TRADING_PATH,
  TERMINAL_ROUTES,
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

describe("COMMAND_CENTER_NAV", () => {
  it("lists mock trading first", () => {
    expect(COMMAND_CENTER_NAV[0]?.href).toBe(MOCK_TRADING_PATH);
    expect(COMMAND_CENTER_NAV.some((item) => item.href === TERMINAL_ROUTES.home)).toBe(true);
  });
});

describe("isPaperDeskTabKey", () => {
  it("validates tab keys", () => {
    expect(isPaperDeskTabKey("positions")).toBe(true);
    expect(isPaperDeskTabKey("invalid")).toBe(false);
  });
});

describe("isTerminalRoute", () => {
  it("matches terminal paths", () => {
    expect(isTerminalRoute("/terminal")).toBe(true);
    expect(isTerminalRoute("/terminal/execution")).toBe(true);
    expect(isTerminalRoute("/paper-desk")).toBe(false);
  });
});
