import type { Metadata } from "next";
import { Sora, Manrope, JetBrains_Mono } from "next/font/google";
import "./globals.css";
import { GlobalRiskRibbon } from "@/components/GlobalRiskRibbon";
import AppShell from "@/components/AppShell";
import { M3Providers } from "@/components/ui/M3Providers";
import { SessionProvider } from "@/context/SessionContext";

const sora = Sora({
  subsets: ["latin"],
  weight: ["600", "700", "800"],
  variable: "--font-sora",
  display: "swap",
});

const manrope = Manrope({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-manrope",
  display: "swap",
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-jetbrains-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: "ICC — Institutional Command Center",
  description: "Google Material 3 institutional trading platform for BTC futures research, Trade Engine execution, and risk monitoring.",
  icons: {
    icon: "/branding/in-loop-logo.png",
    apple: "/branding/in-loop-logo.png",
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning className={`${sora.variable} ${manrope.variable} ${jetbrainsMono.variable}`}>
      <head>
        <meta name="theme-color" content="#ffffff" />
        <meta name="color-scheme" content="light dark" />
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var t=localStorage.getItem("m3-theme");document.documentElement.setAttribute("data-theme",t==="light"||t==="dark"?t:"light");}catch(e){document.documentElement.setAttribute("data-theme","light");}})();`,
          }}
        />
      </head>
      <body className={manrope.className}>
        <SessionProvider>
          <M3Providers>
            <AppShell>
              <GlobalRiskRibbon />
              {children}
            </AppShell>
          </M3Providers>
        </SessionProvider>
      </body>
    </html>
  );
}
