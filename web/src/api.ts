export type User = { id: string; role: string };
export type Session = { user: User; expiresAt: string; hardExpiresAt: string };
export type Container = { id: string; kind: string; metaCiphertext: string; metaVersion: number; changeSeq: number; keyGeneration: number };
export type Comment = { id: string; authorUserId: string; username: string; bodyCiphertext: string; keyGeneration: number; createdAt: string };
export type AdminUser = { id: string; username: string; role: string; status: string; quotaBytes: number; createdAt: string };
export type Change = { id: string; kind: string; changeSeq: number; deleted: boolean };
export type Note = { id: string; title: string; body: string; version: number; updatedAt: string };

type APIError = { error?: { code?: string; message?: string } };

function csrfToken(): string {
  return document.cookie.split("; ").find((v) => v.startsWith("csrf_token="))?.slice(11) ?? "";
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const method = (init.method ?? "GET").toUpperCase();
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (init.body && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  if (["POST", "PUT", "PATCH", "DELETE"].includes(method)) headers.set("X-CSRF-Token", csrfToken());
  const response = await fetch(path, { ...init, headers, credentials: "include" });
  if (!response.ok) {
    let detail: APIError = {};
    try { detail = await response.json() as APIError; } catch { /* opaque server error */ }
    throw new Error(detail.error?.message ?? `Request failed (${response.status})`);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

export async function loginParams(username: string) {
  return request<{ loginSalt: string; iterations: number }>("/api/v1/auth/login-params", {
    method: "POST", body: JSON.stringify({ username }),
  });
}

export async function login(username: string, authSecret: string) {
  return request<Session>("/api/v1/auth/login", {
    method: "POST", body: JSON.stringify({ username, authSecret }),
  });
}

export const session = () => request<Session>("/api/v1/auth/session");
export const serverTheme = () => request<{ defaultTheme: string }>("/api/v1/theme");
export const logout = () => request<void>("/api/v1/auth/logout", { method: "POST" });
export const containers = () => request<Container[]>("/api/v1/containers");

export function createContainer(kind = "workbook", metaCiphertext = "") {
  return request<Container>("/api/v1/containers", {
    method: "POST", body: JSON.stringify({ kind, metaCiphertext }),
  });
}

export function updateContainer(id: string, metaCiphertext: string, baseVersion: number) {
  return request<{ metaVersion: number; changeSeq: number }>(`/api/v1/containers/${encodeURIComponent(id)}`, {
    method: "PATCH", body: JSON.stringify({ metaCiphertext, baseVersion }),
  });
}

export async function serviceStatus() {
  const [health, ready] = await Promise.all([fetch("/healthz"), fetch("/readyz")]);
  return { health: health.ok, ready: ready.ok };
}
export const adminUsers = () => request<AdminUser[]>("/api/v1/admin/users");
export const adminAudit = () => request<Array<Record<string, string>>>("/api/v1/admin/audit");
export const adminSettings = () => request<{ defaultTheme: string }>("/api/v1/admin/settings");
export function updateAdminSettings(defaultTheme: string) { return request<void>("/api/v1/admin/settings", { method: "PATCH", body: JSON.stringify({ defaultTheme }) }); }
export function updateAdminUser(user: AdminUser) { return request<void>(`/api/v1/admin/users/${encodeURIComponent(user.id)}`, { method: "PATCH", body: JSON.stringify(user) }); }
export const members = (containerID: string) => request<Array<{ userId: string; username: string; role: string }>>(`/api/v1/containers/${encodeURIComponent(containerID)}/members`);
export function inviteMember(containerID: string, inviteeID: string, role: string) { return request<{ id: string; token: string }>(`/api/v1/containers/${encodeURIComponent(containerID)}/invitations`, { method: "POST", body: JSON.stringify({ inviteeId: inviteeID, role }) }); }
export function removeMember(containerID: string, userID: string) { return request<void>(`/api/v1/containers/${encodeURIComponent(containerID)}/members/${encodeURIComponent(userID)}`, { method: "DELETE" }); }
export const comments = (objectID: string) => request<Comment[]>(`/api/v1/objects/${encodeURIComponent(objectID)}/comments`);
export function createComment(objectID: string, bodyCiphertext: string, keyGeneration: number) { return request<{ id: string }>(`/api/v1/objects/${encodeURIComponent(objectID)}/comments`, { method: "POST", body: JSON.stringify({ bodyCiphertext, keyGeneration, mentions: [] }) }); }

export async function changes(containerID: string, since = 0) {
  return request<{ changes: Change[]; nextCursor: string; hasMore: boolean }>(
    `/api/v1/containers/${encodeURIComponent(containerID)}/changes?since=${since}`,
  );
}

export async function createObject(containerID: string) {
  return request<{ id: string; version: number; changeSeq: number }>(
    `/api/v1/containers/${encodeURIComponent(containerID)}/objects`, {
      method: "POST", body: JSON.stringify({ kind: "note" }),
    },
  );
}

export async function readObject(objectID: string, version?: number) {
  const response = await fetch(`/api/v1/objects/${encodeURIComponent(objectID)}${version ? `?version=${version}` : ""}`, {
    credentials: "include", headers: { Accept: "application/octet-stream" },
  });
  if (!response.ok) throw new Error(`Unable to read note (${response.status})`);
  return { bytes: new Uint8Array(await response.arrayBuffer()), version: Number(response.headers.get("X-Kynotes-Version") ?? 0) };
}

export async function saveObject(objectID: string, bytes: Uint8Array, baseVersion: number, keyGeneration = 1) {
  return request<{ version: number; resourceId?: string }>(`/api/v1/objects/${encodeURIComponent(objectID)}`, {
    method: "PUT",
    body: bytes as unknown as BodyInit,
    headers: {
      "Content-Type": "application/octet-stream",
      "X-Kynotes-Base-Version": String(baseVersion),
      "X-Kynotes-Key-Generation": String(keyGeneration),
      "Idempotency-Key": crypto.randomUUID(),
    },
  });
}

export const deleteObject = (objectID: string) => request<void>(`/api/v1/objects/${encodeURIComponent(objectID)}`, { method: "DELETE" });
