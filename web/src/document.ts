import type { PartialBlock } from "@blocknote/core";

export const NOTE_DOCUMENT_FORMAT = "kynotes.blocknote.v1";

export type NoteDocument = {
  format: typeof NOTE_DOCUMENT_FORMAT;
  document: PartialBlock[];
};

export const emptyNoteDocument = (): NoteDocument => ({
  format: NOTE_DOCUMENT_FORMAT,
  document: [{ type: "paragraph", content: "" }],
});

export function parseNoteDocument(body: string): NoteDocument {
  try {
    const value = JSON.parse(body) as Partial<NoteDocument>;
    if (value.format === NOTE_DOCUMENT_FORMAT && Array.isArray(value.document)) {
      return { format: NOTE_DOCUMENT_FORMAT, document: value.document };
    }
  } catch {
    /* Empty or invalid content is treated as a new document. */
  }
  if (!body) return emptyNoteDocument();
  return {
    format: NOTE_DOCUMENT_FORMAT,
    document: [{
      type: "paragraph",
      content: body,
    }],
  };
}

export function stringifyNoteDocument(document: PartialBlock[]): string {
  return JSON.stringify({ format: NOTE_DOCUMENT_FORMAT, document });
}

export function documentText(body: string): string {
  const value = parseNoteDocument(body).document;
  const text: string[] = [];
  const visit = (node: PartialBlock) => {
    if (typeof node.content === "string") text.push(node.content);
    if (Array.isArray(node.content)) {
      node.content.forEach((inline) => {
        if (typeof inline === "object" && inline !== null && "text" in inline && typeof inline.text === "string") text.push(inline.text);
      });
    }
    if (node.type === "image" && typeof node.props?.name === "string") text.push(`[${node.props.name}]`);
    node.children?.forEach(visit);
  };
  value.forEach(visit);
  return text.join(" ");
}

export function isStructuredNoteBody(body: string): boolean {
  try {
    const value = JSON.parse(body) as Partial<NoteDocument>;
    return value.format === NOTE_DOCUMENT_FORMAT && Array.isArray(value.document);
  } catch {
    return false;
  }
}
