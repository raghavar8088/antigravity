import type { Metadata } from "next";
import PaperDeskDashboard from "@/components/PaperDeskDashboard";

export const metadata: Metadata = {
  title: "Paper Desk — Go Engine",
  description: "Live Go Engine paper-trading account: positions, trades, OMS, equity, strategy health.",
};

export const dynamic = "force-dynamic";

export default function PaperDeskPage() {
  return <PaperDeskDashboard />;
}
