import { GET as rankingsGet } from "../rankings/route";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const url = new URL(req.url);
  url.searchParams.set("view", "retirement");
  return rankingsGet(new Request(url.toString(), req));
}
