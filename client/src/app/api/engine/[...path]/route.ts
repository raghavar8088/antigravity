import { NextRequest, NextResponse } from "next/server";

function upstreamBase(): string {
  const raw =
    process.env.INTERNAL_API_URL?.trim() ||
    process.env.ENGINE_URL?.trim() ||
    "http://127.0.0.1:8080";
  return raw.replace(/\/+$/, "");
}

async function proxyToEngine(req: NextRequest, segments: string[]): Promise<Response> {
  const path = segments.length ? `/${segments.join("/")}` : "/";
  const target = `${upstreamBase()}${path}${req.nextUrl.search}`;

  const headers = new Headers();
  const contentType = req.headers.get("content-type");
  if (contentType) {
    headers.set("content-type", contentType);
  }

  const init: RequestInit = {
    method: req.method,
    headers,
    cache: "no-store",
    signal: AbortSignal.timeout(60_000),
  };

  if (req.method !== "GET" && req.method !== "HEAD") {
    init.body = await req.arrayBuffer();
  }

  return fetch(target, init);
}

function passthrough(res: Response): NextResponse {
  const out = new Headers(res.headers);
  out.delete("transfer-encoding");
  return new NextResponse(res.body, { status: res.status, headers: out });
}

type RouteCtx = { params: Promise<{ path: string[] }> };

async function handle(req: NextRequest, ctx: RouteCtx): Promise<NextResponse> {
  const { path } = await ctx.params;
  try {
    const upstream = await proxyToEngine(req, path ?? []);
    return passthrough(upstream);
  } catch (e) {
    const message = e instanceof Error ? e.message : "proxy failed";
    return NextResponse.json(
      { ok: false, error: message, hint: "Set INTERNAL_API_URL to your Go engine base (e.g. https://….onrender.com)" },
      { status: 502 },
    );
  }
}

export const GET = handle;
export const POST = handle;
export const PUT = handle;
export const PATCH = handle;
export const DELETE = handle;
