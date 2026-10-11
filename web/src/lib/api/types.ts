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
  kind: "add" | "del" | "ctx";
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
  rule: string;
  title: string;
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
}

/** The answer to an action, or an error: a code translated as `result.<code>`. */
export interface Result {
  ok: boolean;
  code: string;
  params?: Record<string, string>;
}
