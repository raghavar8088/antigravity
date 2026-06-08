"use client";

import Link from "next/link";

export default function MockTradingError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div
      style={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: 24,
        background: "var(--bg, #0d1117)",
        color: "var(--text-primary, #e6edf3)",
        fontFamily: "system-ui, sans-serif",
      }}
    >
      <div style={{ maxWidth: 480, textAlign: "center" }}>
        <h1 style={{ fontSize: 18, margin: "0 0 8px" }}>Mock Trading failed to load</h1>
        <p style={{ fontSize: 13, color: "var(--text-muted, #8b949e)", margin: "0 0 16px", lineHeight: 1.5 }}>
          {error.message || "A client error stopped this page from rendering."}
          {" "}Live Paper Desk trades are at{" "}
          <Link href="/paper-desk" style={{ color: "var(--accent, #4d7cfe)" }}>/paper-desk</Link>.
        </p>
        <div style={{ display: "flex", gap: 10, justifyContent: "center", flexWrap: "wrap" }}>
          <button
            type="button"
            onClick={() => reset()}
            style={{
              padding: "8px 16px",
              borderRadius: 6,
              border: "none",
              background: "#238636",
              color: "#fff",
              fontWeight: 600,
              cursor: "pointer",
            }}
          >
            Reload
          </button>
          <Link
            href="/paper-desk"
            style={{
              padding: "8px 16px",
              borderRadius: 6,
              border: "1px solid #30363d",
              color: "inherit",
              textDecoration: "none",
              fontWeight: 600,
            }}
          >
            Open Paper Desk
          </Link>
        </div>
      </div>
    </div>
  );
}
