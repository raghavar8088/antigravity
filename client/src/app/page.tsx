import { redirect } from "next/navigation";

/**
 * Root redirect — send all users to the institutional terminal.
 * Auth middleware will forward unauthenticated sessions to /login.
 */
export default function Home() {
  redirect("/terminal");
}
