"use client";

/**
 * High Volume Crypto Trading — the module shell.
 *
 * HAND-WRITTEN, unlike DeskView.tsx beside it. That file is generated from the
 * Crypto Scalp Desk page by scripts/clone_high_volume_page.py so a fix to the
 * scalp desk reaches this desk instead of the two drifting apart; this one owns
 * the tab strip and is never generated over.
 *
 * Two tabs, because they answer two different questions:
 *
 *   Desk           all 20,088 streams, ranked. A sort of every record the
 *                  process holds, which is how candidates are FOUND.
 *   Paper Trading  the nine streams the owner picked off that board, trading
 *                  one $500 balance. Which is how a candidate is TESTED.
 *
 * Splitting them is the point. A leaderboard row is earned while competing with
 * 20,087 other streams for three concurrency slots, so it reports what a stream
 * did with the fills it happened to get. The book reports what nine of them do
 * with a balance of their own. Rendering both under one heading would invite
 * exactly the reading this desk keeps having to correct — that a high rank and
 * a working strategy are the same claim.
 */

import { useState } from "react";
import { DeskTabs, type DeskTabItem } from "@/components/desk/ui";
import DeskView from "./DeskView";
import PaperTradingTab from "./PaperTradingTab";

type Tab = "desk" | "paper";

const TABS: DeskTabItem<Tab>[] = [
  { key: "desk", label: "Desk" },
  { key: "paper", label: "High Volume Crypto Paper Trading" },
];

export default function HighVolumeCryptoPage() {
  const [tab, setTab] = useState<Tab>("desk");

  return (
    <div style={{ minHeight: "100%", background: "var(--desk-surface-dim)" }}>
      {/* The strip sits above both views, each of which renders its own
          breadcrumb and heading. Kept out of the views deliberately: DeskView is
          regenerated from another page, so anything the generator owns would be
          replaced on the next run. */}
      <div
        style={{
          background: "var(--desk-surface)",
          borderBottom: "1px solid var(--desk-outline)",
        }}
      >
        <div className="desk-page" style={{ paddingTop: 0, paddingBottom: 0, gap: 0 }}>
          <DeskTabs items={TABS} active={tab} onChange={setTab} variant="primary" />
        </div>
      </div>

      {/* Mounted one at a time. The two views poll different endpoints on
          different intervals, and keeping the hidden one alive would double the
          request rate against a single-process engine for a page nobody is
          looking at. */}
      {tab === "desk" ? <DeskView /> : <PaperTradingTab />}
    </div>
  );
}
