import { describe, expect, it } from "vitest";
import { contextualNotes, graphEdges, indexNotes, noteTags, noteTasks, searchNotes } from "./knowledge";
import { isStructuredNoteBody, stringifyNoteDocument } from "./document";

const notes = [
  { id: "a", title: "Launch", body: "See [[b]] #product\n- [ ] Ship beta", updatedAt: "" },
  { id: "b", title: "Beta", body: "#product", updatedAt: "" },
];

describe("local knowledge projections", () => {
  it("indexes links, tags, tasks and context without server data", () => {
    expect(noteTags(notes[0])).toEqual(["product"]);
    expect(noteTasks(notes[0])).toEqual(["Ship beta"]);
    expect(graphEdges(notes)).toEqual([{ from: "a", to: "b" }]);
    expect(searchNotes(notes, "beta").map((note) => note.id)).toEqual(["a", "b"]);
    expect(contextualNotes(notes, notes[0]).map((note) => note.id)).toEqual(["b"]);
  });

  it("keeps the caller's ordering so the sort control survives search", () => {
    const sorted = [notes[1], notes[0]];
    expect(searchNotes(indexNotes(sorted), "").map((match) => match.note.id)).toEqual(["b", "a"]);
    expect(searchNotes(indexNotes(sorted), "product").map((match) => match.note.id)).toEqual(["b", "a"]);
  });

  it("searches flattened text but keeps the structured note it came from", () => {
    const body = stringifyNoteDocument([
      { type: "heading", props: { level: 2 }, content: "Launch" },
      { type: "paragraph", content: [{ type: "text", text: "Ship it", styles: { bold: true } }] },
    ]);
    const index = indexNotes([{ id: "a", title: "Launch", body, updatedAt: "" }]);
    const [match] = searchNotes(index, "ship it");
    expect(match.body).toBe("Launch Ship it");
    // Reopening a note from the list must not feed the editor flattened text.
    expect(match.note.body).toBe(body);
    expect(isStructuredNoteBody(match.note.body)).toBe(true);
  });
});
