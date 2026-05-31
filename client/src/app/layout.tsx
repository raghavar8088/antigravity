import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import "./globals.css";

const inter = Inter({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-inter",
  display: "swap",
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  variable: "--font-mono-var",
  display: "swap",
});

export const metadata: Metadata = {
  title: "BTC Terminal — Institutional Research Platform",
  description: "Professional BTC mock trading and research terminal with live strategy signals, risk management, and analytics.",
  icons: {
    icon: "/branding/in-loop-logo.png",
    apple: "/branding/in-loop-logo.png",
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html
      lang="en"
      data-theme="dark"
      className={`${inter.variable} ${jetbrainsMono.variable}`}
    >
      <head>
        <meta name="theme-color" content="#0d1117" />
        <meta name="color-scheme" content="dark" />
      </head>
      <body className={inter.className}>{children}</body>
    </html>
  );
}
