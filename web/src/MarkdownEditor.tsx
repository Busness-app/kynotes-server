import { defaultValueCtx, Editor, rootCtx } from "@milkdown/kit/core";
import { clipboard } from "@milkdown/kit/plugin/clipboard";
import { history } from "@milkdown/kit/plugin/history";
import { listener, listenerCtx } from "@milkdown/kit/plugin/listener";
import { trailing } from "@milkdown/kit/plugin/trailing";
import { commonmark } from "@milkdown/kit/preset/commonmark";
import { gfm } from "@milkdown/kit/preset/gfm";
import { useEffect, useRef } from "react";

export function MarkdownEditor({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const root = useRef<HTMLDivElement>(null);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

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
    return () => {
      void editor.destroy();
    };
  }, []);

  return <div ref={root} className="milkdown-editor" aria-label="Markdown editor" />;
}
