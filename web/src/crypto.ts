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

function buffer(value: Uint8Array): ArrayBuffer {
  return value.slice().buffer as ArrayBuffer;
}

async function objectKey(authSecret: string, containerID: string): Promise<CryptoKey> {
  const root = await crypto.subtle.importKey("raw", buffer(hexBytes(authSecret)), "HKDF", false, ["deriveKey"]);
  return crypto.subtle.deriveKey(
    { name: "HKDF", hash: "SHA-256", salt: buffer(encoder.encode(containerID)), info: buffer(encoder.encode("kynotes/object/v1")) },
    root, { name: "AES-GCM", length: 256 }, false, ["encrypt", "decrypt"],
  );
}

export async function deriveAuthSecret(password: string, salt: string, iterations: number): Promise<string> {
  const rawSalt = fromBase64(salt);
  const passwordKey = await crypto.subtle.importKey("raw", buffer(encoder.encode(password)), "PBKDF2", false, ["deriveBits"]);
  const stretched = await crypto.subtle.deriveBits({ name: "PBKDF2", salt: buffer(rawSalt), iterations, hash: "SHA-256" }, passwordKey, 256);
  const root = await crypto.subtle.importKey("raw", stretched, "HKDF", false, ["deriveBits"]);
  const result = await crypto.subtle.deriveBits({ name: "HKDF", hash: "SHA-256", salt: new ArrayBuffer(0), info: buffer(encoder.encode("kynotes/auth/v1")) }, root, 256);
  return [...new Uint8Array(result)].map((byte) => byte.toString(16).padStart(2, "0")).join("");
}

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
