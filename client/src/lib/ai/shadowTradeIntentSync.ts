import type { ShadowIntentPostBody } from "@/lib/ai/shadowTradeIntentTypes";
import { isDeskShadowIntentsEnabled } from "@/lib/ai/shadowTradeIntentMapper";

/**
 * Fire-and-forget shadow intent when signed in + NEXT_PUBLIC_DESK_SHADOW_INTENTS=1.
 * Never calls Delta placeOrder.
 */
export function persistShadowTradeIntent(
  userId: string | null | undefined,
  body: ShadowIntentPostBody,
): void {
  if (!userId?.trim() || !isDeskShadowIntentsEnabled()) return;

  void (async () => {
    try {
      await fetch("/api/shadow-trade-intents", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
    } catch {
      // shadow log is best-effort
    }
  })();
}
