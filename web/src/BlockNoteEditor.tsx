import { BlockNoteView } from "@blocknote/mantine";
import { useCreateBlockNote } from "@blocknote/react";
import type { Block, PartialBlock } from "@blocknote/core";
import { useEffect, useRef } from "react";
import "@blocknote/mantine/style.css";

type Props = {
  noteID: string;
  initialContent: PartialBlock[];
  legacyMarkdown?: string;
  onChange: (document: Block[]) => void;
  uploadFile: (file: File) => Promise<string>;
  resolveFileUrl: (url: string) => Promise<string>;
};

export function BlockNoteEditor({ noteID, initialContent, legacyMarkdown, onChange, uploadFile, resolveFileUrl }: Props) {
  const onChangeRef = useRef(onChange);
  const uploadRef = useRef(uploadFile);
  const resolveRef = useRef(resolveFileUrl);
  onChangeRef.current = onChange;
  uploadRef.current = uploadFile;
  resolveRef.current = resolveFileUrl;
  const editor = useCreateBlockNote({
    initialContent: legacyMarkdown ? undefined : initialContent,
    uploadFile: (file) => uploadRef.current(file),
    resolveFileUrl: (url) => resolveRef.current(url),
  }, [noteID]);

  useEffect(() => {
    if (!legacyMarkdown) return;
    const blocks = editor.tryParseMarkdownToBlocks(legacyMarkdown);
    if (blocks.length) editor.replaceBlocks(editor.document, blocks);
  }, [editor, legacyMarkdown]);

  return (
    <div className="blocknote-editor">
      <BlockNoteView editor={editor} onChange={(current) => onChangeRef.current(current.document)} />
    </div>
  );
}
