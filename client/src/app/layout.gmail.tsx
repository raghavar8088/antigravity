import type { Metadata } from "next";
import { Roboto, Roboto_Mono } from "next/font/google";
import "./globals.css";

const roboto = Roboto({
  subsets: ["latin"],
  variable: "--font-ui",
  weight: ["400", "500", "700"],
});

const robotoMono = Roboto_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
  weight: ["400", "500", "700"],
});

export const metadata: Metadata = {
  title: "RAIG | Trading Workspace",
  description: "RAIG Bitcoin trading workspace with live engine, AI review, and execution telemetry.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className={`${roboto.variable} ${robotoMono.variable} ${roboto.className}`}>
        {children}
      </body>
    </html>
  );
}
