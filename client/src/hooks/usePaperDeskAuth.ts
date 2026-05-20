"use client";

import { useCallback, useEffect, useState } from "react";

export type PaperDeskAuthUser = {
  id: string;
  email: string | null;
};

export function usePaperDeskAuth() {
  const [user, setUser] = useState<PaperDeskAuthUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const res = await fetch("/api/auth/session", { cache: "no-store", credentials: "include" });
      const body = (await res.json()) as {
        ok?: boolean;
        user?: { id: string; email: string | null } | null;
      };
      if (body.ok && body.user?.id) {
        setUser({ id: body.user.id, email: body.user.email ?? null });
      } else {
        setUser(null);
      }
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const signInWithEmail = useCallback(
    async (email: string) => {
      setMessage(null);
      const trimmed = email.trim();
      if (!trimmed) {
        setMessage("Enter your email");
        return false;
      }
      try {
        const res = await fetch("/api/auth/signin", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ email: trimmed }),
          credentials: "include",
        });
        const body = (await res.json()) as { ok?: boolean; error?: string; user?: { id: string; email: string } };
        if (!res.ok || !body.ok) {
          setMessage(body.error ?? "Sign-in failed");
          return false;
        }
        if (body.user) {
          setUser({ id: body.user.id, email: body.user.email });
        }
        await refresh();
        return true;
      } catch {
        setMessage("Sign-in failed — please try again");
        return false;
      }
    },
    [refresh],
  );

  const signOut = useCallback(async () => {
    try {
      await fetch("/api/auth/signout", { method: "POST", credentials: "include" });
    } catch {
      // ignore
    }
    setUser(null);
    setMessage(null);
  }, []);

  return {
    configured: true,
    user,
    loading,
    message,
    signInWithEmail,
    signOut,
    refresh,
  };
}
