"use client";

/**
 * Delta Paper Trading — the Delta Exchange India terminal, with paper money.
 *
 * Real quotes, real contract specs, real funding and the venue's REAL L2 order
 * book: a market order here is walked through actual resting depth and receives
 * the size-weighted price it would have paid, rather than being filled at the
 * last print. Everything a trading screen needs is here — market, limit, stop
 * and stop-limit orders, leverage, brackets, netted positions, liquidation
 * against the venue's own maintenance margin, funding every eight hours, and
 * the order and trade history behind it.
 *
 * NO KEYS, NO ROUTE TO A BROKER. Every input is a public Delta market-data
 * endpoint. Nothing on this page can place a real order.
 */

import Link from "next/link";
import { DeskBanner, DeskSectionHeader } from "@/components/desk/ui";
import TradingTerminal from "@/components/paperTrading/TradingTerminal";

export default function DeltaPaperTradingPage() {
  return (
    <main style={{ padding: "var(--desk-space-5)", maxWidth: 1720, margin: "0 auto" }}>
      <nav className="desk-label-md" style={{ marginBottom: 10, opacity: 0.7 }}>
        Home <span aria-hidden>›</span> Delta Paper Trading
      </nav>

      <DeskSectionHeader
        title="Delta Paper Trading"
        subtitle={
          "The Delta Exchange India terminal on live market data, traded with paper money. Market, " +
          "limit, stop and stop-limit orders against the venue's real order book, with leverage, " +
          "brackets, netted positions, funding and liquidation modelled the way Delta does them."
        }
        actions={
          <Link
            href="/forex-paper-trading"
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
            Forex Paper Trading →
          </Link>
        }
      />

      <div style={{ marginBottom: "var(--desk-space-4)" }}>
        <DeskBanner variant="info" title="Paper money, real market">
          Prices, contract specifications, funding rates and order-book depth are live from Delta
          Exchange India. The balance is imaginary. This module holds no API key and has no
          order-routing path — there is no code here that could reach a real order, on any venue.
        </DeskBanner>
      </div>

      <TradingTerminal venue="delta" />
    </main>
  );
}
