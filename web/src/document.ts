import type { JSONContent } from "@tiptap/core";
import { MarkdownManager } from "@tiptap/markdown";
import StarterKit from "@tiptap/starter-kit";
import Image from "@tiptap/extension-image";
import Link from "@tiptap/extension-link";

export const NOTE_DOCUMENT_FORMAT = "kynotes.tiptap.v1";
const markdownManager = new MarkdownManager({ extensions: [StarterKit, Image, Link] });

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
    const document = markdownManager.parse(body);
    if (document.type === "doc") return stringifyNoteDocument(document);
  } catch (error) {
    throw new Error(`Unable to convert saved note: ${error instanceof Error ? error.message : "invalid document"}`);
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
