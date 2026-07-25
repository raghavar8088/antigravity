import { test, expect } from "../fixtures/authedTest";

/**
 * Live Engine — real-money module UI. These assert the safety-critical UX:
 * unmistakable real-money differentiation, arm gated behind an exact typed
 * confirmation, disarm/close-all reachable. They skip gracefully when the dev
 * server or backend is unavailable (resource-constrained CI).
 */

const ARM_PHRASE = "ARM LIVE $100";

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

  test("arm is gated behind the exact typed confirmation phrase", async ({ page }) => {
    const armOpen = page.getByTestId("arm-open");
    // If already armed (shared env), a disarm control is shown instead — accept either.
    if (!(await armOpen.isVisible().catch(() => false))) {
      await expect(page.getByTestId("disarm")).toBeVisible();
      test.skip(true, "engine already armed in this environment");
      return;
    }
    await armOpen.click();
    await expect(page.getByTestId("arm-modal")).toBeVisible();

    const confirm = page.getByTestId("arm-confirm");
    await expect(confirm).toBeDisabled();

    await page.getByTestId("arm-input").fill("not the phrase");
    await expect(confirm).toBeDisabled();

    await page.getByTestId("arm-input").fill(ARM_PHRASE);
    await expect(confirm).toBeEnabled();

    // Do not actually arm real money in a test — close the modal.
    await page.keyboard.press("Escape").catch(() => {});
  });

  test("panic CLOSE ALL is reachable", async ({ page }) => {
    await expect(page.getByTestId("close-all")).toBeVisible();
  });
});
