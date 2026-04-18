import { NextResponse } from "next/server";
import * as http from "node:http";

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
