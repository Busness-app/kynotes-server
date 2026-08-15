# KyNotes Server

KyNotes is a self-hosted, zero-knowledge note management service. It stores
encrypted Markdown notes and attachments and synchronizes them across trusted
web, Android, and iOS clients. Desktop support is deferred.

The backend is written in Go and runs in Docker. It is intended for
individuals and small teams of fewer than 20 people. The complete architecture
is documented in [DESIGN.md](DESIGN.md).

## MVP

- Client-derived account login using KyPassword's authentication and key
  management contracts.
- QR-code or deep-link pairing for device-wrapped keys.
- Encrypted personal Workbooks, folders, Markdown notes, and attachments.
- Explicit device sync selection for Workbooks, Projects, and Teams.
- Offline editing with HTTPS synchronization and conflict rejection.
- Personal Inbox and personal Workbook task views.
- Projects as top-level containers.
- Team collaboration with roles, invitations, comments, mentions, presence,
  notifications, and encrypted activity history.
- Tasks stored in Markdown with YAML front matter and checkboxes. Support due
  dates, recurrence, priority, status, assignees, subtasks, and reminders.
- Push notifications through the existing KyPost model, with HTTPS pull
  fallback. Push payloads must not contain note content or keys.
- One self-contained data directory that can be backed up while the server is
  stopped.
- Arbitrary encrypted binary attachments, with resumable uploads, immutable
  content-addressed storage, container-scoped deduplication, and a default
  maximum size of approximately 25 MB. Deterministic/convergent encryption is
  acceptable.

## Security requirements

- All communication except FCM/APNs delivery uses HTTPS.
- The server must never receive plaintext note content, attachment content, or
  private encryption keys.
- Device enrollment starts on the authenticated website and transfers a
  device-wrapped key through a QR code or deep link.
- Device revocation removes its server-side key envelopes and requests a
  best-effort local key and data wipe on its next connection.
- Team membership changes rotate keys for future content. Previously accessed
  plaintext cannot be recalled.
- Recovery uses an exported recovery code. Using it revokes all device keys
  and all active web sessions; devices must be enrolled again.
- Administrators may manage users, quotas, backups, and audit logs, but cannot
  decrypt user content.

## Data and synchronization

- Use SQLite for metadata and an encrypted, content-addressed filesystem blob
  store for notes and attachments.
- Clients maintain local encrypted stores and indexes. Full-text search is
  client-side.
- Each mutable object has a monotonically increasing version. Updates are
  accepted only when the client's base version matches the current version.
- Conflicting encrypted uploads are rejected and preserved for manual review;
  there is no automatic merge.
- Attachments synchronize separately from notes. A note saves without a
  completed attachment upload; clients download attachments lazily.
- Incomplete uploads expire after 15 minutes. Deleted notes release all current
  and historical attachment references for globally controlled, configurable
  garbage collection, which administrators may disable.
- HTTPS request/response APIs provide synchronization. Push or polling tells a
  client that changes are available; the client then pulls ciphertext.

## Later phases

Templates, web clipping, freehand drawing, richer media editing, public
publishing, and desktop clients are out of the MVP.
