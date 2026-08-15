import { describe, expect, it } from "vitest";
import { documentText, emptyNoteDocument, normalizeNoteBody, parseNoteDocument, stringifyNoteDocument } from "./document";

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

  it("does not silently hide an unrecognized body", () => {
    expect(documentText("not Markdown")).toBe("not Markdown");
  });

  it("imports legacy Markdown formatting and attachment images", async () => {
    const body = await normalizeNoteBody("# Heading\n\n**Bold**\n\n![diagram](attachment://att_image)");
    const document = parseNoteDocument(body).document;
    expect(document.content?.map((node) => node.type)).toEqual(["heading", "paragraph", "image"]);
    expect(document.content?.[0]?.content?.[0]?.text).toBe("Heading");
    expect(document.content?.[1]?.content?.[0]?.marks).toEqual([{ type: "bold" }]);
    expect(document.content?.[2]?.attrs?.src).toBe("attachment://att_image");
  });
});
