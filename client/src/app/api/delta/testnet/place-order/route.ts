import { blockedDirectExecutionRoute } from "@/lib/risk/blockedExecutionRoute";

export const dynamic = "force-dynamic";

/** Direct testnet order placement is retired — use POST /api/execution/request with venue=delta */
export async function POST() {
  return blockedDirectExecutionRoute("/api/delta/testnet/place-order");
}
