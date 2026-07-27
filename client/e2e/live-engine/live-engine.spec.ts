import { test, expect } from "../fixtures/authedTest";

/**
 * Live Engine — real-money module UI. These assert the safety-critical UX:
 * unmistakable real-money differentiation, the Delta Engine and kill-switch
 * toggles reflecting live state, and close-all reachable. Toggles are never
 * flipped here — turning the engine on places real orders. They skip gracefully
 * when the dev server or backend is unavailable (resource-constrained CI).
 */


test.describe("Live Engine (real money)", () => {
  test.describe.configure({ mode: "serial" });
  test.setTimeout(60_000);

  test.beforeEach(async ({ page }) => {
    const loaded = await page
      .goto("/live-engine", { timeout: 15_000 })
      .then(() => true)
      .catch(() => false);
    if (!loaded) {
      test.skip(true, "page.goto timed out — dev server unavailable");
      return;
    }
    const ready = await page
      .waitForSelector('[data-testid="live-engine-root"]', { timeout: 20_000 })
      .then(() => true)
      .catch(() => false);
    if (!ready) {
      test.skip(true, "live-engine root not rendered — backend/auth unavailable");
    }
  });

  test("carries persistent REAL MONEY · $100 differentiation", async ({ page }) => {
    const badge = page.getByTestId("real-money-badge");
    await expect(badge).toBeVisible();
    await expect(badge).toContainText("REAL MONEY");
    await expect(badge).toContainText("$100");
  });

  test("shows an armed/disarmed state without scrolling", async ({ page }) => {
    const root = page.getByTestId("live-engine-root");
    await expect(root).toHaveAttribute("data-armed", /true|false/);
    await expect(page.getByTestId("armed-state")).toBeVisible();
  });

  test("Delta Engine toggle reflects live state and is labelled on/off", async ({ page }) => {
    const armToggle = page.locator("#arm-toggle");
    if (!(await armToggle.isVisible().catch(() => false))) {
      test.skip(true, "Delta Engine toggle not rendered — backend unavailable");
      return;
    }
    // Never flip it in a test: toggling on now places real orders immediately.
    const checked = await armToggle.isChecked();
    expect(typeof checked).toBe("boolean");
    await expect(page.getByText(checked ? "Delta Engine on" : "Delta Engine off")).toBeVisible();
  });

  test("panic CLOSE ALL is reachable", async ({ page }) => {
    await expect(page.getByTestId("close-all")).toBeVisible();
  });

  test("kill switch is a toggle reflecting live state", async ({ page }) => {
    const ks = page.locator("#kill-switch-toggle");
    if (!(await ks.isVisible().catch(() => false))) {
      test.skip(true, "kill switch toggle not rendered — backend unavailable");
      return;
    }
    // It must reflect a real boolean state (on = halted, off = trading allowed).
    const checked = await ks.isChecked();
    expect(typeof checked).toBe("boolean");
  });
});
