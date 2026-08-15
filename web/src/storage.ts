const databaseName = "kynotes-web";
const storeName = "notes";

export type CachedNote = { id: string; containerID: string; version: number; payload: Uint8Array; updatedAt: string };

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
