const ENGINE_PROXY_PREFIX = "/api/engine";

/**
 * Base URL for the Go trading engine (no trailing slash).
 *
 * - `NEXT_PUBLIC_API_URL`: browser calls the engine host directly (engine must allow CORS).
 * - Deployed Next (non-localhost) without that var: same-origin `/api/engine/...` proxy;
 *   set `INTERNAL_API_URL` on the server to the engine base URL (e.g. Render).
 */
export function resolveEngineApiUrl(): string {
  const fromEnv = process.env.NEXT_PUBLIC_API_URL?.trim();
  if (fromEnv) {
    return fromEnv.replace(/\/+$/, "");
  }
  if (typeof window !== "undefined") {
    const host = window.location.hostname;
    if (host && host !== "localhost" && host !== "127.0.0.1") {
      return `${window.location.origin}${ENGINE_PROXY_PREFIX}`;
    }
  }
  return "http://localhost:8080";
}
