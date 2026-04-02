import type { Metadata } from 'next'
import { Inter, Orbitron } from 'next/font/google'
import './globals.css'

const inter = Inter({ subsets: ['latin'], variable: '--font-inter' })
const orbitron = Orbitron({ subsets: ['latin'], variable: '--font-orbitron', weight: ['400','600','700','800','900'] })

export const metadata: Metadata = {
  title: 'RAIG 888 · Autonomous Trading Engine',
  description: 'Military-Grade AI Bitcoin Scalping System',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className={`${inter.variable} ${orbitron.variable} ${inter.className}`}>
        {children}
      </body>
    </html>
  )
}
