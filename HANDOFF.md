# KyNotes Server Handoff

## Current state

Latest code commit: `b440bd2` (`fix: apply the note sort order to search results`). This handoff file is the only new uncommitted file. The frontend editor is BlockNote, embedded into the Go server image from `web/dist`.

Note bodies use encrypted `NotePayload` values. The body is a `kynotes.blocknote.v1` JSON envelope. Plaintext stays in browser memory; note bodies and attachment payloads are encrypted before IndexedDB/server storage.

## Editor history and current behavior

- `3267e70`: Tiptap document editor
- `466e178`: BlockNote replacement
- `acd30d7`: ignore mount-time editor changes
- `0454f17`: serialize saves
- `51a73eb`: flush drafts when a browser tab is hidden
- `c155445`: convert legacy Tiptap documents to BlockNote blocks
- `6a51b2d`: preserve unsaved edits when navigating away
- `2e003ae`: preserve structured bodies in search/context projections
- `b440bd2`: apply sorting to search results

Workspace navigation flushes the dirty note before clearing the editor. Settings/Admin navigation keeps the editor mounted and hides it instead of remounting it.

## Last reported issue

The user reports: create formatted content, confirm it saves, switch to another workspace, return to the original workspace, and find the text present but formatting gone. The source contains fixes for lifecycle, save, and legacy-format paths, but the deployed behavior has not been confirmed resolved. Test the Docker image, not only Vite.

## Verification

```bash
cd web
npm test
npm run build
cd ..
go test ./internal/web
go vet ./...
gofmt -l .
git diff --check
go test -race ./...
```

## Docker deployment

`docker-compose.local.yml` exposes `8081:8080`. From the repository root:

```bash
docker compose -f docker-compose.yml -f docker-compose.local.yml build --no-cache
docker compose -f docker-compose.yml -f docker-compose.local.yml up -d
git rev-parse --short HEAD
docker compose ps
curl http://127.0.0.1:8081/healthz
```

From another machine, use the Docker host address, for example `http://192.168.1.91:8081/`.

## Formatting-loss investigation

Capture metadata only; never log plaintext note content. Compare these boundaries:

1. BlockNote `current.document` after formatting.
2. The `kynotes.blocknote.v1` body passed to `editBody`.
3. The encrypted payload written by `putNote`.
4. The decrypted payload selected by `selectContainer` after returning.
5. The `initialContent` passed to `BlockNoteEditor`.

If styles exist at step 2 but not step 4, inspect server version/cache selection. If styles exist at step 4 but the screen is plain, inspect BlockNote hydration/CSS. If step 2 is plain, inspect the BlockNote `onChange` callback.

## Important files

- `web/src/BlockNoteEditor.tsx`: lazy editor wrapper and hydration gate.
- `web/src/document.ts`: BlockNote envelope, legacy conversion, and text projection.
- `web/src/main.tsx`: loading, workspace navigation, save queue, autosave, attachments, and editor integration.
- `web/src/storage.ts`: encrypted IndexedDB notes and retry queues.
- `web/src/crypto.ts`: browser encryption/decryption.
- `internal/web`: embedded production bundle.
- `Dockerfile`: frontend build and server embedding.
