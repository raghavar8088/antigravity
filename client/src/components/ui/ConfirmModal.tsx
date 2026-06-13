"use client";

import { useRef, useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { cn } from "./cn";

export interface ConfirmModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string;
  /** Red warning block — shown in a danger box below description */
  consequence?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  /** If set, user must type this string exactly before confirming */
  typeToConfirm?: string;
  /** Marks the confirm action as destructive (red button) */
  destructive?: boolean;
  onConfirm: () => void;
  onCancel?: () => void;
  loading?: boolean;
}

export function ConfirmModal({
  open,
  onOpenChange,
  title,
  description,
  consequence,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  typeToConfirm,
  destructive = true,
  onConfirm,
  onCancel,
  loading = false,
}: ConfirmModalProps) {
  const [typed, setTyped] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  const canConfirm = typeToConfirm ? typed === typeToConfirm : true;

  function handleCancel() {
    setTyped("");
    onOpenChange(false);
    onCancel?.();
  }

  function handleConfirm() {
    if (!canConfirm || loading) return;
    setTyped("");
    onConfirm();
  }

  return (
    <Dialog.Root open={open} onOpenChange={(v) => { if (!v) handleCancel(); }}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
        <Dialog.Content
          className={cn(
            "fixed left-1/2 top-1/2 z-50 w-full max-w-[440px] -translate-x-1/2 -translate-y-1/2",
            "rounded-[var(--radius-lg)] border border-[var(--color-border)]",
            "bg-[var(--color-bg-surface)] shadow-[var(--shadow-elevated)]",
            "p-6 focus:outline-none",
            "data-[state=open]:animate-in data-[state=closed]:animate-out",
            "data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0",
            "data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95",
          )}
          onOpenAutoFocus={(e) => { e.preventDefault(); inputRef.current?.focus(); }}
        >
          <Dialog.Title className="text-[var(--color-text-primary)] text-[16px] font-semibold mb-2">
            {title}
          </Dialog.Title>

          {description ? (
            <Dialog.Description className="text-[var(--color-text-secondary)] text-[13px] mb-4 leading-relaxed">
              {description}
            </Dialog.Description>
          ) : null}

          {consequence ? (
            <div
              className="rounded-[var(--radius-md)] border border-[rgba(239,68,68,0.3)] bg-[rgba(239,68,68,0.08)] p-3 mb-4"
              role="alert"
            >
              <p className="text-[var(--color-loss)] text-[12px] leading-relaxed font-medium">
                ⚠ {consequence}
              </p>
            </div>
          ) : null}

          {typeToConfirm ? (
            <div className="mb-4">
              <label className="block text-[var(--color-text-secondary)] text-[12px] mb-1.5">
                Type <code className="text-[var(--color-loss)] font-mono bg-[rgba(239,68,68,0.08)] px-1 rounded">{typeToConfirm}</code> to confirm
              </label>
              <input
                ref={inputRef}
                type="text"
                value={typed}
                onChange={(e) => setTyped(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") handleConfirm(); }}
                className={cn(
                  "w-full rounded-[var(--radius-md)] border bg-[var(--color-bg-elevated)]",
                  "px-3 py-2 font-mono text-[13px] text-[var(--color-text-primary)]",
                  "outline-none transition-[border-color]",
                  typed === typeToConfirm
                    ? "border-[var(--color-profit)]"
                    : "border-[var(--color-border)] focus:border-[var(--color-border-focus)]",
                )}
                placeholder={typeToConfirm}
                aria-label={`Type ${typeToConfirm} to confirm`}
                autoComplete="off"
              />
            </div>
          ) : null}

          <div className="flex items-center justify-end gap-2 mt-2">
            <button
              type="button"
              onClick={handleCancel}
              className={cn(
                "px-4 py-2 rounded-[var(--radius-md)] text-[13px] font-medium transition-colors",
                "text-[var(--color-text-secondary)] border border-[var(--color-border)]",
                "hover:bg-[var(--color-bg-elevated)] hover:text-[var(--color-text-primary)]",
              )}
            >
              {cancelLabel}
            </button>
            <button
              type="button"
              onClick={handleConfirm}
              disabled={!canConfirm || loading}
              className={cn(
                "px-4 py-2 rounded-[var(--radius-md)] text-[13px] font-semibold transition-colors",
                destructive
                  ? "bg-[var(--color-loss)] text-white hover:bg-[rgba(239,68,68,0.85)] disabled:bg-[var(--color-bg-elevated)] disabled:text-[var(--color-text-muted)]"
                  : "bg-[var(--color-info)] text-white hover:bg-[rgba(59,130,246,0.85)] disabled:opacity-50",
                "disabled:cursor-not-allowed",
              )}
            >
              {loading ? "Processing…" : confirmLabel}
            </button>
          </div>

          <Dialog.Close
            className="absolute right-4 top-4 text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] transition-colors"
            aria-label="Close"
          >
            ✕
          </Dialog.Close>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
