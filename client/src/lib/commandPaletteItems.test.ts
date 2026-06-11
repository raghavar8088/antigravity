import { describe, expect, it } from "vitest";
import { COMMAND_PALETTE_ITEMS, PAGE_TITLES, resolvePageTitle } from "@/lib/commandPaletteItems";
import { TERMINAL_ROUTES } from "@/lib/navRoutes";

describe("commandPaletteItems", () => {
  it("includes all command center nav routes", () => {
    const hrefs = COMMAND_PALETTE_ITEMS.filter((i) => i.group === "Navigate").map((i) => i.href);
    expect(hrefs).toContain(TERMINAL_ROUTES.home);
    expect(hrefs).toContain(TERMINAL_ROUTES.risk);
    expect(hrefs).toContain(TERMINAL_ROUTES.execution);
  });

  it("resolves page titles for terminal sub-routes", () => {
    expect(resolvePageTitle(TERMINAL_ROUTES.analytics)).toBe("Analytics");
    expect(resolvePageTitle("/terminal/unknown")).toBe("Command Center");
  });

  it("maps every terminal route to a page title", () => {
    for (const path of Object.values(TERMINAL_ROUTES)) {
      expect(PAGE_TITLES[path]).toBeTruthy();
    }
  });
});
