import { NextResponse } from "next/server";
import { getLatestVerificationEvents } from "@/lib/verificationTrack/verificationTrackMongo";

export const dynamic = "force-dynamic";

export async function GET() {
  const events = await getLatestVerificationEvents(30);
  return NextResponse.json({ ok: true, events });
}
