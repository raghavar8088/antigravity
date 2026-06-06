import { redirect } from "next/navigation";

/** Redirects legacy /sign-in URL to the new /login page. */
export default function SignInRedirect() {
  redirect("/login");
}
