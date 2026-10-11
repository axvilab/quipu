// The background collection of transcripts. On the very first run there is nothing to show yet,
// so App shows the progress instead of the screens; later runs only make the screens reload when
// they stored something new.

import { api } from "./api/client";
import type { Collection } from "./api/types";
import { notifyChanged } from "./changes.svelte";

export const collection = $state<{ data: Collection | null }>({ data: null });

/** watchCollection polls every second while collecting for the first time, then every 20 s. */
export function watchCollection() {
  $effect(() => {
    let timer: ReturnType<typeof setTimeout>;
    let stopped = false;
    async function poll() {
      try {
        const next = await api.collection();
        const before = collection.data;
        collection.data = next;
        // The first data arrived, or a later run stored something new: screens load again.
        if (before && (before.first !== next.first || before.generation !== next.generation)) notifyChanged();
      } catch {
        // keep the last state
      }
      if (!stopped) timer = setTimeout(poll, collection.data?.first ? 1000 : 20_000);
    }
    poll();
    return () => {
      stopped = true;
      clearTimeout(timer);
    };
  });
}
