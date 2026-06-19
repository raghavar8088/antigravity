/**
 * telegramAlert.ts — Minimal Telegram alert sender for executor liveness alerts.
 *
 * Requires TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID. Silently no-ops (with a
 * console warning) if either is unset, mirroring isMongoConfigured()'s
 * fail-open pattern elsewhere in this codebase — a missing alert channel
 * must never crash the cron route it's called from.
 */

export function isTelegramConfigured(): boolean {
  return Boolean(process.env.TELEGRAM_BOT_TOKEN?.trim() && process.env.TELEGRAM_CHAT_ID?.trim());
}

export async function sendTelegramAlert(message: string): Promise<{ sent: boolean; error?: string }> {
  const token = process.env.TELEGRAM_BOT_TOKEN?.trim();
  const chatId = process.env.TELEGRAM_CHAT_ID?.trim();

  if (!token || !chatId) {
    console.warn("[telegramAlert] TELEGRAM_BOT_TOKEN/TELEGRAM_CHAT_ID not set — alert not sent:", message);
    return { sent: false, error: "TELEGRAM_NOT_CONFIGURED" };
  }

  try {
    const res = await fetch(`https://api.telegram.org/bot${token}/sendMessage`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ chat_id: chatId, text: message }),
    });
    if (!res.ok) {
      const body = await res.text().catch(() => "");
      return { sent: false, error: `HTTP ${res.status}: ${body}` };
    }
    return { sent: true };
  } catch (err) {
    return { sent: false, error: err instanceof Error ? err.message : "TELEGRAM_SEND_FAILED" };
  }
}
