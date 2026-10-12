// A newer quipu release: found when the user checks (or once a day if they turned that on),
// installed by the Go server, which then restarts itself. The page waits for the new version.

import { api } from "./api/client";
import type { UpdateInfo } from "./api/types";
import { report } from "./toast.svelte";

export const updates = $state<{ info: UpdateInfo | null; busy: "" | "check" | "install" }>({ info: null, busy: "" });

export async function loadUpdate() {
  try {
    updates.info = await api.update();
  } catch {
    // no update info: the version shows alone
  }
}

/** checkUpdate asks GitHub through quipu; it says so when there is nothing newer. */
export async function checkUpdate() {
  updates.busy = "check";
  try {
    updates.info = await api.checkUpdate();
    if (!updates.info.available) report({ ok: true, code: "update-latest", params: { version: updates.info.current } });
  } catch (e) {
    report(e);
  } finally {
    updates.busy = "";
  }
}

export async function setAutoUpdate(on: boolean) {
  try {
    report(await api.setAutoUpdate(on));
  } catch (e) {
    report(e);
  }
  await loadUpdate();
}

/** installUpdate installs the release and reloads the page once the new version answers. */
export async function installUpdate() {
  const want = updates.info?.latest;
  updates.busy = "install";
  try {
    const res = await api.installUpdate();
    report(res);
    if (res.code !== "update-installed" || !want) return;
    for (let i = 0; i < 90; i++) {
      await new Promise((r) => setTimeout(r, 1000));
      try {
        if ((await api.session()).version === want) {
          location.reload();
          return;
        }
      } catch {
        // restarting: not answering yet
      }
    }
    report({ ok: false, code: "update-no-restart", params: { version: want } });
  } catch (e) {
    report(e);
  } finally {
    updates.busy = "";
    loadUpdate();
  }
}
