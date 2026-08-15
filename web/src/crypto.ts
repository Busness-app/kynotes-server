const encoder = new TextEncoder();
const decoder = new TextDecoder();

function hexBytes(value: string): Uint8Array {
  const bytes = new Uint8Array(value.length / 2);
  for (let i = 0; i < bytes.length; i++) bytes[i] = Number.parseInt(value.slice(i * 2, i * 2 + 2), 16);
  return bytes;
}

function base64(bytes: Uint8Array): string {
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

function fromBase64(value: string): Uint8Array {
  const binary = atob(value);
  return Uint8Array.from(binary, (char) => char.charCodeAt(0));
}
export { fromBase64 };

function buffer(value: Uint8Array): ArrayBuffer {
  return value.slice().buffer as ArrayBuffer;
}

async function derivedKey(authSecret: string, containerID: string, info: string): Promise<CryptoKey> {
  const root = await crypto.subtle.importKey("raw", buffer(hexBytes(authSecret)), "HKDF", false, ["deriveKey"]);
  return crypto.subtle.deriveKey(
    { name: "HKDF", hash: "SHA-256", salt: buffer(encoder.encode(containerID)), info: buffer(encoder.encode(info)) },
    root, { name: "AES-GCM", length: 256 }, false, ["encrypt", "decrypt"],
  );
}

async function objectKey(authSecret: string, containerID: string) {
  return derivedKey(authSecret, containerID, "kynotes/object/v1");
}

export async function encryptContainerMeta(authSecret: string, containerID: string, name: string): Promise<Uint8Array> {
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const key = await derivedKey(authSecret, containerID, "kynotes/container-meta/v1");
  const ciphertext = await crypto.subtle.encrypt({ name: "AES-GCM", iv: buffer(iv) }, key, buffer(encoder.encode(JSON.stringify({ name }))));
  const result = new Uint8Array(iv.byteLength + ciphertext.byteLength);
  result.set(iv); result.set(new Uint8Array(ciphertext), iv.byteLength);
  return result;
}

export async function decryptContainerMeta(authSecret: string, containerID: string, bytes: Uint8Array): Promise<{ name: string }> {
  if (bytes.byteLength < 13) throw new Error("Encrypted workspace metadata is too short");
  const key = await derivedKey(authSecret, containerID, "kynotes/container-meta/v1");
  const plaintext = await crypto.subtle.decrypt({ name: "AES-GCM", iv: buffer(bytes.slice(0, 12)) }, key, buffer(bytes.slice(12)));
  const result = JSON.parse(decoder.decode(plaintext)) as { name?: unknown };
  if (typeof result.name !== "string") throw new Error("Invalid workspace metadata");
  return { name: result.name };
}

export const encryptComment = (authSecret: string, containerID: string, body: string, section = "") => encryptWithInfo(authSecret, containerID, "kynotes/comment/v1", { body, section });
export async function decryptComment(authSecret: string, containerID: string, bytes: Uint8Array): Promise<{ body: string; section?: string }> { return decryptWithInfo(authSecret, containerID, "kynotes/comment/v1", bytes) as Promise<{ body: string; section?: string }>; }
export const encryptAttachmentMetadata = (authSecret: string, containerID: string, metadata: { name: string; type: string; size: number }) => encryptWithInfo(authSecret, containerID, "kynotes/attachment-meta/v1", metadata);
export async function decryptAttachmentMetadata(authSecret: string, containerID: string, bytes: Uint8Array): Promise<{ name: string; type: string; size: number }> { return decryptWithInfo(authSecret, containerID, "kynotes/attachment-meta/v1", bytes) as Promise<{ name: string; type: string; size: number }>; }
export async function encryptAttachment(authSecret: string, containerID: string, plaintext: Uint8Array): Promise<Uint8Array> {
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const key = await objectKey(authSecret, containerID);
  const ciphertext = await crypto.subtle.encrypt({ name: "AES-GCM", iv: buffer(iv) }, key, buffer(plaintext));
  const result = new Uint8Array(iv.byteLength + ciphertext.byteLength);
  result.set(iv); result.set(new Uint8Array(ciphertext), iv.byteLength);
  return result;
}
export async function decryptAttachment(authSecret: string, containerID: string, bytes: Uint8Array): Promise<Uint8Array> {
  const key = await objectKey(authSecret, containerID);
  return new Uint8Array(await crypto.subtle.decrypt({ name: "AES-GCM", iv: buffer(bytes.slice(0, 12)) }, key, buffer(bytes.slice(12))));
}
async function encryptWithInfo(authSecret: string, containerID: string, info: string, value: unknown): Promise<Uint8Array> { const iv = crypto.getRandomValues(new Uint8Array(12)); const key = await derivedKey(authSecret, containerID, info); const ciphertext = await crypto.subtle.encrypt({ name: "AES-GCM", iv: buffer(iv) }, key, buffer(encoder.encode(JSON.stringify(value)))); const result = new Uint8Array(iv.byteLength + ciphertext.byteLength); result.set(iv); result.set(new Uint8Array(ciphertext), iv.byteLength); return result; }
async function decryptWithInfo(authSecret: string, containerID: string, info: string, bytes: Uint8Array): Promise<unknown> { if (bytes.byteLength < 13) throw new Error("Encrypted comment is too short"); const key = await derivedKey(authSecret, containerID, info); const plaintext = await crypto.subtle.decrypt({ name: "AES-GCM", iv: buffer(bytes.slice(0, 12)) }, key, buffer(bytes.slice(12))); return JSON.parse(decoder.decode(plaintext)); }

export async function deriveAuthSecret(password: string, salt: string, iterations: number): Promise<string> {
  const rawSalt = fromBase64(salt);
  const passwordKey = await crypto.subtle.importKey("raw", buffer(encoder.encode(password)), "PBKDF2", false, ["deriveBits"]);
  const stretched = await crypto.subtle.deriveBits({ name: "PBKDF2", salt: buffer(rawSalt), iterations, hash: "SHA-256" }, passwordKey, 256);
  const root = await crypto.subtle.importKey("raw", stretched, "HKDF", false, ["deriveBits"]);
  const result = await crypto.subtle.deriveBits({ name: "HKDF", hash: "SHA-256", salt: new ArrayBuffer(0), info: buffer(encoder.encode("kynotes/auth/v1")) }, root, 256);
  return [...new Uint8Array(result)].map((byte) => byte.toString(16).padStart(2, "0")).join("");
}

export function randomLoginSalt() { const bytes = crypto.getRandomValues(new Uint8Array(16)); let binary = ""; for (const byte of bytes) binary += String.fromCharCode(byte); return btoa(binary); }

export async function encryptNote(authSecret: string, containerID: string, note: NotePayload): Promise<Uint8Array> {
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const key = await objectKey(authSecret, containerID);
  const ciphertext = await crypto.subtle.encrypt({ name: "AES-GCM", iv: buffer(iv) }, key, buffer(encoder.encode(JSON.stringify(note))));
  const result = new Uint8Array(iv.byteLength + ciphertext.byteLength);
  result.set(iv); result.set(new Uint8Array(ciphertext), iv.byteLength);
  return result;
}

export async function decryptNote(authSecret: string, containerID: string, bytes: Uint8Array): Promise<NotePayload> {
  if (bytes.byteLength < 13) throw new Error("Encrypted note is too short");
  const key = await objectKey(authSecret, containerID);
  const plaintext = await crypto.subtle.decrypt({ name: "AES-GCM", iv: buffer(bytes.slice(0, 12)) }, key, buffer(bytes.slice(12)));
  return JSON.parse(decoder.decode(plaintext)) as NotePayload;
}

export type NotePayload = { title: string; body: string };
export { base64 };

export async function encryptSharePayload(note: NotePayload): Promise<{ ciphertext: Uint8Array; key: string }> {
  const key = await crypto.subtle.generateKey({ name: "AES-GCM", length: 256 }, true, ["encrypt", "decrypt"]);
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const ciphertext = await crypto.subtle.encrypt({ name: "AES-GCM", iv: buffer(iv) }, key, buffer(encoder.encode(JSON.stringify(note))));
  const raw = new Uint8Array(await crypto.subtle.exportKey("raw", key));
  const sealed = new Uint8Array(iv.length + ciphertext.byteLength);
  sealed.set(iv); sealed.set(new Uint8Array(ciphertext), iv.length);
  return { ciphertext: sealed, key: base64url(raw) };
}

export async function decryptSharePayload(bytes: Uint8Array, encodedKey: string): Promise<NotePayload> {
  const keyBytes = base64urlDecode(encodedKey);
  const key = await crypto.subtle.importKey("raw", buffer(keyBytes), "AES-GCM", false, ["decrypt"]);
  const plaintext = await crypto.subtle.decrypt({ name: "AES-GCM", iv: buffer(bytes.slice(0, 12)) }, key, buffer(bytes.slice(12)));
  return JSON.parse(decoder.decode(plaintext)) as NotePayload;
}

function base64url(bytes: Uint8Array): string { return base64(bytes).replaceAll("+", "-").replaceAll("/", "_").replaceAll("=", ""); }
function base64urlDecode(value: string): Uint8Array { return fromBase64(value.replaceAll("-", "+").replaceAll("_", "/") + "=".repeat((4 - value.length % 4) % 4)); }
