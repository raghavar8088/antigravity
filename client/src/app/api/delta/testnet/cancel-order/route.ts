import { blockedDirectExecutionRoute } from "@/lib/blockedExecutionRoute";

export const dynamic = "force-dynamic";

/** Direct testnet cancel is retired — cancellations must flow through institutional controls when enabled. */
export async function POST() {
  return blockedDirectExecutionRoute("/api/delta/testnet/cancel-order");
}
