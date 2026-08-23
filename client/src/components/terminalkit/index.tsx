"use client";

/**
 * Terminal Kit — the shared primitives behind the upgraded terminals and screener.
 *
 * Everything here is presentational and pure: each component takes numbers and
 * renders them. None of them fetch, none of them own trading state, and none
 * of them decide anything — so they can be dropped into any of the three
 * modules without dragging a data model along.
 *
 * The design rule they all follow: MOTION MEANS SOMETHING CHANGED. A number
 * flashes because it moved and in which direction; a bar animates because a
 * ratio shifted; a row slides in because it is new. Nothing loops for
 * atmosphere. On a screen people read prices off, idle movement teaches the eye
 * to ignore exactly the movement that matters.
 */

import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";

// ── ticking value ───────────────────────────────────────────────────────────

/**
 * A number that flashes green or red when it changes.
 *
 * The flash is driven by comparing against the PREVIOUS RENDERED value held in
 * a ref, not by a prop the parent has to maintain. That matters because these
 * pages re-render on a poll: a parent-managed direction flag would either flash
 * on every poll (whether or not the price moved) or need the parent to
 * remember the last value for every field on screen.
 */
export function Ticking({
  value,
  format,
  className = "",
  as: Tag = "span",
}: {
  value: number | null | undefined;
  format: (v: number | null | undefined) => string;
  className?: string;
  as?: "span" | "div";
}) {
  const prev = useRef<number | null | undefined>(undefined);
  // The sequence lives in STATE, not a ref: it is part of the rendered `key`,
  // and a ref read during render is a value React does not know changed.
  const [flash, setFlash] = useState<{ dir: "up" | "down"; n: number } | null>(null);

  useEffect(() => {
    const before = prev.current;
    prev.current = value;
    if (before === undefined || value === null || value === undefined || before === null) return;
    if (value === before) return;
    // Bumping `n` remounts the node, so a second change mid-animation restarts
    // the flash rather than being swallowed by the still-running one.
    setFlash((f) => ({ dir: value > before ? "up" : "down", n: (f?.n ?? 0) + 1 }));
    const id = window.setTimeout(() => setFlash(null), 640);
    return () => window.clearTimeout(id);
  }, [value]);

  const dir = flash?.dir ?? null;
  return (
    <Tag
      key={flash ? `${flash.dir}-${flash.n}` : "idle"}
      className={`tk-num tk-flash ${dir === "up" ? "tk-flash-up" : dir === "down" ? "tk-flash-down" : ""} ${className}`}
    >
      {format(value)}
    </Tag>
  );
}

// ── sparkline ───────────────────────────────────────────────────────────────

/**
 * An inline price trace.
 *
 * Coloured by the net move over the window rather than by the last tick: a
 * series that fell all week and bounced in the final hour is red, which is what
 * the shape actually says. Colouring by the last bar would make it green and
 * contradict its own line.
 */
export function Sparkline({
  values,
  width = 74,
  height = 22,
  strokeWidth = 1.4,
}: {
  values: number[];
  width?: number;
  height?: number;
  strokeWidth?: number;
}) {
  const path = useMemo(() => {
    const v = values.filter((x) => Number.isFinite(x));
    if (v.length < 2) return null;
    const min = Math.min(...v);
    const max = Math.max(...v);
    const span = max - min;
    // A perfectly flat series has no span to normalise against; drawing it
    // through the middle is the honest rendering rather than dividing by zero.
    const y = (n: number) => (span === 0 ? height / 2 : height - ((n - min) / span) * (height - 2) - 1);
    const step = width / (v.length - 1);
    return {
      d: v.map((n, i) => `${i === 0 ? "M" : "L"}${(i * step).toFixed(2)},${y(n).toFixed(2)}`).join(" "),
      up: v[v.length - 1]! >= v[0]!,
      area:
        v.map((n, i) => `${i === 0 ? "M" : "L"}${(i * step).toFixed(2)},${y(n).toFixed(2)}`).join(" ") +
        ` L${width},${height} L0,${height} Z`,
    };
  }, [values, width, height]);

  if (!path) {
    return (
      <span style={{ display: "inline-block", width, height, opacity: 0.3, fontSize: 10 }} title="not enough history">
        —
      </span>
    );
  }
  const stroke = path.up ? "var(--desk-success)" : "var(--desk-error)";
  const id = `sp-${Math.abs(hash(path.d))}`;
  return (
    <svg className="tk-spark" width={width} height={height} viewBox={`0 0 ${width} ${height}`} aria-hidden>
      <defs>
        <linearGradient id={id} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={stroke} stopOpacity="0.24" />
          <stop offset="100%" stopColor={stroke} stopOpacity="0" />
        </linearGradient>
      </defs>
      <path d={path.area} fill={`url(#${id})`} />
      <path className="tk-spark-line" d={path.d} stroke={stroke} strokeWidth={strokeWidth} />
    </svg>
  );
}

function hash(s: string): number {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) | 0;
  return h;
}

// ── meters ──────────────────────────────────────────────────────────────────

/** A left-anchored bar: how far along a range a value sits. */
export function Meter({ pct, tone = "primary", title }: { pct: number; tone?: "primary" | "success" | "error" | "warning"; title?: string }) {
  const clamped = Math.max(0, Math.min(100, pct));
  return (
    <div className="tk-meter" title={title}>
      <div className="tk-meter-fill" style={{ left: 0, width: `${clamped}%`, background: `var(--desk-${tone})` }} />
    </div>
  );
}

/**
 * A meter anchored at the centre, filling out toward one side.
 *
 * For readings whose meaning is "how far from balanced, and which way" — book
 * imbalance being the obvious one. A left-anchored bar cannot express that: it
 * would render a perfectly balanced book as half full, which looks like a
 * deficit rather than parity.
 */
export function SplitMeter({ value, title }: { value: number; title?: string }) {
  const v = Math.max(-1, Math.min(1, value));
  const half = Math.abs(v) * 50;
  const positive = v >= 0;
  return (
    <div className="tk-meter tk-meter-split" title={title}>
      <div
        className="tk-meter-fill"
        style={{
          left: positive ? "50%" : `${50 - half}%`,
          width: `${half}%`,
          background: positive ? "var(--desk-success)" : "var(--desk-error)",
        }}
      />
      <span className="tk-meter-mid" />
    </div>
  );
}

// ── depth chart ─────────────────────────────────────────────────────────────

export type Level = { price: number; size: number };

/**
 * Cumulative depth either side of the touch.
 *
 * The ladder answers "what is resting at this price"; this answers "how much
 * would a market order of size N move me", which is the question a size box is
 * really asking. Cumulative, so the curve's steepness IS the slippage — a wall
 * shows as a cliff and a thin book as a gentle ramp.
 */
export function DepthChart({
  bids,
  asks,
  height = 92,
  precision = 2,
}: {
  bids: Level[];
  asks: Level[];
  height?: number;
  precision?: number;
}) {
  const built = useMemo(() => {
    if (bids.length === 0 || asks.length === 0) return null;
    const cum = (levels: Level[]) => {
      let running = 0;
      return levels.map((l) => {
        running += l.size;
        return { price: l.price, total: running };
      });
    };
    const b = cum(bids);
    const a = cum(asks);
    const maxTotal = Math.max(b[b.length - 1]?.total ?? 0, a[a.length - 1]?.total ?? 0);
    if (maxTotal <= 0) return null;
    const lo = Math.min(b[b.length - 1]!.price, bids[0]!.price);
    const hi = Math.max(a[a.length - 1]!.price, asks[0]!.price);
    if (!(hi > lo)) return null;
    return { b, a, maxTotal, lo, hi };
  }, [bids, asks]);

  if (!built) return null;
  const W = 100;
  const x = (p: number) => ((p - built.lo) / (built.hi - built.lo)) * W;
  const y = (t: number) => height - (t / built.maxTotal) * (height - 4) - 2;

  const bidPath =
    `M${x(built.b[0]!.price).toFixed(2)},${height} ` +
    built.b.map((p) => `L${x(p.price).toFixed(2)},${y(p.total).toFixed(2)}`).join(" ") +
    ` L${x(built.b[built.b.length - 1]!.price).toFixed(2)},${height} Z`;
  const askPath =
    `M${x(built.a[0]!.price).toFixed(2)},${height} ` +
    built.a.map((p) => `L${x(p.price).toFixed(2)},${y(p.total).toFixed(2)}`).join(" ") +
    ` L${x(built.a[built.a.length - 1]!.price).toFixed(2)},${height} Z`;

  return (
    <div>
      <svg width="100%" height={height} viewBox={`0 0 ${W} ${height}`} preserveAspectRatio="none" aria-hidden>
        <path d={bidPath} fill="color-mix(in srgb, var(--desk-success) 22%, transparent)" stroke="var(--desk-success)" strokeWidth="0.4" vectorEffect="non-scaling-stroke" />
        <path d={askPath} fill="color-mix(in srgb, var(--desk-error) 22%, transparent)" stroke="var(--desk-error)" strokeWidth="0.4" vectorEffect="non-scaling-stroke" />
      </svg>
      <div style={{ display: "flex", justifyContent: "space-between", fontSize: "0.62rem", opacity: 0.6, marginTop: 2 }} className="tk-num">
        <span>{built.lo.toFixed(precision)}</span>
        <span style={{ opacity: 0.8 }}>cumulative depth</span>
        <span>{built.hi.toFixed(precision)}</span>
      </div>
    </div>
  );
}

// ── countdown ───────────────────────────────────────────────────────────────

/**
 * Time until the next funding settlement (00:00 / 08:00 / 16:00 UTC).
 *
 * Worth its own component because funding is charged at the STAMP, not
 * pro-rata: a position opened four minutes before one pays a full interval, and
 * a trader looking at a rate with no clock beside it has no way to know that is
 * about to happen.
 */
export function FundingCountdown({ intervalHours = 8 }: { intervalHours?: number }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1_000);
    return () => clearInterval(t);
  }, []);
  const span = intervalHours * 3_600_000;
  const remaining = span - (now % span);
  const h = Math.floor(remaining / 3_600_000);
  const m = Math.floor((remaining % 3_600_000) / 60_000);
  const s = Math.floor((remaining % 60_000) / 1_000);
  const soon = remaining < 10 * 60_000;
  return (
    <span
      className="tk-num"
      style={{ color: soon ? "var(--desk-warning)" : undefined, fontWeight: soon ? 700 : 600 }}
      title="Funding settles at the stamp, not pro-rata — a position opened a minute before one still pays a full interval."
    >
      {String(h).padStart(2, "0")}:{String(m).padStart(2, "0")}:{String(s).padStart(2, "0")}
    </span>
  );
}

// ── live dot ────────────────────────────────────────────────────────────────

/**
 * Feed health, from the age of the data rather than from a boolean.
 *
 * A "connected" flag stays green while a socket is open and silent, which is
 * the exact failure it is supposed to catch. Age cannot lie the same way.
 */
export function LiveDot({ asOfMs, staleAfterMs = 30_000, downAfterMs = 120_000 }: { asOfMs: number | null; staleAfterMs?: number; downAfterMs?: number }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 5_000);
    return () => clearInterval(t);
  }, []);
  if (asOfMs === null) return <span className="tk-live-dot" data-state="down" title="no data received" />;
  const age = now - asOfMs;
  const state = age > downAfterMs ? "down" : age > staleAfterMs ? "stale" : "live";
  const label =
    state === "live" ? `live · updated ${Math.round(age / 1000)}s ago` : state === "stale" ? `stale · ${Math.round(age / 1000)}s old` : `no update in ${Math.round(age / 1000)}s`;
  return <span className="tk-live-dot" data-state={state === "live" ? undefined : state} title={label} />;
}

// ── panel ───────────────────────────────────────────────────────────────────

export function Panel({
  title,
  actions,
  children,
  accent = false,
  padding = 14,
}: {
  title?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  accent?: boolean;
  padding?: number;
}) {
  return (
    <section className={`tk-panel ${accent ? "tk-panel-accent" : ""}`}>
      {title ? (
        <header
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 10,
            padding: "9px 14px",
            borderBottom: "1px solid var(--desk-outline-variant)",
          }}
        >
          <span className="desk-label-md" style={{ fontWeight: 700, letterSpacing: "0.04em", textTransform: "uppercase", fontSize: "0.68rem" }}>
            {title}
          </span>
          {actions}
        </header>
      ) : null}
      <div style={{ padding }}>{children}</div>
    </section>
  );
}

// ── segmented control ───────────────────────────────────────────────────────

export function Segmented<T extends string>({
  options,
  value,
  onChange,
  size = "md",
}: {
  options: { key: T; label: string; title?: string }[];
  value: T;
  onChange: (v: T) => void;
  size?: "sm" | "md";
}) {
  return (
    <div className="tk-seg" role="group">
      {options.map((o) => (
        <button
          key={o.key}
          type="button"
          className="tk-seg-btn"
          aria-pressed={o.key === value}
          title={o.title}
          onClick={() => onChange(o.key)}
          style={size === "sm" ? { padding: "3px 9px", minHeight: 26, fontSize: "0.69rem" } : undefined}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

// ── stat ────────────────────────────────────────────────────────────────────

/** A compact labelled figure for the dense header strips. */
export function Stat({
  label,
  value,
  sub,
  tone,
  title,
  spark,
}: {
  label: string;
  value: ReactNode;
  sub?: ReactNode;
  tone?: "success" | "error" | "warning" | null;
  title?: string;
  spark?: number[];
}) {
  return (
    <div title={title} style={{ minWidth: 0 }}>
      <div className="desk-label-md" style={{ fontWeight: 500, opacity: 0.62, fontSize: "0.64rem", textTransform: "uppercase", letterSpacing: "0.06em", whiteSpace: "nowrap" }}>
        {label}
      </div>
      <div
        className="tk-num"
        style={{
          fontSize: "1.02rem",
          fontWeight: 700,
          marginTop: 2,
          color: tone ? `var(--desk-${tone})` : undefined,
          whiteSpace: "nowrap",
        }}
      >
        {value}
      </div>
      {spark && spark.length > 1 ? (
        <div style={{ marginTop: 1 }}>
          <Sparkline values={spark} width={64} height={16} />
        </div>
      ) : sub ? (
        <div className="desk-label-md" style={{ fontWeight: 400, opacity: 0.6, fontSize: "0.64rem", whiteSpace: "nowrap" }}>
          {sub}
        </div>
      ) : null}
    </div>
  );
}

// ── drawer ──────────────────────────────────────────────────────────────────

export function Drawer({ open, onClose, title, subtitle, children }: { open: boolean; onClose: () => void; title: ReactNode; subtitle?: ReactNode; children: ReactNode }) {
  useEffect(() => {
    if (!open) return undefined;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    // The page behind a drawer must not scroll with it — otherwise closing
    // returns the reader somewhere they never navigated to.
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      window.removeEventListener("keydown", onKey);
      document.body.style.overflow = prevOverflow;
    };
  }, [open, onClose]);

  if (!open) return null;
  return (
    <>
      <div className="tk-drawer-scrim" onClick={onClose} aria-hidden />
      <aside className="tk-drawer" role="dialog" aria-modal="true">
        <header
          style={{
            position: "sticky",
            top: 0,
            zIndex: 1,
            display: "flex",
            alignItems: "flex-start",
            justifyContent: "space-between",
            gap: 12,
            padding: "16px 20px",
            background: "var(--desk-surface)",
            borderBottom: "1px solid var(--desk-outline)",
          }}
        >
          <div style={{ minWidth: 0 }}>
            <h2 className="desk-title-md" style={{ margin: 0 }}>
              {title}
            </h2>
            {subtitle ? (
              <p className="desk-label-md" style={{ fontWeight: 400, opacity: 0.7, marginTop: 3 }}>
                {subtitle}
              </p>
            ) : null}
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            style={{
              flexShrink: 0,
              width: 34,
              height: 34,
              borderRadius: 9,
              border: "1px solid var(--desk-outline)",
              background: "var(--desk-surface-container)",
              color: "var(--desk-on-surface-variant)",
              cursor: "pointer",
              fontSize: 16,
              lineHeight: 1,
            }}
          >
            ×
          </button>
        </header>
        <div style={{ padding: 20 }}>{children}</div>
      </aside>
    </>
  );
}

// ── keyboard hint ───────────────────────────────────────────────────────────

export function Kbd({ children }: { children: ReactNode }) {
  return <span className="tk-kbd">{children}</span>;
}

/** Heat colour for a signed percentage, used by the sector map. */
export function heatColor(value: number | null, scale = 8): string {
  if (value === null || !Number.isFinite(value)) return "var(--desk-surface-container)";
  const t = Math.max(-1, Math.min(1, value / scale));
  const strength = 12 + Math.abs(t) * 46;
  return t >= 0
    ? `color-mix(in srgb, var(--desk-success) ${strength}%, var(--desk-surface))`
    : `color-mix(in srgb, var(--desk-error) ${strength}%, var(--desk-surface))`;
}
