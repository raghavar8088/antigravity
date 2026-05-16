import type { Metadata } from "next";
import { Roboto, Roboto_Mono } from "next/font/google";
import "./globals.css";

const roboto = Roboto({
  subsets: ["latin"],
  weight: ["400", "500", "700"],
  variable: "--font-desk-body",
  display: "swap",
});

const robotoMono = Roboto_Mono({
  subsets: ["latin"],
  weight: ["400", "500"],
  variable: "--font-desk-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: "in.loop.com",
  description: "Autonomous trading in Indian and crypto markets.",
  icons: {
    icon: "/branding/in-loop-logo.png",
    apple: "/branding/in-loop-logo.png",
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" data-theme="light" className={`${roboto.variable} ${robotoMono.variable}`}>
      <head>
        <meta name="theme-color" content="#f8f9fa" />
      </head>
      <body className={roboto.className}>{children}</body>
    </html>
  );
}
