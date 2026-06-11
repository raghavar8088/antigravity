# OBJECTIVE 8 — REAL-TIME EVENT CENTER

## Route

`/terminal/events` → `EventCenter.tsx`

## APIs

- `GET /api/event-center` — REST poll 3s
- `GET /api/engine/events` — SSE stream 3s

## Event Types

FILL · ORDER · POSITION_OPEN · RISK_EVENT · KILL_SWITCH · (RECON via reconciliation API)

## Features

Newest first, filter by type/severity, search, auto-scroll toggle.

## Source

`platformEvents.ts` — Mongo `paper_trades`, `paper_positions`, `paper_orders`, `paper_state`, engine kill switch.
