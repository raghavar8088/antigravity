import { redirect } from "next/navigation";
import { MOCK_TRADING_PATH } from "@/lib/utils/navRoutes";

/** Retired BTC futures paper desk — redirects to mock trading. */
export default function BtcFutureTradingLegacyPage() {
  redirect(MOCK_TRADING_PATH);
}
