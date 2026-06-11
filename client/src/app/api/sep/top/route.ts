import { GET as rankingsGet } from "../rankings/route";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const url = new URL(req.url);
  url.searchParams.set("view", "top");
  url.searchParams.set("limit", url.searchParams.get("limit") ?? "20");
  return rankingsGet(new Request(url.toString(), req));
}
