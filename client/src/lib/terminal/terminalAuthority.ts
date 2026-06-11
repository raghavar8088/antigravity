import type { TerminalSnapshot } from "./terminalTypes";

export type AuthoritySource = "ws" | "rest" | "none";

export type TerminalAuthorityState = TerminalSnapshot & {
  authoritySource: AuthoritySource;
  restUnavailable: boolean;
  hasAuthority: boolean;
};

/** True when WS or REST has delivered at least one authoritative snapshot. */
export function terminalHasAuthority(state: TerminalAuthorityState): boolean {
  if (state.authoritySource === "ws" && state.connected) return true;
  if (state.authoritySource === "rest" && state.updatedAt !== "" && !state.restUnavailable) return true;
  return false;
}

export function terminalAuthorityLabel(state: TerminalAuthorityState): string {
  if (state.loading) return "LOADING";
  if (!terminalHasAuthority(state)) return "BACKEND AUTHORITY UNAVAILABLE";
  if (state.authoritySource === "ws") return "WS LIVE";
  return "REST AUTHORITY";
}
