"use client";

import { useCallback, useEffect, useState } from "react";

export type OwnerAuthUser = {
  id: string;      // always "owner_admin"
  username: string;
};

export function useOwnerAuth() {
  const [user, setUser] = useState<OwnerAuthUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch("/api/auth/session", {
        cache: "no-store",
        credentials: "include",
      });
      const body = (await res.json()) as {
        ok?: boolean;
        user?: { id: string; email: string | null } | null;
      };
      if (body.ok && body.user?.id) {
        setUser({ id: body.user.id, username: body.user.email ?? body.user.id });
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

  const signIn = useCallback(
    async (username: string, password: string): Promise<boolean> => {
      setError(null);
      try {
        const res = await fetch("/api/auth/login", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ username: username.trim(), password }),
          credentials: "include",
        });
        const body = (await res.json()) as {
          ok?: boolean;
          error?: string;
          user?: { id: string; username: string };
        };
        if (!res.ok || !body.ok) {
          setError(body.error ?? "Login failed");
          return false;
        }
        if (body.user) {
          setUser({ id: body.user.id, username: body.user.username });
        }
        await refresh();
        return true;
      } catch {
        setError("Login failed — please try again");
        return false;
      }
    },
    [refresh],
  );

  const signOut = useCallback(async () => {
    try {
      await fetch("/api/auth/signout", { method: "POST", credentials: "include" });
    } catch {
      // ignore network errors
    }
    setUser(null);
    setError(null);
  }, []);

  return { user, loading, error, signIn, signOut, refresh };
}
