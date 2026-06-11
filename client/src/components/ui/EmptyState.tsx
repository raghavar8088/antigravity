"use client";

import type { ReactNode } from "react";

type EmptyStateProps = {
  title: string;
  subtitle?: string;
  action?: ReactNode;
  icon?: ReactNode;
};

export function EmptyState({ title, subtitle, action, icon }: EmptyStateProps) {
  return (
    <div className="m3-empty-state" role="status">
      {icon ? <div className="m3-empty-state__icon" aria-hidden>{icon}</div> : null}
      <p className="m3-empty-state__title">{title}</p>
      {subtitle ? <p className="m3-empty-state__subtitle">{subtitle}</p> : null}
      {action}
    </div>
  );
}
