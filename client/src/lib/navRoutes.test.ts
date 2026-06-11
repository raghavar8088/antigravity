import { describe, expect, it } from "vitest";
import {
  COMMAND_CENTER_PATH,
  COMMAND_CENTER_NAV,
  isNavItemActive,
  isPaperDeskRoute,
  isPaperDeskTabKey,
  isTerminalRoute,
  legacyPaperDeskRedirect,
  paperDeskHref,
  TERMINAL_ROUTES,
} from "./navRoutes";

describe("isPaperDeskRoute", () => {
  it("matches canonical and alias paths", () => {
    expect(isPaperDeskRoute("/paper-desk")).toBe(true);
    expect(isPaperDeskRoute("/paper-desk/settings")).toBe(true);
    expect(isPaperDeskRoute("/paperdesk")).toBe(true);
    expect(isPaperDeskRoute("/paperdesk/trades")).toBe(true);
  });

  it("does not match unrelated routes", () => {
    expect(isPaperDeskRoute("/")).toBe(false);
    expect(isPaperDeskRoute("/mock-trading")).toBe(false);
    expect(isPaperDeskRoute("/terminal")).toBe(false);
  });
});

describe("isTerminalRoute", () => {
  it("matches command center paths", () => {
    expect(isTerminalRoute(COMMAND_CENTER_PATH)).toBe(true);
    expect(isTerminalRoute(TERMINAL_ROUTES.execution)).toBe(true);
    expect(isTerminalRoute("/paper-desk")).toBe(false);
  });
});

describe("legacyPaperDeskRedirect", () => {
  it("maps legacy tabs to terminal routes", () => {
    expect(legacyPaperDeskRedirect()).toBe(TERMINAL_ROUTES.home);
    expect(legacyPaperDeskRedirect("positions")).toBe(TERMINAL_ROUTES.execution);
    expect(legacyPaperDeskRedirect("trades")).toBe(TERMINAL_ROUTES.journal);
    expect(legacyPaperDeskRedirect("orders")).toBe(TERMINAL_ROUTES.events);
    expect(legacyPaperDeskRedirect("equity")).toBe(TERMINAL_ROUTES.analytics);
    expect(legacyPaperDeskRedirect("strategies")).toBe(TERMINAL_ROUTES.strategies);
    expect(legacyPaperDeskRedirect("invalid")).toBe(TERMINAL_ROUTES.home);
  });
});

describe("isNavItemActive", () => {
  it("highlights command center home exactly", () => {
    expect(
      isNavItemActive("/terminal", {
        href: TERMINAL_ROUTES.home,
        exactMatch: true,
      }),
    ).toBe(true);
    expect(
      isNavItemActive("/terminal/execution", {
        href: TERMINAL_ROUTES.home,
        exactMatch: true,
      }),
    ).toBe(false);
  });
});

describe("paperDeskHref", () => {
  it("redirects to command center routes", () => {
    expect(paperDeskHref()).toBe(TERMINAL_ROUTES.home);
    expect(paperDeskHref("trades")).toBe(TERMINAL_ROUTES.journal);
  });
});

describe("isPaperDeskTabKey", () => {
  it("validates tab keys", () => {
    expect(isPaperDeskTabKey("positions")).toBe(true);
    expect(isPaperDeskTabKey("invalid")).toBe(false);
  });
});

describe("COMMAND_CENTER_NAV", () => {
  it("includes required operator surfaces", () => {
    const labels = COMMAND_CENTER_NAV.map((n) => n.label);
    expect(labels).toContain("Command Center");
    expect(labels).toContain("Execution");
    expect(labels).toContain("Health");
    expect(labels).not.toContain("Paper Desk");
  });
});
