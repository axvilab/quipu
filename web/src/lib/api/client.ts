// The only way the frontend talks to quipu. Today it uses the HTTP API; a native window can
// implement the same functions over its bindings without touching any screen (redesign step 6).

import type {
  Advice,
  BackupInfo,
  BackupResult,
  CacheReport,
  EditorFile,
  Live,
  Overview,
  Doctor,
  Profiles,
  Project,
  RestorePlan,
  Result,
  Session,
} from "./types";

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    readonly params: Record<string, string> = {},
  ) {
    super(code);
  }
}

async function request<T>(method: "GET" | "POST", path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    credentials: "same-origin",
    headers: body === undefined ? {} : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    const r = data as Partial<Result>;
    throw new ApiError(res.status, r.code ?? "internal", r.params ?? {});
  }
  return data as T;
}

const get = <T>(path: string) => request<T>("GET", path);
const post = (path: string, body: unknown = {}) => request<Result>("POST", path, body);

export const api = {
  session: () => get<Session>("/api/session"),
  overview: (days: number) => get<Overview>(`/api/overview?days=${days}`),
  project: (dir: string, days: number) => get<Project>(`/api/projects/${encodeURIComponent(dir)}?days=${days}`),
  cache: (days: number) => get<CacheReport>(`/api/cache?days=${days}`),
  advice: () => get<Advice>("/api/advice"),
  apply: (dir: string, id: string) => post("/api/advice/apply", { dir, id }),
  undo: (id: string) => post("/api/advice/undo", { id }),
  readFile: (path: string) => get<EditorFile>(`/api/file?path=${encodeURIComponent(path)}`),
  saveFile: (path: string, content: string, version: string) => post("/api/file", { path, content, version }),
  live: () => get<Live>("/api/live"),
  doctor: () => get<Doctor>("/api/doctor"),
  setShim: (on: boolean) => post("/api/shim", { on }),
  backup: () => get<BackupInfo>("/api/backup"),
  createBackup: (opts: { gitDir: string; push: boolean; allowSecrets: boolean }) =>
    request<BackupResult>("POST", "/api/backup", opts),
  planRestore: (source: string) => request<RestorePlan>("POST", "/api/restore/plan", { source }),
  restore: (source: string) => post("/api/restore", { source }),
  watchRemoteControl: (on: boolean) => post("/api/live/watch", { on }),
  tokens: (text: string) => request<{ tokens: number }>("POST", "/api/tokens", { text }),
  profiles: () => get<Profiles>("/api/profiles"),
  addProfile: (name: string, share: string[]) => post("/api/profiles", { name, share }),
  removeProfile: (name: string) => post(`/api/profiles/${encodeURIComponent(name)}/remove`),
  signIn: (name: string, opts: { email: string; browser: string; private: boolean }) =>
    post(`/api/profiles/${encodeURIComponent(name)}/sign-in`, opts),
  openPrivate: (name: string, browser: string) => post(`/api/profiles/${encodeURIComponent(name)}/private`, { browser }),
  setRule: (folder: string, profile: string) => post("/api/rules", { folder, profile }),
  deleteRule: (folder: string) => post("/api/rules/delete", { folder }),
};
