import { redirect } from "next/navigation";
import { MOCK_TRADING_PATH } from "@/lib/navRoutes";

export default function Home() {
  redirect(MOCK_TRADING_PATH);
}
