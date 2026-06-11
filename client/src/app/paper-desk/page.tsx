import { redirect } from "next/navigation";
import { legacyPaperDeskRedirect } from "@/lib/navRoutes";

type PaperDeskLegacyPageProps = {
  searchParams: Promise<{ tab?: string }>;
};

/** Legacy route — redirects to Institutional Command Center. */
export default async function PaperDeskLegacyPage({ searchParams }: PaperDeskLegacyPageProps) {
  const { tab } = await searchParams;
  redirect(legacyPaperDeskRedirect(tab));
}
