import { NextResponse } from "next/server";

export function mongoUnconfigured() {
  return NextResponse.json(
    { ok: false, code: "MONGO_NOT_CONFIGURED", error: "MongoDB not configured" },
    { status: 503 },
  );
}

export function mongoUnavailable(detail = "unknown") {
  return NextResponse.json(
    { ok: false, code: "MONGO_UNAVAILABLE", error: "MongoDB unavailable", detail },
    { status: 503 },
  );
}
