import { NextResponse } from "next/server";
import * as http from "node:http";
import * as https from "node:https";

function makeProxyRequest(
  method: "GET" | "POST",
  path: string,
  body?: string,
): Promise<{ status: number; body: string }> {
  const proxyUrl = process.env.DELTA_PROXY_URL;
  if (!proxyUrl) return Promise.resolve({ status: 0, body: "DELTA_PROXY_URL not set" });

  return new Promise((resolve) => {
    const u = new URL(proxyUrl);
    const isHttps = u.protocol === "https:";
    const headers: Record<string, string> = {
      "Accept": "application/json",
      "User-Agent": "probe/1.0",
      "api-key": "probe-test",
      "timestamp": String(Math.floor(Date.now() / 1000)),
      "signature": "probe-sig",
    };
    if (body) {
      headers["Content-Type"] = "application/json";
      headers["Content-Length"] = String(Buffer.byteLength(body));
    }
    const opts: http.RequestOptions = {
      hostname: u.hostname,
      port: Number(u.port) || (isHttps ? 443 : 80),
      path,
      method,
      headers,
      timeout: 10000,
    };
    const mod = isHttps ? https : http;
    const req = mod.request(opts, (res) => {
      let raw = "";
      res.on("data", (c) => { raw += String(c); });
      res.on("end", () => resolve({ status: res.statusCode ?? 0, body: raw.slice(0, 600) }));
    });
    req.on("error", (e) => resolve({ status: 0, body: e.message }));
    req.on("timeout", () => { req.destroy(); resolve({ status: 0, body: "timeout" }); });
    if (body) req.write(body);
    req.end();
  });
}

export async function GET() {
  const proxyUrl = process.env.DELTA_PROXY_URL ?? "(not set)";
  const postBody = JSON.stringify({ product_id: 27, size: 1, side: "sell", order_type: "market_order" });

  const [getResult, postResult] = await Promise.all([
    makeProxyRequest("GET", "/v2/products?contract_types=call_options&page_size=3"),
    makeProxyRequest("POST", "/v2/orders", postBody),
  ]);

  return NextResponse.json({ proxyUrl, get: getResult, post: postResult });
}
