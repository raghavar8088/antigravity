"use client";

import { forwardRef, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes } from "react";
import { cn } from "./cn";

type TextFieldProps = InputHTMLAttributes<HTMLInputElement> & {
  label?: string;
  hint?: string;
  error?: string;
};

export const TextField = forwardRef<HTMLInputElement, TextFieldProps>(function TextField(
  { label, hint, error, className, id, ...rest },
  ref,
) {
  const inputId = id ?? label?.toLowerCase().replace(/\s+/g, "-");
  return (
    <label className={cn("m3-field", className)} htmlFor={inputId}>
      {label ? <span className="m3-field__label">{label}</span> : null}
      <input ref={ref} id={inputId} className={cn("m3-field__input", error && "m3-field__input--error")} {...rest} />
      {error ? <span className="m3-field__error" role="alert">{error}</span> : hint ? <span className="m3-field__hint">{hint}</span> : null}
    </label>
  );
});

export function Select({ label, children, className, id, ...rest }: SelectHTMLAttributes<HTMLSelectElement> & { label?: string }) {
  const selectId = id ?? label?.toLowerCase().replace(/\s+/g, "-");
  return (
    <label className={cn("m3-field", className)} htmlFor={selectId}>
      {label ? <span className="m3-field__label">{label}</span> : null}
      <select id={selectId} className="m3-field__select" {...rest}>{children}</select>
    </label>
  );
}

export function Switch({
  checked,
  onCheckedChange,
  label,
  ariaLabel,
}: {
  checked: boolean;
  onCheckedChange: (v: boolean) => void;
  label?: ReactNode;
  ariaLabel?: string;
}) {
  return (
    <label className="m3-switch">
      <input
        type="checkbox"
        role="switch"
        checked={checked}
        onChange={(e) => onCheckedChange(e.target.checked)}
        aria-label={ariaLabel ?? (typeof label === "string" ? label : undefined)}
        className="m3-switch__input"
      />
      <span className="m3-switch__track" aria-hidden />
      {label ? <span className="m3-switch__label">{label}</span> : null}
    </label>
  );
}

export function IconButton({
  children,
  label,
  className,
  ...rest
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { label: string }) {
  return (
    <button type="button" className={cn("m3-icon-btn", className)} aria-label={label} {...rest}>
      {children}
    </button>
  );
}
