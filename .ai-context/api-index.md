# API Index

## Main API Families
- `/api/auth/*`: session, sign-in, sign-out, and auth checks.
- `/api/paper-*`: BTC paper desk state, trades, replay, diagnostics, OMS, and worker/cron behavior.
- `/api/options/*`: BTC options positions, trades, strategies, stats, and reset.
- `/api/nifty/*`: NIFTY candles, option chain, stream, VIX, seed engine, and state.
- `/api/nifty-options/*`: NIFTY options workflows.
- `/api/nifty-options-selling/*`: NIFTY options selling workflows.
- `/api/nifty-stocks/*`: NIFTY stock engine/state workflows.
- `/api/delta-live/*`: Delta live stats, trades, enablement, mode, and manual orders.
- `/api/angelone/*`: AngelOne funds, orders, cancel order, and broker actions.
- `/api/engine/[...path]`: proxy from Next.js to the Go engine through `INTERNAL_API_URL`.
- `/api/cron/*`: scheduled worker endpoints.
- `/api/admin/*`: administrative controls such as kill/reset/migration.
- `/api/ai-app-tracker/*`: AI app tracker capture/reporting.

## Debugging Route Issues
Trace route behavior in this order:

```text
route handler
-> auth/session or admin secret
-> domain helper / broker client
-> persistence or engine proxy
-> response shape
-> hook/component consumer
```

## Query Hints
- Ask Graphify for exact route files before reading source.
- Use `dependencies.json` to identify helper modules used by a route.
- Read raw route files only after identifying the specific endpoint family.
