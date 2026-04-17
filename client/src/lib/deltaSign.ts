import * as crypto from "crypto";
import * as https from "node:https";
import { URL } from "node:url";
import { HttpsProxyAgent } from "https-proxy-agent";

export function deltaSign(
  method: string,
  path: string,
  body: string,
  ts: string,
  secret: string,
): string {
  return crypto.createHmac("sha256", secret).update(method + ts + path + body).digest("hex");
}

export function nowTs(): string {
  return String(Math.floor(Date.now() / 1000));
}

const BASE = "https://cdn.india.deltaex.org";
const TESTNET = "https://cdn-ind.testnet.deltaex.org";

export function deltaBase(testnet?: boolean): string {
  const isTestnet = testnet !== undefined ? testnet : process.env.DELTA_TESTNET === "true";
  return isTestnet ? TESTNET : BASE;
}

export type DeltaKeyOverride = {
  apiKey?: string;
  apiSecret?: string;
  testnet?: boolean;
};

// Low-level HTTPS request using Node.js native https module.
// Uses https-proxy-agent when DELTA_PROXY_URL is set, which properly handles
// the HTTP CONNECT tunnel needed to reach HTTPS endpoints through a proxy.
function httpsRequest(
  urlStr: string,
  method: string,
  headers: Record<string, string>,
  body?: string,
  proxyUrl?: string,
): Promise<{ ok: boolean; data: unknown; status: number }> {
  return new Promise((resolve) => {
    const url = new URL(urlStr);
    const options: https.RequestOptions = {
      hostname: url.hostname,
      port: Number(url.port) || 443,
      path: url.pathname + url.search,
      method,
      headers,
      agent: proxyUrl ? new HttpsProxyAgent(proxyUrl) : undefined,
      timeout: 15000,
    };

    const req = https.request(options, (res) => {
      let raw = "";
      res.on("data", (chunk) => { raw += String(chunk); });
      res.on("end", () => {
        let data: unknown;
        try { data = JSON.parse(raw); } catch { data = {}; }
        const status = res.statusCode ?? 0;
        resolve({ ok: status >= 200 && status < 300, data, status });
      });
    });

    req.on("error", (err) => {
      resolve({ ok: false, data: { error: err.message }, status: 0 });
    });

    req.on("timeout", () => {
      req.destroy();
      resolve({ ok: false, data: { error: "request timeout" }, status: 0 });
    });

    if (body) req.write(body);
    req.end();
  });
}

export async function deltaFetch(
  path: string,
  method = "GET",
  body = "",
  overrides?: DeltaKeyOverride,
): Promise<{ ok: boolean; data: unknown; status: number }> {
  const key = (overrides?.apiKey ?? "").trim() || (process.env.DELTA_API_KEY ?? "");
  const secret = (overrides?.apiSecret ?? "").trim() || (process.env.DELTA_API_SECRET ?? "");
  if (!key || !secret) {
    return { ok: false, data: { error: "DELTA_API_KEY / DELTA_API_SECRET not set" }, status: 500 };
  }

  const ts = nowTs();
  const sig = deltaSign(method, path, body, ts, secret);
  const base = deltaBase(overrides?.testnet);
  const proxyUrl = process.env.DELTA_PROXY_URL;

  return httpsRequest(
    base + path,
    method,
    {
      "api-key": key,
      "timestamp": ts,
      "signature": sig,
      "Content-Type": "application/json",
      "Accept": "application/json",
    },
    body || undefined,
    proxyUrl,
  );
}

export async function deltaPost(
  path: string,
  body: unknown,
  overrides?: DeltaKeyOverride,
): Promise<{ ok: boolean; data: unknown; status: number }> {
  const key = (overrides?.apiKey ?? "").trim() || (process.env.DELTA_API_KEY ?? "");
  const secret = (overrides?.apiSecret ?? "").trim() || (process.env.DELTA_API_SECRET ?? "");
  if (!key || !secret) return { ok: false, data: { error: "keys not set" }, status: 500 };

  const bodyStr = JSON.stringify(body);
  const ts = nowTs();
  const sig = deltaSign("POST", path, bodyStr, ts, secret);
  const base = deltaBase(overrides?.testnet);
  const proxyUrl = process.env.DELTA_PROXY_URL;

  return httpsRequest(
    base + path,
    "POST",
    {
      "api-key": key,
      "timestamp": ts,
      "signature": sig,
      "Content-Type": "application/json",
      "Accept": "application/json",
    },
    bodyStr,
    proxyUrl,
  );
}
