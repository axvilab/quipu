// Running Claude Code sessions, refreshed every 30 seconds while quipu is open: the Remote Control
// banner shows on every screen, so the state lives here.

import { api } from "./api/client";
import type { Live } from "./api/types";

export const live = $state<{ data: Live | null }>({ data: null });

export async function refreshLive() {
  try {
    live.data = await api.live();
  } catch {
    // keep the last state; the banner must not flicker on a hiccup
  }
}

/** watchLive refreshes now, every 30 s and after quipu changes something; use it in App. */
export function watchLive() {
  $effect(() => {
    refreshLive();
    const timer = setInterval(refreshLive, 30_000);
    window.addEventListener("quipu:changed", refreshLive);
    return () => {
      clearInterval(timer);
      window.removeEventListener("quipu:changed", refreshLive);
    };
  });
}
