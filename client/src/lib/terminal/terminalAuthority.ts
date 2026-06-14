import type { TerminalSnapshot } from "./terminalTypes";

export type AuthoritySource = "ws" | "rest" | "none";

export type TerminalAuthorityState = TerminalSnapshot & {
  authoritySource: AuthoritySource;
  restUnavailable: boolean;
  hasAuthority: boolean;
};

/** True only after WS or REST has delivered at least one authoritative snapshot delta. */
export function terminalHasAuthority(
  state: Pick<TerminalAuthorityState, "authoritySource" | "connected" | "updatedAt" | "restUnavailable">,
): boolean {
  if (state.restUnavailable || state.updatedAt === "") return false;
  if (state.authoritySource === "ws" && state.connected) return true;
  if (state.authoritySource === "rest") return true;
  return false;
}

export function terminalAuthorityLabel(state: TerminalAuthorityState): string {
  if (state.loading) return "Updating";
  if (!terminalHasAuthority(state)) return "Stale";
  if (state.authoritySource === "ws") return "Live";
  return "Polling";
}
