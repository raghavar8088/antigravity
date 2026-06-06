/**
 * Single-owner authentication primitives.
 *
 * All devices resolve to OWNER_ACCOUNT_KEY so every MongoDB query lands on
 * the same document set regardless of which browser or machine issued it.
 *
 * Password hashing: Node 20 built-in scrypt (NIST-approved, stronger than
 * bcrypt — no external dependency required).
 *
 * Env vars consumed here:
 *   ADMIN_USERNAME        — owner's login username
 *   ADMIN_PASSWORD_HASH   — scrypt hash produced by scripts/generate-password-hash.mjs
 */

import crypto, { type BinaryLike } from "node:crypto";
import { promisify } from "node:util";

const scryptAsync = promisify<BinaryLike, BinaryLike, number, Buffer>(crypto.scrypt);

export const OWNER_ACCOUNT_KEY = "mock_trading_default";

// ── Hashing ──────────────────────────────────────────────────────────────────

export async function hashPassword(password: string): Promise<string> {
  const salt = crypto.randomBytes(16).toString("hex");
  const derived = await scryptAsync(password, salt, 64);
  return `scrypt:${salt}:${derived.toString("hex")}`;
}

export async function verifyPassword(
  password: string,
  storedHash: string,
): Promise<boolean> {
  const parts = storedHash.split(":");
  if (parts.length !== 3 || parts[0] !== "scrypt") return false;
  const [, salt, keyHex] = parts as [string, string, string];
  try {
    const derived = await scryptAsync(password, salt, 64);
    const stored = Buffer.from(keyHex, "hex");
    if (derived.length !== stored.length) return false;
    return crypto.timingSafeEqual(derived, stored);
  } catch {
    return false;
  }
}

// ── Credential resolution ─────────────────────────────────────────────────────

export function getAdminCredentials(): {
  username: string;
  passwordHash: string;
} | null {
  const username = process.env.ADMIN_USERNAME?.trim();
  const passwordHash = process.env.ADMIN_PASSWORD_HASH?.trim();
  if (!username || !passwordHash) return null;
  return { username, passwordHash };
}

export function isOwnerAuthConfigured(): boolean {
  return getAdminCredentials() !== null;
}
