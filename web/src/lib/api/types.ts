// The JSON contract of the quipu API. Mirrors the structs in internal/ui/service.go; change both
// together.

export interface Totals {
  sessions: number;
  requests: number;
  prompt: number;
  cacheRead: number;
  cacheWrite: number;
  output: number;
  misses: number;
  hitRate: number;
}

export interface Start {
  sessions: number;
  median: number;
  latest: number;
  latestAt?: string;
}

export interface DiffLine {
  kind: "add" | "del" | "ctx" | "file";
  line?: number;
  text: string;
}

export interface Finding {
  id: string;
  rule: string;
  params: Record<string, string>;
  path?: string;
  diff?: DiffLine[];
  saves: number;
  canApply: boolean;
  dir: string;
  projects: string[];
  scope: "user" | "project";
}

export interface ProjectRow {
  dir: string;
  path: string;
  name: string;
  totals: Totals;
  start: Start;
  findings: number;
}

export interface Overview {
  days: number;
  totals: Totals;
  previous: Totals;
  projects: ProjectRow[];
  attention: Finding[];
}

export interface Item {
  label: string;
  scope: string;
  kind: string;
  always: boolean;
  tokens: number;
  path: string;
  editable: boolean;
}

export interface Day {
  day: string;
  totals: Totals;
}

export interface Project {
  dir: string;
  path: string;
  name: string;
  totals: Totals;
  previous: Totals;
  start: Start;
  items: Item[];
  alwaysTotal: number;
  onDemandTotal: number;
  findings: Finding[];
  daily: Day[];
  changes: Change[];
}

export interface Miss {
  reason: string;
  count: number;
  missedTokens: number;
}

export interface CacheReport {
  days: number;
  totals: Totals;
  previous: Totals;
  daily: Day[];
  misses: Miss[];
}

export interface Change {
  id: string;
  path: string;
  files: string[];
  rule: string;
  title: string;
  params: Record<string, string>;
  time: string;
  undone: boolean;
}

export interface Advice {
  findings: Finding[];
  changes: Change[];
}

export interface SignIn {
  state: "" | "starting" | "open";
  url?: string;
}

export interface ProfileInfo {
  name: string;
  account: string;
  plan: string;
  signedIn: boolean;
  current: boolean;
  dir: string;
  shares: string[];
  signIn: SignIn;
}

export interface Rule {
  path: string;
  profile: string;
}

export interface Browser {
  id: string;
  name: string;
  default: boolean;
}

export interface Profiles {
  profiles: ProfileInfo[];
  rules: Rule[];
  folders: string[];
  shareable: string[];
  browsers: Browser[];
}

export interface Session {
  version: string;
  profile: string;
  profiles: ProfileInfo[];
  userFile: string;
}

/** The answer to an action, or an error: a code translated as `result.<code>`. */
export interface Result {
  ok: boolean;
  code: string;
  params?: Record<string, string>;
}

/** An instruction file opened in the editor. */
export interface EditorFile {
  path: string;
  name: string;
  content: string;
  version: string;
  tokens: number;
  always: boolean;
  exists: boolean;
}

/** A running Claude Code session. */
export interface LiveSession {
  pid: number;
  path: string;
  name: string;
  started: string;
  remoteControl: "on" | "off" | "unknown";
  source: "command" | "hook" | "";
  seen?: string;
}

export interface Live {
  sessions: LiveSession[];
  watching: boolean;
  autoStart: boolean;
}

export interface Archive {
  path: string;
  name: string;
  size: number;
  time: string;
}

export interface BackupInfo {
  folder: string;
  gitDir: string;
  archives: Archive[];
}

export interface BackupSecret {
  path: string;
  line: number;
  kind: string;
}

export interface BackupResult {
  ok: boolean;
  path?: string;
  files: number;
  changed: boolean;
  secrets: BackupSecret[];
}

export interface RestorePlan {
  source: string;
  created: string;
  from: string;
  version: string;
  new: number;
  changed: number;
  same: number;
  files: { path: string; state: "new" | "changed" }[];
  links: number;
}
