import type { Metadata } from "next";
import { Inter, Roboto, Roboto_Mono } from "next/font/google";
import "./globals.css";

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
  weight: ["400", "500", "600", "700"],
  display: "swap",
  fallback: ["system-ui", "-apple-system", "sans-serif"],
  preload: false,
});

const roboto = Roboto({
  subsets: ["latin"],
  variable: "--font-ui",
  weight: ["400", "500", "700"],
  display: "swap",
  fallback: ["system-ui", "-apple-system", "sans-serif"],
  preload: false,
});

const robotoMono = Roboto_Mono({
  subsets: ["latin"],
  variable: "--font-mono-var",
  weight: ["400", "500", "700"],
  display: "swap",
  fallback: ["ui-monospace", "monospace"],
  preload: false,
});

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
      <body className={`${inter.variable} ${roboto.variable} ${robotoMono.variable} ${roboto.className}`}>
        {children}
      </body>
    </html>
  );
}
