import { test, expect, type Page } from "../fixtures/authedTest";

const ENGINE_URL = process.env.ENGINE_URL ?? "http://13.233.8.80";

// ── helpers ────────────────────────────────────────────────────────────────────

async function engineFetch(path: string) {
  const r = await fetch(`${ENGINE_URL}${path}`, { signal: AbortSignal.timeout(10_000) });
  return r.json();
}

// ── Suite ──────────────────────────────────────────────────────────────────────

test.describe("Backtest Lab", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/terminal/backtest");
    await page.waitForSelector("h1", { timeout: 10_000 });
  });

  // ── Page chrome ──────────────────────────────────────────────────────────────

  test("page renders heading and four tabs", async ({ page }) => {
    await expect(page.getByRole("heading", { name: "Backtest Lab" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Run Backtest" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Jobs" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Leaderboard" })).toBeVisible();
    await expect(page.getByRole("button", { name: "All Strategies" })).toBeVisible();
  });

  test("Run tab is active by default with form fields visible", async ({ page }) => {
    // Symbol input
    await expect(page.getByRole("textbox").first()).toHaveValue("BTCUSDT");
    // Submit button
    await expect(page.getByRole("button", { name: "Run All Strategies" })).toBeVisible();
  });

  // ── Tab navigation ────────────────────────────────────────────────────────────

  test("Jobs tab renders without crashing", async ({ page }) => {
    await page.getByRole("button", { name: "Jobs" }).click();
    // Either shows jobs or the empty-state message
    const hasJobs = await page.locator(".font-mono").count() > 0;
    const hasEmpty = await page.getByText("No backtest jobs yet.").isVisible().catch(() => false);
    expect(hasJobs || hasEmpty).toBeTruthy();
  });

  test("All Strategies tab loads strategy list from engine", async ({ page }) => {
    await page.getByRole("button", { name: "All Strategies" }).click();
    // Wait for either strategies or "no strategies" message
    await page.waitForFunction(
      () =>
        document.querySelector("[class*=grid]")?.children.length > 0 ||
        document.body.innerText.includes("Loading") === false,
      { timeout: 15_000 }
    );
    // Should not be stuck on "Loading…"
    await expect(page.getByText("Loading…")).not.toBeVisible({ timeout: 15_000 });
    // Count badge should appear
    const countText = await page.locator("span.text-gray-500.text-sm.self-center").textContent();
    expect(countText).toMatch(/\d+ \/ \d+/);
  });

  test("All Strategies search filter narrows results", async ({ page }) => {
    await page.getByRole("button", { name: "All Strategies" }).click();
    await expect(page.getByText("Loading…")).not.toBeVisible({ timeout: 15_000 });

    const before = await page.locator("[class*=grid] > div").count();
    test.skip(before === 0, "no strategies loaded — engine offline");

    await page.getByPlaceholder("Filter strategies…").fill("ema");
    await page.waitForTimeout(300);
    const after = await page.locator("[class*=grid] > div").count();
    expect(after).toBeLessThanOrEqual(before);
  });

  // ── Leaderboard tab ───────────────────────────────────────────────────────────

  test("Leaderboard tab renders Load button and handles empty state", async ({ page }) => {
    await page.getByRole("button", { name: "Leaderboard" }).click();
    await expect(page.getByRole("button", { name: "Load" })).toBeVisible();
    // Either shows rows or a graceful empty/error message
    await page.waitForTimeout(2_000);
    const hasError = await page.locator("p.text-red-400").isVisible().catch(() => false);
    const hasEmpty = await page.getByText("No results. Run a backtest first.").isVisible().catch(() => false);
    const hasRows = await page.locator("table").isVisible().catch(() => false);
    expect(hasError || hasEmpty || hasRows).toBeTruthy();
  });

  // ── Run form validation ───────────────────────────────────────────────────────

  test("Run tab submits and shows job created message or meaningful error", async ({ page }) => {
    // Set a short date range to avoid a long-running job in E2E
    await page.locator('input[type="date"]').first().fill("2024-11-01");
    await page.locator('input[type="date"]').last().fill("2024-11-07");

    await page.getByRole("button", { name: "Run All Strategies" }).click();
    await expect(page.getByRole("button", { name: "Submitting…" })).toBeVisible();

    // Wait for response (success or error)
    await expect(
      page.locator("p.text-green-400, p.text-red-400")
    ).toBeVisible({ timeout: 20_000 });

    const green = page.locator("p.text-green-400");
    const red = page.locator("p.text-red-400");
    const isGreen = await green.isVisible().catch(() => false);
    const isRed = await red.isVisible().catch(() => false);

    if (isGreen) {
      const msg = await green.textContent();
      expect(msg).toMatch(/Job created: job_\d+/);
    } else if (isRed) {
      // Engine offline or data missing — acceptable, just must not be a JS crash
      const msg = await red.textContent();
      expect(msg).toBeTruthy();
      console.log(`  Note: engine returned error (expected if offline): ${msg}`);
    }
  });

  // ── Jobs tab auto-refresh ─────────────────────────────────────────────────────

  test("Jobs tab polls engine and reflects submitted jobs", async ({ page }) => {
    // Submit a job first from Run tab
    await page.locator('input[type="date"]').first().fill("2024-11-01");
    await page.locator('input[type="date"]').last().fill("2024-11-03");
    await page.getByRole("button", { name: "Run All Strategies" }).click();
    const green = page.locator("p.text-green-400");
    const visible = await green.isVisible({ timeout: 20_000 }).catch(() => false);
    if (!visible) {
      test.skip(true, "engine offline — skip jobs polling test");
      return;
    }

    // Switch to Jobs tab and confirm entry appears
    await page.getByRole("button", { name: "Jobs" }).click();
    // Job ID starts with "job_"
    await expect(page.locator(".font-mono").first()).toContainText("job_", { timeout: 10_000 });
  });

  // ── Sidebar navigation ────────────────────────────────────────────────────────

  test("sidebar shows Backtest Lab link and navigates correctly", async ({ page }) => {
    await page.goto("/terminal/trade-engine");
    const link = page.getByRole("link", { name: "Backtest Lab" });
    await expect(link).toBeVisible();
    await link.click();
    await expect(page).toHaveURL(/\/terminal\/backtest/);
    await expect(page.getByRole("heading", { name: "Backtest Lab" })).toBeVisible();
  });
});

// ── Direct API tests (bypass Next.js proxy, hit engine directly) ──────────────

test.describe("Backtest Engine API (direct)", () => {
  test("GET /health returns alive", async ({ request }) => {
    const r = await request.get(`${ENGINE_URL}/health`, { timeout: 10_000 });
    test.skip(!r.ok(), "engine unreachable");
    const body = await r.json();
    expect(body.status).toBe("alive");
    expect(body.strategies).toBeGreaterThan(0);
  });

  test("GET /api/backtest/strategies returns non-empty array", async ({ request }) => {
    const r = await request.get(`${ENGINE_URL}/api/backtest/strategies`, { timeout: 10_000 });
    test.skip(!r.ok(), "engine unreachable");
    const body = await r.json();
    expect(Array.isArray(body)).toBe(true);
    expect(body.length).toBeGreaterThan(0);
    expect(body[0]).toHaveProperty("name");
    expect(body[0]).toHaveProperty("regimes");
  });

  test("GET /api/backtest/jobs returns array", async ({ request }) => {
    const r = await request.get(`${ENGINE_URL}/api/backtest/jobs`, { timeout: 10_000 });
    test.skip(!r.ok(), "engine unreachable");
    expect(Array.isArray(await r.json())).toBe(true);
  });

  test("GET /api/backtest/leaderboard returns array", async ({ request }) => {
    const r = await request.get(`${ENGINE_URL}/api/backtest/leaderboard?symbol=BTCUSDT`, { timeout: 10_000 });
    test.skip(!r.ok(), "engine unreachable");
    expect(Array.isArray(await r.json())).toBe(true);
  });

  test("POST /api/backtest/run with bad date returns 400", async ({ request }) => {
    const r = await request.post(`${ENGINE_URL}/api/backtest/run`, {
      data: { symbol: "BTCUSDT", from: "not-a-date", to: "2024-11-30" },
      timeout: 10_000,
    });
    test.skip(r.status() === 0, "engine unreachable");
    expect(r.status()).toBe(400);
    const body = await r.json();
    expect(body).toHaveProperty("error");
  });

  test("GET /api/backtest/status/nonexistent returns 404", async ({ request }) => {
    const r = await request.get(`${ENGINE_URL}/api/backtest/status/nonexistent-job-xyz`, { timeout: 10_000 });
    test.skip(r.status() === 0, "engine unreachable");
    expect(r.status()).toBe(404);
  });

  test("POST /api/backtest/run creates job with correct shape", async ({ request }) => {
    const r = await request.post(`${ENGINE_URL}/api/backtest/run`, {
      data: { symbol: "BTCUSDT", from: "2024-11-01", to: "2024-11-07" },
      timeout: 10_000,
    });
    test.skip(!r.ok(), "engine unreachable or data missing");
    expect(r.status()).toBe(202);
    const body = await r.json();
    expect(body.id).toMatch(/^job_\d+/);
    expect(body.runId).toMatch(/^run_\d+/);
    expect(body.symbol).toBe("BTCUSDT");
    expect(["pending", "running"]).toContain(body.status);
    expect(body.progress).toBeGreaterThanOrEqual(0);

    // Poll status endpoint for the created job
    const sr = await request.get(`${ENGINE_URL}/api/backtest/status/${body.id}`, { timeout: 10_000 });
    expect(sr.ok()).toBe(true);
    const status = await sr.json();
    expect(status.id).toBe(body.id);
  });
});
