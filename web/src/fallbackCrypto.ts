// Pure TypeScript / JavaScript cryptographic fallback for environments
// where window.crypto.subtle is undefined (e.g. non-secure contexts over HTTP/IP).

const SBOX = new Uint8Array([
  0x63, 0x7c, 0x77, 0x7b, 0xf2, 0x6b, 0x6f, 0xc5, 0x30, 0x01, 0x67, 0x2b, 0xfe, 0xd7, 0xab, 0x76,
  0xca, 0x82, 0xc9, 0x7d, 0xfa, 0x59, 0x47, 0xf0, 0xad, 0xd4, 0xa2, 0xaf, 0x9c, 0xa4, 0x72, 0xc0,
  0xb7, 0xfd, 0x93, 0x26, 0x36, 0x3f, 0xf7, 0xcc, 0x34, 0xa5, 0xe5, 0xf1, 0x71, 0xd8, 0x31, 0x15,
  0x04, 0xc7, 0x23, 0xc3, 0x18, 0x96, 0x05, 0x9a, 0x07, 0x12, 0x80, 0xe2, 0xeb, 0x27, 0xb2, 0x75,
  0x09, 0x83, 0x2c, 0x1a, 0x1b, 0x6e, 0x5a, 0xa0, 0x52, 0x3b, 0xd6, 0xb3, 0x29, 0xe3, 0x2f, 0x84,
  0x53, 0xd1, 0x00, 0xed, 0x20, 0xfc, 0xb1, 0x5b, 0x6a, 0xcb, 0xbe, 0x39, 0x4a, 0x4c, 0x58, 0xcf,
  0xd0, 0xef, 0xaa, 0xfb, 0x43, 0x4d, 0x33, 0x85, 0x45, 0xf9, 0x02, 0x7f, 0x50, 0x3c, 0x9f, 0xa8,
  0x51, 0xa3, 0x40, 0x8f, 0x92, 0x9d, 0x38, 0xf5, 0xbc, 0xb6, 0xda, 0x21, 0x10, 0xff, 0xf3, 0xd2,
  0xcd, 0x0c, 0x13, 0xec, 0x5f, 0x97, 0x44, 0x17, 0xc4, 0xa7, 0x7e, 0x3d, 0x64, 0x5d, 0x19, 0x73,
  0x60, 0x81, 0x4f, 0xdc, 0x22, 0x2a, 0x90, 0x88, 0x46, 0xee, 0xb8, 0x14, 0xde, 0x5e, 0x0b, 0xdb,
  0xe0, 0x32, 0x3a, 0x0a, 0x49, 0x06, 0x24, 0x5c, 0xc2, 0xd3, 0xac, 0x62, 0x91, 0x95, 0xe4, 0x79,
  0xe7, 0xc8, 0x37, 0x6d, 0x8d, 0xd5, 0x4e, 0xa9, 0x6c, 0x56, 0xf4, 0xea, 0x65, 0x7a, 0xae, 0x08,
  0xba, 0x78, 0x25, 0x2e, 0x1c, 0xa6, 0xb4, 0xc6, 0xe8, 0xdd, 0x74, 0x1f, 0x4b, 0xbd, 0x8b, 0x8a,
  0x70, 0x3e, 0xb5, 0x66, 0x48, 0x03, 0xf6, 0x0e, 0x61, 0x35, 0x57, 0xb9, 0x86, 0xc1, 0x1d, 0x9e,
  0xe1, 0xf8, 0x98, 0x11, 0x69, 0xd9, 0x8e, 0x94, 0x9b, 0x1e, 0x87, 0xe9, 0xce, 0x55, 0x28, 0xdf,
  0x8c, 0xa1, 0x89, 0x0d, 0xbf, 0xe6, 0x42, 0x68, 0x41, 0x99, 0x2d, 0x0f, 0xb0, 0x54, 0xbb, 0x16,
]);

const RCON = new Uint32Array([
  0x00000000, 0x01000000, 0x02000000, 0x04000000, 0x08000000,
  0x10000000, 0x20000000, 0x40000000, 0x80000000, 0x1b000000, 0x36000000,
]);

const SHA256_K = new Uint32Array([
  0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
  0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
  0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
  0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
  0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
  0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
  0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
  0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
]);

function rotr(x: number, n: number): number {
  return (x >>> n) | (x << (32 - n));
}

export function sha256(data: Uint8Array): Uint8Array {
  let h0 = 0x6a09e667, h1 = 0xbb67ae85, h2 = 0x3c6ef372, h3 = 0xa54ff53a;
  let h4 = 0x510e527f, h5 = 0x9b05688c, h6 = 0x1f83d9ab, h7 = 0x5be0cd19;

  const len = data.length;
  const bitLenHi = Math.floor(len / 0x20000000);
  const bitLenLo = (len * 8) >>> 0;

  const padLen = (len % 64 < 56) ? (56 - (len % 64)) : (120 - (len % 64));
  const totalLen = len + padLen + 8;
  const buf = new Uint8Array(totalLen);
  buf.set(data);
  buf[len] = 0x80;

  const view = new DataView(buf.buffer, buf.byteOffset, buf.byteLength);
  view.setUint32(totalLen - 8, bitLenHi, false);
  view.setUint32(totalLen - 4, bitLenLo, false);

  const w = new Uint32Array(64);

  for (let i = 0; i < totalLen; i += 64) {
    for (let j = 0; j < 16; j++) {
      w[j] = view.getUint32(i + j * 4, false);
    }
    for (let j = 16; j < 64; j++) {
      const s0 = rotr(w[j - 15], 7) ^ rotr(w[j - 15], 18) ^ (w[j - 15] >>> 3);
      const s1 = rotr(w[j - 2], 17) ^ rotr(w[j - 2], 19) ^ (w[j - 2] >>> 10);
      w[j] = (w[j - 16] + s0 + w[j - 7] + s1) >>> 0;
    }

    let a = h0, b = h1, c = h2, d = h3, e = h4, f = h5, g = h6, h = h7;

    for (let j = 0; j < 64; j++) {
      const S1 = rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25);
      const ch = (e & f) ^ (~e & g);
      const temp1 = (h + S1 + ch + SHA256_K[j] + w[j]) >>> 0;
      const S0 = rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22);
      const maj = (a & b) ^ (a & c) ^ (b & c);
      const temp2 = (S0 + maj) >>> 0;

      h = g;
      g = f;
      f = e;
      e = (d + temp1) >>> 0;
      d = c;
      c = b;
      b = a;
      a = (temp1 + temp2) >>> 0;
    }

    h0 = (h0 + a) >>> 0;
    h1 = (h1 + b) >>> 0;
    h2 = (h2 + c) >>> 0;
    h3 = (h3 + d) >>> 0;
    h4 = (h4 + e) >>> 0;
    h5 = (h5 + f) >>> 0;
    h6 = (h6 + g) >>> 0;
    h7 = (h7 + h) >>> 0;
  }

  const out = new Uint8Array(32);
  const outView = new DataView(out.buffer);
  outView.setUint32(0, h0, false);
  outView.setUint32(4, h1, false);
  outView.setUint32(8, h2, false);
  outView.setUint32(12, h3, false);
  outView.setUint32(16, h4, false);
  outView.setUint32(20, h5, false);
  outView.setUint32(24, h6, false);
  outView.setUint32(28, h7, false);
  return out;
}

export function hmacSha256(key: Uint8Array, data: Uint8Array): Uint8Array {
  let k = key;
  if (k.length > 64) {
    k = sha256(k);
  }
  const kPad = new Uint8Array(64);
  kPad.set(k);

  const oKeyPad = new Uint8Array(64);
  const iKeyPad = new Uint8Array(64);
  for (let i = 0; i < 64; i++) {
    oKeyPad[i] = kPad[i] ^ 0x5c;
    iKeyPad[i] = kPad[i] ^ 0x36;
  }

  const innerBuf = new Uint8Array(64 + data.length);
  innerBuf.set(iKeyPad, 0);
  innerBuf.set(data, 64);
  const innerHash = sha256(innerBuf);

  const outerBuf = new Uint8Array(64 + 32);
  outerBuf.set(oKeyPad, 0);
  outerBuf.set(innerHash, 64);
  return sha256(outerBuf);
}

export async function pbkdf2Sha256(
  password: Uint8Array,
  salt: Uint8Array,
  iterations: number,
  keyLen: number,
): Promise<Uint8Array> {
  const numBlocks = Math.ceil(keyLen / 32);
  const out = new Uint8Array(numBlocks * 32);

  for (let block = 1; block <= numBlocks; block++) {
    const saltBlock = new Uint8Array(salt.length + 4);
    saltBlock.set(salt, 0);
    saltBlock[salt.length] = (block >>> 24) & 0xff;
    saltBlock[salt.length + 1] = (block >>> 16) & 0xff;
    saltBlock[salt.length + 2] = (block >>> 8) & 0xff;
    saltBlock[salt.length + 3] = block & 0xff;

    let u = hmacSha256(password, saltBlock);
    const t = new Uint8Array(u);

    // Yield to event loop every 20,000 iterations to avoid freezing UI
    for (let iter = 1; iter < iterations; iter++) {
      u = hmacSha256(password, u);
      for (let j = 0; j < 32; j++) {
        t[j] ^= u[j];
      }
      if (iter % 25000 === 0) {
        await new Promise((resolve) => setTimeout(resolve, 0));
      }
    }
    out.set(t, (block - 1) * 32);
  }

  return out.slice(0, keyLen);
}

export function hkdfSha256(
  ikm: Uint8Array,
  length: number,
  salt: Uint8Array = new Uint8Array(0),
  info: Uint8Array = new Uint8Array(0),
): Uint8Array {
  const actualSalt = salt.length === 0 ? new Uint8Array(32) : salt;
  const prk = hmacSha256(actualSalt, ikm);

  const numBlocks = Math.ceil(length / 32);
  const okm = new Uint8Array(numBlocks * 32);
  let prevT: Uint8Array<ArrayBuffer> = new Uint8Array(0);

  for (let i = 1; i <= numBlocks; i++) {
    const buf = new Uint8Array(prevT.length + info.length + 1);
    buf.set(prevT, 0);
    buf.set(info, prevT.length);
    buf[buf.length - 1] = i;
    prevT = hmacSha256(prk, buf) as Uint8Array<ArrayBuffer>;
    okm.set(prevT, (i - 1) * 32);
  }

  return okm.slice(0, length);
}

// AES-256 Key Expansion
function expandAes256Key(key: Uint8Array): Uint32Array {
  const w = new Uint32Array(60);
  for (let i = 0; i < 8; i++) {
    w[i] = (key[4 * i] << 24) | (key[4 * i + 1] << 16) | (key[4 * i + 2] << 8) | key[4 * i + 3];
  }
  for (let i = 8; i < 60; i++) {
    let temp = w[i - 1];
    if (i % 8 === 0) {
      temp =
        (SBOX[(temp >>> 16) & 0xff] << 24) |
        (SBOX[(temp >>> 8) & 0xff] << 16) |
        (SBOX[temp & 0xff] << 8) |
        SBOX[(temp >>> 24) & 0xff];
      temp ^= RCON[Math.floor(i / 8)];
    } else if (i % 8 === 4) {
      temp =
        (SBOX[(temp >>> 24) & 0xff] << 24) |
        (SBOX[(temp >>> 16) & 0xff] << 16) |
        (SBOX[(temp >>> 8) & 0xff] << 8) |
        SBOX[temp & 0xff];
    }
    w[i] = (w[i - 8] ^ temp) >>> 0;
  }
  return w;
}

function subWord(w: number): number {
  return (
    (SBOX[(w >>> 24) & 0xff] << 24) |
    (SBOX[(w >>> 16) & 0xff] << 16) |
    (SBOX[(w >>> 8) & 0xff] << 8) |
    SBOX[w & 0xff]
  );
}

function gmul(a: number, b: number): number {
  let p = 0;
  for (let i = 0; i < 8; i++) {
    if (b & 1) p ^= a;
    const hi = a & 0x80;
    a = (a << 1) & 0xff;
    if (hi) a ^= 0x1b;
    b >>>= 1;
  }
  return p;
}

function mixColumn(c: number): number {
  const a0 = (c >>> 24) & 0xff, a1 = (c >>> 16) & 0xff, a2 = (c >>> 8) & 0xff, a3 = c & 0xff;
  const r0 = gmul(a0, 2) ^ gmul(a1, 3) ^ a2 ^ a3;
  const r1 = a0 ^ gmul(a1, 2) ^ gmul(a2, 3) ^ a3;
  const r2 = a0 ^ a1 ^ gmul(a2, 2) ^ gmul(a3, 3);
  const r3 = gmul(a0, 3) ^ a1 ^ a2 ^ gmul(a3, 2);
  return (r0 << 24) | (r1 << 16) | (r2 << 8) | r3;
}

function aesEncryptBlock(w: Uint32Array, block: Uint8Array, out: Uint8Array, outOffset: number = 0) {
  let s0 = (block[0] << 24) | (block[1] << 16) | (block[2] << 8) | block[3];
  let s1 = (block[4] << 24) | (block[5] << 16) | (block[6] << 8) | block[7];
  let s2 = (block[8] << 24) | (block[9] << 16) | (block[10] << 8) | block[11];
  let s3 = (block[12] << 24) | (block[13] << 16) | (block[14] << 8) | block[15];

  s0 ^= w[0]; s1 ^= w[1]; s2 ^= w[2]; s3 ^= w[3];

  for (let round = 1; round <= 13; round++) {
    const t0 = subWord((s0 & 0xff000000) | (s1 & 0x00ff0000) | (s2 & 0x0000ff00) | (s3 & 0x000000ff));
    const t1 = subWord((s1 & 0xff000000) | (s2 & 0x00ff0000) | (s3 & 0x0000ff00) | (s0 & 0x000000ff));
    const t2 = subWord((s2 & 0xff000000) | (s3 & 0x00ff0000) | (s0 & 0x0000ff00) | (s1 & 0x000000ff));
    const t3 = subWord((s3 & 0xff000000) | (s0 & 0x00ff0000) | (s1 & 0x0000ff00) | (s2 & 0x000000ff));

    s0 = (mixColumn(t0) ^ w[round * 4]) >>> 0;
    s1 = (mixColumn(t1) ^ w[round * 4 + 1]) >>> 0;
    s2 = (mixColumn(t2) ^ w[round * 4 + 2]) >>> 0;
    s3 = (mixColumn(t3) ^ w[round * 4 + 3]) >>> 0;
  }

  // Round 14 (no MixColumns)
  const t0 = subWord((s0 & 0xff000000) | (s1 & 0x00ff0000) | (s2 & 0x0000ff00) | (s3 & 0x000000ff)) ^ w[56];
  const t1 = subWord((s1 & 0xff000000) | (s2 & 0x00ff0000) | (s3 & 0x0000ff00) | (s0 & 0x000000ff)) ^ w[57];
  const t2 = subWord((s2 & 0xff000000) | (s3 & 0x00ff0000) | (s0 & 0x0000ff00) | (s1 & 0x000000ff)) ^ w[58];
  const t3 = subWord((s3 & 0xff000000) | (s0 & 0x00ff0000) | (s1 & 0x0000ff00) | (s2 & 0x000000ff)) ^ w[59];

  out[outOffset] = (t0 >>> 24) & 0xff; out[outOffset + 1] = (t0 >>> 16) & 0xff;
  out[outOffset + 2] = (t0 >>> 8) & 0xff; out[outOffset + 3] = t0 & 0xff;

  out[outOffset + 4] = (t1 >>> 24) & 0xff; out[outOffset + 5] = (t1 >>> 16) & 0xff;
  out[outOffset + 6] = (t1 >>> 8) & 0xff; out[outOffset + 7] = t1 & 0xff;

  out[outOffset + 8] = (t2 >>> 24) & 0xff; out[outOffset + 9] = (t2 >>> 16) & 0xff;
  out[outOffset + 10] = (t2 >>> 8) & 0xff; out[outOffset + 11] = t2 & 0xff;

  out[outOffset + 12] = (t3 >>> 24) & 0xff; out[outOffset + 13] = (t3 >>> 16) & 0xff;
  out[outOffset + 14] = (t3 >>> 8) & 0xff; out[outOffset + 15] = t3 & 0xff;
}

// GF(2^128) multiplication for GHASH (NIST SP 800-38D)
function ghashMul(x: Uint8Array, y: Uint8Array): Uint8Array {
  const z = new Uint8Array(16);
  const v = new Uint8Array(y);

  for (let i = 0; i < 128; i++) {
    const byteIdx = Math.floor(i / 8);
    const bitIdx = 7 - (i % 8);
    if ((x[byteIdx] >>> bitIdx) & 1) {
      for (let j = 0; j < 16; j++) z[j] ^= v[j];
    }
    const lsb = v[15] & 1;
    for (let j = 15; j > 0; j--) {
      v[j] = (v[j] >>> 1) | ((v[j - 1] & 1) << 7);
    }
    v[0] >>>= 1;
    if (lsb) {
      v[0] ^= 0xe1;
    }
  }
  return z;
}

function ghash(h: Uint8Array, aad: Uint8Array, ciphertext: Uint8Array): Uint8Array {
  const y = new Uint8Array(16);

  const processData = (data: Uint8Array) => {
    for (let i = 0; i < data.length; i += 16) {
      const block = new Uint8Array(16);
      const slice = data.slice(i, Math.min(i + 16, data.length));
      block.set(slice);
      for (let j = 0; j < 16; j++) y[j] ^= block[j];
      const res = ghashMul(y, h);
      y.set(res);
    }
  };

  processData(aad);
  processData(ciphertext);

  const lenBlock = new Uint8Array(16);
  const lenView = new DataView(lenBlock.buffer);
  const aadBits = aad.length * 8;
  const cipherBits = ciphertext.length * 8;
  lenView.setUint32(0, Math.floor(aadBits / 0x100000000), false);
  lenView.setUint32(4, aadBits >>> 0, false);
  lenView.setUint32(8, Math.floor(cipherBits / 0x100000000), false);
  lenView.setUint32(12, cipherBits >>> 0, false);

  for (let j = 0; j < 16; j++) y[j] ^= lenBlock[j];
  return ghashMul(y, h);
}

export function aes256GcmEncrypt(
  key: Uint8Array,
  iv: Uint8Array,
  plaintext: Uint8Array,
  additionalData: Uint8Array = new Uint8Array(0),
): Uint8Array {
  if (key.length !== 32) throw new Error("AES-256 requires 32-byte key");
  if (iv.length !== 12) throw new Error("AES-GCM requires 12-byte IV");

  const w = expandAes256Key(key);

  const h = new Uint8Array(16);
  aesEncryptBlock(w, new Uint8Array(16), h);

  const j0 = new Uint8Array(16);
  j0.set(iv);
  j0[15] = 1;

  const ciphertext = new Uint8Array(plaintext.length);
  const counter = new Uint8Array(j0);
  const keyStream = new Uint8Array(16);

  for (let i = 0; i < plaintext.length; i += 16) {
    // Increment 32-bit counter
    for (let c = 15; c >= 12; c--) {
      counter[c] = (counter[c] + 1) & 0xff;
      if (counter[c] !== 0) break;
    }
    aesEncryptBlock(w, counter, keyStream);
    const blockLen = Math.min(16, plaintext.length - i);
    for (let j = 0; j < blockLen; j++) {
      ciphertext[i + j] = plaintext[i + j] ^ keyStream[j];
    }
  }

  const s = ghash(h, additionalData, ciphertext);
  const tagMask = new Uint8Array(16);
  aesEncryptBlock(w, j0, tagMask);

  const tag = new Uint8Array(16);
  for (let i = 0; i < 16; i++) tag[i] = s[i] ^ tagMask[i];

  const result = new Uint8Array(ciphertext.length + 16);
  result.set(ciphertext);
  result.set(tag, ciphertext.length);
  return result;
}

export function aes256GcmDecrypt(
  key: Uint8Array,
  iv: Uint8Array,
  ciphertextAndTag: Uint8Array,
  additionalData: Uint8Array = new Uint8Array(0),
): Uint8Array {
  if (key.length !== 32) throw new Error("AES-256 requires 32-byte key");
  if (iv.length !== 12) throw new Error("AES-GCM requires 12-byte IV");
  if (ciphertextAndTag.length < 16) throw new Error("Ciphertext too short");

  const ciphertext = ciphertextAndTag.slice(0, ciphertextAndTag.length - 16);
  const tag = ciphertextAndTag.slice(ciphertextAndTag.length - 16);

  const w = expandAes256Key(key);
  const h = new Uint8Array(16);
  aesEncryptBlock(w, new Uint8Array(16), h);

  const j0 = new Uint8Array(16);
  j0.set(iv);
  j0[15] = 1;

  const s = ghash(h, additionalData, ciphertext);
  const tagMask = new Uint8Array(16);
  aesEncryptBlock(w, j0, tagMask);

  let diff = 0;
  for (let i = 0; i < 16; i++) {
    diff |= tag[i] ^ (s[i] ^ tagMask[i]);
  }
  if (diff !== 0) {
    throw new Error("Authentication tag verification failed");
  }

  const plaintext = new Uint8Array(ciphertext.length);
  const counter = new Uint8Array(j0);
  const keyStream = new Uint8Array(16);

  for (let i = 0; i < ciphertext.length; i += 16) {
    for (let c = 15; c >= 12; c--) {
      counter[c] = (counter[c] + 1) & 0xff;
      if (counter[c] !== 0) break;
    }
    aesEncryptBlock(w, counter, keyStream);
    const blockLen = Math.min(16, ciphertext.length - i);
    for (let j = 0; j < blockLen; j++) {
      plaintext[i + j] = ciphertext[i + j] ^ keyStream[j];
    }
  }

  return plaintext;
}
