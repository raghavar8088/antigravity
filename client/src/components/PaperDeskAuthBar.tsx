"use client";

import { useState } from "react";
import { usePaperDeskAuth } from "@/hooks/usePaperDeskAuth";
import { DeskBanner } from "@/components/desk/ui/DeskBanner";
import { DeskButton } from "@/components/desk/ui/DeskButton";

export function PaperDeskAuthBar({ compact = false }: { compact?: boolean }) {
  const { configured, user, loading, message, signInWithEmail, signOut } = usePaperDeskAuth();
  const [email, setEmail] = useState("");
  const [pending, setPending] = useState(false);

  if (!configured) {
    return (
      <DeskBanner variant="warning" title="Cloud sync disabled">
        Sign in requires Supabase. Add keys per{" "}
        <a href="https://github.com/raghavar8088/antigravity/blob/main/client/docs/DEPLOY.md" style={{ textDecoration: "underline" }}>
          DEPLOY.md
        </a>{" "}
        (magic link + RLS), then reload.
      </DeskBanner>
    );
  }

  if (loading) {
    return <p className="desk-label-md">Checking sign-in…</p>;
  }

  if (user) {
    return (
      <div style={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 8, fontSize: "0.8125rem" }}>
        <span className="desk-body-md">
          Signed in as <span className="desk-mono">{user.email ?? user.id.slice(0, 8)}</span>
        </span>
        <DeskButton variant="outlined" onClick={() => void signOut()} style={{ minHeight: 36, padding: "0 12px" }}>
          Sign out
        </DeskButton>
        {!compact ? (
          <span className="desk-label-md" style={{ fontWeight: 400 }}>
            Trades sync to your account across devices.
          </span>
        ) : null}
      </div>
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      {!compact ? <span className="desk-label-md">Sign in to sync paper trades across devices.</span> : null}
      <form
        style={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 8 }}
        onSubmit={(e) => {
          e.preventDefault();
          setPending(true);
          void signInWithEmail(email).finally(() => setPending(false));
        }}
      >
        <input
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder="you@example.com"
          autoComplete="email"
          style={{
            flex: "1 1 180px",
            minHeight: 44,
            padding: "0 12px",
            borderRadius: "var(--desk-radius-input)",
            border: "1px solid var(--desk-outline)",
            background: "var(--desk-surface)",
          }}
        />
        <DeskButton type="submit" disabled={pending} style={{ minHeight: 44 }}>
          {pending ? "Sending…" : "Sign in"}
        </DeskButton>
        <a href="/sign-in" className="desk-label-md" style={{ color: "var(--desk-primary)" }}>
          Sign-in page
        </a>
      </form>
      {message ? <p className="desk-label-md">{message}</p> : null}
    </div>
  );
}
