import { redirect } from "next/navigation";

type PaperdeskAliasPageProps = {
  searchParams: Promise<{ tab?: string }>;
};

/** Legacy alias — /paperdesk → /paper-desk */
export default async function PaperdeskAliasPage({ searchParams }: PaperdeskAliasPageProps) {
  const params = await searchParams;
  const tab = params.tab?.trim();
  redirect(tab ? `/paper-desk?tab=${encodeURIComponent(tab)}` : "/paper-desk");
}
