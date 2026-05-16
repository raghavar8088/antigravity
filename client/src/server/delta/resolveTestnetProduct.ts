import { deltaPublicFetch } from "@/lib/deltaSign";

type ProductRow = {
  id?: number;
  symbol?: string;
  contract_type?: string;
};

/**
 * Resolve perpetual futures `product_id` on Delta testnet by symbol (e.g. BTCUSD).
 */
export async function resolveTestnetPerpProductId(symbol: string): Promise<number | null> {
  const sym = symbol.trim().toUpperCase();
  const res = await deltaPublicFetch(
    "/v2/products?contract_types=perpetual_futures&page_size=250",
    { testnet: true },
  );
  if (!res.ok) return null;
  const products = (res.data as { result?: ProductRow[] }).result ?? [];
  const exact = products.find((p) => (p.symbol ?? "").toUpperCase() === sym);
  if (exact?.id) return exact.id;
  const partial = products.find((p) => (p.symbol ?? "").toUpperCase().includes(sym));
  return partial?.id ?? null;
}
