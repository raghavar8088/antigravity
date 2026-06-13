import type { Metadata } from "next";
import { Roboto, Roboto_Mono } from "next/font/google";
import "./globals.css";
import { GlobalRiskRibbon } from "@/components/GlobalRiskRibbon";
import { M3Providers } from "@/components/ui/M3Providers";
import { SessionProvider } from "@/context/SessionContext";

const roboto = Roboto({
  subsets: ["latin"],
  weight: ["400", "500", "700"],
  variable: "--font-roboto",
  display: "swap",
});

const robotoMono = Roboto_Mono({
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  variable: "--font-roboto-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: "ICC — Institutional Command Center",
  description: "Google Material 3 institutional trading platform for BTC futures research, mock trading, and risk monitoring.",
  icons: {
    icon: "/branding/in-loop-logo.png",
    apple: "/branding/in-loop-logo.png",
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning className={`${roboto.variable} ${robotoMono.variable}`}>
      <head>
        <meta name="theme-color" content="#131314" />
        <meta name="color-scheme" content="dark light" />
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var t=localStorage.getItem("m3-theme");var d=window.matchMedia("(prefers-color-scheme: dark)").matches;if(t==="light"||t==="dark")document.documentElement.setAttribute("data-theme",t);else document.documentElement.setAttribute("data-theme",d?"dark":"light");}catch(e){document.documentElement.setAttribute("data-theme","dark");}})();`,
          }}
        />
      </head>
      <body className={roboto.className}>
        <SessionProvider>
          <M3Providers>
            <GlobalRiskRibbon />
            {children}
          </M3Providers>
        </SessionProvider>
      </body>
    </html>
  );
}
