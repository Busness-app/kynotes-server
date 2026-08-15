import { describe, expect, it } from "vitest";
import { contextualNotes, graphEdges, noteTags, noteTasks, searchNotes } from "./knowledge";

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
});
