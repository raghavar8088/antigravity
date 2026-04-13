/**
 * Delta Exchange HMAC-SHA256 signing utility.
 */
import * as crypto from "crypto";

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

const BASE = "https://api.india.delta.exchange";
const TESTNET = "https://cdn-ind.testnet.deltaex.org";

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
  const sig = deltaSign(method, path, body, ts, secret);

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

export async function deltaPost(
  path: string,
  body: unknown,
): Promise<{ ok: boolean; data: unknown; status: number }> {
  const key = process.env.DELTA_API_KEY ?? "";
  const secret = process.env.DELTA_API_SECRET ?? "";
  if (!key || !secret) return { ok: false, data: { error: "keys not set" }, status: 500 };

  const bodyStr = JSON.stringify(body);
  const ts = nowTs();
  const sig = deltaSign("POST", path, bodyStr, ts, secret);

  const res = await fetch(deltaBase() + path, {
    method: "POST",
    headers: {
      "api-key": key,
      "timestamp": ts,
      "signature": sig,
      "Content-Type": "application/json",
      "Accept": "application/json",
    },
    body: bodyStr,
    cache: "no-store",
  });

  let data: unknown;
  try { data = await res.json(); } catch { data = {}; }
  return { ok: res.ok, data, status: res.status };
}
