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
  if (!body) return emptyNoteDocument();
  return {
    format: NOTE_DOCUMENT_FORMAT,
    document: {
      type: "doc",
      content: [{ type: "paragraph", content: [{ type: "text", text: body }] }],
    },
  };
}

export function stringifyNoteDocument(document: JSONContent): string {
  return JSON.stringify({ format: NOTE_DOCUMENT_FORMAT, document });
}

export async function normalizeNoteBody(body: string): Promise<string> {
  try {
    const value = JSON.parse(body) as Partial<NoteDocument> & JSONContent;
    if ((value.format === NOTE_DOCUMENT_FORMAT && value.document?.type === "doc") || value.type === "doc") {
      return stringifyNoteDocument(parseNoteDocument(body).document);
    }
  } catch {
    /* Legacy content is parsed below. */
  }
  if (!body) return stringifyNoteDocument(emptyNoteDocument().document);
  try {
    const [{ MarkdownManager }, { default: StarterKit }, { default: Image }, { default: Link }] = await Promise.all([
      import("@tiptap/markdown"),
      import("@tiptap/starter-kit"),
      import("@tiptap/extension-image"),
      import("@tiptap/extension-link"),
    ]);
    const document = new MarkdownManager({ extensions: [StarterKit, Image, Link] }).parse(body);
    if (document.type === "doc") return stringifyNoteDocument(document);
  } catch {
    /* Preserve content as plain text if legacy Markdown cannot be parsed. */
  }
  return stringifyNoteDocument(parseNoteDocument(body).document);
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
