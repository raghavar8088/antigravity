/**
 * Routes Angel One API calls through the Lightsail engine (whitelisted IP)
 * instead of calling Angel One directly from Vercel (which gets blocked by IP restriction).
 */

const ENGINE_URL = (process.env.LIGHTSAIL_ENGINE_URL ?? "").replace(/\/$/, "");

export function isEngineProxyConfigured(): boolean {
  return ENGINE_URL !== "";
}

export async function engineProxyFetch(path: string, body: unknown): Promise<Response> {
  if (!ENGINE_URL) {
    throw new Error("LIGHTSAIL_ENGINE_URL is not set — cannot proxy Angel One request");
  }
  return fetch(`${ENGINE_URL}/api/angel-proxy`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ path, body }),
    next: { revalidate: 0 },
  });
}
