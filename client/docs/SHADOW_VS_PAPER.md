# Shadow intents vs paper trades

Compare **durable paper closes** (`paper_trades`) with **shadow log rows** (`shadow_trade_intents`) to see whether paper desk activity is being recorded for a future testnet path. Shadow rows are **not** orders — they never call Delta `placeOrder`.

Related: [LIVE_TRADING_PHASE.md](./LIVE_TRADING_PHASE.md) (P3-C), [DEPLOY.md](./DEPLOY.md) (auth + migrations).

---

## Table alignment

| | `paper_trades` | `shadow_trade_intents` |
|---|----------------|------------------------|
| User key | `account_key` (Supabase `auth.users.id`) | `user_id` (same UUID when signed in) |
| Close identity | `client_trade_id` (unique) | `client_intent_id` — **same UUID on close** when P3-C is on |
| Day bucket | `date(closed_at AT TIME ZONE 'UTC')` | `date(created_at AT TIME ZONE 'UTC')` for `intent_kind = 'close'` |
| Opens | N/A (closes only) | `intent_kind = 'open'` only if `NEXT_PUBLIC_DESK_SHADOW_LOG_OPEN=1` |

**Expected when P3-C is fully enabled:** one shadow **close** per paper **close** for the same user, joined on `client_intent_id = client_trade_id`.

---

## Parameters

Replace before running in Supabase SQL Editor:

```sql
-- Your auth user id (same as paper_trades.account_key when cloud-synced)
\set user_id '00000000-0000-0000-0000-000000000000'
\set day '2026-05-16'   -- UTC calendar day
```

Without psql meta-commands, substitute literals:

```sql
-- WHERE account_key = 'YOUR_USER_UUID'
-- AND day = '2026-05-16'
```

---

## 1) Daily counts per user

```sql
with params as (
  select
    'YOUR_USER_UUID'::text as user_id,
    '2026-05-16'::date as day
),
paper_day as (
  select count(*)::int as paper_closes
  from public.paper_trades p
  cross join params
  where p.account_key = params.user_id
    and (p.closed_at at time zone 'UTC')::date = params.day
),
shadow_day as (
  select
    count(*) filter (where s.intent_kind = 'close')::int as shadow_closes,
    count(*) filter (where s.intent_kind = 'open')::int as shadow_opens,
    count(*) filter (where s.intent_kind = 'close' and s.would_place_testnet)::int as closes_would_testnet,
    count(*) filter (where s.intent_kind = 'close' and not s.would_place_testnet)::int as closes_would_not_testnet
  from public.shadow_trade_intents s
  cross join params
  where s.user_id = params.user_id
    and (s.created_at at time zone 'UTC')::date = params.day
)
select
  params.user_id,
  params.day,
  paper_day.paper_closes,
  shadow_day.shadow_closes,
  shadow_day.shadow_opens,
  shadow_day.closes_would_testnet,
  shadow_day.closes_would_not_testnet,
  paper_day.paper_closes - shadow_day.shadow_closes as close_count_delta
from params, paper_day, shadow_day;
```

| `close_count_delta` | Typical meaning |
|---------------------|-----------------|
| `0` | Aligned close logging |
| `> 0` | Paper synced but shadow missing (see mismatches below) |
| `< 0` | Shadow closes without matching paper row (rare; investigate duplicates or wrong user id) |

---

## 2) Matched closes (sanity check)

```sql
select
  p.client_trade_id,
  p.closed_at as paper_closed_at,
  s.created_at as shadow_logged_at,
  p.symbol,
  p.side,
  p.strategy_id,
  p.exit_reason,
  p.notional as paper_notional,
  s.notional as shadow_notional,
  s.would_place_testnet,
  (p.exit_reason is distinct from s.exit_reason) as exit_reason_mismatch,
  (abs(p.notional - s.notional) > 0.01) as notional_mismatch
from public.paper_trades p
inner join public.shadow_trade_intents s
  on s.client_intent_id = p.client_trade_id
  and s.user_id = p.account_key
  and s.intent_kind = 'close'
where p.account_key = 'YOUR_USER_UUID'
  and (p.closed_at at time zone 'UTC')::date = '2026-05-16'::date
order by p.closed_at desc;
```

---

## 3) Mismatches

### Paper closes with no shadow close (shadow missing)

```sql
select
  p.client_trade_id,
  p.closed_at,
  p.symbol,
  p.side,
  p.strategy_id,
  p.strategy_name,
  p.exit_reason,
  p.net_pnl
from public.paper_trades p
left join public.shadow_trade_intents s
  on s.client_intent_id = p.client_trade_id
  and s.intent_kind = 'close'
  and s.user_id = p.account_key
where p.account_key = 'YOUR_USER_UUID'
  and (p.closed_at at time zone 'UTC')::date = '2026-05-16'::date
  and s.id is null
order by p.closed_at desc;
```

**Common causes**

- `NEXT_PUBLIC_DESK_SHADOW_INTENTS` not `1` in the browser build that ran the desk.
- User was **logged out** — paper still runs locally; cloud `POST /api/paper-trades` and shadow POST are skipped.
- Shadow `POST` failed (network / 401 / 503) — best-effort fire-and-forget; paper sync may still succeed.
- Row closed **before** P3-C deploy or before migration `005_shadow_trade_intents.sql`.

### Shadow closes with no paper row (orphan shadow)

```sql
select
  s.client_intent_id,
  s.created_at,
  s.symbol,
  s.side,
  s.strategy_id,
  s.exit_reason,
  s.would_place_testnet
from public.shadow_trade_intents s
left join public.paper_trades p
  on p.client_trade_id = s.client_intent_id
  and p.account_key = s.user_id
where s.user_id = 'YOUR_USER_UUID'
  and s.intent_kind = 'close'
  and (s.created_at at time zone 'UTC')::date = '2026-05-16'::date
  and p.id is null
order by s.created_at desc;
```

**Common causes**

- Paper upsert failed but shadow succeeded (unusual).
- Legacy data copied to `paper_trades` under a different `account_key` than current `user_id`.
- Manual / test insert into `shadow_trade_intents`.

### Multi-user daily rollup (ops)

```sql
select
  coalesce(p.account_key, s.user_id) as user_id,
  coalesce((p.closed_at at time zone 'UTC')::date, (s.created_at at time zone 'UTC')::date) as day,
  count(distinct p.client_trade_id) as paper_closes,
  count(distinct s.id) filter (where s.intent_kind = 'close') as shadow_closes
from public.paper_trades p
full outer join public.shadow_trade_intents s
  on s.client_intent_id = p.client_trade_id
  and s.user_id = p.account_key
  and s.intent_kind = 'close'
where (p.closed_at is not null and (p.closed_at at time zone 'UTC')::date >= current_date - 7)
   or (s.created_at is not null and (s.created_at at time zone 'UTC')::date >= current_date - 7)
group by 1, 2
order by day desc, user_id;
```

---

## `would_place_testnet = false` — what it means

Set **only on the server** when a shadow row is inserted (`computeWouldPlaceTestnetShadow()` in `client/src/server/delta/shadowWouldPlaceTestnet.ts`). It does **not** mean “the desk would not have traded paper.” It means: **at insert time, the app would not have been ready to place a real testnet order** (even though no order is placed for shadow).

| Condition | `would_place_testnet` | Notes |
|-----------|------------------------|--------|
| `DELTA_TESTNET` is not `true` or `1` | `false` | Testnet execution adapter refuses mainnet; shadow still logs |
| `DELTA_API_KEY` or `DELTA_API_SECRET` missing/empty on server | `false` | Credentials check throws |
| `DELTA_TESTNET=1` **and** both keys set | `true` | Ready for testnet *infrastructure*; still no auto order from shadow |

**Not stored in the row (client / feature flags)**

| Condition | Effect on shadow table | `would_place_testnet` |
|-----------|------------------------|------------------------|
| `NEXT_PUBLIC_DESK_SHADOW_INTENTS` ≠ `1` | No shadow rows at all | — |
| User logged out | No shadow POST | — |
| `NEXT_PUBLIC_DESK_TESTNET_OPS` ≠ `1` | Manual testnet panel hidden; **does not** change shadow flag | Independent |
| `NEXT_PUBLIC_DESK_SHADOW_LOG_OPEN` ≠ `1` | No `open` intents; **closes still logged** if shadow on | Unchanged for closes |

### SQL: breakdown of `would_place_testnet` by day

```sql
select
  (created_at at time zone 'UTC')::date as day,
  intent_kind,
  would_place_testnet,
  count(*)::int as n
from public.shadow_trade_intents
where user_id = 'YOUR_USER_UUID'
  and (created_at at time zone 'UTC')::date >= current_date - 14
group by 1, 2, 3
order by day desc, intent_kind, would_place_testnet;
```

If all closes show `would_place_testnet = false`, fix **server** env on Vercel/local (`DELTA_TESTNET=1`, keys set) — not the desk policy env vars.

---

## Interpretation checklist

1. Run migration **005** and confirm `NEXT_PUBLIC_DESK_SHADOW_INTENTS=1` on the deployment users hit.
2. User must be **signed in** (`account_key` / `user_id` = auth UUID).
3. Compare daily counts (§1); investigate §3 mismatches if `close_count_delta` ≠ 0.
4. Use §`would_place_testnet` breakdown to separate “shadow logging off” from “testnet keys off.”
5. For live orders, use [LIVE_TRADING_PHASE.md](./LIVE_TRADING_PHASE.md) P3-B (`NEXT_PUBLIC_DESK_TESTNET_OPS=1`) — separate from shadow.
