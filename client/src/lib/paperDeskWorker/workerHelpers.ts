/**
 * Shared stateless helpers for the 24/7 paper desk worker.
 * Safe to import in both Node scripts and Next.js route handlers.
 */

/**
 * Normalises a raw base URL string so the worker never builds `https://https://…`.
 *
 * Rules:
 *   - Already has a protocol (`http://` / `https://`) → use as-is, strip trailing slashes.
 *   - No protocol → prepend `https://`, strip trailing slashes.
 *   - Empty / undefined → fall back to `http://localhost:3000`.
 */
export function normalizeBaseUrl(raw?: string): string {
  if (!raw || !raw.trim()) return "http://localhost:3000";
  const trimmed = raw.trim().replace(/\/+$/, "");
  if (trimmed.startsWith("https://") || trimmed.startsWith("http://")) {
    return trimmed;
  }
  return `https://${trimmed}`;
}
