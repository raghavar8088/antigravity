import { NextResponse } from "next/server";
import { getAngelJWT, isAngelConfigured, angelMissingEnv, commonHeaders, BASE_URL } from "@/lib/angelAuth";
import { MCX_COMMODITY_MAP } from "@/lib/mcxCommodities";

function pad2(n: number): string {
  return String(n).padStart(2, "0");
}

function toISTString(date: Date): string {
  const istOffset = 5 * 60 + 30;
  const istMs = date.getTime() + istOffset * 60 * 1000;
  const d = new Date(istMs);
  return `${d.getUTCFullYear()}-${pad2(d.getUTCMonth() + 1)}-${pad2(d.getUTCDate())} ${pad2(d.getUTCHours())}:${pad2(d.getUTCMinutes())}`;
}

export type MCXCandle = { time: number; open: number; high: number; low: number; close: number; volume: number };

async function fetchToken(jwt: string, searchQuery: string): Promise<string | null> {
  try {
    const res = await fetch(`${BASE_URL}/rest/secure/angelbroking/order/v1/searchScrip`, {
      method: "POST",
      headers: commonHeaders(jwt),
      body: JSON.stringify({ exchange: "MCX", searchscrip: searchQuery }),
      next: { revalidate: 0 },
    });
    if (!res.ok) return null;
    const payload = await res.json() as {
      status?: boolean;
      data?: Array<{ tradingsymbol?: string; symboltoken?: string; instrumenttype?: string; exch_seg?: string; expiry?: string }>;
    };
    if (!payload.status || !Array.isArray(payload.data)) return null;
    const futures = payload.data.filter(
      (d) => d.instrumenttype === "FUTCOM" && d.exch_seg === "MCX" &&
             d.tradingsymbol?.toUpperCase().startsWith(searchQuery.toUpperCase()),
    );
    if (!futures.length) return null;
    futures.sort((a, b) => {
      const p = (s?: string) => { try { return s ? new Date(s).getTime() : 0; } catch { return 0; } };
      return p(a.expiry) - p(b.expiry);
    });
    return futures[0].symboltoken ?? null;
  } catch { return null; }
}

export async function GET(request: Request) {
  if (!isAngelConfigured()) {
    return NextResponse.json({ ok: false, candles: [], count: 0, error: angelMissingEnv() });
  }

  const { searchParams } = new URL(request.url);
  const commodityId = searchParams.get("commodity") ?? "CRUDEOIL";
  const interval = searchParams.get("interval") ?? "ONE_MINUTE";

  const commodity = MCX_COMMODITY_MAP.get(commodityId);
  if (!commodity) {
    return NextResponse.json({ ok: false, candles: [], count: 0, error: `Unknown commodity: ${commodityId}` });
  }

  try {
    const jwt = await getAngelJWT();
    const token = await fetchToken(jwt, commodity.searchQuery);
    if (!token) {
      return NextResponse.json({ ok: false, candles: [], count: 0, error: `Could not resolve token for ${commodityId}` });
    }

    const now = new Date();
    const istOffset = (5 * 60 + 30) * 60 * 1000;
    const nowIST = new Date(now.getTime() + istOffset);

    // MCX opens at 09:00 IST
    const fromUTC = new Date(Date.UTC(nowIST.getUTCFullYear(), nowIST.getUTCMonth(), nowIST.getUTCDate(), 3, 30)); // 09:00 IST
    const fromdate = toISTString(fromUTC);

    // MCX closes at 23:30 IST (for most commodities)
    const toUTC2330 = new Date(Date.UTC(nowIST.getUTCFullYear(), nowIST.getUTCMonth(), nowIST.getUTCDate(), 18, 0)); // 23:30 IST
    const toUTC = now < toUTC2330 ? now : toUTC2330;
    const todate = toISTString(toUTC);

    const res = await fetch(`${BASE_URL}/rest/secure/angelbroking/historical/v1/getCandleData`, {
      method: "POST",
      headers: commonHeaders(jwt),
      body: JSON.stringify({ exchange: "MCX", symboltoken: token, interval, fromdate, todate }),
      next: { revalidate: 0 },
    });

    if (!res.ok) {
      return NextResponse.json({ ok: false, candles: [], count: 0, error: `Angel One returned ${res.status}` });
    }

    const payload = await res.json() as { status?: boolean; data?: unknown[][] };
    if (!payload.status || !Array.isArray(payload.data)) {
      return NextResponse.json({ ok: false, candles: [], count: 0, error: "Candle fetch failed" });
    }

    const candles: MCXCandle[] = payload.data.map((row) => ({
      time: new Date(row[0] as string).getTime(),
      open: Number(row[1]) || 0,
      high: Number(row[2]) || 0,
      low: Number(row[3]) || 0,
      close: Number(row[4]) || 0,
      volume: Number(row[5]) || 0,
    }));

    return NextResponse.json({ ok: true, candles, count: candles.length, commodityId, token });
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({ ok: false, candles: [], count: 0, error: message });
  }
}
