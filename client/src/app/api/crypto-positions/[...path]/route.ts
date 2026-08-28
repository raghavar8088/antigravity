/**
 * Crypto Positions API.
 *
 * Reads on GET, mutations on POST. Everything runs in this process: Delta's
 * public market data in, MongoDB for the book, and no path of any kind to a
 * broker — this desk holds no key and signs no request.
 *
 * The margin gate lives in the engine's executeBasket, not here. A route that
 * could place a fill without going through it would make the gate advisory,
 * and the gate is the only thing keeping a paper account from writing cheques
 * its balance cannot cover.
 */

import { NextRequest, NextResponse } from "next/server";
import {
  closePositions,
  contractSpecs,
  executeBasket,
  exitPosition,
  getOptionChain,
  getSnapshot,
  getTopMovers,
  listOptionExpiries,
  listPerpetuals,
  listUnderlyings,
  livePositions,
  placeOrder,
  previewBasket,
  reducePosition,
  Rejected,
  rollToAtm,
  summary,
} from "@/lib/cryptoPositions/engine";
import {
  createAccount,
  deleteAccount,
  editAccount,
  listAccounts,
  listOrders,
  NotConfigured,
  resetAccount,
} from "@/lib/cryptoPositions/store";
import type { BasketLeg, TransactionType } from "@/lib/cryptoPositions/types";

export const dynamic = "force-dynamic";

function ok(data: unknown): NextResponse {
  return NextResponse.json({ ok: true, ...(data as object) });
}

function fail(e: unknown): NextResponse {
  if (e instanceof Rejected) {
    return NextResponse.json({ ok: false, error: e.message }, { status: 400 });
  }
  if (e instanceof NotConfigured) {
    return NextResponse.json({ ok: false, error: e.message }, { status: 503 });
  }
  const message = e instanceof Error ? e.message : "Unexpected error";
  return NextResponse.json({ ok: false, error: message }, { status: 500 });
}

function qp(req: NextRequest, key: string): string | null {
  const v = req.nextUrl.searchParams.get(key);
  return v && v.trim() ? v.trim() : null;
}

function qn(req: NextRequest, key: string, dflt: number): number {
  const raw = qp(req, key);
  if (!raw) return dflt;
  const n = Number(raw);
  return Number.isFinite(n) ? n : dflt;
}

function requireAccount(req: NextRequest): string {
  const id = qp(req, "account_id");
  if (!id) throw new Rejected("account_id is required.");
  return id;
}

export async function GET(req: NextRequest, ctx: { params: Promise<{ path: string[] }> }) {
  const { path } = await ctx.params;
  const route = (path ?? []).join("/");
  try {
    switch (route) {
      case "accounts":
        return ok({ accounts: await listAccounts() });

      case "underlyings":
        return ok({ underlyings: await listUnderlyings() });

      case "options/expiries": {
        const u = qp(req, "underlying");
        if (!u) throw new Rejected("underlying is required.");
        return ok({ expiries: await listOptionExpiries(u) });
      }

      case "options/chain": {
        const u = qp(req, "underlying");
        const e = qp(req, "expiry");
        if (!u || !e) throw new Rejected("underlying and expiry are required.");
        return ok({ chain: await getOptionChain(u, e) });
      }

      case "perpetuals":
        return ok({ perpetuals: await listPerpetuals() });

      case "specs":
        return ok({ specs: await contractSpecs() });

      case "top-movers":
        return ok(await getTopMovers(Math.max(1, Math.min(qn(req, "limit", 10), 50))));

      case "positions": {
        const accountId = requireAccount(req);
        const statusRaw = qp(req, "status");
        const status = statusRaw === "OPEN" || statusRaw === "CLOSED" ? statusRaw : undefined;
        const [positions, s] = await Promise.all([livePositions(accountId, status), summary(accountId)]);
        return ok({ positions, summary: s });
      }

      case "orders":
        return ok({ orders: await listOrders(requireAccount(req)) });

      case "summary":
        return ok({ summary: await summary(requireAccount(req)) });

      case "status": {
        // What the page's freshness line reads from.
        const s = await getSnapshot();
        return ok({
          builtAt: s.builtAt,
          options: s.options.length,
          perpetuals: s.perpetuals.length,
          underlyings: s.underlyings.length,
        });
      }

      default:
        return NextResponse.json({ ok: false, error: `Unknown route ${route}` }, { status: 404 });
    }
  } catch (e) {
    return fail(e);
  }
}

type Body = {
  account_id?: string;
  name?: string;
  initial_capital?: number;
  symbol?: string;
  transaction_type?: TransactionType;
  lots?: number;
  order_type?: "MARKET" | "LIMIT";
  limit_price?: number | null;
  position_id?: string;
  position_ids?: string[];
  legs?: BasketLeg[];
};

export async function POST(req: NextRequest, ctx: { params: Promise<{ path: string[] }> }) {
  const { path } = await ctx.params;
  const route = (path ?? []).join("/");
  try {
    const body = (await req.json().catch(() => ({}))) as Body;
    const accountOf = () => {
      if (!body.account_id) throw new Rejected("account_id is required.");
      return body.account_id;
    };

    switch (route) {
      case "accounts":
        return ok({ account: await createAccount(body.name ?? "Account", body.initial_capital) });

      case "accounts/edit": {
        const a = await editAccount(accountOf(), {
          name: body.name,
          initialCapital: body.initial_capital,
        });
        if (!a) throw new Rejected("No such account.");
        return ok({ account: a });
      }

      case "accounts/delete":
        await deleteAccount(accountOf());
        return ok({ deleted: true });

      case "basket/preview":
        return ok({ preview: await previewBasket(accountOf(), body.legs ?? []) });

      case "basket/execute":
        return ok(await executeBasket(accountOf(), body.legs ?? []));

      case "orders": {
        if (!body.symbol) throw new Rejected("symbol is required.");
        if (!body.transaction_type) throw new Rejected("transaction_type is required.");
        const r = await placeOrder({
          accountId: accountOf(),
          symbol: body.symbol,
          transactionType: body.transaction_type,
          lots: body.lots ?? 1,
          orderType: body.order_type ?? "MARKET",
          limitPrice: body.limit_price ?? null,
        });
        return ok(r);
      }

      case "positions/exit": {
        if (!body.position_id) throw new Rejected("position_id is required.");
        return ok(await exitPosition(accountOf(), body.position_id));
      }

      case "positions/close-many": {
        const ids = body.position_ids ?? [];
        if (ids.length === 0) throw new Rejected("position_ids is required.");
        return ok(await closePositions(accountOf(), ids));
      }

      case "positions/reduce": {
        if (!body.position_id) throw new Rejected("position_id is required.");
        return ok(await reducePosition(accountOf(), body.position_id, body.lots ?? 1));
      }

      // Omitting position_ids rolls every option leg — the "all legs" button.
      case "positions/roll-atm":
        return ok(await rollToAtm(accountOf(), body.position_ids));

      case "reset":
        return ok(await resetAccount(accountOf()));

      default:
        return NextResponse.json({ ok: false, error: `Unknown route ${route}` }, { status: 404 });
    }
  } catch (e) {
    return fail(e);
  }
}
