"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

type DatesRes = { ok: boolean; days?: string[]; disabled?: boolean; reason?: string; error?: string };

export default function TradeHistoryArchivePage() {
  const [days, setDays] = useState<string[]>([]);
  const [disabled, setDisabled] = useState(false);
  const [reason, setReason] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/trade-history/dates", { cache: "no-store" });
      const j = (await res.json()) as DatesRes;
      if (!j.ok) {
        setError(j.error ?? "Failed to load dates");
        setDays([]);
        return;
      }
      if (j.disabled) {
        setDisabled(true);
        setReason(j.reason ?? "Database not configured");
        setDays([]);
        return;
      }
      setDisabled(false);
      setReason(null);
      setDays(j.days ?? []);
    } catch (e) {
      setError(String(e));
      setDays([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return (
    <div className="min-h-screen bg-zinc-950 text-zinc-100">
      <div className="mx-auto max-w-3xl px-6 py-10">
        <div className="mb-8 flex flex-wrap items-center justify-between gap-4">
          <div>
            <h1 className="text-xl font-semibold tracking-tight">Daily trade archive</h1>
            <p className="mt-2 text-sm text-zinc-400">
              One Excel workbook per UTC calendar day, aggregating archived trades from BTC futures, BTC options,
              NIFTY options, stocks, MCX, and crypto equity. Rows are stored in Postgres and are not removed when
              you reset paper accounts, as long as they were captured by a daily archive run.
            </p>
          </div>
          <Link
            href="/"
            className="rounded-md border border-zinc-600 px-3 py-1.5 text-sm text-zinc-200 hover:bg-zinc-900"
          >
            ← Workspace
          </Link>
        </div>

        {loading ? (
          <p className="text-sm text-zinc-400">Loading…</p>
        ) : error ? (
          <p className="text-sm text-red-400">{error}</p>
        ) : disabled ? (
          <p className="text-sm text-amber-300">{reason}</p>
        ) : days.length === 0 ? (
          <div className="rounded-lg border border-zinc-800 bg-zinc-900/50 p-4 text-sm text-zinc-400">
            <p>No archive days yet. After you set <code className="text-zinc-300">DATABASE_URL</code> and{" "}
            <code className="text-zinc-300">CRON_SECRET</code>, schedule the daily job (Vercel Cron or any cron client)
            to call:
          </p>
            <pre className="mt-3 overflow-x-auto rounded bg-black/40 p-3 text-xs text-zinc-300">
{`GET /api/trade-history/daily-run
Authorization: Bearer <CRON_SECRET>

# Optional: archive a specific UTC day
GET /api/trade-history/daily-run?date=2026-04-21&secret=<CRON_SECRET>`}
            </pre>
            <p className="mt-3">
              By default the job archives <strong>yesterday (UTC)</strong>. Each run upserts into the archive; reruns
              for the same day do not duplicate rows.
            </p>
          </div>
        ) : (
          <ul className="divide-y divide-zinc-800 rounded-lg border border-zinc-800 bg-zinc-900/40">
            {days.map((d) => (
              <li key={d} className="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
                <span className="font-mono text-sm text-zinc-200">{d}</span>
                <a
                  className="rounded-md bg-emerald-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-600"
                  href={`/api/trade-history/download?date=${encodeURIComponent(d)}`}
                >
                  Download .xlsx
                </a>
              </li>
            ))}
          </ul>
        )}

        <p className="mt-8 text-xs text-zinc-500">
          Tip: same-day resets can still drop trades before the nightly run captures them. If you need finer retention,
          run the archive endpoint more than once per day with explicit <code className="text-zinc-400">date</code>{" "}
          values.
        </p>
      </div>
    </div>
  );
}
