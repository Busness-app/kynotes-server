import type { JSONContent } from "@tiptap/core";

export const NOTE_DOCUMENT_FORMAT = "kynotes.tiptap.v1";

export type NoteDocument = {
  format: typeof NOTE_DOCUMENT_FORMAT;
  document: JSONContent;
};

export const emptyNoteDocument = (): NoteDocument => ({
  format: NOTE_DOCUMENT_FORMAT,
  document: { type: "doc", content: [{ type: "paragraph" }] },
});

export function parseNoteDocument(body: string): NoteDocument {
  try {
    const value = JSON.parse(body) as Partial<NoteDocument>;
    if (value.format === NOTE_DOCUMENT_FORMAT && value.document?.type === "doc") {
      return { format: NOTE_DOCUMENT_FORMAT, document: value.document };
    }
    if ((value as JSONContent).type === "doc") {
      return { format: NOTE_DOCUMENT_FORMAT, document: value as JSONContent };
    }
  } catch {
    /* Empty or invalid content is treated as a new document. */
  }
  return emptyNoteDocument();
}

export function stringifyNoteDocument(document: JSONContent): string {
  return JSON.stringify({ format: NOTE_DOCUMENT_FORMAT, document });
}

export function documentText(body: string): string {
  const value = parseNoteDocument(body).document;
  const text: string[] = [];
  const visit = (node: JSONContent) => {
    if (typeof node.text === "string") text.push(node.text);
    if (node.type === "image" && typeof node.attrs?.alt === "string") text.push(`[${node.attrs.alt}]`);
    node.content?.forEach(visit);
  };
  visit(value);
  return text.join(" ");
}
