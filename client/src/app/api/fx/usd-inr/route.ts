/**
 * USD → INR rate for displaying desk money in rupees.
 *
 * Server-side so the rate is fetched once and shared, and so a provider outage
 * is visible here rather than in every component.
 *
 * The rate is returned WITH its age. A converted figure is only as true as the
 * rate behind it, and money shown to a precision the rate does not support is
 * the kind of number people act on. If the fetch fails this returns ok:false
 * and no rate — the caller is expected to keep showing dollars rather than
 * convert with a guess, because a wrong rate silently restates every P&L on
 * the page.
 */
import { NextResponse } from "next/server";

const PROVIDER = "https://open.er-api.com/v6/latest/USD";

/** Cached for an hour; the provider itself updates daily. */
export const revalidate = 3600;

export async function GET() {
  try {
    const res = await fetch(PROVIDER, {
      next: { revalidate },
      signal: AbortSignal.timeout(15_000),
    });
    if (!res.ok) {
      return NextResponse.json({ ok: false, error: `rate provider HTTP ${res.status}` }, { status: 502 });
    }

    const data = (await res.json()) as {
      result?: string;
      rates?: Record<string, number>;
      time_last_update_utc?: string;
    };

    const rate = data.rates?.INR;
    // A zero or missing rate must not pass as a number: it would render every
    // amount on the desk as ₹0.00, which reads as "no money at stake".
    if (data.result !== "success" || typeof rate !== "number" || !Number.isFinite(rate) || rate <= 0) {
      return NextResponse.json({ ok: false, error: "rate provider returned no usable INR rate" }, { status: 502 });
    }

    return NextResponse.json({
      ok: true,
      rate,
      asOf: data.time_last_update_utc ?? null,
      provider: "open.er-api.com",
    });
  } catch (e) {
    return NextResponse.json(
      { ok: false, error: e instanceof Error ? e.message : String(e) },
      { status: 502 },
    );
  }
}
