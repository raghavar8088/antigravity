"use client";

import { useState } from "react";
import { usePaperDeskAuth } from "@/hooks/usePaperDeskAuth";
import { useOwnerAuth } from "@/hooks/useOwnerAuth";
import { DeskBanner } from "@/components/desk/ui/DeskBanner";
import { DeskButton } from "@/components/desk/ui/DeskButton";

export function PaperDeskAuthBar({ compact = false }: { compact?: boolean }) {
  const { user, loading, message } = usePaperDeskAuth();
  const { signIn, signOut } = useOwnerAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [pending, setPending] = useState(false);

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
            All devices share the same account.
          </span>
        ) : null}
      </div>
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      {!compact ? <span className="desk-label-md">Sign in to sync paper trades across all devices.</span> : null}
      <form
        style={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 8 }}
        onSubmit={(e) => {
          e.preventDefault();
          setPending(true);
          void signIn(username, password).finally(() => setPending(false));
        }}
      >
        <input
          type="text"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          placeholder="Username"
          autoComplete="username"
          autoCorrect="off"
          autoCapitalize="none"
          style={{
            flex: "1 1 140px",
            minHeight: 44,
            padding: "0 12px",
            borderRadius: "var(--desk-radius-input)",
            border: "1px solid var(--desk-outline)",
            background: "var(--desk-surface)",
          }}
        />
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="Password"
          autoComplete="current-password"
          style={{
            flex: "1 1 140px",
            minHeight: 44,
            padding: "0 12px",
            borderRadius: "var(--desk-radius-input)",
            border: "1px solid var(--desk-outline)",
            background: "var(--desk-surface)",
          }}
        />
        <DeskButton type="submit" disabled={pending || !username || !password} style={{ minHeight: 44 }}>
          {pending ? "Signing in…" : "Sign in"}
        </DeskButton>
        <a href="/login" className="desk-label-md" style={{ color: "var(--desk-primary)" }}>
          Full login page
        </a>
      </form>
      {message ? <p className="desk-label-md">{message}</p> : null}
    </div>
  );
}
