import { i18n } from "./i18n.svelte";

/** tokens formats a token count compactly: 950, 12.3K, 4.5M, 1.2B (localized separators). */
export function tokens(n: number): string {
  return new Intl.NumberFormat(i18n.locale, { notation: "compact", maximumFractionDigits: 1 }).format(n);
}

export function count(n: number): string {
  return new Intl.NumberFormat(i18n.locale).format(n);
}

export function percent(f: number, digits = 1): string {
  return new Intl.NumberFormat(i18n.locale, { style: "percent", maximumFractionDigits: digits }).format(f);
}

/** change is the relative change from previous to current, or null when there is no baseline. */
export function change(current: number, previous: number): number | null {
  if (!previous) return null;
  return (current - previous) / previous;
}

export function day(iso: string, opts: Intl.DateTimeFormatOptions = { day: "numeric", month: "short" }): string {
  return new Intl.DateTimeFormat(i18n.locale, opts).format(new Date(iso));
}

export function dateTime(iso: string): string {
  return new Intl.DateTimeFormat(i18n.locale, { dateStyle: "medium", timeStyle: "short" }).format(new Date(iso));
}
