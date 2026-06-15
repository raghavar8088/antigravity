/**
 * Server-side engine kill switch release (no browser session required).
 */

const ENGINE_BASE = (process.env.INTERNAL_API_URL ?? process.env.ENGINE_URL ?? "http://127.0.0.1:8080").replace(/\/+$/, "");

export async function releaseEngineKillSwitch(): Promise<boolean> {
  try {
    const res = await fetch(`${ENGINE_BASE}/api/system/resume`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Service-Name": "vercel-ks-auto-clear" },
      body: JSON.stringify({ confirm: "RESUME" }),
      cache: "no-store",
      signal: AbortSignal.timeout(8_000),
    });
    if (res.ok) return true;
  } catch {
    // fall through to admin release
  }

  const adminSecret = process.env.ENGINE_ADMIN_SECRET?.trim();
  if (!adminSecret) return false;

  try {
    const res = await fetch(`${ENGINE_BASE}/api/admin/ks/release`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Engine-Admin-Secret": adminSecret,
        "X-Service-Name": "vercel-ks-auto-clear",
      },
      cache: "no-store",
      signal: AbortSignal.timeout(8_000),
    });
    return res.ok;
  } catch {
    return false;
  }
}
