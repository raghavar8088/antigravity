/**
 * Base URL for the Go trading engine (no trailing slash).
 * Set NEXT_PUBLIC_API_URL in production (e.g. Render) so the Vercel UI hits the engine, not :8080 on the Vercel host.
 */
export function resolveEngineApiUrl(): string {
  const fromEnv = process.env.NEXT_PUBLIC_API_URL?.trim();
  if (fromEnv) {
    return fromEnv.replace(/\/+$/, "");
  }
  if (typeof window !== "undefined") {
    const host = window.location.hostname;
    if (host && host !== "localhost" && host !== "127.0.0.1") {
      const port = process.env.NEXT_PUBLIC_ENGINE_PORT || "8080";
      return `${window.location.protocol}//${host}:${port}`;
    }
  }
  return "http://localhost:8080";
}
