"use client";

import { useState } from "react";
import { usePaperDeskAuth } from "@/hooks/usePaperDeskAuth";

export function PaperDeskAuthBar() {
  const { configured, user, loading, message, signInWithEmail, signOut } = usePaperDeskAuth();
  const [email, setEmail] = useState("");
  const [pending, setPending] = useState(false);

  if (!configured) {
    return (
      <p className="text-[10px] text-amber-700">
        Cloud sync disabled — set <span className="font-mono">NEXT_PUBLIC_SUPABASE_ANON_KEY</span> in{" "}
        <span className="font-mono">.env.local</span>.
      </p>
    );
  }

  if (loading) {
    return <p className="text-[10px] text-zinc-500">Checking sign-in…</p>;
  }

  if (user) {
    return (
      <div className="flex flex-wrap items-center gap-2 text-[11px] text-zinc-600">
        <span>
          Signed in as <span className="font-mono text-zinc-900">{user.email ?? user.id.slice(0, 8)}</span>
        </span>
        <button
          type="button"
          onClick={() => void signOut()}
          className="rounded border border-zinc-200 bg-white px-2 py-0.5 text-[10px] font-medium text-zinc-700 hover:bg-zinc-50"
        >
          Sign out
        </button>
        <span className="text-zinc-400">Trades sync to your account across devices.</span>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">
      <span className="text-[11px] text-zinc-600">Sign in to sync paper trades across devices.</span>
      <form
        className="flex flex-wrap items-center gap-2"
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
          className="rounded border border-zinc-200 px-2 py-1 text-[11px] text-zinc-800"
          autoComplete="email"
        />
        <button
          type="submit"
          disabled={pending}
          className="rounded border border-sky-200 bg-sky-50 px-2 py-1 text-[10px] font-medium text-sky-900 hover:bg-sky-100 disabled:opacity-50"
        >
          {pending ? "Sending…" : "Sign in"}
        </button>
        <a href="/sign-in" className="text-[10px] text-sky-700 underline">
          Sign-in page
        </a>
      </form>
      {message ? <p className="w-full text-[10px] text-zinc-600">{message}</p> : null}
    </div>
  );
}
