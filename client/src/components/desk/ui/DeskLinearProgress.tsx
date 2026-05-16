"use client";

export function DeskLinearProgress({ visible = true }: { visible?: boolean }) {
  if (!visible) return null;
  return (
    <div
      role="progressbar"
      aria-label="Loading market data"
      style={{
        position: "sticky",
        top: "var(--desk-header-height)",
        zIndex: 90,
        height: 3,
        width: "100%",
        background: "var(--desk-surface-container)",
        overflow: "hidden",
      }}
    >
      <div className="desk-linear-progress-bar" />
    </div>
  );
}
