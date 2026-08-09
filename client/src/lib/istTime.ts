/**
 * IST (Asia/Kolkata) time formatting — the one place the app converts instants
 * for display.
 *
 * Every desk here is operated from India, including the crypto desks that trade
 * around the clock. Rendering UTC meant an operator had to add 5:30 in their
 * head to answer "did this fire during the session?", and a trade closed at
 * 02:00 IST showed as the *previous* calendar day. Trade times are read under
 * time pressure; a format that needs mental arithmetic is a format that gets
 * misread.
 *
 * Fixed offset rather than `Intl` + `timeZone: "Asia/Kolkata"`, deliberately:
 *
 *   - India has observed no DST since 1945 and IST is UTC+05:30 year-round, so
 *     a fixed offset is not an approximation here — it is exact.
 *   - It needs no tzdata. Slim server images frequently ship without it, and
 *     `Intl` silently falls back to UTC when the zone is unknown: the label
 *     would read IST while the digits stayed UTC, which is worse than showing
 *     UTC honestly.
 *   - It renders identically on server and client, so no hydration mismatch.
 *
 * Times render on a 12-hour clock with AM/PM, matching how the operator reads
 * a clock anywhere else on their machine.
 *
 * Storage and wire formats stay UTC. This module is display only.
 */

const IST_OFFSET_MS = (5 * 60 + 30) * 60 * 1000;

/** Label to append wherever a rendered time could be mistaken for UTC. */
export const IST_LABEL = "IST";

type Parts = { y: string; mo: string; d: string; h: string; mi: string; s: string; ap: string };

/**
 * Shift the instant by +05:30, then read it back with the UTC getters. Reading
 * with UTC getters is what makes this independent of whatever zone the browser
 * or server happens to be in.
 */
function istParts(input: string | number | Date | null | undefined): Parts | null {
  if (input === null || input === undefined || input === "") return null;
  const t = new Date(input);
  if (Number.isNaN(t.getTime())) return null;
  const s = new Date(t.getTime() + IST_OFFSET_MS);
  const p2 = (n: number) => String(n).padStart(2, "0");
  const h24 = s.getUTCHours();
  // 12-hour clock: midnight and noon are both 12, not 0.
  const h12 = h24 % 12 === 0 ? 12 : h24 % 12;
  return {
    y: String(s.getUTCFullYear()),
    mo: p2(s.getUTCMonth() + 1),
    d: p2(s.getUTCDate()),
    // Padded, even though a wall clock writes 1:05 PM. These render in dense
    // monospace tables where a ragged hour column costs more legibility than
    // the leading zero does.
    h: p2(h12),
    mi: p2(s.getUTCMinutes()),
    s: p2(s.getUTCSeconds()),
    ap: h24 < 12 ? "AM" : "PM",
  };
}

/** "MM-DD hh:mm AM/PM" — the compact form for dense trade tables. */
export function fmtIST(input: string | number | Date | null | undefined, fallback = "—"): string {
  const p = istParts(input);
  return p ? `${p.mo}-${p.d} ${p.h}:${p.mi} ${p.ap}` : fallback;
}

/** "MM-DD hh:mm:ss AM/PM" — where the second matters, such as audit trails. */
export function fmtISTSeconds(input: string | number | Date | null | undefined, fallback = "—"): string {
  const p = istParts(input);
  return p ? `${p.mo}-${p.d} ${p.h}:${p.mi}:${p.s} ${p.ap}` : fallback;
}

/** "hh:mm AM/PM" — clock to the minute, where seconds are noise. */
export function fmtISTClockShort(input: string | number | Date | null | undefined, fallback = "—"): string {
  const p = istParts(input);
  return p ? `${p.h}:${p.mi} ${p.ap}` : fallback;
}

/** "hh:mm:ss AM/PM" — clock only, for "updated at" style captions. */
export function fmtISTClock(input: string | number | Date | null | undefined, fallback = "—"): string {
  const p = istParts(input);
  return p ? `${p.h}:${p.mi}:${p.s} ${p.ap}` : fallback;
}

/** "YYYY-MM-DD hh:mm:ss AM/PM" — the unambiguous long form. */
export function fmtISTFull(input: string | number | Date | null | undefined, fallback = "—"): string {
  const p = istParts(input);
  return p ? `${p.y}-${p.mo}-${p.d} ${p.h}:${p.mi}:${p.s} ${p.ap}` : fallback;
}

/**
 * "YYYY-MM-DD" for the IST calendar day an instant falls in.
 *
 * Use this for any client-side day bucketing. Note that a bucket is only
 * genuinely IST if whatever produced the rows grouped them the same way — a
 * server that grouped by UTC day cannot be corrected by relabelling it here.
 */
export function istDayKey(input: string | number | Date | null | undefined, fallback = ""): string {
  const p = istParts(input);
  return p ? `${p.y}-${p.mo}-${p.d}` : fallback;
}

/**
 * Format a plain "YYYY-MM-DD" that is ALREADY an IST calendar day.
 *
 * Takes the string apart rather than parsing it as a Date: `new Date("2026-08-09")`
 * is midnight UTC, which is 05:30 IST the same day but would shift to the
 * previous day under any negative-offset rendering. Reformatting a date that
 * carries no time of day must never go through an instant.
 */
export function fmtISTDayLabel(day: string | null | undefined, fallback = "—"): string {
  if (!day) return fallback;
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(day.trim());
  return m ? `${m[1]}-${m[2]}-${m[3]}` : day;
}
