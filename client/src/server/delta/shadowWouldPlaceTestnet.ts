import { getDeltaServerCredentials, isDeltaTestnetExecutionEnabled } from "@/server/delta/deltaConfig";

/** Server-side: testnet keys configured and DELTA_TESTNET enabled (no order placed). */
export function computeWouldPlaceTestnetShadow(): boolean {
  if (!isDeltaTestnetExecutionEnabled()) return false;
  try {
    getDeltaServerCredentials();
    return true;
  } catch {
    return false;
  }
}
