"use client";

import React, { useState, useEffect, useRef } from "react";

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
  const scrollRef = useRef<HTMLDivElement>(null);

  // Poll for pending signals
  useEffect(() => {
    const fetchPending = async () => {
      try {
        const res = await fetch(`${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}/api/ai/pending`);
        const data = await res.json();
        setPending(data);
        
        // Auto-fill if there is a new signal and input is empty
        if (data.length > 0 && !input && !loading) {
          const sig = data[0];
          setInput(`Proposed ${sig.signal.action} for ${sig.signal.symbol} via ${sig.strategyName}. Audit and execute?`);
        }
      } catch (e) {
        console.error("Failed to fetch pending signals", e);
      }
    };

    const timer = setInterval(fetchPending, 3000);

    // Bridge Status Polling
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
      } catch (e) {}
    };
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
        setStatus({ type: "success", msg: "Submitting to ChatGPT for final audit..." });
        setInput("");
        // Optimistic clear
        setPending(prev => prev.filter(p => p.id !== signalId));
      } else {
        setStatus({ type: "error", msg: "Submission failed. Please try again." });
      }
    } catch (err) {
      setStatus({ type: "error", msg: "Network error. Is the engine running?" });
    } finally {
      setLoading(false);
      setTimeout(() => setStatus(null), 5000);
    }
  };

  const heartbeatLabel = bridge.lastHeartbeat
    ? new Date(bridge.lastHeartbeat).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })
    : "never";
  const eventLabel = bridge.lastEventAt
    ? `${bridge.lastEvent || "none"} (${new Date(bridge.lastEventAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })})`
    : (bridge.lastEvent || "none");
  const errorLabel = bridge.lastErrorAt
    ? `${bridge.lastError || "none"} (${new Date(bridge.lastErrorAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })})`
    : (bridge.lastError || "none");

  return (
    <div className="glass-panel p-4 mb-4" style={{ border: "2px solid var(--accent-dim)" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 15 }}>
        <div style={{ 
          width: 8, height: 8, borderRadius: "50%", 
          background: pending.length > 0 ? "var(--green)" : "var(--text-muted)",
          boxShadow: pending.length > 0 ? "0 0 10px var(--green)" : "none"
        }} />
        <h2 style={{ fontSize: 13, fontWeight: 800, letterSpacing: "0.05em", color: "white" }}>
          AI COMMAND CENTER
        </h2>
        {pending.length > 0 && (
          <span style={{ fontSize: 10, background: "var(--green-dim)", color: "var(--green)", padding: "2px 8px", borderRadius: 4, fontWeight: 700 }}>
            {pending.length} SIGNAL{pending.length > 1 ? "S" : ""} PENDING
          </span>
        )}
        <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 6 }}>
          <div style={{ 
            width: 6, height: 6, borderRadius: "50%", 
            background: bridge.online ? "var(--green)" : "var(--red)",
            boxShadow: bridge.online ? "0 0 8px var(--green)" : "none"
          }} />
          <span style={{ fontSize: 9, fontWeight: 700, color: "var(--text-muted)", textTransform: "uppercase", letterSpacing: "0.05em" }}>
            BRIDGE: {bridge.online ? "ONLINE" : "OFFLINE"}
          </span>
        </div>
      </div>

      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))",
          gap: 10,
          marginBottom: 14,
        }}
      >
        {[
          { label: "Heartbeat", value: heartbeatLabel, tone: "var(--text-secondary)" },
          { label: "Beat Age", value: `${bridge.secondsSinceBeat}s`, tone: bridge.online ? "var(--green)" : "var(--red)" },
          { label: "Queued", value: `${bridge.pendingSignals}`, tone: bridge.pendingSignals > 0 ? "var(--green)" : "var(--text-secondary)" },
          { label: "Replay Cache", value: `${bridge.processedSignalKeys}`, tone: "var(--text-secondary)" },
        ].map((item) => (
          <div
            key={item.label}
            style={{
              background: "var(--surface-2)",
              border: "1px solid var(--border)",
              borderRadius: 10,
              padding: "9px 12px",
            }}
          >
            <div style={{ fontSize: 9, color: "var(--text-muted)", textTransform: "uppercase", letterSpacing: "0.08em" }}>
              {item.label}
            </div>
            <div style={{ fontSize: 12, fontWeight: 700, color: item.tone, marginTop: 4 }}>
              {item.value}
            </div>
          </div>
        ))}
      </div>

      <div
        style={{
          display: "grid",
          gridTemplateColumns: "1fr 1fr",
          gap: 10,
          marginBottom: 14,
        }}
      >
        <div style={{ background: "var(--surface-2)", border: "1px solid var(--border)", borderRadius: 10, padding: "9px 12px" }}>
          <div style={{ fontSize: 9, color: "var(--text-muted)", textTransform: "uppercase", letterSpacing: "0.08em" }}>
            Last Bridge Event
          </div>
          <div style={{ fontSize: 11, fontWeight: 600, color: "var(--text-secondary)", marginTop: 4, lineHeight: 1.4 }}>
            {eventLabel}
          </div>
        </div>
        <div style={{ background: "var(--surface-2)", border: "1px solid var(--border)", borderRadius: 10, padding: "9px 12px" }}>
          <div style={{ fontSize: 9, color: "var(--text-muted)", textTransform: "uppercase", letterSpacing: "0.08em" }}>
            Last Bridge Error
          </div>
          <div style={{ fontSize: 11, fontWeight: 600, color: bridge.lastError ? "var(--red)" : "var(--text-secondary)", marginTop: 4, lineHeight: 1.4 }}>
            {errorLabel}
          </div>
        </div>
      </div>

      <form onSubmit={handleSubmit} style={{ position: "relative" }}>
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder={pending.length > 0 ? "Type instructions or click Submit..." : "Waiting for strategy signals..."}
          disabled={loading}
          style={{
            width: "100%",
            background: "var(--surface-3)",
            border: "1px solid var(--border)",
            borderRadius: 12,
            padding: "14px 100px 14px 20px",
            fontSize: 14,
            color: "white",
            outline: "none",
            transition: "border-color 0.2s",
            boxShadow: "inset 0 2px 4px rgba(0,0,0,0.2)"
          }}
          onFocus={(e) => e.target.style.borderColor = "var(--accent)"}
          onBlur={(e) => e.target.style.borderColor = "var(--border)"}
        />
        
        <button
          type="submit"
          disabled={loading || pending.length === 0}
          style={{
            position: "absolute",
            right: 8,
            top: 8,
            bottom: 8,
            padding: "0 20px",
            background: pending.length > 0 ? "var(--accent)" : "var(--surface-4)",
            color: "white",
            border: "none",
            borderRadius: 8,
            fontSize: 12,
            fontWeight: 700,
            cursor: pending.length > 0 ? "pointer" : "not-allowed",
            transition: "all 0.2s",
            opacity: loading ? 0.7 : 1
          }}
        >
          {loading ? "SENDING..." : "SUBMIT"}
        </button>
      </form>

      {status && (
        <div style={{ 
          marginTop: 10, fontSize: 11, fontWeight: 600,
          color: status.type === "success" ? "var(--green)" : "var(--red)",
          display: "flex", alignItems: "center", gap: 6
        }}>
          <span>{status.type === "success" ? "✓" : "⚠"}</span>
          {status.msg}
        </div>
      )}

      {pending.length > 0 && (
        <div style={{ marginTop: 15, display: "flex", gap: 10, overflowX: "auto", paddingBottom: 5 }}>
          {pending.map(p => (
            <div key={p.id} style={{ 
              minWidth: 180, background: "var(--surface-2)", border: "1px solid var(--border)", 
              borderRadius: 8, padding: "8px 12px", cursor: "pointer"
            }} onClick={() => setInput(`Analyze ${p.strategyName} ${p.signal.action} for ${p.signal.symbol}. Should I take it?`)}>
              <div style={{ fontSize: 9, color: "var(--text-muted)", marginBottom: 4 }}>{p.id}</div>
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                <span style={{ fontSize: 12, fontWeight: 800, color: p.signal.action === "BUY" ? "var(--green)" : "var(--red)" }}>
                  {p.signal.action} {p.signal.symbol}
                </span>
                <span style={{ fontSize: 10, color: "var(--text-secondary)" }}>{p.strategyName}</span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
