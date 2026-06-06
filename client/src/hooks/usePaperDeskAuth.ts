"use client";

/**
 * Thin wrapper around useOwnerAuth that preserves the original hook API so
 * existing call sites (BTCFutureTradingScalper, PaperDeskAuthBar, etc.) don't
 * need to be updated. Internally delegates to the new username+password auth.
 */

import { useOwnerAuth } from "@/hooks/useOwnerAuth";

export type PaperDeskAuthUser = {
  id: string;
  email: string | null;
};

export function usePaperDeskAuth() {
  const { user, loading, error, signIn, signOut, refresh } = useOwnerAuth();

  return {
    configured: true,
    user: user ? ({ id: user.id, email: user.username } as PaperDeskAuthUser) : null,
    loading,
    message: error,
    /** @deprecated — use signIn(username, password) via useOwnerAuth directly */
    signInWithEmail: async (_email: string) => false,
    signIn,
    signOut,
    refresh,
  };
}
