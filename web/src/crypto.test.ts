import { describe, expect, it } from "vitest";
import { decryptNote, deriveAuthSecret, encryptNote } from "./crypto";

describe("browser crypto", () => {
  it("matches the server auth fixture", async () => {
    await expect(deriveAuthSecret("correct horse battery staple", "MDEyMzQ1Njc4OWFiY2RlZg==", 100000)).resolves.toBe(
      "b9eb85992f985b432a3feaf4f5ea0b7b7960a5da42c640a3b9d93a83fc5bef1d",
    );
  });

  it("round-trips encrypted note content", async () => {
    const secret = "b9eb85992f985b432a3feaf4f5ea0b7b7960a5da42c640a3b9d93a83fc5bef1d";
    const note = { title: "Private", body: "Only the browser sees this." };
    const ciphertext = await encryptNote(secret, "cnt_test", note);
    expect(new TextDecoder().decode(ciphertext)).not.toContain(note.body);
    await expect(decryptNote(secret, "cnt_test", ciphertext)).resolves.toEqual(note);
  });
});
