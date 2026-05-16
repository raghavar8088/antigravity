"use client";

import { useCallback, useEffect, useState } from "react";
import { createBrowserSupabase, isSupabaseAuthConfigured } from "@/lib/supabase/client";

export type PaperDeskAuthUser = {
  id: string;
  email: string | null;
};

export function usePaperDeskAuth() {
  const [user, setUser] = useState<PaperDeskAuthUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState<string | null>(null);
  const configured = isSupabaseAuthConfigured();

  const refresh = useCallback(async () => {
    if (!configured) {
      setUser(null);
      setLoading(false);
      return;
    }
    try {
      const res = await fetch("/api/auth/session", { cache: "no-store", credentials: "include" });
      const body = (await res.json()) as {
        ok?: boolean;
        user?: { id: string; email: string | null } | null;
      };
      if (body.ok && body.user?.id) {
        setUser({ id: body.user.id, email: body.user.email });
      } else {
        setUser(null);
      }
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, [configured]);

  useEffect(() => {
    void refresh();
    if (!configured) return;
    const supabase = createBrowserSupabase();
    const {
      data: { subscription },
    } = supabase.auth.onAuthStateChange(() => {
      void refresh();
    });
    return () => subscription.unsubscribe();
  }, [configured, refresh]);

  const signInWithEmail = useCallback(
    async (email: string) => {
      setMessage(null);
      if (!configured) {
        setMessage("Supabase auth not configured");
        return false;
      }
      const trimmed = email.trim();
      if (!trimmed) {
        setMessage("Enter your email");
        return false;
      }
      const supabase = createBrowserSupabase();
      const redirectTo = `${window.location.origin}/auth/callback?next=${encodeURIComponent(window.location.pathname)}`;
      const { error } = await supabase.auth.signInWithOtp({
        email: trimmed,
        options: { emailRedirectTo: redirectTo },
      });
      if (error) {
        setMessage(error.message);
        return false;
      }
      setMessage("Check your email for the sign-in link.");
      return true;
    },
    [configured],
  );

  const signOut = useCallback(async () => {
    if (!configured) return;
    const supabase = createBrowserSupabase();
    await supabase.auth.signOut();
    setUser(null);
    setMessage(null);
    await refresh();
  }, [configured, refresh]);

  return {
    configured,
    user,
    loading,
    message,
    signInWithEmail,
    signOut,
    refresh,
  };
}
