import type { PartialBlock } from "@blocknote/core";

export const NOTE_DOCUMENT_FORMAT = "kynotes.blocknote.v1";

export type NoteDocument = {
  format: typeof NOTE_DOCUMENT_FORMAT;
  document: PartialBlock[];
};

type LegacyNode = {
  type?: string;
  text?: string;
  attrs?: Record<string, unknown>;
  marks?: Array<{ type?: string; attrs?: Record<string, unknown> }>;
  content?: LegacyNode[];
};

function legacyInline(nodes: LegacyNode[] = []): unknown[] {
  return nodes.flatMap((node) => {
    if (node.type === "hardBreak") return [{ type: "text", text: "\n" }];
    if (node.type !== "text" || typeof node.text !== "string") return legacyInline(node.content);
    const styles: Record<string, unknown> = {};
    let link: { href: string; content: unknown[] } | undefined;
    for (const mark of node.marks ?? []) {
      if (mark.type === "bold") styles.bold = true;
      if (mark.type === "italic") styles.italic = true;
      if (mark.type === "strike") styles.strike = true;
      if (mark.type === "code") styles.code = true;
      if (mark.type === "underline") styles.underline = true;
      if (mark.type === "link" && typeof mark.attrs?.href === "string") {
        link = { href: mark.attrs.href, content: [{ type: "text", text: node.text, styles }] };
      }
    }
    if (link) return [{ type: "link", ...link }];
    return [{ type: "text", text: node.text, styles }];
  });
}

function legacyBlocks(nodes: LegacyNode[] = []): PartialBlock[] {
  return nodes.flatMap((node): PartialBlock[] => {
    const content = legacyInline(node.content);
    switch (node.type) {
      case "paragraph": return [{ type: "paragraph", content: content as never }];
      case "heading": return [{ type: "heading", props: { level: Number(node.attrs?.level) || 1 }, content: content as never }];
      case "bulletList": return legacyList(node.content, "bulletListItem");
      case "orderedList": return legacyList(node.content, "numberedListItem");
      case "blockquote": return [{ type: "quote", children: legacyBlocks(node.content) }];
      case "codeBlock": return [{ type: "codeBlock", props: { language: typeof node.attrs?.language === "string" ? node.attrs.language : "" }, content: node.content?.map((child) => child.text ?? "").join("") ?? "" }];
      case "image": return [{ type: "image", props: { url: typeof node.attrs?.src === "string" ? node.attrs.src : "", name: typeof node.attrs?.alt === "string" ? node.attrs.alt : "" } }];
      case "horizontalRule": return [{ type: "divider" }];
      default: return node.content ? legacyBlocks(node.content) : [];
    }
  });
}

function legacyList(nodes: LegacyNode[] = [], type: "bulletListItem" | "numberedListItem"): PartialBlock[] {
  return nodes.filter((node) => node.type === "listItem").map((node) => {
    const blocks = legacyBlocks(node.content);
    const first = blocks[0] ?? { type: "paragraph", content: "" };
    return { ...first, type, children: blocks.slice(1) } as PartialBlock;
  });
}

function legacyTiptapDocument(body: string): PartialBlock[] | undefined {
  try {
    const value = JSON.parse(body) as { format?: string; document?: LegacyNode } & LegacyNode;
    const document = value.format === "kynotes.tiptap.v1" ? value.document : value;
    if (document?.type !== "doc") return undefined;
    return legacyBlocks(document.content);
  } catch {
    return undefined;
  }
}

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
  const legacy = legacyTiptapDocument(body);
  if (legacy) return { format: NOTE_DOCUMENT_FORMAT, document: legacy.length ? legacy : emptyNoteDocument().document };
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
    if (value.format === NOTE_DOCUMENT_FORMAT && Array.isArray(value.document)) return true;
  } catch {
    /* Try the legacy Tiptap envelope below. */
  }
  return legacyTiptapDocument(body) !== undefined;
}
