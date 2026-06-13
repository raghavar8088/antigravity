"use client";

import { useCallback, useState } from "react";
import type { ShadowIntentListItem } from "@/lib/ai/shadowTradeIntentTypes";
import { DeskButton } from "@/components/desk/ui/DeskButton";
import { DeskChip } from "@/components/desk/ui/DeskChip";
import type { DeskColumn } from "@/components/desk/ui/DeskDataTable";
import { DeskDataTable } from "@/components/desk/ui/DeskDataTable";
import { DeskEmptyState } from "@/components/desk/ui/DeskEmptyState";
import { DeskSectionHeader } from "@/components/desk/ui/DeskSectionHeader";

const SHADOW_LOG_LIMIT = 20;

export function ShadowIntentLogPanel({
  enabled,
  signedIn,
}: {
  enabled: boolean;
  signedIn: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [intents, setIntents] = useState<ShadowIntentListItem[] | null>(null);

  const load = useCallback(async () => {
    if (!signedIn) return;
    setLoading(true);
    setError(null);
    try {
      const res = await fetch(`/api/shadow-trade-intents?limit=${SHADOW_LOG_LIMIT}`, {
        credentials: "include",
        cache: "no-store",
      });
      const body = (await res.json()) as {
        ok?: boolean;
        error?: string;
        intents?: ShadowIntentListItem[];
      };
      if (!res.ok || !body.ok) {
        setError(body.error ?? `HTTP ${res.status}`);
        setIntents(null);
        return;
      }
      setIntents(body.intents ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Load failed");
      setIntents(null);
    } finally {
      setLoading(false);
    }
  }, [signedIn]);

  if (!enabled) return null;

  const columns: DeskColumn<ShadowIntentListItem>[] = [
    {
      id: "time",
      header: "Time",
      cell: (row) => row.createdAt.slice(11, 19),
    },
    {
      id: "kind",
      header: "Kind",
      cell: (row) => <DeskChip tone="primary">{row.intentKind}</DeskChip>,
    },
    {
      id: "sym",
      header: "Symbol",
      cell: (row) => row.symbol,
    },
    {
      id: "side",
      header: "Side",
      cell: (row) => row.side,
    },
    {
      id: "detail",
      header: "Detail",
      cell: (row) =>
        row.intentKind === "close"
          ? `${row.entryPrice.toFixed(0)}→${row.exitPrice?.toFixed(0) ?? "—"} ${row.exitReason ?? ""}`
          : `@ ${row.entryPrice.toFixed(0)}`,
    },
    {
      id: "testnet",
      header: "Testnet",
      align: "right",
      cell: (row) => (
        <DeskChip tone={row.wouldPlaceTestnet ? "success" : "default"}>
          {row.wouldPlaceTestnet ? "Would place" : "No"}
        </DeskChip>
      ),
    },
  ];

  return (
    <div style={{ marginTop: 16 }}>
      <DeskSectionHeader
        title={`Shadow log (last ${SHADOW_LOG_LIMIT})`}
        subtitle="Paper intents only — no testnet orders"
        actions={
          <>
            <DeskButton variant="outlined" style={{ minHeight: 36 }} onClick={() => setOpen((v) => !v)}>
              {open ? "Collapse" : "Expand"}
            </DeskButton>
            {signedIn ? (
              <DeskButton variant="outlined" style={{ minHeight: 36 }} disabled={loading} onClick={() => void load()}>
                {loading ? "Loading…" : "Refresh"}
              </DeskButton>
            ) : null}
          </>
        }
      />
      {open ? (
        !signedIn ? (
          <p className="desk-label-md">Sign in to record and view shadow intents.</p>
        ) : error ? (
          <p className="desk-label-md" style={{ color: "var(--desk-error)" }}>
            {error}
          </p>
        ) : (
          <DeskDataTable
            columns={columns}
            rows={intents ?? []}
            getRowKey={(row) => row.id}
            minWidth={520}
            empty={<DeskEmptyState title="No shadow intents yet" />}
          />
        )
      ) : null}
    </div>
  );
}
