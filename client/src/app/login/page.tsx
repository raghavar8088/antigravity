"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useOwnerAuth } from "@/hooks/useOwnerAuth";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Skeleton } from "@/components/ui/Skeleton";
import { TextField } from "@/components/ui/FormControls";

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
      router.replace(searchParams.get("next") ?? "/terminal");
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
    if (ok) router.replace(searchParams.get("next") ?? "/terminal");
  }

  if (loading) {
    return (
      <main className="m3-auth-page">
        <div className="m3-auth-card">
          <Skeleton width="60%" height={24} />
          <Skeleton width="100%" height={40} className="m3-auth-skeleton-gap" />
          <Skeleton width="100%" height={40} />
        </div>
      </main>
    );
  }

  return (
    <main className="m3-auth-page">
      <div className="m3-auth-brand">
        <span className="m3-nav-rail__logo">ICC</span>
        <h1 className="m3-auth-brand__title">Institutional Command Center</h1>
        <p className="m3-auth-brand__subtitle">Owner access — all devices share one account</p>
      </div>

      <Card title="Sign in" subtitle="Session valid for 24 hours">
        <form onSubmit={(e) => void handleSubmit(e)} className="m3-auth-form">
          <TextField
            ref={usernameRef}
            label="Username"
            id="username"
            type="text"
            autoComplete="username"
            autoCorrect="off"
            autoCapitalize="none"
            spellCheck={false}
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            disabled={pending}
          />
          <TextField
            label="Password"
            id="password"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            disabled={pending}
          />
          {error ? <p className="m3-field__error" role="alert">{error}</p> : null}
          <Button variant="filled" type="submit" disabled={pending || !username || !password} className="m3-auth-submit">
            {pending ? "Signing in…" : "Sign in"}
          </Button>
        </form>
      </Card>
    </main>
  );
}

export default function LoginPage() {
  return (
    <Suspense
      fallback={
        <main className="m3-auth-page">
          <Skeleton width={320} height={200} />
        </main>
      }
    >
      <LoginForm />
    </Suspense>
  );
}
