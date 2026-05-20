/**
 * Lightweight HS256 JWT helpers for MongoDB-backed session auth.
 * No external library — uses Node.js built-in `crypto`.
 * Env: AUTH_JWT_SECRET (required server-side).
 */

import crypto from "node:crypto";

export const SESSION_COOKIE = "raig_session";
const EXPIRY_SECS = 30 * 24 * 60 * 60; // 30 days

export type SessionPayload = { userId: string; email: string };

function getSecret(): string {
  const s = process.env.AUTH_JWT_SECRET?.trim();
  if (!s) throw new Error("AUTH_JWT_SECRET env var is not set");
  return s;
}

function enc(obj: unknown): string {
  return Buffer.from(JSON.stringify(obj)).toString("base64url");
}

function dec(s: string): unknown {
  return JSON.parse(Buffer.from(s, "base64url").toString("utf-8"));
}

export function signSession(userId: string, email: string): string {
  const secret = getSecret();
  const header = enc({ alg: "HS256", typ: "JWT" });
  const payload = enc({
    userId,
    email,
    iat: Math.floor(Date.now() / 1000),
    exp: Math.floor(Date.now() / 1000) + EXPIRY_SECS,
  });
  const body = `${header}.${payload}`;
  const sig = crypto.createHmac("sha256", secret).update(body).digest("base64url");
  return `${body}.${sig}`;
}

export function verifySession(token: string): SessionPayload | null {
  try {
    const secret = getSecret();
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const [header, payload, sig] = parts as [string, string, string];
    const body = `${header}.${payload}`;
    const expected = crypto.createHmac("sha256", secret).update(body).digest("base64url");

    // constant-time comparison — both digests are base64url so same length
    const aBuf = Buffer.from(sig, "base64url");
    const bBuf = Buffer.from(expected, "base64url");
    if (aBuf.length !== bBuf.length) return null;
    if (!crypto.timingSafeEqual(aBuf, bBuf)) return null;

    const p = dec(payload) as { userId?: unknown; email?: unknown; exp?: unknown };
    if (typeof p.exp === "number" && p.exp < Math.floor(Date.now() / 1000)) return null;
    if (typeof p.userId !== "string" || typeof p.email !== "string") return null;
    return { userId: p.userId, email: p.email };
  } catch {
    return null;
  }
}

export function isMongoAuthConfigured(): boolean {
  return (
    typeof process.env.MONGODB_URI === "string" &&
    process.env.MONGODB_URI.trim().length > 0 &&
    typeof process.env.AUTH_JWT_SECRET === "string" &&
    process.env.AUTH_JWT_SECRET.trim().length > 0
  );
}
