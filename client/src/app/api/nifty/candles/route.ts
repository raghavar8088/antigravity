import { NextResponse } from "next/server";
import { getAngelJWT, isAngelConfigured, angelMissingEnv, commonHeaders, BASE_URL } from "@/lib/angelAuth";

function pad2(n: number): string {
  return String(n).padStart(2, "0");
}

/** Format a Date as "YYYY-MM-DD HH:mm" in IST (UTC+5:30) */
function toISTString(date: Date): string {
  // IST = UTC + 5h30m
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

export type Candle = {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
};

export async function GET(request: Request) {
  if (!isAngelConfigured()) {
    return NextResponse.json({
      ok: false,
      candles: [],
      count: 0,
      error: `Not configured — set these env vars: ${angelMissingEnv()}`,
    });
  }

  const { searchParams } = new URL(request.url);
  const interval = searchParams.get("interval") || "ONE_MINUTE";

  // Build IST date range: today 09:00 to 15:30 (or now if market is open)
  const now = new Date();
  const istOffset = (5 * 60 + 30) * 60 * 1000;
  const nowIST = new Date(now.getTime() + istOffset);

  // Build fromdate: today at 09:00 IST
  const fromUTC = new Date(Date.UTC(nowIST.getUTCFullYear(), nowIST.getUTCMonth(), nowIST.getUTCDate(), 3, 30)); // 09:00 IST = 03:30 UTC
  const fromdate = toISTString(fromUTC);

  // Build todate: today at 15:30 IST, capped at now
  const toUTC930 = new Date(Date.UTC(nowIST.getUTCFullYear(), nowIST.getUTCMonth(), nowIST.getUTCDate(), 10, 0)); // 15:30 IST = 10:00 UTC
  const toUTC = now < toUTC930 ? now : toUTC930;
  const todate = toISTString(toUTC);

  try {
    const jwt = await getAngelJWT();

    const res = await fetch(`${BASE_URL}/rest/secure/angelbroking/historical/v1/getCandleData`, {
      method: "POST",
      headers: commonHeaders(jwt),
      body: JSON.stringify({
        exchange: "NSE",
        symboltoken: "99926000",
        interval,
        fromdate,
        todate,
      }),
      next: { revalidate: 0 },
    });

    if (!res.ok) {
      return NextResponse.json({ ok: false, candles: [], count: 0, error: `Angel One returned ${res.status}` });
    }

    const payload = await res.json() as {
      status?: boolean;
      message?: string;
      errorcode?: string;
      data?: unknown[][];
    };

    if (!payload.status || !Array.isArray(payload.data)) {
      const msg = [payload.message, payload.errorcode].filter(Boolean).join(" ") || "Candle fetch failed";
      return NextResponse.json({ ok: false, candles: [], count: 0, error: msg });
    }

    const candles: Candle[] = payload.data.map((row) => ({
      time: new Date(row[0] as string).getTime(),
      open: Number(row[1]) || 0,
      high: Number(row[2]) || 0,
      low: Number(row[3]) || 0,
      close: Number(row[4]) || 0,
      volume: Number(row[5]) || 0,
    }));

    return NextResponse.json({ ok: true, candles, count: candles.length });
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({ ok: false, candles: [], count: 0, error: message });
  }
}
