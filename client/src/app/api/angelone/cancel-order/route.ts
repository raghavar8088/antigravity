import { blockedDirectExecutionRoute } from "@/lib/blockedExecutionRoute";

export const dynamic = "force-dynamic";

/** Direct Angel One cancel is retired — cancellations flow through the institutional gateway when enabled */
export async function DELETE() {
  return blockedDirectExecutionRoute("/api/angelone/cancel-order");
}
