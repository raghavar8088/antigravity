import * as crypto from "crypto";

const DELTA_BASE = process.env.DELTA_API_BASE_URL?.replace(/\/$/, "") ?? "https://api.india.delta.exchange";
const API_KEY = process.env.DELTA_API_KEY ?? "";
const API_SECRET = process.env.DELTA_API_SECRET ?? "";

export { DELTA_BASE, API_KEY as DELTA_API_KEY };

export function isDeltaConfigured(): boolean {
  return !!(API_KEY && API_SECRET);
}

export function deltaHeaders(method: string, path: string, body = ""): Record<string, string> {
  const timestamp = String(Math.floor(Date.now() / 1000));
  const sigPayload = method + timestamp + path + body;
  const signature = crypto.createHmac("sha256", API_SECRET).update(sigPayload).digest("hex");
  return {
    "Accept": "application/json",
    "Content-Type": "application/json",
    "api-key": API_KEY,
    "timestamp": timestamp,
    "signature": signature,
    "User-Agent": "RAIG-Trading/1.0",
  };
}
