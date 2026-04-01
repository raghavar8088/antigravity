"use client";

import useFearGreed from "@/hooks/useFearGreed";

function colorFor(value: number): string {
  if (value <= 24) return "#F44336";        // Extreme Fear — red
  if (value <= 44) return "#FF7043";        // Fear — orange-red
  if (value <= 55) return "#FFA726";        // Neutral — amber
  if (value <= 75) return "#66BB6A";        // Greed — green
  return "#00D09C";                         // Extreme Greed — Groww green
}

function Arc({ value }: { value: number }) {
  const color = colorFor(value);
  const r = 30;
  const cx = 44, cy = 44;
  const startAngle = -180;
  const endAngle = startAngle + (value / 100) * 180;

  const toRad = (deg: number) => (deg * Math.PI) / 180;
  const x1 = cx + r * Math.cos(toRad(startAngle));
  const y1 = cy + r * Math.sin(toRad(startAngle));
  const x2 = cx + r * Math.cos(toRad(endAngle));
  const y2 = cy + r * Math.sin(toRad(endAngle));
  const largeArc = value > 50 ? 1 : 0;

  const trackX1 = cx + r * Math.cos(toRad(-180));
  const trackY1 = cy + r * Math.sin(toRad(-180));
  const trackX2 = cx + r * Math.cos(toRad(0));
  const trackY2 = cy + r * Math.sin(toRad(0));

  return (
    <svg width={88} height={50} viewBox="0 0 88 50">
      {/* Track */}
      <path
        d={`M ${trackX1} ${trackY1} A ${r} ${r} 0 0 1 ${trackX2} ${trackY2}`}
        fill="none"
        stroke="var(--surface-3)"
        strokeWidth={7}
        strokeLinecap="round"
      />
      {/* Value arc */}
      {value > 0 && (
        <path
          d={`M ${x1} ${y1} A ${r} ${r} 0 ${largeArc} 1 ${x2} ${y2}`}
          fill="none"
          stroke={color}
          strokeWidth={7}
          strokeLinecap="round"
        />
      )}
    </svg>
  );
}

export default function FearGreedWidget() {
  const fg = useFearGreed();

  if (!fg) return null;

  const color = colorFor(fg.value);

  return (
    <div
      style={{
        background: "var(--surface-2)",
        border: "1px solid var(--border)",
        borderRadius: 12,
        padding: "12px 16px",
        display: "flex",
        alignItems: "center",
        gap: 14,
      }}
    >
      {/* Gauge */}
      <div style={{ position: "relative", flexShrink: 0 }}>
        <Arc value={fg.value} />
        <div
          style={{
            position: "absolute",
            bottom: 0,
            left: 0,
            right: 0,
            textAlign: "center",
            fontSize: 15,
            fontWeight: 800,
            color,
            lineHeight: 1,
          }}
        >
          {fg.value}
        </div>
      </div>

      {/* Labels */}
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: 10, fontWeight: 600, letterSpacing: "0.12em", color: "var(--text-muted)", textTransform: "uppercase", marginBottom: 3 }}>
          Fear &amp; Greed
        </div>
        <div style={{ fontSize: 13, fontWeight: 700, color }}>
          {fg.classification}
        </div>
        <div style={{ fontSize: 10, color: "var(--text-muted)", marginTop: 2 }}>
          BTC sentiment · daily
        </div>
      </div>
    </div>
  );
}
