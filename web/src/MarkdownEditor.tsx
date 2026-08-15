import { defaultValueCtx, Editor, editorViewCtx, rootCtx } from "@milkdown/kit/core";
import { clipboard } from "@milkdown/kit/plugin/clipboard";
import { history } from "@milkdown/kit/plugin/history";
import { listener, listenerCtx } from "@milkdown/kit/plugin/listener";
import { trailing } from "@milkdown/kit/plugin/trailing";
import { commonmark } from "@milkdown/kit/preset/commonmark";
import { gfm } from "@milkdown/kit/preset/gfm";
import { useEffect, useRef } from "react";

type MarkdownEditorProps = {
  value: string;
  onChange: (value: string) => void;
  imageSources?: Record<string, string>;
  onReady?: (insert: (value: string) => void) => void;
  onImageReady?: (insert: (source: string, alt: string) => void) => void;
};

export function MarkdownEditor({ value, onChange, imageSources = {}, onReady, onImageReady }: MarkdownEditorProps) {
  const root = useRef<HTMLDivElement>(null);
  const onChangeRef = useRef(onChange);
  const imageSourcesRef = useRef(imageSources);
  onChangeRef.current = onChange;
  imageSourcesRef.current = imageSources;

  function resolveImages() {
    root.current?.querySelectorAll<HTMLImageElement>("img").forEach((image) => {
      const source = image.getAttribute("src");
      const resolved = source ? imageSourcesRef.current[source] : undefined;
      if (resolved && image.src !== resolved) image.src = resolved;
    });
  }

  useEffect(() => {
    if (!root.current) return;
    const editor = Editor.make()
      .config((ctx) => {
        ctx.set(rootCtx, root.current!);
        ctx.set(defaultValueCtx, value);
      })
      .use(commonmark)
      .use(gfm)
      .use(listener)
      .use(history)
      .use(trailing)
      .use(clipboard)
      .config((ctx) => {
        ctx.get(listenerCtx).markdownUpdated((_ctx, markdown) => onChangeRef.current(markdown));
      });
    void editor.create();
    const insert = (markdown: string) => {
      editor.action((ctx) => {
        const view = ctx.get(editorViewCtx);
        const { from, to } = view.state.selection;
        view.dispatch(view.state.tr.insertText(markdown, from, to));
        view.focus();
      });
    };
    const insertImage = (source: string, alt: string) => {
      editor.action((ctx) => {
        const view = ctx.get(editorViewCtx);
        const image = view.state.schema.nodes.image;
        if (!image) return;
        view.dispatch(view.state.tr.replaceSelectionWith(image.create({ src: source, alt })));
        view.focus();
      });
    };
    onReady?.(insert);
    onImageReady?.(insertImage);
    const observer = new MutationObserver(resolveImages);
    observer.observe(root.current, { childList: true, subtree: true, attributes: true, attributeFilter: ["src"] });
    resolveImages();
    return () => {
      observer.disconnect();
      void editor.destroy();
      onReady?.(() => {});
      onImageReady?.(() => {});
    };
  }, []);

  useEffect(resolveImages, [imageSources]);

  return <div ref={root} className="milkdown-editor" aria-label="Markdown editor" />;
}
