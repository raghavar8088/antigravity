"use client";

export interface SparklineProps {
  data: number[];
  width?: number;
  height?: number;
  /** Explicit color — defaults to profit green or loss red based on net change */
  color?: string;
  strokeWidth?: number;
  className?: string;
  /** Render at 100% container width (viewBox coordinate space still uses `width`), for full-bleed chart cards rather than small inline tiles. */
  stretch?: boolean;
}

export function Sparkline({ data, width = 80, height = 24, color, strokeWidth = 1.5, className, stretch = false }: SparklineProps) {
  if (data.length < 2) return null;

  const min = Math.min(...data);
  const max = Math.max(...data);
  const range = max - min || 1;

  const xStep = width / (data.length - 1);
  const points = data.map((v, i) => {
    const x = i * xStep;
    const y = height - ((v - min) / range) * (height - 2) - 1;
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  });

  const polyline = points.join(" ");
  const netChange = data[data.length - 1] - data[0];
  const lineColor = color ?? (netChange >= 0 ? "var(--color-profit)" : "var(--color-loss)");

  // Area fill path: polyline + bottom-right + bottom-left
  const first = points[0].split(",");
  const last = points[points.length - 1].split(",");
  const areaPath = `M${polyline.replace(/ /g, "L")} L${last[0]},${height} L${first[0]},${height} Z`;

  return (
    <svg
      width={stretch ? "100%" : width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      preserveAspectRatio={stretch ? "none" : undefined}
      className={className}
      aria-hidden
    >
      <defs>
        <linearGradient id={`sg-${width}-${height}`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={lineColor} stopOpacity={0.2} />
          <stop offset="100%" stopColor={lineColor} stopOpacity={0} />
        </linearGradient>
      </defs>
      <path d={areaPath} fill={`url(#sg-${width}-${height})`} />
      <polyline
        points={polyline}
        fill="none"
        stroke={lineColor}
        strokeWidth={strokeWidth}
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  );
}
