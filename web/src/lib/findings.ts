import type { Change, Finding } from "./api/types";
import { has, t } from "./i18n.svelte";

/** title of a finding in the current language. */
export function findingTitle(f: Finding): string {
  const params = { ...f.params, count: f.params.count ?? f.projects.length };
  const key = f.params.state ? `rule.${f.rule}.title.${f.params.state}` : `rule.${f.rule}.title`;
  return t(has(key) ? key : `rule.${f.rule}.title`, params);
}

/** title of a change quipu made, in the current language when the change recorded its params. */
export function changeTitle(c: Change): string {
  if (!Object.keys(c.params).length) return c.title; // made before params were recorded
  const key = c.params.state ? `rule.${c.rule}.title.${c.params.state}` : `rule.${c.rule}.title`;
  const found = has(key) ? key : `rule.${c.rule}.title`;
  return has(found) ? t(found, { ...c.params, count: c.params.count ?? 1 }) : c.title;
}

export function findingDetail(f: Finding): string {
  const key = f.params.state ? `rule.${f.rule}.detail.${f.params.state}` : `rule.${f.rule}.detail`;
  return has(key) ? t(key, f.params) : "";
}

/** where says where the fix goes: the user's own settings, several projects or one. */
export function findingWhere(f: Finding): string {
  if (f.scope === "user") return t("finding.where.user", { count: f.projects.length });
  if (f.projects.length > 1) return t("finding.where.projects", { count: f.projects.length });
  const p = f.projects[0] ?? "";
  return p.split("/").filter(Boolean).pop() ?? p;
}
