import React, { lazy, Suspense, useEffect, useMemo, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  addAdminTeamMember,
  adminAudit,
  adminTeams,
  adminUsers,
  APIRequestError,
  changePassword,
  changes,
  comments,
  containers,
  createAdminUser,
  createAdminTeam,
  createComment,
  createContainer,
  createObject,
  createSealedShareLink,
  deleteObject,
  fetchShareCiphertext,
  inviteMember,
  login,
  loginParams,
  logout,
  members,
  notifications,
  objectConflicts,
  presence,
  readObject,
  removeMember,
  resetAdminPassword,
  saveObject,
  serverTheme,
  serviceStatus,
  session,
  updateAdminUser,
  updateContainer,
  updatePresence,
  type AdminTeam,
  type AdminUser,
  type Container,
  type Note,
  type Session,
} from "./api";
import {
  decryptComment,
  decryptContainerMeta,
  decryptNote,
  decryptSharePayload,
  deriveAuthSecret,
  encryptComment,
  encryptContainerMeta,
  encryptNote,
  encryptSharePayload,
  fromBase64,
  randomLoginSalt,
  type NotePayload,
} from "./crypto";
import {
  clearQueuedSave,
  deleteNote as deleteCachedNote,
  getNote,
  pendingSaves,
  putNote,
  queueSave,
} from "./storage";
import {
  applyStoredTheme,
  applyTheme,
  getStoredTheme,
  hasStoredTheme,
  THEME_OPTIONS,
  type ThemeName,
} from "./theme";
import { contextualNotes, graphEdges, searchNotes } from "./knowledge";
import "./styles.css";

const MarkdownEditor = lazy(() => import("./MarkdownEditor").then((module) => ({ default: module.MarkdownEditor })));

type AuthState = {
  username: string;
  authSecret: string;
  user: Session["user"];
};
type PlainComment = {
  id: string;
  username: string;
  body: string;
  section?: string;
  createdAt: string;
};

function App() {
  const [auth, setAuth] = useState<AuthState | null>(null);
  const [checking, setChecking] = useState(true);
  useEffect(() => {
    applyStoredTheme();
    if (!hasStoredTheme())
      void serverTheme()
        .then((value) => {
          if (THEME_OPTIONS.includes(value.defaultTheme as ThemeName))
            applyTheme(value.defaultTheme as ThemeName);
        })
        .catch(() => {});
    // A session cookie proves server authentication, not local decryption.
    // Never enter the workspace with an empty auth secret after a reload.
    session()
      .then(() => setAuth(null))
      .catch(() => {})
      .finally(() => setChecking(false));
  }, []);
  if (checking) return <main className="center">Loading KyNotes…</main>;
  if (location.pathname.startsWith("/share/")) return <SharedNote />;
  return auth ? (
    <Workspace
      auth={auth}
      onLogout={() => {
        void logout().finally(() => setAuth(null));
      }}
    />
  ) : (
    <Login onLogin={setAuth} />
  );
}

function SharedNote() {
  const [state, setState] = useState<{ note?: NotePayload; error?: string }>(
    {},
  );
  useEffect(() => {
    const token = location.pathname.split("/").pop() ?? "";
    const key = location.hash.slice(1);
    if (!token || !key) {
      setState({ error: "This link is missing its decryption key." });
      return;
    }
    void fetchShareCiphertext(token)
      .then((bytes) => decryptSharePayload(bytes, key))
      .then((note) => setState({ note }))
      .catch((error) =>
        setState({
          error:
            error instanceof Error
              ? error.message
              : "Unable to decrypt this note.",
        }),
      );
  }, []);
  if (state.error)
    return (
      <main className="center">
        <section className="auth-card">
          <h2>Encrypted link unavailable</h2>
          <p className="error">{state.error}</p>
        </section>
      </main>
    );
  if (!state.note)
    return <main className="center">Decrypting encrypted note…</main>;
  return (
    <main className="auth-page">
      <article className="shared-note">
        <div className="eyebrow">ENCRYPTED KYNOTES LINK</div>
        <h1>{state.note.title}</h1>
        <div className="shared-note-body">{markdown(state.note.body)}</div>
      </article>
    </main>
  );
}

function Login({ onLogin }: { onLogin: (auth: AuthState) => void }) {
  const [username, setUsername] = useState(() => sessionStorage.getItem("kynotes-last-username") ?? "");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setError("");
    setBusy(true);
    try {
      const params = await loginParams(username.trim());
      const authSecret = await deriveAuthSecret(
        password,
        params.loginSalt,
        params.iterations,
      );
      const result = await login(username.trim(), authSecret);
      sessionStorage.setItem("kynotes-last-username", username.trim());
      onLogin({ username: username.trim(), authSecret, user: result.user });
      setPassword("");
    } catch (error) {
      setError(error instanceof Error ? error.message : "Unable to sign in");
    } finally {
      setBusy(false);
    }
  }
  return (
    <main className="auth-page">
      <section className="auth-card">
        <div className="eyebrow">PRIVATE NOTES</div>
        <h1>Keep the thread.</h1>
        <p className="lede">
          Your notes are encrypted in this browser before they leave it.
        </p>
        <form onSubmit={submit}>
          <label>
            Username
            <input
              autoComplete="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              required
            />
          </label>
          <label>
            Password
            <input
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              required
            />
          </label>
          {error && <p className="error">{error}</p>}
          <button disabled={busy}>
            {busy ? "Unlocking…" : "Unlock KyNotes"}
          </button>
        </form>
        <p className="hint">
          The password is used in memory to derive your authentication and
          note-encryption keys. It is never stored.
        </p>
      </section>
    </main>
  );
}

function markdown(text: string): React.ReactNode[] {
  return text.split("\n").map((line, index) => {
    const source =
      line.startsWith("## ") || line.startsWith("# ") || line.startsWith("- ")
        ? line.replace(/^#{1,2} |^- /, "")
        : line;
    const inline = source
      .split(/(\*\*.*?\*\*|`.*?`|\*.*?\*)/g)
      .map((part, partIndex) =>
        part.startsWith("**") ? (
          <strong key={partIndex}>{part.slice(2, -2)}</strong>
        ) : part.startsWith("*") ? (
          <em key={partIndex}>{part.slice(1, -1)}</em>
        ) : part.startsWith("`") ? (
          <code key={partIndex}>{part.slice(1, -1)}</code>
        ) : (
          part
        ),
      );
    const body = line.startsWith("## ") ? (
      <h3 key={index}>{inline}</h3>
    ) : line.startsWith("# ") ? (
      <h2 key={index}>{inline}</h2>
    ) : line.startsWith("- ") ? (
      <li key={index}>{inline}</li>
    ) : (
      <p key={index}>{inline}</p>
    );
    return body;
  });
}

function Workspace({
  auth,
  onLogout,
}: {
  auth: AuthState;
  onLogout: () => void;
}) {
  const [items, setItems] = useState<Container[]>([]);
  const [names, setNames] = useState<Record<string, string>>({});
  const [selected, setSelected] = useState<Container | null>(null);
  const [notes, setNotes] = useState<Note[]>([]);
  const [selectedNote, setSelectedNote] = useState<Note | null>(null);
  const [commentsForNote, setCommentsForNote] = useState<PlainComment[]>([]);
  const [commentText, setCommentText] = useState("");
  const [commentSection, setCommentSection] = useState("");
  const [commitReceipt, setCommitReceipt] = useState("");
  const [conflicted, setConflicted] = useState<Set<string>>(new Set());
  const [lastSavedAt, setLastSavedAt] = useState("");
  const [syncStatus, setSyncStatus] = useState<"saved" | "local" | "syncing" | "attention">("saved");
  const draining = useRef(false);
  const syncChannel = useRef<BroadcastChannel | null>(null);
  const selectedNoteRef = useRef<Note | null>(null);
  selectedNoteRef.current = selectedNote;
  const [presenceForContainer, setPresenceForContainer] = useState<
    Array<{ userId: string; state: string }>
  >([]);
  const [notificationCount, setNotificationCount] = useState(0);
  const [membersForTeam, setMembersForTeam] = useState<
    Array<{ userId: string; username: string; role: string }>
  >([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [view, setView] = useState<"workspace" | "settings" | "admin">(
    "workspace",
  );
  const [sort, setSort] = useState<"updated" | "title">("updated");
  const [query, setQuery] = useState("");
  const [editorMode, setEditorMode] = useState<"markdown" | "wysiwyg">(
    "markdown",
  );
  const pinsKey = `kynotes-pins-${auth.user.id}`;
  const pinned = useMemo(() => {
    try {
      return new Set(
        JSON.parse(localStorage.getItem(pinsKey) || "[]") as string[],
      );
    } catch {
      return new Set<string>();
    }
  }, [pinsKey, notes]);
  const nameOf = (container: Container) =>
    names[container.id] || `Workspace ${container.id.slice(4, 10)}`;
  const orderedNotes = useMemo(
    () =>
      [...notes].sort((a, b) => {
        const pinDiff = Number(pinned.has(b.id)) - Number(pinned.has(a.id));
        if (pinDiff) return pinDiff;
        return sort === "title"
          ? a.title.localeCompare(b.title)
          : b.updatedAt.localeCompare(a.updatedAt);
      }),
    [notes, pinned, sort],
  );
  const visibleNotes = useMemo(
    () => searchNotes(orderedNotes, query),
    [orderedNotes, query],
  );
  const relatedNotes = useMemo(
    () => contextualNotes(notes, selectedNote ?? undefined),
    [notes, selectedNote],
  );
  const links = useMemo(() => graphEdges(notes), [notes]);
  useEffect(() => {
    void loadContainers();
  }, []);
  useEffect(() => {
    const channel = typeof BroadcastChannel === "undefined" ? null : new BroadcastChannel("kynotes-sync");
    syncChannel.current = channel;
    channel?.addEventListener("message", () => void drainQueue());
    const retry = () => void drainQueue();
    window.addEventListener("online", retry);
    void drainQueue();
    const timer = window.setInterval(retry, 15000);
    return () => {
      window.removeEventListener("online", retry);
      window.clearInterval(timer);
      channel?.close();
      syncChannel.current = null;
    };
  }, []);
  useEffect(() => {
    if (commitReceipt) setError(`Server commit receipt: ${commitReceipt}`);
  }, [commitReceipt]);
  useEffect(() => {
    if (!selected) return;
    void presence(selected.id)
      .then(setPresenceForContainer)
      .catch(() => setPresenceForContainer([]));
    void updatePresence(
      selected.id,
      selectedNote ? "editing" : "viewing",
    ).catch(() => {});
    const timer = window.setInterval(() => {
      void presence(selected.id)
        .then(setPresenceForContainer)
        .catch(() => {});
      void updatePresence(
        selected.id,
        selectedNote ? "editing" : "viewing",
      ).catch(() => {});
    }, 30000);
    return () => window.clearInterval(timer);
  }, [selected?.id, selectedNote?.id]);
  useEffect(() => {
    const refresh = () =>
      void notifications()
        .then((value) => setNotificationCount(value.length))
        .catch(() => setNotificationCount(0));
    refresh();
    const timer = window.setInterval(refresh, 90000);
    return () => window.clearInterval(timer);
  }, []);
  useEffect(() => {
    document.title = notificationCount
      ? `(${notificationCount}) KyNotes`
      : "KyNotes";
  }, [notificationCount]);
  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      if (
        (event.ctrlKey || event.metaKey) &&
        event.shiftKey &&
        event.key.toLowerCase() === "l" &&
        selectedNote
      ) {
        event.preventDefault();
        void shareNote();
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [selectedNote]);
  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      if (
        (event.ctrlKey || event.metaKey) &&
        event.key.toLowerCase() === "s" &&
        selectedNote
      ) {
        event.preventDefault();
        void save(selectedNote);
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [selectedNote, selected]);
  useEffect(() => {
    if (!dirty || !selectedNote) return;
    const timer = window.setTimeout(() => {
      void save(selectedNote, true);
    }, 900);
    return () => window.clearTimeout(timer);
  }, [selectedNote?.title, selectedNote?.body, dirty]);
  useEffect(() => {
    if (!dirty || !selectedNote) return;
    const timer = window.setInterval(() => {
      void save(selectedNote, true);
    }, 15000);
    return () => window.clearInterval(timer);
  }, [selectedNote, dirty]);
  async function loadContainers() {
    try {
      const value = await containers();
      const nextNames: Record<string, string> = {};
      for (const item of value) {
        try {
          if (item.metaCiphertext)
            nextNames[item.id] = (
              await decryptContainerMeta(
                auth.authSecret,
                item.id,
                fromBase64(item.metaCiphertext),
              )
            ).name;
        } catch {
          /* encrypted metadata may be unavailable in a session-only resume */
        }
      }
      setNames(nextNames);
      setItems(value);
      if (value[0]) await selectContainer(value[0]);
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Unable to load workspaces",
      );
    }
  }
  async function selectContainer(container: Container) {
    setSelected(container);
    setSelectedNote(null);
    setCommentsForNote([]);
    try {
      const result = await changes(container.id);
      const loaded: Note[] = [];
      for (const change of result.changes.filter(
        (entry) => entry.kind === "object" && !entry.deleted,
      )) {
        try {
          const object = await readObject(change.id);
          const cached = await getNote(change.id);
          const payload =
            cached && cached.version >= object.version
              ? await decryptNote(auth.authSecret, container.id, cached.payload)
              : await decryptNote(auth.authSecret, container.id, object.bytes);
          loaded.push({
            id: change.id,
            ...payload,
            version:
              cached && cached.version >= object.version
                ? cached.version
                : object.version,
            updatedAt:
              cached && cached.version >= object.version
                ? cached.updatedAt
                : new Date().toISOString(),
          });
        } catch {
          const cached = await getNote(change.id);
          if (cached) {
            try {
              const payload = await decryptNote(
                auth.authSecret,
                container.id,
                cached.payload,
              );
              loaded.push({
                id: change.id,
                ...payload,
                version: cached.version,
                updatedAt: cached.updatedAt,
              });
            } catch {
              /* Ignore an invalid local draft. */
            }
          }
        }
      }
      setNotes(loaded);
      if (container.kind === "team")
        setMembersForTeam(await members(container.id));
      else setMembersForTeam([]);
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Unable to load workspace",
      );
    }
  }
  async function selectNote(note: Note) {
    setSelectedNote(note);
    setDirty(false);
    setLastSavedAt(note.version > 0 ? note.updatedAt : "");
    try { const conflicts = await objectConflicts(note.id); setConflicted((value) => { const next = new Set(value); if (conflicts.some((item) => !item.resolved)) next.add(note.id); else next.delete(note.id); return next; }); } catch { /* conflict metadata is advisory */ }
    try {
      const remote = await comments(note.id);
      const decoded: PlainComment[] = [];
      for (const item of remote) {
        try {
          const decrypted = await decryptComment(
            auth.authSecret,
            selected!.id,
            fromBase64(item.bodyCiphertext),
          );
          decoded.push({
            id: item.id,
            username: item.username,
            body: decrypted.body,
            section: decrypted.section,
            createdAt: item.createdAt,
          });
        } catch {
          /* Ignore comments encrypted for another key. */
        }
      }
      setCommentsForNote(decoded);
    } catch {
      setCommentsForNote([]);
    }
  }
  async function newWorkspace() {
    const name = prompt("Personal workspace name", "My personal workspace")?.trim();
    if (!name) return;
    setBusy(true);
    try {
      const container = await createContainer("workbook");
      const encrypted = await encryptContainerMeta(
        auth.authSecret,
        container.id,
        name,
      );
      const encoded = btoa(String.fromCharCode(...encrypted));
      const result = await updateContainer(
        container.id,
        encoded,
        container.metaVersion,
      );
      const named = {
        ...container,
        metaCiphertext: encoded,
        metaVersion: result.metaVersion,
        changeSeq: result.changeSeq,
      };
      setNames((value) => ({ ...value, [named.id]: name }));
      setItems((value) => [...value, named]);
      await selectContainer(named);
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Unable to create workspace",
      );
    } finally {
      setBusy(false);
    }
  }
  async function newTeamWorkspace(teamContainer: Container) {
    const name = prompt("Team workspace name", "New workspace")?.trim();
    if (!name) return;
    setBusy(true);
    try {
      const container = await createContainer("workbook", "", teamContainer.id);
      const encrypted = await encryptContainerMeta(auth.authSecret, container.id, name);
      const encoded = btoa(String.fromCharCode(...encrypted));
      const result = await updateContainer(container.id, encoded, container.metaVersion);
      const named = { ...container, metaCiphertext: encoded, metaVersion: result.metaVersion, changeSeq: result.changeSeq };
      setNames((value) => ({ ...value, [named.id]: name }));
      setItems((value) => [...value, named]);
      await selectContainer(named);
    } catch (error) {
      setError(error instanceof Error ? error.message : "Unable to create team workspace");
    } finally {
      setBusy(false);
    }
  }
  async function renameWorkspace() {
    if (!selected) return;
    const name = prompt("Workspace name", nameOf(selected))?.trim();
    if (!name) return;
    setBusy(true);
    try {
      const encrypted = await encryptContainerMeta(
        auth.authSecret,
        selected.id,
        name,
      );
      const encoded = btoa(String.fromCharCode(...encrypted));
      const result = await updateContainer(
        selected.id,
        encoded,
        selected.metaVersion,
      );
      const next = {
        ...selected,
        metaCiphertext: encoded,
        metaVersion: result.metaVersion,
        changeSeq: result.changeSeq,
      };
      setNames((value) => ({ ...value, [selected.id]: name }));
      setItems((value) =>
        value.map((entry) => (entry.id === next.id ? next : entry)),
      );
      setSelected(next);
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Unable to rename workspace",
      );
    } finally {
      setBusy(false);
    }
  }
  function togglePin(note: Note) {
    const next = new Set(pinned);
    next.has(note.id) ? next.delete(note.id) : next.add(note.id);
    localStorage.setItem(pinsKey, JSON.stringify([...next]));
    setNotes((value) => [...value]);
  }
  async function newNote() {
    if (!selected) return;
    setBusy(true);
    try {
      const object = await createObject(selected.id);
      const note = {
        id: object.id,
        title: "Untitled note",
        body: "# Untitled note\n\n",
        version: 0,
        updatedAt: new Date().toISOString(),
      };
      setNotes((value) => [note, ...value]);
      setSelectedNote(note);
      setDirty(true);
      persistDraft(note);
      await save(note, true);
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Unable to create note",
      );
    } finally {
      setBusy(false);
    }
  }
  async function save(note: Note, automatic = false) {
    if (!selected) return;
    if (!automatic) setBusy(true);
    try {
      const payload: NotePayload = { title: note.title, body: note.body };
      const encrypted = await encryptNote(
        auth.authSecret,
        selected.id,
        payload,
      );
      const savedAt = new Date().toISOString();
      await putNote({
        id: note.id,
        containerID: selected.id,
        version: note.version,
        payload: encrypted,
        updatedAt: savedAt,
      });
      try {
        const result = await saveObject(
          note.id,
          encrypted,
          note.version,
          selected.keyGeneration,
        );
        await clearQueuedSave(note.id);
        setCommitReceipt(result.commitReceipt ?? "");
          setConflicted((value) => { const next = new Set(value); next.delete(note.id); return next; });
          setSyncStatus("saved");
        const saved = { ...note, version: result.version, updatedAt: savedAt };
        setLastSavedAt(savedAt);
        setSyncStatus("saved");
        setNotes((value) =>
          value.map((entry) => (entry.id === saved.id ? saved : entry)),
        );
        setSelectedNote(saved);
        setDirty(false);
      } catch (error) {
        if (error instanceof APIRequestError && error.code === "version_conflict") {
          setConflicted((value) => new Set(value).add(note.id));
          setSyncStatus("attention");
          setError("This note changed on another device. Your encrypted draft is preserved locally; review the conflict before saving again.");
        } else {
          await queueSave({ id: note.id, containerID: selected.id, version: note.version, payload: encrypted, updatedAt: savedAt, keyGeneration: selected.keyGeneration });
          syncChannel.current?.postMessage({ type: "queued", id: note.id });
          setSyncStatus("local");
          setError("Saved locally; encrypted change queued for the server.");
        }
      }
    } catch (error) {
      setError(error instanceof Error ? error.message : "Unable to save note");
    } finally {
      if (!automatic) setBusy(false);
    }
  }

  async function drainQueue() {
    if (draining.current) return;
    draining.current = true;
    try {
      const queued = await pendingSaves();
      if (!queued.length) return;
      setSyncStatus("syncing");
      let remaining = false;
      let attention = false;
      for (const item of queued) {
        try {
          const result = await saveObject(item.id, item.payload, item.version, item.keyGeneration ?? 1);
          await clearQueuedSave(item.id);
          setNotes((value) => value.map((note) => note.id === item.id ? { ...note, version: result.version, updatedAt: item.updatedAt } : note));
          if (selectedNoteRef.current?.id === item.id) {
            setSelectedNote((note) => note?.id === item.id ? { ...note, version: result.version, updatedAt: item.updatedAt } : note);
            setLastSavedAt(item.updatedAt);
            setDirty(false);
          }
        } catch (error) {
          if (error instanceof APIRequestError && error.code === "version_conflict") {
            await clearQueuedSave(item.id);
            setConflicted((value) => new Set(value).add(item.id));
            attention = true;
          } else {
            remaining = true;
          }
        }
      }
      setSyncStatus(attention ? "attention" : remaining ? "local" : "saved");
    } catch {
      setSyncStatus("local");
    } finally {
      draining.current = false;
    }
  }
  async function remove(note: Note) {
    if (!confirm("Delete this note?")) return;
    try {
      await deleteObject(note.id);
      await deleteCachedNote(note.id);
      setNotes((value) => value.filter((entry) => entry.id !== note.id));
      setSelectedNote(null);
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Unable to delete note",
      );
    }
  }
  function persistDraft(note: Note) {
    if (!selected) return;
    void encryptNote(auth.authSecret, selected.id, {
      title: note.title,
      body: note.body,
    })
      .then((payload) =>
        putNote({
          id: note.id,
          containerID: selected.id,
          version: note.version,
          payload,
          updatedAt: new Date().toISOString(),
        }),
      )
      .catch(() => {});
  }
  function editBody(value: string) {
    if (selectedNote) {
      const next = { ...selectedNote, body: value };
      setSelectedNote(next);
      setDirty(true);
      persistDraft(next);
    }
  }
  function insertMarkdown(value: string) {
    if (!selectedNote) return;
    editBody(
      `${selectedNote.body}${selectedNote.body.endsWith("\n") ? "" : "\n"}${value}\n`,
    );
  }
  async function addComment() {
    if (!selectedNote || !selected || !commentText.trim()) return;
    setBusy(true);
    try {
      const encrypted = await encryptComment(
        auth.authSecret,
        selected.id,
        commentText.trim(),
        commentSection.trim(),
      );
      await createComment(
        selectedNote.id,
        btoa(String.fromCharCode(...encrypted)),
        selected.keyGeneration,
      );
      setCommentText("");
      setCommentSection("");
      await selectNote(selectedNote);
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Unable to add comment",
      );
    } finally {
      setBusy(false);
    }
  }
  async function invite() {
    if (!selected) return;
    const userID = prompt("User ID to invite");
    if (!userID) return;
    try {
      const result = await inviteMember(selected.id, userID, "editor");
      setError(`Invitation created. Share token: ${result.token}`);
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Unable to invite member",
      );
    }
  }
  async function removeTeamMember(userID: string) {
    if (!selected || !confirm("Remove this person from the team?")) return;
    try {
      await removeMember(selected.id, userID);
      setMembersForTeam(await members(selected.id));
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Unable to remove member",
      );
    }
  }
  async function shareNote() {
    if (!selectedNote) return;
    try {
      const sealed = await encryptSharePayload({
        title: selectedNote.title,
        body: selectedNote.body,
      });
      const link = await createSealedShareLink(
        sealed.ciphertext,
        new Date(Date.now() + 7 * 86400000).toISOString(),
      );
      await navigator.clipboard.writeText(
        `${location.origin}/share/${link.token}#${sealed.key}`,
      );
      setError(
        "Encrypted share link copied. The key is only in the URL fragment.",
      );
    } catch (error) {
      setError(
        error instanceof Error
          ? error.message
          : "Unable to create encrypted link",
      );
    }
  }
  const personal = items.filter((item) => item.kind !== "team" && !item.teamId);
  const teams = items.filter((item) => item.kind === "team");
  const teamWorkspaces = (teamID: string) => items.filter((item) => item.teamId === teamID);
  return (
    <main className="app-shell">
      <header className="topbar">
        <div className="brand">
          <span className="brand-mark">K</span>
          <span>KyNotes</span>
        </div>
        <div className="top-actions">
          <span className={`sync-dot sync-${syncStatus}`}>
            ● {dirty ? "Unsaved changes" : syncStatus === "syncing" ? "Syncing…" : syncStatus === "local" ? "Saved locally" : syncStatus === "attention" ? "Needs attention" : "Saved to server"}
          </span>
          {selectedNote && (
            <button
              className="save-button"
              disabled={busy}
              onClick={() => void save(selectedNote)}
            >
              {busy ? "Saving…" : "Save"}
            </button>
          )}
          <button className="quiet" onClick={() => setView("settings")}>
            Settings
          </button>
          {auth.user.role === "admin" && (
            <button className="quiet" onClick={() => setView("admin")}>
              Admin
            </button>
          )}
          <button className="quiet" onClick={onLogout}>
            Lock
          </button>
        </div>
      </header>
      {view !== "workspace" ? (
        <SettingsView
          admin={view === "admin"}
          authSecret={auth.authSecret}
          username={auth.username}
          onBack={() => setView("workspace")}
        />
      ) : (
        <div className="workspace">
          <aside className="sidebar">
            <div className="section-label">PERSONAL</div>
            {personal.map((container) => (
              <button
                className={`nav-item ${selected?.id === container.id ? "selected" : ""}`}
                key={container.id}
                onClick={() => void selectContainer(container)}
              >
                <span className="nav-icon">◈</span>
                <span>{nameOf(container)}</span>
              </button>
            ))}
            <div className="section-label team-label">TEAMS</div>
            {teams.map((container) => (
              <React.Fragment key={container.id}>
                <button
                  className={`nav-item ${selected?.id === container.id ? "selected" : ""}`}
                  onClick={() => void selectContainer(container)}
                >
                  <span className="nav-icon">◇</span>
                  <span>{nameOf(container)}</span>
                </button>
                {teamWorkspaces(container.id).map((workspace) => (
                  <button
                    className={`nav-item nested-nav-item ${selected?.id === workspace.id ? "selected" : ""}`}
                    key={workspace.id}
                    onClick={() => void selectContainer(workspace)}
                  >
                    <span className="nav-icon">◈</span>
                    <span>{nameOf(workspace)}</span>
                  </button>
                ))}
                {selected?.id === container.id && (
                  <button className="new-workspace" disabled={busy} onClick={() => void newTeamWorkspace(container)}>
                    ＋ New team workspace
                  </button>
                )}
              </React.Fragment>
            ))}
            <button
              className="new-workspace personal-create"
              disabled={busy}
              onClick={() => void newWorkspace()}
            >
              ＋ New personal workspace
            </button>
            {selected && (
              <button
                className="new-workspace"
                disabled={busy}
                onClick={() => void renameWorkspace()}
              >
                ✎ Rename workspace
              </button>
            )}
            {selected?.kind === "team" && (
              <>
                <button className="new-workspace" onClick={() => void invite()}>
                  ＋ Add person
                </button>
                {membersForTeam.map((member) => (
                  <div className="member-row" key={member.userId}>
                    <span>
                      {member.username} · {member.role}
                    </span>
                    {member.userId !== auth.user.id && (
                      <button
                        className="quiet"
                        onClick={() => void removeTeamMember(member.userId)}
                      >
                        Remove
                      </button>
                    )}
                  </div>
                ))}
              </>
            )}
            <div className="sidebar-bottom">
              <div className="section-label">ACCOUNT</div>
              <div className="account-chip">
                <span className="avatar">{auth.user.id.slice(-2)}</span>
                <span>{auth.username || auth.user.id.slice(0, 12)}</span>
              </div>
            </div>
          </aside>
          <section className="note-list">
            <div className="list-header">
              <div>
                <div className="section-label">
                  {selected ? nameOf(selected) : "WORKSPACE"}
                </div>
                <h2 className="workspace-title">{selected ? nameOf(selected) : "Select a workspace"}</h2>
                {selected && <div className="workspace-kind">{selected.kind === "team" ? "Team workspace" : "Personal workspace"}</div>}
                {selected && <h3 className="notes-heading">Notes</h3>}
              </div>
              <div className="list-actions">
                <input
                  aria-label="Search notes"
                  className="note-search"
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="Search"
                />
                <select
                  value={sort}
                  onChange={(event) =>
                    setSort(event.target.value as "updated" | "title")
                  }
                >
                  <option value="updated">Recent</option>
                  <option value="title">Title</option>
                </select>
                <button
                  className="icon-button"
                  disabled={!selected || busy}
                  onClick={() => void newNote()}
                >
                  ＋
                </button>
              </div>
            </div>
            {visibleNotes.map((note) => (
              <div
                className={`note-row-wrap ${selectedNote?.id === note.id ? "selected" : ""}`}
                key={note.id}
              >
                <button
                  className="note-row"
                  onClick={() => void selectNote(note)}
                >
                  <strong>{note.title || "Untitled note"}</strong>
                  <span>
                    {note.body.replace(/^#+\s*/gm, "").slice(0, 64) ||
                      "Empty note"}
                  </span>
                </button>
                <button
                  className="pin-button"
                  title={pinned.has(note.id) ? "Unpin note" : "Pin note"}
                  onClick={() => togglePin(note)}
                >
                  {pinned.has(note.id) ? "★" : "☆"}
                </button>
              </div>
            ))}
            {selected && notes.length === 0 && (
              <div className="empty-list">
                No notes yet.
                <br />
                Create the first one.
              </div>
            )}
            {relatedNotes.length > 0 && (
              <div className="context-panel">
                <div className="section-label">RESURFACING</div>
                {relatedNotes.map((note) => (
                  <button
                    className="context-link"
                    key={note.id}
                    onClick={() => void selectNote(note)}
                  >
                    {note.title || "Untitled note"}
                  </button>
                ))}
              </div>
            )}
            {links.length > 0 && (
              <div className="context-panel">
                <div className="section-label">KNOWLEDGE GRAPH</div>
                <span className="config-muted">
                  {links.length} local link{links.length === 1 ? "" : "s"}
                </span>
              </div>
            )}
          </section>
          <section className="editor">
            <div className="editor-meta">
              <span>
                {selectedNote
                  ? `Version ${selectedNote.version || "draft"}`
                  : "Ready"}
              </span>
              <span className="encrypted">
                {dirty
                  ? "Autosaving…"
                  : syncStatus === "local"
                    ? "Encrypted locally · waiting to sync"
                    : syncStatus === "attention"
                      ? "Conflict needs attention"
                      : lastSavedAt
                        ? `Saved on server ${new Date(lastSavedAt).toLocaleTimeString()}`
                        : "Encrypted locally"}
              </span>
            </div>
            {selectedNote ? (
              <>
                {conflicted.has(selectedNote.id) && (
                  <div className="conflict-banner" role="alert">
                    This note has a newer encrypted version on the server. Your local draft is preserved; reload the note before saving again.
                  </div>
                )}
                <input
                  className="title-input"
                  value={selectedNote.title}
                  onChange={(event) => {
                    const next = { ...selectedNote, title: event.target.value };
                    setSelectedNote(next);
                    setDirty(true);
                    persistDraft(next);
                  }}
                />
                <div className="format-toolbar">
                  <button
                    className={editorMode === "markdown" ? "active" : ""}
                    onClick={() => setEditorMode("markdown")}
                  >
                    Markdown
                  </button>
                  <button
                    className={editorMode === "wysiwyg" ? "active" : ""}
                    onClick={() => setEditorMode("wysiwyg")}
                  >
                    WYSIWYG
                  </button>
                  {editorMode === "markdown" && <>
                    <button onClick={() => insertMarkdown("**bold text**")}>Bold</button>
                    <button onClick={() => insertMarkdown("*italic text*")}>Italic</button>
                    <button onClick={() => insertMarkdown("## Heading")}>Heading</button>
                    <button onClick={() => insertMarkdown("- list item")}>List</button>
                    <button onClick={() => insertMarkdown("`code`")}>Code</button>
                  </>}
                </div>
                <div className="single-pane-editor">
                  {editorMode === "markdown" ? (
                    <textarea
                      className="body-input"
                      value={selectedNote.body}
                      onChange={(event) => editBody(event.target.value)}
                      placeholder="Write Markdown…"
                    />
                  ) : (
                    <Suspense fallback={<div className="milkdown-editor editor-loading">Loading editor…</div>}>
                      <MarkdownEditor
                        key={`${selectedNote.id}-${editorMode}`}
                        value={selectedNote.body}
                        onChange={editBody}
                      />
                    </Suspense>
                  )}
                </div>
                <div className="editor-actions">
                  <button
                    className="danger quiet"
                    onClick={() => void remove(selectedNote)}
                  >
                    Delete
                  </button>
                  <button
                    disabled={busy}
                    onClick={() => void save(selectedNote)}
                  >
                    {busy ? "Saving…" : dirty ? "Save now" : "Saved"}
                  </button>
                </div>
                <section className="comments">
                  <h3>Comments</h3>
                  {commentsForNote.map((comment) => (
                    <div className="comment" key={comment.id}>
                      <strong>
                        {comment.username}
                        {comment.section ? ` · § ${comment.section}` : ""}
                      </strong>
                      <span>{comment.body}</span>
                    </div>
                  ))}
                  <div className="comment-compose">
                    <input
                      value={commentSection}
                      onChange={(event) =>
                        setCommentSection(event.target.value)
                      }
                      placeholder="Section (optional)"
                    />
                    <input
                      value={commentText}
                      onChange={(event) => setCommentText(event.target.value)}
                      placeholder="Add a comment…"
                    />
                    <button disabled={busy} onClick={() => void addComment()}>
                      Comment
                    </button>
                  </div>
                </section>
              </>
            ) : (
              <div className="empty-editor">
                <div className="empty-glyph">✦</div>
                <h2>Your private desk.</h2>
                <p>
                  Select a note or create one. The server receives ciphertext
                  only.
                </p>
                {selected && (
                  <button onClick={() => void newNote()}>Create a note</button>
                )}
              </div>
            )}
          </section>
        </div>
      )}
      {error && (
        <button className="toast error" onClick={() => setError("")}>
          {error} ×
        </button>
      )}
    </main>
  );
}

function PasswordSettings({ username }: { username: string }) {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [status, setStatus] = useState("");
  const [busy, setBusy] = useState(false);
  async function submit(event: React.FormEvent) {
    event.preventDefault();
    const confirmation = (
      (event.currentTarget as HTMLFormElement).elements.namedItem(
        "confirm",
      ) as HTMLInputElement
    )?.value;
    if (!next || next !== confirmation) {
      setStatus("New passwords do not match.");
      return;
    }
    setBusy(true);
    try {
      const name = username || prompt("Username")?.trim();
      if (!name) throw new Error("Username is required");
      const oldParams = await loginParams(name);
      const currentAuthSecret = await deriveAuthSecret(
        current,
        oldParams.loginSalt,
        oldParams.iterations,
      );
      const newLoginSalt = randomLoginSalt();
      const newAuthSecret = await deriveAuthSecret(next, newLoginSalt, 600000);
      await changePassword({
        currentAuthSecret,
        newAuthSecret,
        newLoginSalt,
        iterations: 600000,
      });
      setCurrent("");
      setNext("");
      setStatus(
        "Password changed. Existing encrypted notes may require the device re-key flow.",
      );
    } catch (error) {
      setStatus(
        error instanceof Error ? error.message : "Unable to change password",
      );
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="config-card">
      <h2>Change password</h2>
      <p className="config-muted">
        Passwords are converted to client-derived secrets in this browser. They
        are never sent to the server.
      </p>
      <form onSubmit={submit}>
        <label className="field">
          <span>Current password</span>
          <input
            type="password"
            value={current}
            onChange={(event) => setCurrent(event.target.value)}
            required
          />
        </label>
        <label className="field">
          <span>New password</span>
          <input
            type="password"
            value={next}
            onChange={(event) => setNext(event.target.value)}
            required
          />
        </label>
        <label className="field">
          <span>Confirm new password</span>
          <input name="confirm" type="password" required />
        </label>
        <button disabled={busy}>
          {busy ? "Changing…" : "Change password"}
        </button>
      </form>
      {status && <p className="status-line">{status}</p>}
    </section>
  );
}

function AdminCreateUser({ onCreated }: { onCreated: () => void }) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState("user");
  const [busy, setBusy] = useState(false);
  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    try {
      const salt = randomLoginSalt();
      const authSecret = await deriveAuthSecret(password, salt, 600000);
      await createAdminUser({
        username,
        authSecret,
        loginSalt: salt,
        iterations: 600000,
        role,
      });
      setUsername("");
      setPassword("");
      onCreated();
    } catch (error) {
      alert(error instanceof Error ? error.message : "Unable to create user");
    } finally {
      setBusy(false);
    }
  }
  return (
    <form className="admin-create" onSubmit={submit}>
      <h3>Create user</h3>
      <input
        placeholder="Username"
        value={username}
        onChange={(event) => setUsername(event.target.value)}
        required
      />
      <input
        placeholder="Temporary password"
        type="password"
        value={password}
        onChange={(event) => setPassword(event.target.value)}
        required
      />
      <select value={role} onChange={(event) => setRole(event.target.value)}>
        <option>user</option>
        <option>admin</option>
      </select>
      <button disabled={busy}>Create user</button>
    </form>
  );
}

function AdminUserActions({
  user,
  onReset,
}: {
  user: AdminUser;
  onReset: () => void;
}) {
  async function reset() {
    const password = prompt(`New temporary password for ${user.username}`);
    if (!password) return;
    try {
      const salt = randomLoginSalt();
      const secret = await deriveAuthSecret(password, salt, 600000);
      await resetAdminPassword(user.id, {
        newAuthSecret: secret,
        newLoginSalt: salt,
        iterations: 600000,
      });
      onReset();
      alert("Password reset. All existing sessions were revoked.");
    } catch (error) {
      alert(
        error instanceof Error ? error.message : "Unable to reset password",
      );
    }
  }
  return (
    <button className="quiet" onClick={() => void reset()}>
      Reset password
    </button>
  );
}

function AdminTeams({ users, authSecret }: { users: AdminUser[]; authSecret: string }) {
  const [teams, setTeams] = useState<AdminTeam[]>([]);
  const [teamNames, setTeamNames] = useState<Record<string, string>>({});
  const [team, setTeam] = useState("");
  const [user, setUser] = useState("");
  const [role, setRole] = useState("editor");
  async function reload() {
    try {
      const nextTeams = await adminTeams();
      const nextNames: Record<string, string> = {};
      for (const entry of nextTeams) {
        if (!entry.metaCiphertext) continue;
        try {
          nextNames[entry.id] = (
            await decryptContainerMeta(authSecret, entry.id, fromBase64(entry.metaCiphertext))
          ).name;
        } catch {
          /* Metadata encrypted by another account remains opaque. */
        }
      }
      setTeams(nextTeams);
      setTeamNames(nextNames);
    } catch {
      /* The admin page remains usable if the list refresh is unavailable. */
    }
  }
  useEffect(() => {
    void reload();
  }, [authSecret]);
  async function createTeam() {
    const name = prompt("Team name", "New team")?.trim();
    if (!name) return;
    try {
      // The server mints the container ID, which is part of the metadata key.
      // Create first, then immediately replace the empty metadata with ciphertext.
      const created = await createAdminTeam("");
      const encrypted = await encryptContainerMeta(authSecret, created.id, name);
      const encoded = btoa(String.fromCharCode(...encrypted));
      await updateContainer(created.id, encoded, created.metaVersion ?? 0);
      setTeam(created.id);
      await reload();
    } catch (error) {
      alert(error instanceof Error ? error.message : "Unable to create team");
    }
  }
  async function renameTeam() {
    const selected = teams.find((entry) => entry.id === team);
    if (!selected) return;
    const name = prompt("Team name", teamNames[selected.id] ?? "Team")?.trim();
    if (!name) return;
    try {
      const encrypted = await encryptContainerMeta(authSecret, selected.id, name);
      const encoded = btoa(String.fromCharCode(...encrypted));
      await updateContainer(selected.id, encoded, selected.metaVersion ?? 0);
      await reload();
    } catch (error) {
      alert(error instanceof Error ? error.message : "Unable to rename team");
    }
  }
  async function add() {
    if (!team || !user) return;
    try {
      await addAdminTeamMember(team, user, role);
      alert("Person added to team.");
    } catch (error) {
      alert(error instanceof Error ? error.message : "Unable to add person");
    }
  }
  return (
    <section className="config-card">
      <h2>Teams</h2>
      <p className="config-muted">
        Create a team workspace, then add active users to it.
      </p>
      <button onClick={() => void createTeam()}>Create team</button>
      <label className="field">
        <span>Team</span>
        <select value={team} onChange={(event) => setTeam(event.target.value)}>
          <option value="">Select team</option>
          {teams.map((entry) => (
            <option key={entry.id} value={entry.id}>
              {teamNames[entry.id] ?? "Unnamed team"} · {entry.id}
            </option>
          ))}
        </select>
      </label>
      <button className="quiet" onClick={() => void renameTeam()} disabled={!team}>
        Rename team
      </button>
      <label className="field">
        <span>Person</span>
        <select value={user} onChange={(event) => setUser(event.target.value)}>
          <option value="">Select person</option>
          {users
            .filter((entry) => entry.status === "active")
            .map((entry) => (
              <option key={entry.id} value={entry.id}>
                {entry.username}
              </option>
            ))}
        </select>
      </label>
      <label className="field">
        <span>Role</span>
        <select value={role} onChange={(event) => setRole(event.target.value)}>
          <option>admin</option>
          <option>editor</option>
          <option>commenter</option>
          <option>viewer</option>
        </select>
      </label>
      <button onClick={() => void add()}>Add to team</button>
    </section>
  );
}

function SettingsView({
  admin,
  authSecret,
  onBack,
  username,
}: {
  admin: boolean;
  authSecret: string;
  onBack: () => void;
  username: string;
}) {
  const [theme, setTheme] = useState<ThemeName>(getStoredTheme());
  const [status, setStatus] = useState<{
    health: boolean;
    ready: boolean;
  } | null>(null);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [audit, setAudit] = useState<Array<Record<string, string>>>([]);
  useEffect(() => {
    if (admin) {
      void Promise.all([adminUsers(), adminAudit(), serviceStatus()])
        .then(([nextUsers, nextAudit, nextStatus]) => {
          setUsers(nextUsers);
          setAudit(nextAudit);
          setStatus(nextStatus);
        })
        .catch(() => {});
    }
  }, [admin]);
  async function saveUser(user: AdminUser) {
    try {
      await updateAdminUser(user);
      setUsers((value) =>
        value.map((entry) => (entry.id === user.id ? user : entry)),
      );
    } catch (error) {
      alert(error instanceof Error ? error.message : "Unable to update user");
    }
  }
  return (
    <section
      className={`settings-layout ${admin ? "admin-settings-layout" : ""}`}
    >
      <aside className="settings-sidebar">
        <button className="quiet" onClick={onBack}>
          ← Workspace
        </button>
        <div className="section-label">{admin ? "ADMIN" : "SETTINGS"}</div>
        {!admin && (
          <nav className="settings-nav">
            <a href="#appearance">Appearance</a>
            <a href="#password">Password</a>
          </nav>
        )}
      </aside>
      <div className="settings-content">
        <div className="settings-header">
          <div className="section-label">{admin ? "ADMIN" : "PREFERENCES"}</div>
          <h1>{admin ? "Administration" : "Settings"}</h1>
          <p>
            {admin
              ? "Manage people, teams, and metadata-only audit records."
              : "Your browser preferences and account security."}
          </p>
        </div>
        {admin && (
          <nav className="settings-nav admin-main-tabs" aria-label="Administration sections">
            <a href="#server">Server</a>
            <a href="#users">Users</a>
            <a href="#teams">Teams</a>
            <a href="#audit">Audit log</a>
          </nav>
        )}
        {!admin && (
          <>
            <section id="appearance" className="config-card">
              <h2>Appearance</h2>
              <p className="config-muted">
                Choose from the complete KyNotes color selection.
              </p>
              <label className="field">
                <span>Theme</span>
                <select
                  value={theme}
                  onChange={(event) =>
                    setTheme(event.target.value as ThemeName)
                  }
                >
                  {THEME_OPTIONS.map((option) => (
                    <option key={option}>{option}</option>
                  ))}
                </select>
              </label>
              <button onClick={() => applyTheme(theme)}>Apply theme</button>
            </section>
            <div id="password">
              <PasswordSettings username={username} />
            </div>
          </>
        )}
        {admin && (
          <>
            <section id="server" className="config-card">
              <h2>Server status</h2>
              <p className="status-line">
                Health:{" "}
                {status ? (status.health ? "OK" : "failed") : "checking…"} ·
                Readiness:{" "}
                {status ? (status.ready ? "OK" : "failed") : "checking…"}
              </p>
            </section>
            <section id="users" className="config-card">
              <h2>Users</h2>
              <AdminCreateUser
                onCreated={() => void adminUsers().then(setUsers)}
              />
              {users.map((user) => (
                <div className="admin-user" key={user.id}>
                  <strong>{user.username}</strong>
                  <select
                    value={user.role}
                    onChange={(event) =>
                      void saveUser({ ...user, role: event.target.value })
                    }
                  >
                    <option>user</option>
                    <option>admin</option>
                  </select>
                  <select
                    value={user.status}
                    onChange={(event) =>
                      void saveUser({ ...user, status: event.target.value })
                    }
                  >
                    <option>active</option>
                    <option>disabled</option>
                  </select>
                  <AdminUserActions user={user} onReset={() => {}} />
                </div>
              ))}
            </section>
            <div id="teams">
              <AdminTeams users={users} authSecret={authSecret} />
            </div>
            <section id="audit" className="config-card">
              <h2>Audit log</h2>
              {audit.length === 0 ? (
                <p className="config-muted">No audit events recorded.</p>
              ) : (
                audit.map((entry, index) => (
                  <div className="audit-row" key={`${entry.at}-${index}`}>
                    <strong>{entry.event}</strong>
                    <span>
                      {entry.outcome} · {entry.at}
                    </span>
                  </div>
                ))
              )}
            </section>
          </>
        )}
      </div>
    </section>
  );
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
