/** @vitest-environment jsdom */
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { renderToString } from "react-dom/server";
import { DeskCopyButton } from "./DeskCopyButton";

describe("DeskCopyButton — SSR render", () => {
  it("renders the clipboard icon initially (no Copied state)", () => {
    const html = renderToString(<DeskCopyButton text="https://x.test/foo" ariaLabel="Copy link to Foo" />);
    // Clipboard icon = two overlapping rects; check icon shape is present
    expect(html).toContain("<rect");
    // ARIA label is present for screen readers
    expect(html).toContain('aria-label="Copy link to Foo"');
    // Title attribute matches aria label when not copied
    expect(html).toContain('title="Copy link to Foo"');
  });
});

describe("DeskCopyButton — clipboard write", () => {
  const originalClipboard = navigator.clipboard;

  beforeEach(() => {
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: vi.fn().mockResolvedValue(undefined) },
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    Object.defineProperty(navigator, "clipboard", {
      value: originalClipboard,
      writable: true,
      configurable: true,
    });
  });

  it("writes the provided text to clipboard when clicked", async () => {
    // Render the button into a real DOM node and simulate a click.
    const { createRoot } = await import("react-dom/client");
    const { act } = await import("react");
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);
    await act(async () => {
      root.render(<DeskCopyButton text="https://x.test/btc" ariaLabel="Copy link to BTC" />);
    });

    const btn = container.querySelector("button");
    expect(btn).not.toBeNull();
    await act(async () => {
      btn!.click();
    });

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith("https://x.test/btc");

    root.unmount();
    container.remove();
  });
});

describe("moduleDeepLinkUrl — module → URL mapping", () => {
  it("returns canonical paths for modules with dedicated routes", async () => {
    const { moduleDeepLinkUrl } = await import("../WorkspaceNavPanel");
    // jsdom origin defaults to http://localhost:3000 typically; check suffix.
    expect(moduleDeepLinkUrl("btcFutureTrading").endsWith("/btc-future-trading")).toBe(true);
    expect(moduleDeepLinkUrl("nifty").endsWith("/nifty-options")).toBe(true);
    expect(moduleDeepLinkUrl("niftySelling").endsWith("/nifty-selling")).toBe(true);
    expect(moduleDeepLinkUrl("niftyBees").endsWith("/nifty-bees")).toBe(true);
  });

  it("returns workspace root with ?module= for tabs without dedicated routes", async () => {
    const { moduleDeepLinkUrl } = await import("../WorkspaceNavPanel");
    expect(moduleDeepLinkUrl("options").endsWith("/?module=options")).toBe(true);
    expect(moduleDeepLinkUrl("options-selling").endsWith("/?module=options-selling")).toBe(true);
    expect(moduleDeepLinkUrl("btcFuturesScalper").endsWith("/?module=btcFuturesScalper")).toBe(true);
  });
});
