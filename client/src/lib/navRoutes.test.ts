import { describe, expect, it } from "vitest";
import {
  isNavItemActive,
  isPaperDeskRoute,
  isPaperDeskTabKey,
  paperDeskHref,
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

describe("isNavItemActive", () => {
  it("highlights Paper Desk on alias routes", () => {
    expect(
      isNavItemActive("/paperdesk", {
        href: "/paper-desk",
        routeMatcher: isPaperDeskRoute,
      }),
    ).toBe(true);
  });

  it("does not highlight Mock Trading on Paper Desk routes", () => {
    expect(
      isNavItemActive("/paper-desk", {
        href: "/mock-trading",
      }),
    ).toBe(false);
  });
});

describe("paperDeskHref", () => {
  it("builds tab deep links", () => {
    expect(paperDeskHref()).toBe("/paper-desk");
    expect(paperDeskHref("trades")).toBe("/paper-desk?tab=trades");
  });
});

describe("isPaperDeskTabKey", () => {
  it("validates tab keys", () => {
    expect(isPaperDeskTabKey("positions")).toBe(true);
    expect(isPaperDeskTabKey("invalid")).toBe(false);
  });
});
