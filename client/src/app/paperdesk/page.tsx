import { redirect } from "next/navigation";
import { MOCK_TRADING_PATH } from "@/lib/utils/navRoutes";

/** Retired paper desk alias — redirects to mock trading. */
export default function PaperdeskLegacyPage() {
  redirect(MOCK_TRADING_PATH);
}
