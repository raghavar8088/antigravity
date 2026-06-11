import { redirect } from "next/navigation";
import { legacyPaperDeskRedirect } from "@/lib/navRoutes";

type PaperdeskAliasPageProps = {
  searchParams: Promise<{ tab?: string }>;
};

/** Legacy alias — /paperdesk → Command Center */
export default async function PaperdeskAliasPage({ searchParams }: PaperdeskAliasPageProps) {
  const { tab } = await searchParams;
  redirect(legacyPaperDeskRedirect(tab));
}
