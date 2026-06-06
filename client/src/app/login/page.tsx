"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useOwnerAuth } from "@/hooks/useOwnerAuth";

function LoginForm() {
  const { user, loading, error, signIn } = useOwnerAuth();
  const router = useRouter();
  const searchParams = useSearchParams();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [pending, setPending] = useState(false);
  const usernameRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!loading && user) {
      const next = searchParams.get("next") ?? "/";
      router.replace(next);
    }
  }, [loading, user, router, searchParams]);

  useEffect(() => {
    usernameRef.current?.focus();
  }, []);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setPending(true);
    const ok = await signIn(username, password);
    setPending(false);
    if (ok) {
      const next = searchParams.get("next") ?? "/";
      router.replace(next);
    }
  }

  if (loading) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-zinc-950">
        <p className="text-zinc-400 text-sm">Checking session…</p>
      </main>
    );
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-zinc-950 px-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="space-y-1 text-center">
          <h1 className="text-xl font-semibold text-zinc-100 tracking-tight">
            Trading Platform
          </h1>
          <p className="text-sm text-zinc-500">Owner access — all devices share one account</p>
        </div>

        <div
          style={{
            background: "rgba(255,255,255,0.04)",
            border: "1px solid rgba(255,255,255,0.08)",
            borderRadius: 12,
            padding: "28px 24px",
          }}
        >
          <form onSubmit={(e) => void handleSubmit(e)} className="space-y-4">
            <div className="space-y-1">
              <label
                htmlFor="username"
                className="block text-xs font-medium text-zinc-400 uppercase tracking-wider"
              >
                Username
              </label>
              <input
                ref={usernameRef}
                id="username"
                type="text"
                autoComplete="username"
                autoCorrect="off"
                autoCapitalize="none"
                spellCheck={false}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                disabled={pending}
                style={{
                  display: "block",
                  width: "100%",
                  padding: "10px 12px",
                  borderRadius: 8,
                  border: "1px solid rgba(255,255,255,0.12)",
                  background: "rgba(255,255,255,0.06)",
                  color: "#f4f4f5",
                  fontSize: 14,
                  outline: "none",
                  boxSizing: "border-box",
                }}
              />
            </div>

            <div className="space-y-1">
              <label
                htmlFor="password"
                className="block text-xs font-medium text-zinc-400 uppercase tracking-wider"
              >
                Password
              </label>
              <input
                id="password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={pending}
                style={{
                  display: "block",
                  width: "100%",
                  padding: "10px 12px",
                  borderRadius: 8,
                  border: "1px solid rgba(255,255,255,0.12)",
                  background: "rgba(255,255,255,0.06)",
                  color: "#f4f4f5",
                  fontSize: 14,
                  outline: "none",
                  boxSizing: "border-box",
                }}
              />
            </div>

            {error ? (
              <p
                style={{
                  fontSize: 13,
                  color: "#f87171",
                  background: "rgba(248,113,113,0.08)",
                  border: "1px solid rgba(248,113,113,0.2)",
                  borderRadius: 6,
                  padding: "8px 12px",
                  margin: 0,
                }}
              >
                {error}
              </p>
            ) : null}

            <button
              type="submit"
              disabled={pending || !username || !password}
              style={{
                display: "block",
                width: "100%",
                padding: "11px 0",
                borderRadius: 8,
                border: "none",
                background: pending ? "rgba(99,102,241,0.5)" : "#6366f1",
                color: "#fff",
                fontSize: 14,
                fontWeight: 600,
                cursor: pending ? "not-allowed" : "pointer",
                transition: "background 0.15s",
              }}
            >
              {pending ? "Signing in…" : "Sign in"}
            </button>
          </form>
        </div>

        <p className="text-center text-xs text-zinc-600">
          Session valid for 24 hours · All devices see identical data
        </p>
      </div>
    </main>
  );
}

export default function LoginPage() {
  return (
    <Suspense
      fallback={
        <main className="flex min-h-screen items-center justify-center bg-zinc-950">
          <p className="text-zinc-400 text-sm">Loading…</p>
        </main>
      }
    >
      <LoginForm />
    </Suspense>
  );
}
