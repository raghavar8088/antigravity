"use client";

export function ErrorBanner({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className="m3-banner m3-banner--error m3-error-banner" role="alert">
      <span>{message}</span>
      {onRetry ? (
        <button type="button" className="m3-error-banner__retry" onClick={onRetry}>
          Retry
        </button>
      ) : null}
    </div>
  );
}
