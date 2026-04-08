import { NextResponse } from "next/server";
import { getAngelJWT, isAngelConfigured, commonHeaders, BASE_URL } from "@/lib/angelAuth";

const INTERNAL_API_URL = process.env.INTERNAL_API_URL?.replace(/\/$/, "") ?? "http://localhost:8080";

function pad2(n: number): string {
  return String(n).padStart(2, "0");
}

/** Format a Date as "YYYY-MM-DD HH:mm" in IST (UTC+5:30) */
function toISTString(date: Date): string {
  const istOffset = 5 * 60 + 30; // minutes
  const istMs = date.getTime() + istOffset * 60 * 1000;
  const d = new Date(istMs);
  const yyyy = d.getUTCFullYear();
  const mm = pad2(d.getUTCMonth() + 1);
  const dd = pad2(d.getUTCDate());
  const hh = pad2(d.getUTCHours());
  const min = pad2(d.getUTCMinutes());
  return `${yyyy}-${mm}-${dd} ${hh}:${min}`;
}

export async function POST() {
  if (!isAngelConfigured()) {
    return NextResponse.json({ ok: false, error: "Angel One not configured" });
  }

  try {
    const jwt = await getAngelJWT();

    // Build IST date range: today 09:00 to 15:30 (or now if market is open)
    const now = new Date();
    const istOffset = (5 * 60 + 30) * 60 * 1000;
    const nowIST = new Date(now.getTime() + istOffset);

    const fromUTC = new Date(Date.UTC(nowIST.getUTCFullYear(), nowIST.getUTCMonth(), nowIST.getUTCDate(), 3, 30)); // 09:00 IST
    const fromdate = toISTString(fromUTC);

    const toUTC930 = new Date(Date.UTC(nowIST.getUTCFullYear(), nowIST.getUTCMonth(), nowIST.getUTCDate(), 10, 0)); // 15:30 IST
    const toUTC = now < toUTC930 ? now : toUTC930;
    const todate = toISTString(toUTC);

    // Fetch 1-min candles from Angel One
    const candleRes = await fetch(`${BASE_URL}/rest/secure/angelbroking/historical/v1/getCandleData`, {
      method: "POST",
      headers: commonHeaders(jwt),
      body: JSON.stringify({
        exchange: "NSE",
        symboltoken: "99926000",
        interval: "ONE_MINUTE",
        fromdate,
        todate,
      }),
      next: { revalidate: 0 },
    });

    if (!candleRes.ok) {
      return NextResponse.json({ ok: false, error: `Angel One candle fetch returned ${candleRes.status}` });
    }

    const payload = await candleRes.json() as {
      status?: boolean;
      message?: string;
      errorcode?: string;
      data?: unknown[][];
    };

    if (!payload.status || !Array.isArray(payload.data)) {
      const msg = [payload.message, payload.errorcode].filter(Boolean).join(" ") || "Candle fetch failed";
      return NextResponse.json({ ok: false, error: msg });
    }

    // Extract close prices in order
    const closePrices = payload.data.map((row) => Number(row[4]) || 0).filter((v) => v > 0);

    if (closePrices.length === 0) {
      return NextResponse.json({ ok: false, error: "No valid candle data returned" });
    }

    // POST to Go engine
    const engineRes = await fetch(`${INTERNAL_API_URL}/api/nifty-options/inject-candles`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ close_prices: closePrices }),
    });

    if (!engineRes.ok) {
      return NextResponse.json({ ok: false, error: `Engine inject returned ${engineRes.status}` });
    }

    const engineJson = await engineRes.json() as { ok?: boolean; injected?: number };

    return NextResponse.json({
      ok: true,
      candles_fetched: closePrices.length,
      engine_response: engineJson.ok ? "ok" : "error",
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({ ok: false, error: message });
  }
}
