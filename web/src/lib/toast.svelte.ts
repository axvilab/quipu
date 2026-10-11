import { ApiError } from "./api/client";
import type { Result } from "./api/types";
import { t } from "./i18n.svelte";

export interface Toast {
  id: number;
  kind: "ok" | "error";
  text: string;
}

export const toasts = $state<{ list: Toast[] }>({ list: [] });
let next = 1;

function push(kind: Toast["kind"], text: string) {
  const id = next++;
  toasts.list.push({ id, kind, text });
  setTimeout(() => dismiss(id), kind === "error" ? 9000 : 5000);
}

export function dismiss(id: number) {
  toasts.list = toasts.list.filter((x) => x.id !== id);
}

/** report shows the outcome of an action: a Result or an error thrown by the API client. */
export function report(r: Result | unknown) {
  if (r instanceof ApiError) {
    push("error", t(`result.${r.code}`, r.params));
  } else if (r && typeof r === "object" && "code" in r) {
    const res = r as Result;
    push(res.ok ? "ok" : "error", t(`result.${res.code}`, res.params ?? {}));
  } else {
    push("error", t("result.internal", { error: String(r) }));
  }
}
