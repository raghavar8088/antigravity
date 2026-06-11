"use client";

import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";

type SnackbarMessage = {
  id: string;
  text: string;
  action?: { label: string; onClick: () => void };
  tone?: "default" | "success" | "error";
};

type SnackbarContextValue = {
  show: (text: string, opts?: { action?: SnackbarMessage["action"]; tone?: SnackbarMessage["tone"]; duration?: number }) => void;
};

const SnackbarContext = createContext<SnackbarContextValue | null>(null);

export function SnackbarProvider({ children }: { children: ReactNode }) {
  const [messages, setMessages] = useState<SnackbarMessage[]>([]);

  const show = useCallback(
    (text: string, opts?: { action?: SnackbarMessage["action"]; tone?: SnackbarMessage["tone"]; duration?: number }) => {
      const id = crypto.randomUUID();
      setMessages((prev) => [...prev, { id, text, action: opts?.action, tone: opts?.tone ?? "default" }]);
      window.setTimeout(() => setMessages((prev) => prev.filter((m) => m.id !== id)), opts?.duration ?? 4000);
    },
    [],
  );

  const value = useMemo(() => ({ show }), [show]);

  return (
    <SnackbarContext.Provider value={value}>
      {children}
      <div className="m3-snackbar-region" aria-live="polite" aria-relevant="additions">
        {messages.map((m) => (
          <div key={m.id} className={`m3-snackbar m3-snackbar--${m.tone ?? "default"}`} role="status">
            <span>{m.text}</span>
            {m.action ? (
              <button type="button" className="m3-snackbar__action" onClick={m.action.onClick}>
                {m.action.label}
              </button>
            ) : null}
          </div>
        ))}
      </div>
    </SnackbarContext.Provider>
  );
}

export function useSnackbar() {
  const ctx = useContext(SnackbarContext);
  if (!ctx) throw new Error("useSnackbar must be used within SnackbarProvider");
  return ctx;
}
