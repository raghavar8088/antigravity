/**
 * /api/delta/myip — Returns the outbound IP that Vercel uses when calling external APIs.
 * Use this to find the exact IP to whitelist on Delta Exchange.
 */
import { NextResponse } from "next/server";

export async function GET() {
  try {
    const res = await fetch("https://api.ipify.org?format=json", { cache: "no-store" });
    const data = await res.json() as { ip: string };
    return NextResponse.json({ ip: data.ip, message: "Add this IP to Delta Exchange whitelist" });
  } catch {
    return NextResponse.json({ error: "Could not fetch IP" }, { status: 500 });
  }
}
