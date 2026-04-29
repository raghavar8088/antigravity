import { NextResponse } from "next/server";
import { isEngineProxyConfigured, engineProxyFetch } from "@/lib/engineProxy";
import { MCX_COMMODITY_MAP } from "@/lib/mcxCommodities";
import { resolveMCXFutureToken } from "@/lib/mcxTokenResolver";

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

export async function GET(request: Request) {
  if (!isEngineProxyConfigured()) {
    return NextResponse.json({
      ok: false,
      candles: [],
      count: 0,
      error:
        "Not configured — set LIGHTSAIL_ENGINE_URL to your Lightsail engine base so MCX historical data uses the whitelisted IP.",
    });
  }

  const { searchParams } = new URL(request.url);
  const commodityId = searchParams.get("commodity") ?? "CRUDEOIL";
  const interval = searchParams.get("interval") ?? "ONE_MINUTE";

  const commodity = MCX_COMMODITY_MAP.get(commodityId);
  if (!commodity) {
    return NextResponse.json({ ok: false, candles: [], count: 0, error: `Unknown commodity: ${commodityId}` });
  }

  try {
    const resolved = await resolveMCXFutureToken(commodity);
    if (!resolved) {
      return NextResponse.json({ ok: false, candles: [], count: 0, error: `Could not resolve token for ${commodityId} via Angel One searchScrip` });
    }

    // MCX trading window: 09:00–23:30 IST (Mon–Fri).
    // Pick the most recent completed/active session relative to "now" so the
    // request never has fromdate > todate (which Angel One rejects with 400).
    const now = new Date();
    const istOffset = (5 * 60 + 30) * 60 * 1000;
    const nowIST = new Date(now.getTime() + istOffset);

    let sessionDayUTC = new Date(Date.UTC(nowIST.getUTCFullYear(), nowIST.getUTCMonth(), nowIST.getUTCDate()));

    const minutesSinceISTMidnight = nowIST.getUTCHours() * 60 + nowIST.getUTCMinutes();
    const sessionOpenMin = 9 * 60;          // 09:00 IST
    const sessionCloseMin = 23 * 60 + 30;   // 23:30 IST

    if (minutesSinceISTMidnight < sessionOpenMin) {
      sessionDayUTC = new Date(sessionDayUTC.getTime() - 24 * 60 * 60 * 1000);
    }

    let day = nowIST.getUTCDay();
    if (minutesSinceISTMidnight < sessionOpenMin) {
      day = day === 0 ? 6 : day - 1;
    }
    if (day === 0) {
      sessionDayUTC = new Date(sessionDayUTC.getTime() - 2 * 24 * 60 * 60 * 1000);
    } else if (day === 6) {
      sessionDayUTC = new Date(sessionDayUTC.getTime() - 1 * 24 * 60 * 60 * 1000);
    }

    const fromUTC = new Date(sessionDayUTC.getTime() + (3 * 60 + 30) * 60 * 1000); // 09:00 IST = 03:30 UTC
    const sessionCloseUTC = new Date(sessionDayUTC.getTime() + 18 * 60 * 60 * 1000); // 23:30 IST = 18:00 UTC

    const isLiveSession =
      minutesSinceISTMidnight >= sessionOpenMin && minutesSinceISTMidnight < sessionCloseMin;
    const toUTC = isLiveSession && now < sessionCloseUTC ? now : sessionCloseUTC;

    const fromdate = toISTString(fromUTC);
    const todate = toISTString(toUTC);

    const res = await engineProxyFetch("/rest/secure/angelbroking/historical/v1/getCandleData", {
      exchange: "MCX",
      symboltoken: resolved.token,
      interval,
      fromdate,
      todate,
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

    return NextResponse.json({ ok: true, candles, count: candles.length, commodityId, token: resolved.token, tradingSymbol: resolved.tradingSymbol });
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({ ok: false, candles: [], count: 0, error: message });
  }
}
