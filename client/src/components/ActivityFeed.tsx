"use client";

type FeedEntry = {
  id: string;
  message: string;
  tone: "info" | "buy" | "sell" | "win" | "loss" | "admin";
  time: number;
};

const toneClasses: Record<FeedEntry["tone"], string> = {
  info: "text-slate-700",
  buy: "text-blue-700",
  sell: "text-rose-700",
  win: "text-emerald-700",
  loss: "text-red-700",
  admin: "text-amber-700",
};

const toneDots: Record<FeedEntry["tone"], string> = {
  info: "#1a73e8",
  buy: "#1a73e8",
  sell: "#d93025",
  win: "#188038",
  loss: "#d93025",
  admin: "#b06000",
};

export default function ActivityFeed({ entries }: { entries: FeedEntry[] }) {
  if (entries.length === 0) {
    return (
      <div className="glass-panel p-8 text-center text-sm" style={{ color: "var(--text-secondary)" }}>
        Waiting for market and engine activity...
      </div>
    );
  }

  return (
    <div className="glass-panel">
      <div className="flex items-center justify-between border-b px-5 py-4" style={{ borderColor: "var(--border)" }}>
        <h3 className="text-sm font-medium" style={{ color: "var(--text-primary)" }}>
          Activity feed
        </h3>
        <div className="text-xs font-mono" style={{ color: "var(--text-secondary)" }}>
          {entries.length} events
        </div>
      </div>
      <div className="max-h-[420px] overflow-y-auto px-5 py-2">
        {entries.map((entry) => (
          <div
            key={entry.id}
            className="flex items-start justify-between gap-4 border-b py-4 last:border-b-0"
            style={{ borderColor: "var(--border-subtle)" }}
          >
            <div className="flex items-start gap-3">
              <span
                className="mt-1.5 h-2.5 w-2.5 rounded-full"
                style={{ background: toneDots[entry.tone] }}
              />
              <div className={`text-sm ${toneClasses[entry.tone]}`}>
                {entry.message}
              </div>
            </div>
            <div className="shrink-0 text-xs font-mono" style={{ color: "var(--text-secondary)" }}>
              {new Date(entry.time).toLocaleTimeString([], {
                hour: "2-digit",
                minute: "2-digit",
                second: "2-digit",
              })}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
