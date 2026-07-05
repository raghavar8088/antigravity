"use client";

import { useMemo } from "react";

export interface PayoffPoint {
  spot: number;
  pnl: number;
}

export interface PayoffChartProps {
  points: PayoffPoint[];
  breakevens?: number[];
  height?: number;
}

const PAD = { top: 14, right: 20, bottom: 24, left: 60 };
const WIDTH = 900;

export function PayoffChart({ points, breakevens = [], height = 220 }: PayoffChartProps) {
  const geometry = useMemo(() => {
    if (points.length === 0) return null;
    const spots = points.map((p) => p.spot);
    const pnls = points.map((p) => p.pnl);
    const minSpot = Math.min(...spots);
    const maxSpot = Math.max(...spots);
    const minPnl = Math.min(0, ...pnls);
    const maxPnl = Math.max(0, ...pnls);
    const spotRange = maxSpot - minSpot || 1;
    const pnlRange = maxPnl - minPnl || 1;
    const innerW = WIDTH - PAD.left - PAD.right;
    const innerH = height - PAD.top - PAD.bottom;
    const toX = (spot: number) => PAD.left + ((spot - minSpot) / spotRange) * innerW;
    const toY = (pnl: number) => PAD.top + innerH - ((pnl - minPnl) / pnlRange) * innerH;
    return { toX, toY, zeroY: toY(0) };
  }, [points, height]);

  if (!geometry) return null;
  const { toX, toY, zeroY } = geometry;

  const linePath = points.map((p, i) => `${i === 0 ? "M" : "L"}${toX(p.spot).toFixed(1)},${toY(p.pnl).toFixed(1)}`).join(" ");
  const gainPath = `${linePath} L${toX(points[points.length - 1].spot).toFixed(1)},${zeroY.toFixed(1)} L${toX(points[0].spot).toFixed(1)},${zeroY.toFixed(1)} Z`;

  return (
    <svg viewBox={`0 0 ${WIDTH} ${height}`} width="100%" height={height} preserveAspectRatio="none">
      <line
        x1={PAD.left}
        x2={WIDTH - PAD.right}
        y1={zeroY}
        y2={zeroY}
        stroke="var(--text-faint, var(--text-muted))"
        strokeWidth={1}
        strokeDasharray="4 3"
      />
      <defs>
        <clipPath id="pc-gain">
          <rect x={PAD.left} y={PAD.top} width={WIDTH - PAD.left - PAD.right} height={Math.max(0, zeroY - PAD.top)} />
        </clipPath>
        <clipPath id="pc-loss">
          <rect x={PAD.left} y={zeroY} width={WIDTH - PAD.left - PAD.right} height={Math.max(0, height - PAD.bottom - zeroY)} />
        </clipPath>
      </defs>
      <path d={gainPath} fill="var(--gain, var(--green))" opacity={0.12} clipPath="url(#pc-gain)" />
      <path d={gainPath} fill="var(--loss, var(--red))" opacity={0.12} clipPath="url(#pc-loss)" />
      <path d={linePath} fill="none" stroke="var(--accent)" strokeWidth={2} strokeLinejoin="round" />
      {breakevens.map((be, i) => (
        <line
          key={i}
          x1={toX(be)}
          x2={toX(be)}
          y1={PAD.top}
          y2={height - PAD.bottom}
          stroke="var(--text-muted)"
          strokeWidth={1}
          strokeDasharray="2 2"
        />
      ))}
    </svg>
  );
}
