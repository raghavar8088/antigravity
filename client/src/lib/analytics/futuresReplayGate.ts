/** Whether the desk replay gate feature is enabled (set NEXT_PUBLIC_DESK_REPLAY_GATE=1). */
export function deskReplayGateEnabled(): boolean {
  return process.env.NEXT_PUBLIC_DESK_REPLAY_GATE === "1";
}
