export type NoteProjection = {
  id: string;
  title: string;
  body: string;
  updatedAt: string;
};

export function noteLinks(note: NoteProjection): string[] {
  return [...note.body.matchAll(/\[\[([^\]]+)\]\]/g)].map((match) => match[1].trim()).filter(Boolean);
}

export function noteTags(note: NoteProjection): string[] {
  return [...note.body.matchAll(/(?:^|\s)#([a-zA-Z0-9_-]+)/g)].map((match) => match[1].toLowerCase());
}

export function noteTasks(note: NoteProjection): string[] {
  return note.body.split("\n").filter((line) => /^\s*- \[ \] /.test(line)).map((line) => line.replace(/^\s*- \[ \] /, "").trim());
}

export function searchNotes<T extends NoteProjection>(notes: T[], query: string): T[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return notes;
  return notes.filter((note) => `${note.title}\n${note.body}`.toLowerCase().includes(needle));
}

export function contextualNotes<T extends NoteProjection>(notes: T[], active?: T): T[] {
  if (!active) return notes.filter((note) => noteTasks(note).length > 0).slice(0, 5);
  const links = new Set(noteLinks(active));
  return notes.filter((note) => note.id !== active.id && (links.has(note.id) || noteTasks(note).length > 0 || noteTags(note).some((tag) => noteTags(active).includes(tag)))).slice(0, 5);
}

export function graphEdges(notes: NoteProjection[]): Array<{ from: string; to: string }> {
  const ids = new Set(notes.map((note) => note.id));
  return notes.flatMap((note) => noteLinks(note).filter((to) => ids.has(to)).map((to) => ({ from: note.id, to })));
}
