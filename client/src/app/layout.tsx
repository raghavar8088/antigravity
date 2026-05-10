import type { Metadata } from "next";
import "./globals.css";

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
    <html lang="en">
      <head>
        <meta name="theme-color" content="#1c1917" />
      </head>
      <body>
        {children}
      </body>
    </html>
  );
}
