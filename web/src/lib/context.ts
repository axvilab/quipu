// What Claude Code reads in a project, grouped by where it comes from. Each thing appears once:
// a skill's listing (every session) and its body (when used) are one row with two numbers.

import type { Finding, Item, ProjectPlugin } from "./api/types";

export type GroupId = "project" | "memory" | "user" | "claude.ai" | "plugins";

export interface ContextRow {
  key: string;
  label: string;
  kind: string;
  path: string;
  editable: boolean;
  always: number; // tokens in every session
  onUse: number; // tokens when Claude opens it: a skill's body, a rule for some paths, a memory note
  findings: Finding[];
  plugin?: ProjectPlugin & { skills: string[] };
}

export interface ContextGroup {
  id: GroupId;
  rows: ContextRow[];
  always: number;
  onUse: number;
  findings: Finding[];
}

const order: GroupId[] = ["project", "memory", "user", "claude.ai", "plugins"];

function groupOf(scope: string): GroupId {
  if (scope === "project" || scope === "local") return "project";
  if (scope === "memory" || scope === "user" || scope === "claude.ai") return scope;
  return "plugins"; // "plugin <name>"
}

/** deletedPaths lists the files a finding's fix removes lines from or deletes. */
function deletedPaths(f: Finding): Set<string> {
  const out = new Set<string>();
  let file = "";
  for (const d of f.diff ?? []) {
    if (d.kind === "file") file = d.text;
    else if (d.kind === "del" && file) out.add(file);
  }
  return out;
}

// A finding belongs to the file it is about: its own path, the file an @import pulls in, the
// project copies a move deletes, or the plugin it names. Files a fix only touches on the side
// (the MEMORY.md line of a deleted note) are not marked.
const movesCopies = new Set(["shared-block", "copied-extension"]);

function findingsFor(row: ContextRow, findings: Finding[]): Finding[] {
  return findings.filter((f) => {
    if (row.plugin) return (f.rule === "unused-plugin" || f.rule === "plugin-one-project") && f.params.name === row.plugin.name;
    if (f.rule === "unused-skill") return row.kind === "skill" && row.label === f.params.name;
    if (f.rule === "large-import") return row.kind === "import" && (row.label === f.params.ref || row.path.endsWith("/" + f.params.ref));
    if (movesCopies.has(f.rule)) return deletedPaths(f).has(row.path);
    return f.path === row.path;
  });
}

export function contextGroups(items: Item[], plugins: ProjectPlugin[], findings: Finding[]): ContextGroup[] {
  const groups = new Map<GroupId, Map<string, ContextRow>>();
  for (const it of items) {
    const id = groupOf(it.scope);
    const rows = groups.get(id) ?? new Map<string, ContextRow>();
    groups.set(id, rows);
    const pluginName = id === "plugins" ? it.scope.slice("plugin ".length) : "";
    const key = pluginName ? "plugin:" + pluginName : it.kind + ":" + it.path;
    let row = rows.get(key);
    if (!row) {
      row = { key, label: pluginName || it.label, kind: pluginName ? "plugin" : it.kind, path: it.path, editable: it.editable && !pluginName, always: 0, onUse: 0, findings: [] };
      rows.set(key, row);
    }
    if (it.always) row.always += it.tokens;
    else row.onUse += it.tokens;
    if (pluginName && it.kind === "skill" && it.always) {
      row.plugin ??= { name: pluginName, scope: "user", tokens: 0, uses: 0, skills: [] };
      row.plugin.skills.push(it.label);
    }
  }
  // Plugins come with their use; one with only MCP tools has no items but still costs a session.
  if (plugins.length) {
    const rows = groups.get("plugins") ?? new Map<string, ContextRow>();
    groups.set("plugins", rows);
    for (const p of plugins) {
      const key = "plugin:" + p.name;
      const row = rows.get(key) ?? { key, label: p.name, kind: "plugin", path: "", editable: false, always: 0, onUse: 0, findings: [] };
      row.plugin = { ...p, skills: row.plugin?.skills ?? [] };
      rows.set(key, row);
    }
  }

  return order
    .filter((id) => groups.has(id))
    .map((id): ContextGroup => {
      const rows = [...groups.get(id)!.values()].sort((a, b) => b.always - a.always || b.onUse - a.onUse);
      for (const r of rows) r.findings = findingsFor(r, findings);
      const own = id === "claude.ai" ? findings.filter((f) => f.rule === "claude-ai-skills") : [];
      return {
        id,
        rows,
        always: rows.reduce((n, r) => n + r.always, 0),
        onUse: rows.reduce((n, r) => n + r.onUse, 0),
        findings: [...new Set([...own, ...rows.flatMap((r) => r.findings)])],
      };
    });
}
