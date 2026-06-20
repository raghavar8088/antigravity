import { createHash } from "node:crypto";
import { test as base, expect, type Page } from "@playwright/test";
import { E2E_ADMIN_USERNAME, E2E_ADMIN_PASSWORD } from "../../playwright.config";

// Next.js itself renders a hidden role="alert" route announcer
// (#__next-route-announcer__) on every page — exclude it so this only
// matches the login form's actual error banner.
function errorAlert(page: Page) {
  return page.locator('[role="alert"]:not(#__next-route-announcer__)');
}

// middleware.ts and the login route both rate-limit by client IP, read from
// x-forwarded-for/x-real-ip. Hitting `next dev` directly (no reverse proxy)
// means every request has neither header, so all traffic would collapse onto
// one "unknown" bucket and tests would trip each other's rate limits. Each
// test gets a deterministic fake x-forwarded-for derived from its stable
// Playwright testId (not a shared counter — that resets per worker process
// and would still collide under parallel execution) so tests stay isolated
// regardless of run order or worker count.
function fakeIpFor(testId: string): string {
  const hash = createHash("sha1").update(testId).digest();
  return `10.${hash[0]}.${hash[1]}.${hash[2]}`;
}
const test = base.extend({
  extraHTTPHeaders: async ({}, use, testInfo) => {
    await use({ "x-forwarded-for": fakeIpFor(testInfo.testId) });
  },
});

// This file is the only spec that drives the /login UI form directly — every
// other authenticated spec reuses the storageState captured once in
// global-setup.ts (see e2e/fixtures/authedTest.ts) to avoid tripping the
// login rate limiter (5 attempts / 15 min, src/app/api/auth/login/route.ts).
test.describe("login", () => {
  test("redirects unauthenticated visitors away from a protected page", async ({ page }) => {
    await page.goto("/terminal/risk");
    await expect(page).toHaveURL(/\/login\?next=%2Fterminal%2Frisk/);
  });

  test("rejects invalid credentials", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel("Username").fill("not-a-real-user");
    await page.getByLabel("Password").fill("wrong-password");
    await page.getByRole("button", { name: /sign in/i }).click();
    await expect(errorAlert(page)).toContainText(/invalid credentials/i);
    await expect(page).toHaveURL(/\/login/);
  });

  test("signs in successfully and lands on /terminal", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel("Username").fill(E2E_ADMIN_USERNAME);
    await page.getByLabel("Password").fill(E2E_ADMIN_PASSWORD);
    await page.getByRole("button", { name: /sign in/i }).click();
    await expect(page).toHaveURL(/\/terminal/);
  });

  test("honors the ?next= redirect target after login", async ({ page }) => {
    await page.goto("/terminal/risk");
    await expect(page).toHaveURL(/\/login\?next=%2Fterminal%2Frisk/);
    await page.getByLabel("Username").fill(E2E_ADMIN_USERNAME);
    await page.getByLabel("Password").fill(E2E_ADMIN_PASSWORD);
    await page.getByRole("button", { name: /sign in/i }).click();
    await expect(page).toHaveURL(/\/terminal\/risk/);
  });

  test("signing out clears the session and re-guards protected pages", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel("Username").fill(E2E_ADMIN_USERNAME);
    await page.getByLabel("Password").fill(E2E_ADMIN_PASSWORD);
    await page.getByRole("button", { name: /sign in/i }).click();
    await expect(page).toHaveURL(/\/terminal/);

    await page.evaluate(async () => {
      await fetch("/api/auth/signout", { method: "POST", credentials: "include" });
    });
    await page.goto("/terminal/risk");
    await expect(page).toHaveURL(/\/login\?next=%2Fterminal%2Frisk/);
  });

  test("rate-limits repeated failed login attempts", async ({ page }) => {
    await page.goto("/login");
    for (let i = 0; i < 5; i++) {
      await page.getByLabel("Username").fill("not-a-real-user");
      await page.getByLabel("Password").fill(`wrong-password-${i}`);
      await page.getByRole("button", { name: /sign in/i }).click();
      await expect(errorAlert(page)).toBeVisible();
    }
    // 6th attempt within the 15-minute window should be rate-limited (429).
    await page.getByLabel("Username").fill("not-a-real-user");
    await page.getByLabel("Password").fill("wrong-password-final");
    await page.getByRole("button", { name: /sign in/i }).click();
    await expect(errorAlert(page)).toContainText(/too many login attempts/i);
  });
});
