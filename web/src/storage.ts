const databaseName = "kynotes-web";
const storeName = "notes";

export type CachedNote = { id: string; containerID: string; version: number; payload: Uint8Array; updatedAt: string };
export type PendingSave = CachedNote;

function openDatabase(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(databaseName, 1);
    request.onupgradeneeded = () => request.result.createObjectStore(storeName, { keyPath: "id" });
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error ?? new Error("Unable to open local note store"));
  });
}

export async function putNote(note: CachedNote): Promise<void> {
  const db = await openDatabase();
  await new Promise<void>((resolve, reject) => {
    const request = db.transaction(storeName, "readwrite").objectStore(storeName).put(note);
    request.onsuccess = () => resolve(); request.onerror = () => reject(request.error);
  });
  db.close();
}

export async function getNote(id: string): Promise<CachedNote | undefined> {
  const db = await openDatabase();
  const result = await new Promise<CachedNote | undefined>((resolve, reject) => {
    const request = db.transaction(storeName).objectStore(storeName).get(id);
    request.onsuccess = () => resolve(request.result as CachedNote | undefined); request.onerror = () => reject(request.error);
  });
  db.close(); return result;
}

export async function deleteNote(id: string): Promise<void> {
  const db = await openDatabase();
  await new Promise<void>((resolve, reject) => {
    const request = db.transaction(storeName, "readwrite").objectStore(storeName).delete(id);
    request.onsuccess = () => resolve(); request.onerror = () => reject(request.error);
  });
  db.close();
}

const queueKey = "kynotes-pending-saves";
function encode(bytes: Uint8Array): string { let value = ""; for (const byte of bytes) value += String.fromCharCode(byte); return btoa(value); }
function decode(value: string): Uint8Array { return Uint8Array.from(atob(value), (char) => char.charCodeAt(0)); }

export function queueSave(note: PendingSave): void {
  try {
    const queue = JSON.parse(localStorage.getItem(queueKey) || "{}") as Record<string, Omit<PendingSave, "payload"> & { payload: string }>;
    queue[note.id] = { ...note, payload: encode(note.payload) };
    localStorage.setItem(queueKey, JSON.stringify(queue));
  } catch { /* local persistence is best effort; the encrypted IndexedDB draft remains available */ }
}

export function pendingSaves(): PendingSave[] {
  try {
    const queue = JSON.parse(localStorage.getItem(queueKey) || "{}") as Record<string, Omit<PendingSave, "payload"> & { payload: string }>;
    return Object.values(queue).map((note) => ({ ...note, payload: decode(note.payload) }));
  } catch { return []; }
}

export function clearQueuedSave(id: string): void {
  try { const queue = JSON.parse(localStorage.getItem(queueKey) || "{}") as Record<string, unknown>; delete queue[id]; localStorage.setItem(queueKey, JSON.stringify(queue)); } catch { /* optional */ }
}
