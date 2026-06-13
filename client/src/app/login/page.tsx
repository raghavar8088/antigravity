"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useOwnerAuth } from "@/hooks/useOwnerAuth";

// ── Animated background dots ──────────────────────────────────────────────────
const DOT_STYLE: React.CSSProperties = {
  position: "absolute",
  borderRadius: "50%",
  pointerEvents: "none",
  opacity: 0.18,
  animation: "floatDot 12s ease-in-out infinite",
};

function AnimatedBg() {
  const dots = [
    { w: 320, h: 320, top: "-80px", left: "-80px", bg: "#3b82f6", delay: "0s" },
    { w: 200, h: 200, top: "60%", right: "-60px", bg: "#8b5cf6", delay: "3s" },
    { w: 140, h: 140, bottom: "-40px", left: "40%", bg: "#06b6d4", delay: "6s" },
  ];
  return (
    <div style={{ position: "fixed", inset: 0, overflow: "hidden", zIndex: 0, pointerEvents: "none" }}>
      {dots.map((d, i) => (
        <div key={i} style={{ ...DOT_STYLE, width: d.w, height: d.h, top: d.top, left: (d as { left?: string }).left, right: (d as { right?: string }).right, bottom: (d as { bottom?: string }).bottom, background: d.bg, animationDelay: d.delay }} />
      ))}
      <style>{`
        @keyframes floatDot {
          0%, 100% { transform: translateY(0px) scale(1); }
          50% { transform: translateY(-30px) scale(1.08); }
        }
      `}</style>
    </div>
  );
}

// ── Status indicator ──────────────────────────────────────────────────────────
function SystemStatus() {
  const [ok, setOk] = useState<boolean | null>(null);
  useEffect(() => {
    fetch("/api/auth/session", { cache: "no-store" })
      .then((r) => setOk(r.status !== 500))
      .catch(() => setOk(false));
  }, []);

  return (
    <div style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 11, color: "#6b7280" }}>
      <span style={{
        width: 6, height: 6, borderRadius: "50%", display: "inline-block",
        background: ok === null ? "#6b7280" : ok ? "#22c55e" : "#ef4444",
      }} />
      {ok === null ? "Checking…" : ok ? "All systems operational" : "System degraded"}
    </div>
  );
}

// ── Login form ────────────────────────────────────────────────────────────────
function LoginForm() {
  const { user, loading, error, signIn } = useOwnerAuth();
  const router = useRouter();
  const searchParams = useSearchParams();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [pending, setPending] = useState(false);
  const [shake, setShake] = useState(false);
  const usernameRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!loading && user) {
      router.replace(searchParams.get("next") ?? "/terminal");
    }
  }, [loading, user, router, searchParams]);

  useEffect(() => { usernameRef.current?.focus(); }, []);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setPending(true);
    const ok = await signIn(username, password);
    setPending(false);
    if (ok) {
      router.replace(searchParams.get("next") ?? "/terminal");
    } else {
      setShake(true);
      setTimeout(() => setShake(false), 600);
    }
  }

  const inputStyle: React.CSSProperties = {
    width: "100%", boxSizing: "border-box",
    padding: "10px 14px", borderRadius: 8,
    border: "1px solid rgba(255,255,255,0.12)",
    background: "rgba(255,255,255,0.05)",
    color: "#f9fafb", fontSize: 14,
    outline: "none", transition: "border-color 0.2s",
  };

  const labelStyle: React.CSSProperties = {
    fontSize: 12, fontWeight: 600, color: "#9ca3af",
    display: "block", marginBottom: 6, textTransform: "uppercase", letterSpacing: "0.07em",
  };

  return (
    <>
      <AnimatedBg />
      <main style={{
        position: "relative", zIndex: 1,
        minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center",
        padding: 24, background: "transparent",
      }}>
        <div style={{ width: "100%", maxWidth: 400 }}>
          {/* Brand */}
          <div style={{ textAlign: "center", marginBottom: 32 }}>
            <div style={{
              width: 56, height: 56, borderRadius: 16, margin: "0 auto 16px",
              background: "linear-gradient(135deg, #3b82f6, #8b5cf6)",
              display: "flex", alignItems: "center", justifyContent: "center",
              fontSize: 22, fontWeight: 800, color: "#fff",
              boxShadow: "0 8px 32px rgba(59,130,246,0.35)",
            }}>ICC</div>
            <h1 style={{ margin: "0 0 6px", fontSize: 22, fontWeight: 800, color: "#f9fafb" }}>
              Institutional Command Center
            </h1>
            <p style={{ margin: 0, fontSize: 13, color: "#6b7280" }}>
              BTC Futures · Paper Trading Engine
            </p>
          </div>

          {/* Card */}
          <div
            style={{
              background: "rgba(17,24,39,0.85)",
              backdropFilter: "blur(20px)",
              border: "1px solid rgba(255,255,255,0.1)",
              borderRadius: 16, padding: 32,
              boxShadow: "0 24px 64px rgba(0,0,0,0.4)",
              animation: shake ? "shake 0.5s ease" : "none",
            }}
          >
            <h2 style={{ margin: "0 0 4px", fontSize: 16, fontWeight: 700, color: "#f9fafb" }}>Sign in</h2>
            <p style={{ margin: "0 0 24px", fontSize: 12, color: "#6b7280" }}>
              Owner access · Session valid 24 hours
            </p>

            {loading ? (
              <div style={{ textAlign: "center", padding: "32px 0", color: "#6b7280", fontSize: 13 }}>
                Checking session…
              </div>
            ) : (
              <form onSubmit={(e) => void handleSubmit(e)} style={{ display: "flex", flexDirection: "column", gap: 18 }}>
                <div>
                  <label htmlFor="username" style={labelStyle}>Username</label>
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
                    style={inputStyle}
                    onFocus={(e) => { e.target.style.borderColor = "#3b82f6"; }}
                    onBlur={(e) => { e.target.style.borderColor = "rgba(255,255,255,0.12)"; }}
                  />
                </div>
                <div>
                  <label htmlFor="password" style={labelStyle}>Password</label>
                  <input
                    id="password"
                    type="password"
                    autoComplete="current-password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    disabled={pending}
                    style={inputStyle}
                    onFocus={(e) => { e.target.style.borderColor = "#3b82f6"; }}
                    onBlur={(e) => { e.target.style.borderColor = "rgba(255,255,255,0.12)"; }}
                  />
                </div>

                {error && (
                  <div style={{
                    padding: "10px 14px", borderRadius: 8,
                    background: "rgba(239,68,68,0.1)", border: "1px solid rgba(239,68,68,0.3)",
                    color: "#fca5a5", fontSize: 13,
                  }} role="alert">{error}</div>
                )}

                <button
                  type="submit"
                  disabled={pending || !username || !password}
                  style={{
                    padding: "12px 24px", borderRadius: 10, fontSize: 14, fontWeight: 700,
                    border: "none", cursor: pending || !username || !password ? "not-allowed" : "pointer",
                    background: pending || !username || !password
                      ? "rgba(59,130,246,0.3)"
                      : "linear-gradient(135deg, #3b82f6, #6366f1)",
                    color: "#fff",
                    transition: "all 0.2s",
                    boxShadow: pending || !username || !password ? "none" : "0 4px 16px rgba(59,130,246,0.4)",
                  }}
                >
                  {pending ? "Signing in…" : "Sign in →"}
                </button>
              </form>
            )}
          </div>

          {/* Status footer */}
          <div style={{ textAlign: "center", marginTop: 20 }}>
            <SystemStatus />
          </div>
        </div>
      </main>

      <style>{`
        @keyframes shake {
          0%, 100% { transform: translateX(0); }
          20% { transform: translateX(-8px); }
          40% { transform: translateX(8px); }
          60% { transform: translateX(-6px); }
          80% { transform: translateX(6px); }
        }
      `}</style>
    </>
  );
}

export default function LoginPage() {
  return (
    <Suspense fallback={
      <main style={{
        minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center",
        background: "#030712",
      }}>
        <div style={{ color: "#6b7280", fontSize: 14 }}>Loading…</div>
      </main>
    }>
      <LoginForm />
    </Suspense>
  );
}
