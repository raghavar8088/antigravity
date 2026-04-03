"use client";

import React, { useEffect, useState } from "react";

interface PendingSignal {
  id: string;
  strategyName: string;
  signal: {
    action: string;
    symbol: string;
    targetSize: number;
  };
  createdAt: string;
}

interface BridgeStatus {
  online: boolean;
  lastHeartbeat: string;
  secondsSinceBeat: number;
  pendingSignals: number;
  processedSignalKeys: number;
  lastEvent: string;
  lastEventAt: string;
  lastError: string;
  lastErrorAt: string;
}

export default function CommandCenter() {
  const [pending, setPending] = useState<PendingSignal[]>([]);
  const [bridge, setBridge] = useState<BridgeStatus>({
    online: false,
    lastHeartbeat: "",
    secondsSinceBeat: 0,
    pendingSignals: 0,
    processedSignalKeys: 0,
    lastEvent: "",
    lastEventAt: "",
    lastError: "",
    lastErrorAt: "",
  });
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<{ type: "success" | "error"; msg: string } | null>(null);

  useEffect(() => {
    const fetchPending = async () => {
      try {
        const res = await fetch(`${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}/api/ai/pending`);
        const data = await res.json();
        setPending(data);
        if (data.length > 0 && !input && !loading) {
          const sig = data[0];
          setInput(`Review ${sig.strategyName} ${sig.signal.action} on ${sig.signal.symbol}.`);
        }
      } catch {}
    };
    const fetchStatus = async () => {
      try {
        const res = await fetch(`${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}/api/ai/bridge-status`);
        const data = await res.json();
        setBridge({
          online: Boolean(data.online),
          lastHeartbeat: data.lastHeartbeat ?? "",
          secondsSinceBeat: typeof data.secondsSinceBeat === "number" ? data.secondsSinceBeat : 0,
          pendingSignals: typeof data.pendingSignals === "number" ? data.pendingSignals : 0,
          processedSignalKeys: typeof data.processedSignalKeys === "number" ? data.processedSignalKeys : 0,
          lastEvent: data.lastEvent ?? "",
          lastEventAt: data.lastEventAt ?? "",
          lastError: data.lastError ?? "",
          lastErrorAt: data.lastErrorAt ?? "",
        });
      } catch {}
    };
    fetchPending();
    fetchStatus();
    const timer = setInterval(fetchPending, 3000);
    const statusTimer = setInterval(fetchStatus, 5000);
    return () => {
      clearInterval(timer);
      clearInterval(statusTimer);
    };
  }, [input, loading]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || loading || pending.length === 0) return;
    setLoading(true);
    setStatus(null);
    const signalId = pending[0].id;
    try {
      const res = await fetch(`${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}/api/ai/submit`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: signalId, prompt: input }),
      });
      if (res.ok) {
        setStatus({ type: "success", msg: "Sent for final audit." });
        setInput("");
        setPending((prev) => prev.filter((p) => p.id !== signalId));
      } else {
        setStatus({ type: "error", msg: "Submission failed." });
      }
    } catch {
      setStatus({ type: "error", msg: "Engine connection failed." });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="glass-panel p-5">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div>
          <div className="text-[11px] font-medium uppercase tracking-[0.12em]" style={{ color: "var(--text-secondary)" }}>AI command center</div>
          <div className="mt-1 text-sm font-medium" style={{ color: "var(--text-primary)" }}>Bridge queue and manual review</div>
        </div>
        <span className="rounded-full px-3 py-1 text-[11px] font-medium" style={{ background: bridge.online ? "var(--green-dim)" : "var(--red-dim)", color: bridge.online ? "var(--green)" : "var(--red)" }}>
          {bridge.online ? "Bridge online" : "Bridge offline"}
        </span>
      </div>

      <div className="mb-4 grid gap-3 md:grid-cols-4">
        {[
          { label: "Heartbeat", value: bridge.lastHeartbeat ? new Date(bridge.lastHeartbeat).toLocaleTimeString() : "never" },
          { label: "Beat age", value: `${bridge.secondsSinceBeat}s` },
          { label: "Queued", value: `${bridge.pendingSignals}` },
          { label: "Replay cache", value: `${bridge.processedSignalKeys}` },
        ].map((item) => (
          <div key={item.label} className="metric-card">
            <div className="metric-label">{item.label}</div>
            <div className="metric-value">{item.value}</div>
          </div>
        ))}
      </div>

      <form onSubmit={handleSubmit} className="space-y-3">
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder={pending.length > 0 ? "Add AI review instructions..." : "Waiting for pending signals..."}
          disabled={loading}
          className="raig-input"
        />
        <div className="flex items-center justify-between gap-3">
          <div className="text-sm" style={{ color: "var(--text-secondary)" }}>
            {pending.length > 0 ? `${pending.length} signal${pending.length > 1 ? "s" : ""} ready for review.` : "No pending signals."}
          </div>
          <button type="submit" className="btn-primary" disabled={loading || pending.length === 0}>
            {loading ? "Sending..." : "Submit"}
          </button>
        </div>
      </form>

      {status && <div className="mt-3 text-sm" style={{ color: status.type === "success" ? "var(--green)" : "var(--red)" }}>{status.msg}</div>}

      {pending.length > 0 && (
        <div className="mt-4 flex gap-3 overflow-x-auto pb-1">
          {pending.map((p) => (
            <button
              key={p.id}
              type="button"
              onClick={() => setInput(`Review ${p.strategyName} ${p.signal.action} on ${p.signal.symbol}.`)}
              className="min-w-[220px] rounded-[18px] border p-3 text-left"
              style={{ borderColor: "var(--border)", background: "var(--surface-2)" }}
            >
              <div className="text-[11px] font-medium" style={{ color: "var(--text-secondary)" }}>{p.id}</div>
              <div className="mt-1 text-sm font-medium" style={{ color: "var(--text-primary)" }}>{p.signal.action} {p.signal.symbol}</div>
              <div className="mt-1 text-xs" style={{ color: "var(--text-secondary)" }}>{p.strategyName}</div>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
