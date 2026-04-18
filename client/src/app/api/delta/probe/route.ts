import { NextResponse } from "next/server";
import * as http from "node:http";
import * as https from "node:https";

export async function GET() {
  const proxyUrl = process.env.DELTA_PROXY_URL ?? "(not set)";

  const result = await new Promise<{ status: number; body: string }>((resolve) => {
    if (!process.env.DELTA_PROXY_URL) {
      resolve({ status: 0, body: "DELTA_PROXY_URL not set" });
      return;
    }
    const req = http.request(
      `${process.env.DELTA_PROXY_URL}/v2/products?contract_types=call_options&page_size=3`,
      { method: "GET", headers: { Accept: "application/json", "User-Agent": "probe/1.0" }, timeout: 10000 },
      (res) => {
        let raw = "";
        res.on("data", (c) => { raw += String(c); });
        res.on("end", () => resolve({ status: res.statusCode ?? 0, body: raw.slice(0, 500) }));
      },
    );
    req.on("error", (e) => resolve({ status: 0, body: e.message }));
    req.on("timeout", () => { req.destroy(); resolve({ status: 0, body: "timeout" }); });
    req.end();
  });

  return NextResponse.json({ proxyUrl, ...result });
}

// POST probe — tests that Vercel can POST through the proxy to Delta
export async function POST() {
  const proxyUrl = process.env.DELTA_PROXY_URL ?? "(not set)";
  const body = JSON.stringify({ product_id: 27, size: 1, side: "sell", order_type: "market_order" });

  const postResult = await new Promise<{ status: number; body: string }>((resolve) => {
    const makeReq = (mod: typeof http | typeof https, url: string) => {
      const u = new URL(url);
      const opts = {
        hostname: u.hostname,
        port: Number(u.port) || (u.protocol === "https:" ? 443 : 80),
        path: "/v2/orders",
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Content-Length": String(Buffer.byteLength(body)),
          "Accept": "application/json",
          "User-Agent": "probe/1.0",
          "api-key": "probe-test",
          "timestamp": String(Math.floor(Date.now() / 1000)),
          "signature": "probe-sig",
        },
        timeout: 10000,
      };
      const req = mod.request(opts, (res) => {
        let raw = "";
        res.on("data", (c) => { raw += String(c); });
        res.on("end", () => resolve({ status: res.statusCode ?? 0, body: raw.slice(0, 600) }));
      });
      req.on("error", (e) => resolve({ status: 0, body: e.message }));
      req.on("timeout", () => { req.destroy(); resolve({ status: 0, body: "timeout" }); });
      req.write(body);
      req.end();
    };

    if (proxyUrl.startsWith("http://")) makeReq(http, proxyUrl);
    else if (proxyUrl.startsWith("https://")) makeReq(https, proxyUrl);
    else resolve({ status: 0, body: "no proxy url" });
  });

  return NextResponse.json({ proxyUrl, post: postResult });
}
