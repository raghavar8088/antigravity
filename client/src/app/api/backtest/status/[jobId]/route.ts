import { NextRequest, NextResponse } from 'next/server'

const ENGINE_URL = process.env.INTERNAL_API_URL ?? 'http://localhost:8080'

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ jobId: string }> }
) {
  const { jobId } = await params
  try {
    const res = await fetch(
      `${ENGINE_URL}/api/backtest/status/${jobId}`,
      {
        headers: { 'Authorization': request.headers.get('Authorization') ?? '' },
        signal: AbortSignal.timeout(10_000),
      }
    )
    if (!res.ok) {
      return NextResponse.json(
        { error: `Engine returned ${res.status}` },
        { status: res.status }
      )
    }
    const data = await res.json()
    return NextResponse.json(data)
  } catch (err) {
    return NextResponse.json(
      { error: 'Failed to fetch backtest status', detail: String(err) },
      { status: 502 }
    )
  }
}
