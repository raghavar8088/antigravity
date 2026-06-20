import { test, expect } from "../fixtures/authedTest";

// KillSwitchPanel (src/components/killswitch/KillSwitchPanel.tsx, mounted via
// M3AppShell) calls GET /api/killswitch/status, which already falls back to
// a safe "engine_offline" payload (status 200) when the Go engine isn't
// reachable — exactly the state of the E2E webServer, by design (no
// INTERNAL_API_URL/engine running). This verifies that fallback renders as
// an informative banner rather than a broken/blank panel, and that the
// HALT/RESUME controls are correctly suppressed when the engine state is
// unknown (can't safely offer a halt action against a backend you can't see).
test.describe("kill switch panel — engine offline fallback", () => {
  test("shows an offline banner instead of action buttons on /terminal/risk", async ({ page }) => {
    await page.goto("/terminal/risk");
    const banner = page.getByRole("status").filter({ hasText: /kill switch/i });
    await expect(banner).toContainText(/engine offline/i);
    await expect(page.getByRole("button", { name: /halt all trading/i })).toHaveCount(0);
    await expect(page.getByRole("button", { name: /resume trading/i })).toHaveCount(0);
  });
});
