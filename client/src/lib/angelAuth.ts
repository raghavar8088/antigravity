import * as crypto from "crypto";

const BASE_URL = process.env.ANGELONE_BASE_URL?.replace(/\/$/, "") ?? "https://apiconnect.angelone.in";
const API_KEY = process.env.ANGELONE_API_KEY ?? "";
const CLIENT_CODE = process.env.ANGELONE_CLIENT_CODE ?? "";
const PIN = process.env.ANGELONE_PIN ?? "";
const TOTP_SECRET = process.env.ANGELONE_TOTP_SECRET ?? "";
const TOTP_CODE = process.env.ANGELONE_TOTP_CODE ?? "";
const JWT_OVERRIDE = process.env.ANGELONE_JWT_TOKEN ?? "";
const LOCAL_IP = process.env.ANGELONE_CLIENT_LOCAL_IP ?? "127.0.0.1";
const PUBLIC_IP = process.env.ANGELONE_CLIENT_PUBLIC_IP ?? "127.0.0.1";
const MAC = process.env.ANGELONE_MAC_ADDRESS ?? "02:00:00:00:00:00";

export { BASE_URL };

export function isAngelConfigured(): boolean {
  if (JWT_OVERRIDE) return true;
  return !!(API_KEY && CLIENT_CODE && PIN && (TOTP_SECRET || TOTP_CODE));
}

export function angelMissingEnv(): string {
  if (JWT_OVERRIDE) return "";
  const missing: string[] = [];
  if (!API_KEY) missing.push("ANGELONE_API_KEY");
  if (!CLIENT_CODE) missing.push("ANGELONE_CLIENT_CODE");
  if (!PIN) missing.push("ANGELONE_PIN");
  if (!TOTP_SECRET && !TOTP_CODE) missing.push("ANGELONE_TOTP_SECRET or ANGELONE_TOTP_CODE");
  return missing.join(", ");
}

export function generateTOTP(secret: string): string {
  const normalized = secret.toUpperCase().replace(/\s/g, "").replace(/=+$/, "");
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  let bits = "";
  for (const ch of normalized) {
    const val = alphabet.indexOf(ch);
    if (val < 0) continue;
    bits += val.toString(2).padStart(5, "0");
  }
  const bytes = new Uint8Array(Math.floor(bits.length / 8));
  for (let i = 0; i < bytes.length; i++) {
    bytes[i] = parseInt(bits.slice(i * 8, i * 8 + 8), 2);
  }

  const counter = BigInt(Math.floor(Date.now() / 1000 / 30));
  const counterBuf = Buffer.alloc(8);
  counterBuf.writeBigUInt64BE(counter);

  const hmac = crypto.createHmac("sha1", Buffer.from(bytes));
  hmac.update(counterBuf);
  const hash = hmac.digest();

  const offset = hash[hash.length - 1] & 0x0f;
  const code =
    ((hash[offset] & 0x7f) << 24) |
    ((hash[offset + 1] & 0xff) << 16) |
    ((hash[offset + 2] & 0xff) << 8) |
    (hash[offset + 3] & 0xff);

  return String(code % 1_000_000).padStart(6, "0");
}

export function commonHeaders(jwt?: string): Record<string, string> {
  const h: Record<string, string> = {
    "Accept": "application/json",
    "Content-Type": "application/json",
    "X-PrivateKey": API_KEY,
    "X-UserType": "USER",
    "X-SourceID": "WEB",
    "X-ClientLocalIP": LOCAL_IP,
    "X-ClientPublicIP": PUBLIC_IP,
    "X-MACAddress": MAC,
    "User-Agent": "RAIG-LiveDataLab/1.0",
  };
  if (jwt) h["Authorization"] = `Bearer ${jwt}`;
  return h;
}

export async function getAngelJWT(): Promise<string> {
  if (JWT_OVERRIDE) return JWT_OVERRIDE;

  const totp = TOTP_CODE || generateTOTP(TOTP_SECRET);

  const res = await fetch(`${BASE_URL}/rest/auth/angelbroking/user/v1/loginByPassword`, {
    method: "POST",
    headers: commonHeaders(),
    body: JSON.stringify({ clientcode: CLIENT_CODE, password: PIN, totp, state: "LiveDataLab" }),
    next: { revalidate: 0 },
  });

  if (!res.ok) throw new Error(`Angel One login returned status ${res.status}`);

  const payload = await res.json() as {
    status?: boolean;
    message?: string;
    errorcode?: string;
    data?: { jwtToken?: string };
  };

  if (!payload.status || !payload.data?.jwtToken) {
    const msg = [payload.message, payload.errorcode].filter(Boolean).join(" ") || "Angel One login failed";
    throw new Error(msg);
  }

  return payload.data.jwtToken;
}
