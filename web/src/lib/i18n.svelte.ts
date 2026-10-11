// Texts live in src/locales/{uk,en}.json. A value is a string with {param} placeholders, or an
// object of plural forms ({ one, few, many, other }) chosen by the "count" parameter.

import uk from "../locales/uk.json";
import en from "../locales/en.json";

type Entry = string | Partial<Record<Intl.LDMLPluralRule, string>>;
type Dict = Record<string, Entry>;

const dicts: Record<Locale, Dict> = { uk, en };
export const locales = ["uk", "en"] as const;
export type Locale = (typeof locales)[number];

const stored = (() => {
  try {
    return localStorage.getItem("quipu.locale");
  } catch {
    return null;
  }
})();

function detect(): Locale {
  if (stored === "uk" || stored === "en") return stored;
  const langs = navigator.languages ?? [navigator.language];
  // Ukrainian first; Russian-speaking systems in Ukraine also get Ukrainian, everyone else English.
  return langs.some((l) => l.startsWith("uk") || l.startsWith("ru")) ? "uk" : "en";
}

export const i18n = $state({ locale: detect() });

export function setLocale(l: Locale) {
  i18n.locale = l;
  document.documentElement.lang = l;
  try {
    localStorage.setItem("quipu.locale", l);
  } catch {
    // private mode: the choice lasts for this page only
  }
}

/** t translates key; missing keys show the key itself so they are easy to spot. */
export function t(key: string, params: Record<string, string | number> = {}): string {
  const entry = dicts[i18n.locale][key] ?? dicts.en[key];
  if (entry === undefined) return key;
  let text: string;
  if (typeof entry === "string") {
    text = entry;
  } else {
    const n = Number(params.count ?? 0);
    const form = new Intl.PluralRules(i18n.locale).select(n);
    text = entry[form] ?? entry.other ?? Object.values(entry)[0] ?? key;
  }
  return text.replace(/\{(\w+)\}/g, (_, name: string) => String(params[name] ?? `{${name}}`));
}

/** has reports whether a translation exists, for optional texts such as rule details. */
export function has(key: string): boolean {
  return key in dicts[i18n.locale] || key in dicts.en;
}
