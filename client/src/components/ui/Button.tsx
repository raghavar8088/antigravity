"use client";

import * as Slot from "@radix-ui/react-slot";
import type { ButtonHTMLAttributes, ReactNode } from "react";
import { cn } from "./cn";

type ButtonVariant = "filled" | "tonal" | "outlined" | "text" | "danger";

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  asChild?: boolean;
  icon?: ReactNode;
};

const variantClass: Record<ButtonVariant, string> = {
  filled: "m3-btn m3-btn--filled",
  tonal: "m3-btn m3-btn--tonal",
  outlined: "m3-btn m3-btn--outlined",
  text: "m3-btn m3-btn--text",
  danger: "m3-btn m3-btn--danger",
};

export function Button({
  variant = "tonal",
  asChild = false,
  icon,
  className,
  children,
  type = "button",
  ...rest
}: ButtonProps) {
  const Comp = asChild ? Slot.Root : "button";
  return (
    <Comp type={asChild ? undefined : type} className={cn(variantClass[variant], className)} {...rest}>
      {icon}
      {children}
    </Comp>
  );
}
