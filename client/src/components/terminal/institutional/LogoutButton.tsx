"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { NavIcon } from "./NavIcons";

/** Compact header control — signs out (clears raig_session) and redirects to /sign-in. */
export function LogoutButton() {
  const router = useRouter();
  const [loading, setLoading] = useState(false);

  async function onLogout() {
    if (loading) return;
    setLoading(true);
    try {
      await fetch("/api/auth/signout", { method: "POST" });
    } finally {
      router.push("/sign-in");
      router.refresh();
    }
  }

  return (
    <button
      type="button"
      className="m3-icon-btn"
      onClick={onLogout}
      disabled={loading}
      aria-label="Log out"
      title="Log out"
    >
      <NavIcon name="logout" />
    </button>
  );
}
