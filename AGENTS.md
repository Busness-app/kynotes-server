# KyNotes Server

KyNotes Server is a web-based note management app.
Please see the css directory and fonts directory for look and feel.

# Ponytail, lazy senior dev mode

Use the smallest correct change.

1. Reuse what already exists.
2. Prefer stdlib and native platform APIs.
3. Add dependencies only when they remove meaningful code.
4. Fix shared root causes, not one caller.
5. If a shortcut has a limit, mark it with `ponytail:` and name the upgrade path.

Non-trivial logic must include one runnable check (unit test or minimal self-check).

# DOX framework

## Core Contract

- AGENTS.md files are binding contracts for their subtree.
- Read from root to nearest AGENTS.md before editing.
- The nearest AGENTS.md controls local details; parent docs keep global rules.

## Update After Editing

- Run a DOX pass for every meaningful change.
- Update nearest owning AGENTS.md when behavior, responsibilities, or verification changes.
- Keep Child DOX Index entries current and delete stale rules.

## User Preferences

- Best-effort 90-second keyword refresh policy (foreground cadence; background catch-up on resume).
- DOX hierarchy scope is app-only.

## Design

- `DESIGN.md` is the current product and server architecture design.
- Keep implementation changes aligned with its encryption, sync, storage, and
  self-hosting contracts; update the design when those contracts change.
- `IMPLEMENTATION_PLAN.md` is the binding build order. Its Phase 0 contracts
  (module path, crypto formats, API and error schema, IDs, schema, limits) are
  frozen: implement them, do not re-decide them. Changing a frozen contract
  requires updating `DESIGN.md` and the plan in the same change.

## Child DOX Index

- `internal/httpapi`: opaque routing ciphertext, role-gated mutations, and
  device-validated envelope writes are covered by the package integration
  tests; login, device, pairing, and upload endpoints use in-memory token
  buckets. Upload retries are byte-checked, and preview uploads remain
  content-addressed without creating an attachment row. Admin settings/users,
  audit metadata, team membership, and encrypted comment reads/writes are
  session- and role-gated here.
- `internal/app` owns the `.kynotes.lock` data-directory lock; maintenance
  backup refuses to copy a live data directory and restore runs an integrity
  check after replacement.
- `internal/storage/migrations/0008_frozen_contract_columns.sql` exposes the
  frozen audit and idempotency-key schema on databases created by the earlier
  implementation migrations.
- `cmd/kynotes-probe` is the live 12-step client interoperability acceptance
  path; it uses the same session, pairing, envelope, sync, upload, and GC
  contracts as external clients.
- `FRONTEND_IMPLEMENTATION_PLAN.md` defines the separate responsive web MVP,
  browser crypto/local-storage boundaries, sync state machine, and mobile
  reuse path; it does not alter the frozen server plan.
- `web/` is the browser client; keep plaintext in browser memory only, send
  CSRF headers on mutations, and run its `npm test` plus `npm run build` checks.
- `web/` also contains the encrypted local save queue, client-only search,
  contextual resurfacing, graph projections, single-pane Markdown/WYSIWYG
  editing, and encrypted section anchors for comments.
- `internal/httpapi` commit receipts are deterministic SHA-256 commitments over
  opaque object/version metadata and ciphertext digest; share links store only
  token hashes and serve ciphertext without the URL-fragment decryption key.
  Browser-sealed links use a dedicated random content key and never reuse the
  authentication or workspace key.
- `internal/httpapi` presence is TTL-only in memory and membership-gated;
  notifications expose mention metadata only and use the existing 90-second
  foreground refresh cadence in the browser.
- `internal/storage/migrations/0011_sealed_share_links.sql` stores browser-sealed
  ciphertext separately from object-backed share links.
- KyBackup owns backup/restore; cross-server workspace migration is deferred to
  v2.
- `internal/web` embeds the production `web/dist` bundle into the server image;
  update the checked-in embed after frontend bundle changes.
- Verification for server changes: `go test -race ./...`, `go vet ./...`, and
  `gofmt -l .`.
