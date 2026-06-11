"use client";

import type { ReactNode } from "react";
import { Component, type ErrorInfo } from "react";
import { Button } from "./Button";
import { EmptyState } from "./EmptyState";

type Props = { children: ReactNode; fallbackTitle?: string };
type State = { error: Error | null };

export class M3ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("[M3ErrorBoundary]", error, info.componentStack);
  }

  render() {
    if (this.state.error) {
      return (
        <div className="m3-error-boundary">
          <EmptyState
            title={this.props.fallbackTitle ?? "Something went wrong"}
            subtitle={this.state.error.message}
            action={
              <div className="m3-ds-row">
                <Button variant="tonal" onClick={() => this.setState({ error: null })}>Retry</Button>
                <Button variant="outlined" onClick={() => { window.location.href = "/terminal"; }}>Go home</Button>
              </div>
            }
          />
        </div>
      );
    }
    return this.props.children;
  }
}
