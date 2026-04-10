"use client";

import useLiveDataLab, { AngelOneProbeData, DeltaProbeData } from "@/hooks/useLiveDataLab";

function fmt(n: number, decimals = 2): string {
  if (!Number.isFinite(n)) return "-";
  return n.toLocaleString("en-US", {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
}

function StatusDot({ ok }: { ok: boolean }) {
  return (
    <span
      className="inline-block rounded-full"
      style={{
        width: 8,
        height: 8,
        background: ok ? "#22c55e" : "#ef4444",
        flexShrink: 0,
      }}
    />
  );
}

function ProbeStateBanner({
  label,
  detail,
  tone,
}: {
  label: string;
  detail: string;
  tone: "warning" | "error";
}) {
  const styles = tone === "error"
    ? { borderColor: "rgba(239, 68, 68, 0.18)", background: "rgba(239, 68, 68, 0.08)", color: "#b91c1c" }
    : { borderColor: "rgba(245, 158, 11, 0.18)", background: "rgba(245, 158, 11, 0.08)", color: "#b45309" };

  return (
    <div className="rounded-[18px] border px-4 py-3 text-xs" style={styles}>
      <div className="font-semibold uppercase tracking-[0.12em]">{label}</div>
      <div className="mt-1 normal-case tracking-normal">{detail}</div>
    </div>
  );
}

function OhlRow({ open, high, low }: { open: number; high: number; low: number }) {
  return (
    <div className="flex gap-4 text-xs" style={{ color: "var(--text-secondary)" }}>
      <span>O <span style={{ color: "var(--text-primary, #e4e4e7)" }}>{fmt(open)}</span></span>
      <span>H <span style={{ color: "#22c55e" }}>{fmt(high)}</span></span>
      <span>L <span style={{ color: "#ef4444" }}>{fmt(low)}</span></span>
    </div>
  );
}

function BTCCard({ data, loading, error }: { data: DeltaProbeData | null; loading: boolean; error: string }) {
  const ok = data?.ok === true && !error;

  return (
    <div
      className="glass-panel flex-1 px-5 py-5 flex flex-col gap-3"
      style={{ minWidth: 0 }}
    >
      <div className="flex items-center gap-2">
        <StatusDot ok={ok} />
        <span
          className="text-[10px] font-semibold uppercase tracking-[0.18em]"
          style={{ color: "var(--text-secondary)" }}
        >
          Delta Exchange — BTC
        </span>
        {loading && (
          <span className="ml-auto text-[10px]" style={{ color: "var(--text-muted, #71717a)" }}>
            Fetching…
          </span>
        )}
      </div>

      {data && data.ok ? (
        <>
          <div
            className="text-[clamp(1.9rem,4vw,2.6rem)] font-semibold leading-none tracking-tight"
            style={{ color: "var(--text-primary, #e4e4e7)" }}
          >
            ${fmt(data.price)}
          </div>

          <OhlRow open={data.open} high={data.high} low={data.low} />

          <div className="flex flex-wrap gap-4 text-xs" style={{ color: "var(--text-secondary)" }}>
            <span>
              Mark{" "}
              <span style={{ color: "var(--text-primary, #e4e4e7)" }}>${fmt(data.mark_price)}</span>
            </span>
            <span>
              Spot{" "}
              <span style={{ color: "var(--text-primary, #e4e4e7)" }}>${fmt(data.spot_price)}</span>
            </span>
            <span>
              Vol{" "}
              <span style={{ color: "var(--text-primary, #e4e4e7)" }}>{fmt(data.volume, 1)}</span>
            </span>
          </div>

          {data.contract_type && (
            <div className="text-[10px] uppercase tracking-[0.14em]" style={{ color: "var(--text-muted, #71717a)" }}>
              {data.contract_type.replace(/_/g, " ")}
            </div>
          )}

          <div className="text-[10px]" style={{ color: "var(--text-muted, #71717a)" }}>
            Updated {data.fetched_at ? new Date(data.fetched_at).toLocaleTimeString() : "-"}
          </div>
          {!loading && !ok && (
            <ProbeStateBanner
              label="Probe State"
              detail="BTC probe is offline. Use Refresh to retry or inspect the probe endpoint if this persists."
              tone={error ? "error" : "warning"}
            />
          )}
        </>
      ) : (
        <>
          <div
            className="text-[clamp(1.9rem,4vw,2.6rem)] font-semibold leading-none tracking-tight"
            style={{ color: "var(--text-muted, #71717a)" }}
          >
            —
          </div>
          {(error || (data && !data.ok && data.error)) && (
            <div className="text-xs" style={{ color: "#ef4444" }}>
              {error || data?.error}
            </div>
          )}
          {!loading && !error && !data && (
            <div className="text-xs" style={{ color: "var(--text-muted, #71717a)" }}>
              Waiting for first response…
            </div>
          )}
        </>
      )}
    </div>
  );
}

function NiftyCard({ data, loading, error }: { data: AngelOneProbeData | null; loading: boolean; error: string }) {
  const ok = data?.ok === true && !error;
  const notConfigured = data?.configured === false;

  return (
    <div
      className="glass-panel flex-1 px-5 py-5 flex flex-col gap-3"
      style={{ minWidth: 0 }}
    >
      <div className="flex items-center gap-2">
        <StatusDot ok={ok} />
        <span
          className="text-[10px] font-semibold uppercase tracking-[0.18em]"
          style={{ color: "var(--text-secondary)" }}
        >
          Angel One — NIFTY 50
        </span>
        {loading && (
          <span className="ml-auto text-[10px]" style={{ color: "var(--text-muted, #71717a)" }}>
            Fetching…
          </span>
        )}
      </div>

      {data && data.ok ? (
        <>
          <div
            className="text-[clamp(1.9rem,4vw,2.6rem)] font-semibold leading-none tracking-tight"
            style={{ color: "var(--text-primary, #e4e4e7)" }}
          >
            ₹{fmt(data.price)}
          </div>

          <OhlRow open={data.open} high={data.high} low={data.low} />

          <div className="flex flex-wrap gap-4 text-xs" style={{ color: "var(--text-secondary)" }}>
            <span>
              Chg{" "}
              <span style={{ color: data.change >= 0 ? "#22c55e" : "#ef4444" }}>
                {data.change >= 0 ? "+" : ""}
                {fmt(data.change)}
              </span>
            </span>
            <span>
              Chg%{" "}
              <span style={{ color: data.percent_change >= 0 ? "#22c55e" : "#ef4444" }}>
                {data.percent_change >= 0 ? "+" : ""}
                {fmt(data.percent_change, 2)}%
              </span>
            </span>
            <span>
              Close{" "}
              <span style={{ color: "var(--text-primary, #e4e4e7)" }}>₹{fmt(data.close)}</span>
            </span>
          </div>

          {data.exchange && (
            <div className="text-[10px] uppercase tracking-[0.14em]" style={{ color: "var(--text-muted, #71717a)" }}>
              {data.exchange} · {data.symbol_token}
            </div>
          )}

          <div className="text-[10px]" style={{ color: "var(--text-muted, #71717a)" }}>
            Updated {data.fetched_at ? new Date(data.fetched_at).toLocaleTimeString() : "-"}
          </div>
        </>
      ) : (
        <>
          <div
            className="text-[clamp(1.9rem,4vw,2.6rem)] font-semibold leading-none tracking-tight"
            style={{ color: "var(--text-muted, #71717a)" }}
          >
            —
          </div>
          {notConfigured && (
            <div className="text-xs" style={{ color: "#f59e0b" }}>
              Not configured — set ANGELONE_API_KEY, ANGELONE_CLIENT_CODE, ANGELONE_PIN, and ANGELONE_TOTP_SECRET in .env to enable.
            </div>
          )}
          {!notConfigured && (error || (data && !data.ok && data.error)) && (
            <div className="text-xs" style={{ color: "#ef4444" }}>
              {error || data?.error}
            </div>
          )}
          {!loading && !error && !data && (
            <div className="text-xs" style={{ color: "var(--text-muted, #71717a)" }}>
              Waiting for first response…
            </div>
          )}
        </>
      )}
    </div>
  );
}

export default function LiveDataLabPanel() {
  const { deltaData, angelData, deltaLoading, angelLoading, deltaError, angelError, refresh } =
    useLiveDataLab();

  return (
    <div className="space-y-5">
      <div className="glass-panel px-5 py-4 flex items-center justify-between gap-4">
        <div>
          <div
            className="text-[10px] font-semibold uppercase tracking-[0.18em]"
            style={{ color: "var(--text-secondary)" }}
          >
            Live Data Lab
          </div>
          <div className="mt-1 text-xs" style={{ color: "var(--text-muted, #71717a)" }}>
            Probe connectivity to Delta Exchange (BTC) and Angel One SmartAPI (NIFTY 50). Polls every 10 seconds.
          </div>
        </div>
        <button
          onClick={refresh}
          className="groww-tab"
          style={{ flexShrink: 0 }}
        >
          Refresh
        </button>
      </div>

      <div className="flex flex-col gap-5 md:flex-row">
        <BTCCard data={deltaData} loading={deltaLoading} error={deltaError} />
        <NiftyCard data={angelData} loading={angelLoading} error={angelError} />
      </div>
    </div>
  );
}
