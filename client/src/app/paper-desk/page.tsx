import { redirect } from "next/navigation";
import { legacyPaperDeskRedirect } from "@/lib/navRoutes";

type PaperDeskLegacyPageProps = {
  searchParams: Promise<{ tab?: string }>;
};

/** Retired paper desk route — redirects to mock trading. */
export default async function PaperDeskLegacyPage({ searchParams }: PaperDeskLegacyPageProps) {
  const { tab } = await searchParams;
  redirect(legacyPaperDeskRedirect(tab));
}
