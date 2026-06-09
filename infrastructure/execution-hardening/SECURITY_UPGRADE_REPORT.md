# Security Upgrade Report

## Implemented

1. **JWT role claim** — `client/src/lib/jwtSession.ts` includes `role: "TRADER"` for authenticated users
2. **RBAC** — `PermTradeRequest` for human execution intents; `PermTradeExecute` service-only
3. **Route retirement** — 410 on all direct broker Next.js routes
4. **Engine gateway** — `POST /api/execution/request` with authn/authz
5. **Service proxy** — Next.js execution proxy sends `X-Service-Name` + optional `INTERNAL_API_SECRET`
6. **Rate limiting** — existing middleware on `/api/*` (unchanged)
7. **Delta probe** — POST order probe disabled in `delta/probe/route.ts`

## Remaining gaps

- CSRF: SameSite=lax on session cookie (existing); no double-submit token
- Angel One live adapter not wired through ETP (requests rejected)
- Delta `OnClose` uses kill check but not full OMS replay
