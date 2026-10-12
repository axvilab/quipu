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

/** findingRow is the short text of a finding inside its rule's group, whose title explains it. */
export function findingRow(f: Finding): string {
  const params = { ...f.params, count: f.params.count ?? f.projects.length };
  const key = f.params.state && has(`rule.${f.rule}.row.${f.params.state}`) ? `rule.${f.rule}.row.${f.params.state}` : `rule.${f.rule}.row`;
  return has(key) ? t(key, params) : findingTitle(f);
}

/** FindingGroup is the findings of one rule. */
export interface FindingGroup {
  rule: string;
  findings: Finding[];
  saves: number;
  manual: boolean; // nothing in it can be applied by quipu
  fixAll: boolean; // every fix can be applied in one go without costing other projects tokens
}

// Moving a block or a skill into your settings makes the other projects send it too, so those
// are applied one at a time after reading the diff. A block every project already has is safe.
const addsElsewhere = (f: Finding) =>
  f.rule === "copied-extension" || (f.rule === "shared-block" && f.params.state !== "all");

/** groupFindings groups findings by rule: the biggest savings first, the ones to fix by hand last. */
export function groupFindings(findings: Finding[]): FindingGroup[] {
  const byRule = new Map<string, Finding[]>();
  for (const f of findings) byRule.set(f.rule, [...(byRule.get(f.rule) ?? []), f]);
  const groups = [...byRule].map(([rule, fs]): FindingGroup => {
    const sorted = [...fs].sort((a, b) => b.saves - a.saves);
    return {
      rule,
      findings: sorted,
      saves: fs.reduce((n, f) => n + f.saves, 0),
      manual: fs.every((f) => !f.canApply),
      fixAll: fs.length > 1 && fs.every((f) => f.canApply && !addsElsewhere(f)),
    };
  });
  return groups.sort((a, b) => Number(a.manual) - Number(b.manual) || b.saves - a.saves || b.findings.length - a.findings.length);
}
