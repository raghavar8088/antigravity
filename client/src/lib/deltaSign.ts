import * as crypto from "crypto";
import * as https from "node:https";
import * as http from "node:http";
import { URL } from "node:url";

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

const BASE = "https://api.delta.exchange";
const TESTNET = "https://testnet-api.delta.exchange";

export function deltaBase(testnet?: boolean): string {
  const isTestnet = testnet !== undefined ? testnet : process.env.DELTA_TESTNET === "true";
  return isTestnet ? TESTNET : BASE;
}

export type DeltaKeyOverride = {
  apiKey?: string;
  apiSecret?: string;
  testnet?: boolean;
};

// Supports both http:// and https:// targets.
// When DELTA_PROXY_URL is set, it acts as a reverse proxy base — requests go to
// http://VPS_IP:PORT/v2/... and nginx on the VPS forwards to Delta Exchange.
// This avoids HTTPS CONNECT tunneling (which Squid struggled with).
function makeRequest(
  urlStr: string,
  method: string,
  headers: Record<string, string>,
  body?: string,
): Promise<{ ok: boolean; data: unknown; status: number }> {
  return new Promise((resolve) => {
    const url = new URL(urlStr);
    const isHttps = url.protocol === "https:";
    const options: http.RequestOptions = {
      hostname: url.hostname,
      port: Number(url.port) || (isHttps ? 443 : 80),
      path: url.pathname + url.search,
      method,
      headers,
      timeout: 15000,
    };

    const transport = isHttps ? https : http;
    const req = transport.request(options, (res) => {
      let raw = "";
      res.on("data", (chunk) => { raw += String(chunk); });
      res.on("end", () => {
        let data: unknown;
        try { data = JSON.parse(raw); } catch { data = { _raw: raw.slice(0, 300) }; }
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

function resolveBase(overrides?: DeltaKeyOverride): string {
  // If DELTA_PROXY_URL is set, use it as the base (nginx reverse proxy on VPS).
  // Otherwise call Delta CDN directly.
  return process.env.DELTA_PROXY_URL || deltaBase(overrides?.testnet);
}

// Fetch public (unauthenticated) endpoints — no API key required.
// Routes through the proxy (DELTA_PROXY_URL) so the whitelisted VPS IP is used,
// but sends no auth headers (products endpoint is public).
export async function deltaPublicFetch(
  path: string,
  overrides?: DeltaKeyOverride,
): Promise<{ ok: boolean; data: unknown; status: number }> {
  const base = resolveBase(overrides);
  return makeRequest(
    base + path,
    "GET",
    {
      "Accept": "application/json",
      "User-Agent": "Mozilla/5.0",
    },
  );
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
  const base = resolveBase(overrides);

  return makeRequest(
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
  const base = resolveBase(overrides);

  return makeRequest(
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
  );
}
