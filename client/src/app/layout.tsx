import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "RAIG | Trading Workspace",
  description: "RAIG Bitcoin trading workspace with live engine, AI review, and execution telemetry.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <head>
        <meta name="theme-color" content="#ffffff" />
      </head>
      <body>
        {children}
      </body>
    </html>
  );
}
