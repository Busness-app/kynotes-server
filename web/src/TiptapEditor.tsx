import { EditorContent, useEditor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Image from "@tiptap/extension-image";
import Link from "@tiptap/extension-link";
import { useEffect, useRef } from "react";
import type { JSONContent } from "@tiptap/core";

export type EditorActions = {
  toggleBold: () => void;
  toggleItalic: () => void;
  toggleHeading: () => void;
  toggleList: () => void;
  toggleCode: () => void;
  insertImage: (source: string, alt: string) => void;
};

type Props = {
  value: JSONContent;
  onChange: (value: JSONContent) => void;
  imageSources?: Record<string, string>;
  onReady?: (actions: EditorActions) => void;
};

function resolveImages(root: HTMLDivElement | null, sources: Record<string, string>) {
  root?.querySelectorAll<HTMLImageElement>("img").forEach((image) => {
    const source = image.getAttribute("src");
    const resolved = source ? sources[source] : undefined;
    if (resolved && image.src !== resolved) image.src = resolved;
  });
}

export function TiptapEditor({ value, onChange, imageSources = {}, onReady }: Props) {
  const root = useRef<HTMLDivElement>(null);
  const sourcesRef = useRef(imageSources);
  sourcesRef.current = imageSources;
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;
  const editor = useEditor({
    extensions: [
      StarterKit,
      Image.configure({ allowBase64: false, inline: false }),
      Link.configure({ openOnClick: false, autolink: true, linkOnPaste: true }),
    ],
    content: value,
    onUpdate: ({ editor: current }) => onChangeRef.current(current.getJSON()),
    editorProps: { attributes: { class: "tiptap-content" } },
  });

  useEffect(() => {
    if (!editor) return;
    const actions: EditorActions = {
      toggleBold: () => editor.chain().focus().toggleBold().run(),
      toggleItalic: () => editor.chain().focus().toggleItalic().run(),
      toggleHeading: () => editor.chain().focus().toggleHeading({ level: 2 }).run(),
      toggleList: () => editor.chain().focus().toggleBulletList().run(),
      toggleCode: () => editor.chain().focus().toggleCode().run(),
      insertImage: (source, alt) => editor.chain().focus().setImage({ src: source, alt }).run(),
    };
    onReady?.(actions);
    const observer = new MutationObserver(() => resolveImages(root.current, sourcesRef.current));
    if (root.current) observer.observe(root.current, { childList: true, subtree: true, attributes: true, attributeFilter: ["src"] });
    resolveImages(root.current, sourcesRef.current);
    return () => {
      observer.disconnect();
      onReady?.({ toggleBold: () => {}, toggleItalic: () => {}, toggleHeading: () => {}, toggleList: () => {}, toggleCode: () => {}, insertImage: () => {} });
    };
  }, [editor, onReady]);

  useEffect(() => {
    if (!editor || editor.isDestroyed) return;
    if (JSON.stringify(editor.getJSON()) !== JSON.stringify(value)) {
      editor.commands.setContent(value, { emitUpdate: false });
    }
  }, [editor, value]);

  useEffect(() => resolveImages(root.current, imageSources), [imageSources]);

  return <div ref={root} className="tiptap-editor"><EditorContent editor={editor} /></div>;
}
