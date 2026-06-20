import { test, expect } from "../fixtures/authedTest";

// /mobile is the "paged at 2am" emergency view (src/app/mobile/page.tsx) —
// authenticated, optimized for a 375px viewport. Runs only on the
// chromium-mobile project (see playwright.config.ts testMatch/testIgnore).
test.describe("mobile emergency view", () => {
  test("loads on a mobile viewport without crashing", async ({ page }) => {
    const pageErrors: Error[] = [];
    page.on("pageerror", (err) => pageErrors.push(err));

    const response = await page.goto("/mobile");
    expect(response?.ok(), `responded with ${response?.status()}`).toBeTruthy();
    await expect(page).not.toHaveURL(/\/login/);
    expect(pageErrors, `threw: ${pageErrors.map((e) => e.message).join("; ")}`).toHaveLength(0);
  });
});
