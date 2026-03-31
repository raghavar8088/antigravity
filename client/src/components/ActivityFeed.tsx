"use client";

type FeedEntry = {
  id: string;
  message: string;
  tone: "info" | "buy" | "sell" | "win" | "loss" | "admin";
  time: number;
};

const toneClasses: Record<FeedEntry["tone"], string> = {
  info: "text-zinc-300",
  buy: "text-sky-300",
  sell: "text-rose-300",
  win: "text-emerald-300",
  loss: "text-red-300",
  admin: "text-amber-300",
};

export default function ActivityFeed({ entries }: { entries: FeedEntry[] }) {
  if (entries.length === 0) {
    return (
      <div className="rounded-2xl border border-zinc-800/80 bg-zinc-950/70 p-8 text-center text-sm text-zinc-500">
        Waiting for market and engine activity...
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-zinc-800/80 bg-zinc-950/70">
      <div className="border-b border-zinc-800/80 px-5 py-4">
        <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-zinc-400">
          Activity Feed
        </h3>
      </div>
      <div className="max-h-[420px] overflow-y-auto px-5 py-3">
        {entries.map((entry) => (
          <div
            key={entry.id}
            className="flex items-center justify-between gap-4 border-b border-zinc-900/80 py-3 last:border-b-0"
          >
            <div className={`text-sm ${toneClasses[entry.tone]}`}>
              {entry.message}
            </div>
            <div className="shrink-0 text-xs font-mono text-zinc-500">
              {new Date(entry.time).toLocaleTimeString()}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
