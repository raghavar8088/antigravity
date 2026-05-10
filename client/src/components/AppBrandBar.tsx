"use client";

/**
 * Global product chrome — in.loop.com identity only.
 * Individual desks keep their own module titles (Future Trading, Options, etc.).
 */
export default function AppBrandBar() {
  return (
    <header
      className="app-brand-bar"
      style={{
        background: "linear-gradient(180deg, #0c0a09 0%, #1c1917 100%)",
        borderBottom: "1px solid rgba(212, 175, 55, 0.15)",
      }}
    >
      <div
        style={{
          maxWidth: 1680,
          margin: "0 auto",
          padding: "12px 20px",
          display: "flex",
          alignItems: "center",
          gap: 14,
        }}
      >
        <img
          src="/branding/in-loop-logo.png"
          alt="in.loop.com — autonomous trading in Indian and crypto markets"
          style={{
            height: 64,
            width: "auto",
            maxWidth: "min(100%, 900px)",
            objectFit: "contain",
            objectPosition: "left center",
          }}
        />
      </div>
    </header>
  );
}
