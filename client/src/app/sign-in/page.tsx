import { PaperDeskAuthBar } from "@/components/PaperDeskAuthBar";
import Link from "next/link";

export default function SignInPage() {
  return (
    <main className="mx-auto flex min-h-[50vh] max-w-md flex-col justify-center gap-6 p-6">
      <div>
        <h1 className="text-lg font-bold text-zinc-900">Paper desk sign-in</h1>
        <p className="mt-1 text-sm text-zinc-600">
          Enter your email — same trade history on phone and desktop.
        </p>
      </div>
      <PaperDeskAuthBar />
      <Link href="/" className="text-sm text-sky-700 underline">
        Back to dashboard
      </Link>
    </main>
  );
}
