/** @vitest-environment jsdom */
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { renderToString } from "react-dom/server";

// ── helpers ───────────────────────────────────────────────────────────────────

/** Render component to HTML string and return it. */
function html(node: React.ReactNode): string {
  return renderToString(node as React.ReactElement);
}

// ── LiveDeskStatusBar ─────────────────────────────────────────────────────────

describe("LiveDeskStatusBar", () => {
  it("shows Profit mode ON chip when enabled", async () => {
    const { LiveDeskStatusBar } = await import("../LiveDeskStatusBar");
    const out = html(<LiveDeskStatusBar profitModeEnabled isSignedIn={false} />);
    expect(out).toContain("Profit mode ON");
  });

  it("shows Profit mode OFF chip when disabled", async () => {
    const { LiveDeskStatusBar } = await import("../LiveDeskStatusBar");
    const out = html(<LiveDeskStatusBar profitModeEnabled={false} isSignedIn={false} />);
    expect(out).toContain("Profit mode OFF");
  });

  it("shows Signed in chip when isSignedIn=true", async () => {
    const { LiveDeskStatusBar } = await import("../LiveDeskStatusBar");
    const out = html(<LiveDeskStatusBar profitModeEnabled={false} isSignedIn />);
    expect(out).toContain("Signed in");
  });

  it("shows Anonymous chip when isSignedIn=false", async () => {
    const { LiveDeskStatusBar } = await import("../LiveDeskStatusBar");
    const out = html(<LiveDeskStatusBar profitModeEnabled={false} isSignedIn={false} />);
    expect(out).toContain("Anonymous");
  });

  it("renders readiness label when provided", async () => {
    const { LiveDeskStatusBar } = await import("../LiveDeskStatusBar");
    const out = html(
      <LiveDeskStatusBar profitModeEnabled isSignedIn readinessLabel="Paper edge OK" />,
    );
    expect(out).toContain("Paper edge OK");
  });

  it("omits readiness chip when readinessLabel is undefined", async () => {
    const { LiveDeskStatusBar } = await import("../LiveDeskStatusBar");
    const out = html(<LiveDeskStatusBar profitModeEnabled={false} isSignedIn={false} />);
    // Only two chips: profit mode + signed-in status
    const chipMatches = out.match(/desk-chip/g) ?? [];
    expect(chipMatches.length).toBe(2);
  });
});

// ── ProfitModeChecklist ───────────────────────────────────────────────────────

describe("ProfitModeChecklist", () => {
  const DISMISS_KEY = "test_ns_checklist_dismissed";

  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    localStorage.clear();
  });

  it("renders when not dismissed and totalTrades < 10", async () => {
    const { ProfitModeChecklist } = await import("../ProfitModeChecklist");
    const out = html(<ProfitModeChecklist storageNamespace="test_ns" totalTrades={0} />);
    expect(out).toContain("checklist");
  });

  it("does not render when localStorage dismiss key is set", async () => {
    localStorage.setItem(DISMISS_KEY, "1");
    const { ProfitModeChecklist } = await import("../ProfitModeChecklist");
    const out = html(<ProfitModeChecklist storageNamespace="test_ns" totalTrades={0} />);
    // Dismissed: renders nothing (empty string or just hydration comment)
    expect(out.replace(/<!--.*?-->/g, "").trim()).toBe("");
  });

  it("marks first_trade item done when totalTrades >= 1", async () => {
    const { ProfitModeChecklist } = await import("../ProfitModeChecklist");
    const out = html(<ProfitModeChecklist storageNamespace="test_ns" totalTrades={1} />);
    expect(out).toContain("First trade recorded");
    // The detail hint for first_trade should NOT appear (item is done)
    expect(out).not.toContain("Keep this tab open");
  });

  it("shows 4/4 complete label when totalTrades >= 10", async () => {
    const { ProfitModeChecklist } = await import("../ProfitModeChecklist");
    const out = html(<ProfitModeChecklist storageNamespace="test_ns" totalTrades={10} />);
    // All done — checkmark suffix
    expect(out).toContain("✓");
  });
});

// ── EntryDebugPanel visibility ────────────────────────────────────────────────

describe("EntryDebugPanel env visibility", () => {
  const originalEnv = { ...process.env };

  afterEach(() => {
    // Restore env flags after each test
    Object.assign(process.env, originalEnv);
    delete process.env.NEXT_PUBLIC_DESK_ENTRY_DEBUG;
    delete process.env.NEXT_PUBLIC_BTC_FT_ENTRY_DEBUG;
  });

  it("returns null when neither env flag is set and forceVisible is false", async () => {
    delete process.env.NEXT_PUBLIC_DESK_ENTRY_DEBUG;
    delete process.env.NEXT_PUBLIC_BTC_FT_ENTRY_DEBUG;
    const { EntryDebugPanel } = await import("../btcFutures/EntryDebugPanel");
    const out = html(
      <EntryDebugPanel
        entryDebug={null}
        pauseEntries={false}
        drawdownLocked={false}
        sessionSkips={{ minMove: 0, regime: 0, spread: 0, session: 0, category: 0, lowPriority: 0, regimeBreakdown: "" }}
      />,
    );
    expect(out.replace(/<!--.*?-->/g, "").trim()).toBe("");
  });

  it("renders when NEXT_PUBLIC_BTC_FT_ENTRY_DEBUG=1", async () => {
    process.env.NEXT_PUBLIC_BTC_FT_ENTRY_DEBUG = "1";
    const { EntryDebugPanel } = await import("../btcFutures/EntryDebugPanel");
    const out = html(
      <EntryDebugPanel
        entryDebug={null}
        pauseEntries={false}
        drawdownLocked={false}
        sessionSkips={{ minMove: 0, regime: 0, spread: 0, session: 0, category: 0, lowPriority: 0, regimeBreakdown: "" }}
      />,
    );
    expect(out).toContain("Entry debug");
  });

  it("renders when forceVisible=true regardless of env flags", async () => {
    delete process.env.NEXT_PUBLIC_DESK_ENTRY_DEBUG;
    delete process.env.NEXT_PUBLIC_BTC_FT_ENTRY_DEBUG;
    const { EntryDebugPanel } = await import("../btcFutures/EntryDebugPanel");
    const out = html(
      <EntryDebugPanel
        forceVisible
        entryDebug={null}
        pauseEntries={false}
        drawdownLocked={false}
        sessionSkips={{ minMove: 0, regime: 0, spread: 0, session: 0, category: 0, lowPriority: 0, regimeBreakdown: "" }}
      />,
    );
    expect(out).toContain("Entry debug");
  });
});
