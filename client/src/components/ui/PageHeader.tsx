"use client";

import type { ReactNode } from "react";
import { cn } from "./cn";

export function PageHeader({
  title,
  subtitle,
  actions,
  breadcrumb,
}: {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
  breadcrumb?: ReactNode;
}) {
  return (
    <header className="m3-page-header">
      {breadcrumb ? <div className="m3-breadcrumb">{breadcrumb}</div> : null}
      <div className="m3-page-header__row">
        <div>
          <h1 className="m3-page-header__title">{title}</h1>
          {subtitle ? <p className="m3-page-header__subtitle">{subtitle}</p> : null}
        </div>
        {actions ? <div className="m3-page-header__actions">{actions}</div> : null}
      </div>
    </header>
  );
}

export function SectionHeader({
  title,
  subtitle,
  actions,
  className,
}: {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("m3-section-header", className)}>
      <div>
        <h2 className="m3-section-header__title">{title}</h2>
        {subtitle ? <p className="m3-section-header__subtitle">{subtitle}</p> : null}
      </div>
      {actions}
    </div>
  );
}
