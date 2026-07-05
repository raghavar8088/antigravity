"use client";

import { useMemo, useState } from "react";

export interface LineChartPoint {
  ts: number;
  value: number;
}

export interface LineChartProps {
  points: LineChartPoint[];
  color?: string;
  height?: number;
  fillToZero?: boolean;
  valueSuffix?: string;
  formatValue?: (value: number) => string;
}

const PAD = { top: 14, right: 74, bottom: 24, left: 10 };
const WIDTH = 900;

export function LineChart({ points, color = "var(--accent)", height = 220, fillToZero = false, valueSuffix = "", formatValue }: LineChartProps) {
  const [hoverIndex, setHoverIndex] = useState<number | null>(null);

  const fmt = formatValue ?? ((v: number) => v.toLocaleString("en-US", { maximumFractionDigits: 2 }));

  const geometry = useMemo(() => {
    if (points.length === 0) return null;
    const values = points.map((p) => p.value);
    const min = fillToZero ? Math.min(0, ...values) : Math.min(...values);
    const max = fillToZero ? Math.max(0, ...values) : Math.max(...values);
    const range = max - min || 1;
    const innerW = WIDTH - PAD.left - PAD.right;
    const innerH = height - PAD.top - PAD.bottom;
    const xStep = points.length > 1 ? innerW / (points.length - 1) : 0;
    const toX = (i: number) => PAD.left + i * xStep;
    const toY = (v: number) => PAD.top + innerH - ((v - min) / range) * innerH;
    const zeroY = toY(Math.max(min, Math.min(max, 0)));
    return { min, max, innerW, innerH, xStep, toX, toY, zeroY };
  }, [points, height, fillToZero]);

  if (!geometry || points.length === 0) return null;
  const { toX, toY, zeroY } = geometry;

  const linePath = points.map((p, i) => `${i === 0 ? "M" : "L"}${toX(i).toFixed(1)},${toY(p.value).toFixed(1)}`).join(" ");
  const areaPath = fillToZero
    ? `${linePath} L${toX(points.length - 1).toFixed(1)},${zeroY.toFixed(1)} L${toX(0).toFixed(1)},${zeroY.toFixed(1)} Z`
    : `${linePath} L${toX(points.length - 1).toFixed(1)},${(height - PAD.bottom).toFixed(1)} L${toX(0).toFixed(1)},${(height - PAD.bottom).toFixed(1)} Z`;

  const gridLines = [0.25, 0.5, 0.75].map((f) => PAD.top + geometry.innerH * f);

  const last = points[points.length - 1];
  const hovered = hoverIndex != null ? points[hoverIndex] : null;

  return (
    <div style={{ position: "relative", width: "100%" }}>
      <svg
        viewBox={`0 0 ${WIDTH} ${height}`}
        width="100%"
        height={height}
        preserveAspectRatio="none"
        onMouseMove={(e) => {
          const rect = e.currentTarget.getBoundingClientRect();
          const relX = ((e.clientX - rect.left) / rect.width) * WIDTH;
          const idx = Math.round((relX - PAD.left) / (geometry.xStep || 1));
          setHoverIndex(Math.max(0, Math.min(points.length - 1, idx)));
        }}
        onMouseLeave={() => setHoverIndex(null)}
      >
        <defs>
          <linearGradient id="lc-area" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={color} stopOpacity={0.16} />
            <stop offset="100%" stopColor={color} stopOpacity={0} />
          </linearGradient>
        </defs>
        {gridLines.map((y) => (
          <line key={y} x1={PAD.left} x2={WIDTH - PAD.right} y1={y} y2={y} stroke="var(--panel-border, var(--border))" strokeWidth={1} />
        ))}
        <path d={areaPath} fill="url(#lc-area)" />
        <path d={linePath} fill="none" stroke={color} strokeWidth={2} strokeLinejoin="round" strokeLinecap="round" />
        {hovered && hoverIndex != null ? (
          <>
            <line
              x1={toX(hoverIndex)}
              x2={toX(hoverIndex)}
              y1={PAD.top}
              y2={height - PAD.bottom}
              stroke="var(--text-faint, var(--text-muted))"
              strokeWidth={1}
              strokeDasharray="3 3"
            />
            <circle cx={toX(hoverIndex)} cy={toY(hovered.value)} r={4} fill={color} stroke="var(--canvas, var(--surface))" strokeWidth={2} />
          </>
        ) : null}
      </svg>
      <div
        style={{
          position: "absolute",
          top: PAD.top - 4,
          right: 0,
          background: "var(--panel, var(--surface))",
          border: "1px solid var(--panel-border, var(--border))",
          borderRadius: 6,
          padding: "2px 6px",
          fontFamily: "var(--font-mono)",
          fontVariantNumeric: "tabular-nums",
          fontSize: 11,
          color: "var(--text, var(--text-primary))",
        }}
      >
        {fmt(last.value)}
        {valueSuffix}
      </div>
      {hovered ? (
        <div
          style={{
            position: "absolute",
            top: 4,
            left: `${(toX(hoverIndex!) / WIDTH) * 100}%`,
            transform: "translateX(-50%)",
            background: "var(--text, var(--text-primary))",
            borderRadius: 8,
            padding: "6px 10px",
            boxShadow: "var(--shadow-md, var(--shadow-lg))",
            pointerEvents: "none",
            whiteSpace: "nowrap",
          }}
        >
          <div style={{ color: "rgba(255,255,255,0.6)", fontSize: 10.5 }}>{new Date(hovered.ts).toLocaleDateString()}</div>
          <div style={{ color: "#fff", fontSize: 12.5, fontWeight: 600, fontFamily: "var(--font-mono)" }}>
            {fmt(hovered.value)}
            {valueSuffix}
          </div>
        </div>
      ) : null}
    </div>
  );
}
