import { describe, expect, it } from "vitest";
import type { PartialBlock } from "@blocknote/core";
import { documentText, parseNoteDocument, stringifyNoteDocument } from "./document";

describe("note document format", () => {
  it("round-trips structured content without Markdown", () => {
    const document: PartialBlock[] = [
      { type: "heading", props: { level: 2 }, content: "Launch" },
      { type: "paragraph", content: "Ship it." },
    ];
    const parsed = parseNoteDocument(stringifyNoteDocument(document));
    expect(parsed.document).toEqual(document);
    expect(documentText(stringifyNoteDocument(document))).toBe("Launch Ship it.");
  });

  it("keeps image-only documents addressable", () => {
    const document: PartialBlock[] = [{ type: "image", props: { url: "attachment://att_image", name: "diagram" } }];
    expect(parseNoteDocument(stringifyNoteDocument(document)).document).toEqual(document);
    expect(documentText(stringifyNoteDocument(document))).toBe("[diagram]");
  });

  it("round-trips inline formatting marks", () => {
    const document: PartialBlock[] = [{
      type: "paragraph",
      content: [{ type: "text", text: "Important", styles: { bold: true, italic: true } }],
    }];
    const serialized = stringifyNoteDocument(document);
    expect(parseNoteDocument(serialized).document).toEqual(document);
  });

  it("does not silently hide an unrecognized body", () => {
    expect(documentText("not Markdown")).toBe("not Markdown");
  });

  it("keeps legacy content visible until the editor imports it", () => {
    expect(documentText("# Heading\n\n**Bold**")).toContain("# Heading");
  });
});
