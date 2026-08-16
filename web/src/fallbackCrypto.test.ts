import { describe, expect, it } from "vitest";
import {
  aes256GcmDecrypt,
  aes256GcmEncrypt,
  hkdfSha256,
  hmacSha256,
  pbkdf2Sha256,
  sha256,
} from "./fallbackCrypto";

describe("fallback crypto", () => {
  it("computes correct SHA-256 and HMAC-SHA256", () => {
    const data = new TextEncoder().encode("hello world");
    const hash = sha256(data);
    const hashHex = [...hash].map((b) => b.toString(16).padStart(2, "0")).join("");
    expect(hashHex).toBe("b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9");

    const key = new TextEncoder().encode("secret-key");
    const hmac = hmacSha256(key, data);
    const hmacHex = [...hmac].map((b) => b.toString(16).padStart(2, "0")).join("");
    expect(hmacHex).toBe("095d5a21fe6d0646db223fdf3de6436bb8dfb2fab0b51677ecf6441fcf5f2a67");
  });

  it("computes correct PBKDF2-HMAC-SHA256 for KyNotes auth vector", async () => {
    const password = new TextEncoder().encode("correct horse battery staple");
    const salt = Uint8Array.from(atob("MDEyMzQ1Njc4OWFiY2RlZg=="), (c) => c.charCodeAt(0));
    const derived = await pbkdf2Sha256(password, salt, 100000, 32);
    const root = hkdfSha256(derived, 32, new Uint8Array(0), new TextEncoder().encode("kynotes/auth/v1"));
    const authSecret = [...root].map((b) => b.toString(16).padStart(2, "0")).join("");
    expect(authSecret).toBe("b9eb85992f985b432a3feaf4f5ea0b7b7960a5da42c640a3b9d93a83fc5bef1d");
  });

  it("encrypts and decrypts AES-256-GCM correctly with authentication tag", () => {
    const key = new Uint8Array(32);
    for (let i = 0; i < 32; i++) key[i] = i + 1;
    const iv = new Uint8Array(12);
    for (let i = 0; i < 12; i++) iv[i] = i + 10;

    const plaintext = new TextEncoder().encode("Top secret encrypted note payload for KyNotes!");
    const ciphertextAndTag = aes256GcmEncrypt(key, iv, plaintext);

    expect(ciphertextAndTag.length).toBe(plaintext.length + 16);

    const decrypted = aes256GcmDecrypt(key, iv, ciphertextAndTag);
    expect(new TextDecoder().decode(decrypted)).toBe("Top secret encrypted note payload for KyNotes!");

    // Tampered ciphertext fails
    const tampered = new Uint8Array(ciphertextAndTag);
    tampered[0] ^= 1;
    expect(() => aes256GcmDecrypt(key, iv, tampered)).toThrow();

    // Tampered tag fails
    const tamperedTag = new Uint8Array(ciphertextAndTag);
    tamperedTag[tamperedTag.length - 1] ^= 1;
    expect(() => aes256GcmDecrypt(key, iv, tamperedTag)).toThrow();
  });
});
