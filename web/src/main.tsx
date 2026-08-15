import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { changes, containers, createContainer, createObject, deleteObject, login, loginParams, logout, readObject, saveObject, session, type Container, type Note } from "./api";
import { decryptNote, deriveAuthSecret, encryptNote, type NotePayload } from "./crypto";
import { deleteNote as deleteCachedNote, getNote, putNote } from "./storage";
import "./styles.css";

type AuthState = { username: string; authSecret: string; user: string };

function App() {
  const [auth, setAuth] = useState<AuthState | null>(null);
  const [checking, setChecking] = useState(true);
  useEffect(() => { session().then((value) => setAuth({ username: "", authSecret: "", user: value.user.id })).catch(() => {}).finally(() => setChecking(false)); }, []);
  if (checking) return <main className="center">Loading KyNotes…</main>;
  return auth ? <Workspace auth={auth} onLogout={() => { void logout().finally(() => setAuth(null)); }} /> : <Login onLogin={setAuth} />;
}

function Login({ onLogin }: { onLogin: (auth: AuthState) => void }) {
  const [username, setUsername] = useState(""); const [password, setPassword] = useState(""); const [error, setError] = useState(""); const [busy, setBusy] = useState(false);
  async function submit(event: React.FormEvent) {
    event.preventDefault(); setError(""); setBusy(true);
    try {
      const params = await loginParams(username.trim());
      const authSecret = await deriveAuthSecret(password, params.loginSalt, params.iterations);
      const result = await login(username.trim(), authSecret);
      onLogin({ username: username.trim(), authSecret, user: result.user.id });
      setPassword("");
    } catch (error) { setError(error instanceof Error ? error.message : "Unable to sign in"); }
    finally { setBusy(false); }
  }
  return <main className="auth-page"><section className="auth-card"><div className="eyebrow">PRIVATE NOTES</div><h1>Keep the thread.</h1><p className="lede">Your notes are encrypted in this browser before they leave it.</p><form onSubmit={submit}><label>Username<input autoComplete="username" value={username} onChange={(event) => setUsername(event.target.value)} required /></label><label>Password<input type="password" autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} required /></label>{error && <p className="error">{error}</p>}<button disabled={busy}>{busy ? "Unlocking…" : "Unlock KyNotes"}</button></form><p className="hint">The password is used in memory to derive your authentication and note-encryption keys. It is never stored.</p></section></main>;
}

function Workspace({ auth, onLogout }: { auth: AuthState; onLogout: () => void }) {
  const [items, setItems] = useState<Container[]>([]); const [selected, setSelected] = useState<Container | null>(null); const [notes, setNotes] = useState<Note[]>([]); const [selectedNote, setSelectedNote] = useState<Note | null>(null); const [error, setError] = useState(""); const [busy, setBusy] = useState(false);
  useEffect(() => { void loadContainers(); }, []);
  async function loadContainers() { try { const value = await containers(); setItems(value); if (value[0]) await selectContainer(value[0]); } catch (error) { setError(error instanceof Error ? error.message : "Unable to load workspaces"); } }
  async function selectContainer(container: Container) { setSelected(container); setSelectedNote(null); try { const result = await changes(container.id); const loaded: Note[] = []; for (const change of result.changes.filter((entry) => entry.kind === "object" && !entry.deleted)) { try { const object = await readObject(change.id); const payload = await decryptNote(auth.authSecret, container.id, object.bytes); loaded.push({ id: change.id, ...payload, version: object.version, updatedAt: new Date().toISOString() }); } catch { /* Ignore objects encrypted for another key until the device flow is available. */ } } setNotes(loaded); } catch (error) { setError(error instanceof Error ? error.message : "Unable to load notes"); } }
  async function newWorkspace() { setBusy(true); try { const container = await createContainer(); setItems((value) => [...value, container]); await selectContainer(container); } catch (error) { setError(error instanceof Error ? error.message : "Unable to create workspace"); } finally { setBusy(false); } }
  async function newNote() { if (!selected) return; setBusy(true); try { const object = await createObject(selected.id); const note = { id: object.id, title: "Untitled note", body: "", version: 0, updatedAt: new Date().toISOString() }; setNotes((value) => [note, ...value]); setSelectedNote(note); } catch (error) { setError(error instanceof Error ? error.message : "Unable to create note"); } finally { setBusy(false); } }
  async function save(note: Note) { if (!selected) return; setBusy(true); try { const payload: NotePayload = { title: note.title, body: note.body }; const encrypted = await encryptNote(auth.authSecret, selected.id, payload); const result = await saveObject(note.id, encrypted, note.version); const saved = { ...note, version: result.version, updatedAt: new Date().toISOString() }; await putNote({ id: saved.id, containerID: selected.id, version: saved.version, payload: encrypted, updatedAt: saved.updatedAt }); setNotes((value) => value.map((entry) => entry.id === saved.id ? saved : entry)); setSelectedNote(saved); } catch (error) { setError(error instanceof Error ? error.message : "Unable to save note"); } finally { setBusy(false); } }
  async function remove(note: Note) { if (!confirm("Delete this note?")) return; try { await deleteObject(note.id); await deleteCachedNote(note.id); setNotes((value) => value.filter((entry) => entry.id !== note.id)); setSelectedNote(null); } catch (error) { setError(error instanceof Error ? error.message : "Unable to delete note"); } }
  return <main className="app-shell"><header className="topbar"><div className="brand"><span className="brand-mark">K</span><span>KyNotes</span></div><div className="top-actions"><span className="sync-dot">● Local key active</span><button className="quiet" onClick={onLogout}>Lock</button></div></header><div className="workspace"><aside className="sidebar"><div className="section-label">WORKSPACES</div>{items.map((container) => <button className={`nav-item ${selected?.id === container.id ? "selected" : ""}`} key={container.id} onClick={() => void selectContainer(container)}><span className="nav-icon">◈</span><span>{container.id.slice(4, 12)}</span></button>)}<button className="new-workspace" disabled={busy} onClick={() => void newWorkspace()}>＋ New workspace</button><div className="sidebar-bottom"><div className="section-label">ACCOUNT</div><div className="account-chip"><span className="avatar">{auth.user.slice(-2)}</span><span>{auth.username || auth.user.slice(0, 12)}</span></div></div></aside><section className="note-list"><div className="list-header"><div><div className="section-label">WORKSPACE</div><h2>{selected ? selected.id.slice(4, 12) : "Select a workspace"}</h2></div><button className="icon-button" disabled={!selected || busy} onClick={() => void newNote()}>＋</button></div>{notes.map((note) => <button className={`note-row ${selectedNote?.id === note.id ? "selected" : ""}`} key={note.id} onClick={() => setSelectedNote(note)}><strong>{note.title || "Untitled note"}</strong><span>{note.body.slice(0, 64) || "Empty note"}</span></button>)}{selected && notes.length === 0 && <div className="empty-list">No notes yet.<br />Create the first one.</div>}</section><section className="editor"><div className="editor-meta"><span>{selectedNote ? `Version ${selectedNote.version || "draft"}` : "Ready"}</span><span className="encrypted">Encrypted locally</span></div>{selectedNote ? <><input className="title-input" value={selectedNote.title} onChange={(event) => setSelectedNote({ ...selectedNote, title: event.target.value })} /><textarea className="body-input" value={selectedNote.body} onChange={(event) => setSelectedNote({ ...selectedNote, body: event.target.value })} placeholder="Start writing…" /><div className="editor-actions"><button className="danger quiet" onClick={() => void remove(selectedNote)}>Delete</button><button disabled={busy} onClick={() => void save(selectedNote)}>{busy ? "Saving…" : "Save encrypted note"}</button></div></> : <div className="empty-editor"><div className="empty-glyph">✦</div><h2>Your private desk.</h2><p>Select a note or create one. The server receives ciphertext only.</p>{selected && <button onClick={() => void newNote()}>Create a note</button>}</div>}</section></div>{error && <button className="toast error" onClick={() => setError("")}>{error} ×</button>}</main>;
}

createRoot(document.getElementById("root")!).render(<React.StrictMode><App /></React.StrictMode>);
