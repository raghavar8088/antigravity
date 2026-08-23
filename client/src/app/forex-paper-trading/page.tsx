"use client";

/**
 * Forex Paper Trading — a retail FX/CFD terminal, with paper money.
 *
 * A PEER of Delta Paper Trading rather than a section inside it: the same
 * engine pointed at a different market, with lots instead of contracts, pips
 * instead of ticks, swap instead of funding, hedged tickets instead of a netted
 * position, and an account managed on margin level with a margin call and a
 * stop out rather than per-position liquidation. Neither desk depends on the
 * other; the cross-links between them are convenience, not hierarchy.
 *
 * WHAT IS REAL AND WHAT IS MODELLED is stated on the page rather than buried.
 * Mid prices and candles for FX majors and crosses, metals, energies, indices
 * and crypto CFDs are live from Yahoo Finance. The SPREAD, the SWAP and the
 * COMMISSION are modelled per instrument and per account tier, because no free
 * feed publishes a retail broker's bid, ask or overnight financing — and the
 * spread is the entire cost base of a short-term strategy, so quoting an
 * invented one as though a broker had published it would produce a P&L that
 * looks precise and is not.
 */

import Link from "next/link";
import { DeskBanner, DeskSectionHeader } from "@/components/desk/ui";
import TradingTerminal from "@/components/paperTrading/TradingTerminal";

export default function ForexPaperTradingPage() {
  return (
    <main style={{ padding: "var(--desk-space-5)", maxWidth: 1720, margin: "0 auto" }}>
      <nav className="desk-label-md" style={{ marginBottom: 10, opacity: 0.7 }}>
        Home <span aria-hidden>›</span> Forex Paper Trading
      </nav>

      <DeskSectionHeader
        title="Forex Paper Trading"
        subtitle={
          "A retail FX and CFD terminal on live prices, traded with paper money. Lots and pips, " +
          "leverage to 1:2000, hedged tickets, swap at the 21:00 UTC rollover — tripled on Wednesday " +
          "— and an account managed on margin level with a margin call and a stop out."
        }
        actions={
          <Link
            href="/delta-paper-trading"
            className="desk-label-md"
            style={{
              display: "inline-flex",
              alignItems: "center",
              minHeight: 44,
              padding: "0 18px",
              borderRadius: "var(--desk-radius-button)",
              border: "1px solid var(--desk-outline)",
              color: "var(--desk-primary)",
              fontWeight: 600,
            }}
          >
            ← Delta Paper Trading
          </Link>
        }
      />

      <div style={{ marginBottom: "var(--desk-space-4)" }}>
        <DeskBanner variant="warning" title="Real prices, modelled spreads">
          Mid prices and candles are live. The <strong>spread, swap and commission are MODELLED</strong>{" "}
          per instrument and per account tier — no free feed publishes a retail broker&apos;s bid and ask,
          and none publishes one broker&apos;s overnight financing. Every quote on this desk is flagged
          accordingly, because on a short-term strategy the spread is the whole cost base. The money is
          paper and there is no order-routing path to any broker.
        </DeskBanner>
      </div>

      <TradingTerminal venue="forex" />
    </main>
  );
}
