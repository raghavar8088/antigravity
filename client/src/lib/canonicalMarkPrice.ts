/**
 * Single source of truth for the BTC mark price across all desk consumers.
 * Prevents divergence when watchlist and engine poll update at different cadences.
 */

type MarkPriceEntry = { price: number; source: string; updatedAtMs: number };

let _canonical: MarkPriceEntry | null = null;

export function setCanonicalMark(price: number, source: string): void {
  if (!Number.isFinite(price) || price <= 0) return;
  _canonical = { price, source, updatedAtMs: Date.now() };
}

export function getCanonicalMark(): number | null {
  if (markAgeMs() > 30_000) {
    console.warn(
      "[MarkPrice] Stale mark — age > 30s, last:",
      _canonical?.price ?? "none",
    );
  }
  return _canonical?.price ?? null;
}

export function markAgeMs(): number {
  if (!_canonical) return Infinity;
  return Date.now() - _canonical.updatedAtMs;
}

export function getCanonicalMarkEntry(): Readonly<MarkPriceEntry> | null {
  return _canonical;
}
