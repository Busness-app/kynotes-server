import { describe, expect, it } from "vitest";
import { documentText, emptyNoteDocument, parseNoteDocument, stringifyNoteDocument } from "./document";

describe("note document format", () => {
  it("round-trips structured content without Markdown", () => {
    const document = emptyNoteDocument().document;
    document.content = [
      { type: "heading", attrs: { level: 2 }, content: [{ type: "text", text: "Launch" }] },
      { type: "paragraph", content: [{ type: "text", text: "Ship it." }] },
    ];
    const parsed = parseNoteDocument(stringifyNoteDocument(document));
    expect(parsed.document).toEqual(document);
    expect(documentText(stringifyNoteDocument(document))).toBe("Launch Ship it.");
  });

  it("keeps image-only documents addressable", () => {
    const document = { type: "doc", content: [{ type: "image", attrs: { src: "attachment://att_image", alt: "diagram" } }] };
    expect(parseNoteDocument(stringifyNoteDocument(document)).document).toEqual(document);
    expect(documentText(stringifyNoteDocument(document))).toBe("[diagram]");
  });

  it("starts invalid content as an empty document", () => {
    expect(parseNoteDocument("not Markdown").document).toEqual(emptyNoteDocument().document);
  });
});
