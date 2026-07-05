"use client";

export interface HistogramBucket {
  from: number;
  to: number;
  count: number;
}

export interface HistogramProps {
  buckets: HistogramBucket[];
}

export function Histogram({ buckets }: HistogramProps) {
  if (buckets.length === 0) return null;
  const maxCount = Math.max(1, ...buckets.map((b) => b.count));

  return (
    <div style={{ padding: "12px 16px 8px" }}>
      <div style={{ display: "flex", alignItems: "flex-end", gap: 2, height: 140 }}>
        {buckets.map((b, i) => {
          const mid = (b.from + b.to) / 2;
          const color = mid >= 0 ? "var(--gain, var(--green))" : "var(--loss, var(--red))";
          const h = Math.max(2, (b.count / maxCount) * 140);
          return (
            <div
              key={i}
              title={`${b.from.toFixed(2)} to ${b.to.toFixed(2)}: ${b.count}`}
              style={{ flex: 1, height: h, background: color, borderRadius: "4px 4px 0 0" }}
            />
          );
        })}
      </div>
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          marginTop: 6,
          fontFamily: "var(--font-mono)",
          fontVariantNumeric: "tabular-nums",
          fontSize: 10.5,
          color: "var(--text-faint, var(--text-muted))",
        }}
      >
        <span>{buckets[0].from.toFixed(2)}</span>
        <span>{buckets[buckets.length - 1].to.toFixed(2)}</span>
      </div>
    </div>
  );
}
