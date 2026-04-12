/**
 * Pure-JS HMAC-SHA256 signing for Delta Exchange API requests.
 * Uses the Web Crypto API which is available in all Next.js runtimes.
 */

export async function deltaSign(
  method: string,
  path: string,
  body: string,
  ts: string,
  secret: string,
): Promise<string> {
  const enc = new TextEncoder();
  const key = await crypto.subtle.importKey(
    "raw",
    enc.encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const sig = await crypto.subtle.sign("HMAC", key, enc.encode(method + ts + path + body));
  return Array.from(new Uint8Array(sig))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

export function nowTs(): string {
  return String(Math.floor(Date.now() / 1000));
}

const BASE = "https://api.india.delta.exchange";
const TESTNET = "https://testnet-api.india.delta.exchange";

export function deltaBase(): string {
  return process.env.DELTA_TESTNET === "true" ? TESTNET : BASE;
}

export async function deltaFetch(
  path: string,
  method = "GET",
  body = "",
): Promise<{ ok: boolean; data: unknown; status: number }> {
  const key = process.env.DELTA_API_KEY ?? "";
  const secret = process.env.DELTA_API_SECRET ?? "";
  if (!key || !secret) {
    return { ok: false, data: { error: "DELTA_API_KEY / DELTA_API_SECRET not set" }, status: 500 };
  }

  const ts = nowTs();
  const sig = await deltaSign(method, path, body, ts, secret);

  const res = await fetch(deltaBase() + path, {
    method,
    headers: {
      "api-key": key,
      "timestamp": ts,
      "signature": sig,
      "Content-Type": "application/json",
      "Accept": "application/json",
    },
    body: body || undefined,
    cache: "no-store",
  });

  let data: unknown;
  try { data = await res.json(); } catch { data = {}; }
  return { ok: res.ok, data, status: res.status };
}
