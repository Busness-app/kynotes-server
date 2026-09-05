# KyNotes blob mirror: attachments to S3, SFTP, SMB or a local mount (Plan C)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Back up the attachment store, which the capsule never carries (decision 2026-09-05), by mirroring every blob to one off-box target, and pull them back on restore.

**Architecture:** Blobs are content-addressed (sha256 digest is the filename), immutable, and already ciphertext (the browser encrypts attachment bytes and metadata before upload; `AGENTS.md:104`). So the mirror is a set difference, not an archive: every digest in the `blobs` table that has no `blob_replicas` row gets one `Put` under its digest, and restore is one `Get` per digest the restored database expects. No tar, no incremental state beyond one table, safe to run twice. The target clients are copied from `kyrecovery-server/internal/replication` (S3 SigV4 in stdlib, SFTP with a pinned host key, SMB 2/3, local directory), each gaining a `Get`. Credentials come from config or environment like every other kynotes secret; nothing is stored in the database.

**Tech Stack:** Go 1.26.6; stdlib for S3 and local; `github.com/pkg/sftp` + `golang.org/x/crypto/ssh` for SFTP; `github.com/hirochachacha/go-smb2` for SMB (the same pins kyrecovery-server uses).

**Spec:** Yoshi, 2026-09-05: "rename them, and give them the ability to send data elsewhere. Possibly steal backup code from KyRecovery." Board folder `kynotes-kyrecovery-deposit`. Plan B (`2026-09-04-kynotes-kyrecovery-backup.md`) renames the plaintext commands and excludes attachments from the capsule; this plan is the other half.

## Global Constraints

- The whole-directory copy (`copy-data-dir`) stays local-only. It contains the plaintext database and the secret keys; those go off-box only inside a sealed capsule. What this plan sends elsewhere is the blob directory, which is ciphertext. Say this in the docs.
- One target per instance. `KYNOTES_BLOB_TARGET` is a URL: `s3://bucket/prefix`, `sftp://user@host:22/dir`, `smb://host/share/dir`, or `file:///mnt/backup/kynotes-blobs`. Credentials in `KYNOTES_BLOB_TARGET_ACCESS_KEY`, `KYNOTES_BLOB_TARGET_SECRET` (S3 secret, SFTP password or PEM private key, SMB password), `KYNOTES_BLOB_TARGET_HOST_KEY` (SFTP SHA256 fingerprint, required), `KYNOTES_BLOB_TARGET_S3_ENDPOINT`, `KYNOTES_BLOB_TARGET_S3_REGION`. Same keys under `backup.blob_target*` in the YAML.
- The SFTP client refuses an unpinned host (`UnknownHostKeyError` names the fingerprint the server presented). No pin, no upload. Copy this rule, do not soften it.
- The SMB client's known limitation (a server that grants a guest session unsigned can swallow uploads) is documented in kyrecovery's README; copy the paragraph and its advice into `docs/DEPLOYMENT.md`.
- Mirror never deletes remotely. Blob GC removes local files; the remote copy is history until an operator prunes it by hand. (`fetch-blobs` only pulls digests the database names, so remote leftovers are harmless.)
- A `Put` that fails leaves no `blob_replicas` row; the next run retries. A blob that was uploaded but whose row write failed is re-uploaded, which is idempotent for every target.
- Every run writes one audit row `admin.blob_mirror_run` with uploaded, skipped and failed counts and the first failure's text through `recoveryclient.AuditSafe`.
- Branch `feat/blob-mirror` in worktree `.claude/worktrees/mirror`. Depends on Plan A (audit helper, config section) and Plan B Task 5 (`storage.RecordAuditOutcome`, the backup loop). Can be built in parallel with Plan B Tasks 2 through 4 and merged after.

## File map

| File | Responsibility |
|---|---|
| `internal/offsite/target.go` (fallback only; preferred: `ky-primitives/offsite`) | `Target` interface `{Put(ctx, name string, r io.Reader, size int64) error; Get(ctx, name string) (io.ReadCloser, error); Test(ctx) error}`; `Parse(cfg config.BlobTarget) (Target, error)` |
| `internal/offsite/s3.go` (from kyrecovery `s3.go`) | SigV4 `PutObject` + new `GetObject` (signed GET, `UNSIGNED-PAYLOAD` hash) |
| `internal/offsite/sftp.go` (from kyrecovery) | pinned host key, temp+rename `Put`, `Get` via `client.Open` |
| `internal/offsite/smb.go` (from kyrecovery) | SMB 2/3 with signing, `Put`, `Get` via `share.Open` |
| `internal/offsite/local.go` | directory target: temp+rename `Put`, `os.Open` `Get` |
| `internal/offsite/*_test.go` (from kyrecovery) | in-process SFTP and SMB servers, stalled-server budget, host-key refusal, PEM-never-as-password |
| `internal/storage/migrations/0014_blob_replicas.sql` | `blob_replicas(digest TEXT PRIMARY KEY REFERENCES blobs(digest) ON DELETE CASCADE, target TEXT NOT NULL, uploaded_at TEXT NOT NULL)` |
| `internal/mirror/mirror.go` | `Sync(ctx, db, blobs, target, targetKey) (Stats, error)`; `Fetch(ctx, db, blobs, target) (Stats, error)`; `Pending(db) (int, error)` |
| `cmd/kynotes-server/main.go` | `mirror-blobs`, `fetch-blobs --to <datadir>`, `test-blob-target` |
| `internal/app/serve.go` | mirror after each scheduled capsule run and on Back up now |
| `internal/httpapi/backup_routes.go`, `web/src/components/AdminBackup.tsx` | mirror status in the backup status body and a fact card |
| `docs/DEPLOYMENT.md`, `docs/RESTORE.md`, `README.md` | target setup, the SMB caveat, restore order |

---

### Task 1: Config

**Files:** `internal/config/config.go`, `internal/config/blob_target_test.go`, `kynotes.example.yaml`, `docker-compose.yml`

**Interfaces:**
```go
type BlobTarget struct {
	URL        string `yaml:"url"`          // KYNOTES_BLOB_TARGET
	AccessKey  string `yaml:"access_key"`   // KYNOTES_BLOB_TARGET_ACCESS_KEY (S3 key id, SFTP/SMB user when absent from the URL)
	Secret     string `yaml:"secret"`       // KYNOTES_BLOB_TARGET_SECRET
	HostKey    string `yaml:"host_key"`     // KYNOTES_BLOB_TARGET_HOST_KEY (sftp only, SHA256:... fingerprint)
	S3Endpoint string `yaml:"s3_endpoint"`  // KYNOTES_BLOB_TARGET_S3_ENDPOINT (R2, MinIO)
	S3Region   string `yaml:"s3_region"`    // KYNOTES_BLOB_TARGET_S3_REGION (default us-east-1)
}
```
added to `Backup` as `BlobTarget BlobTarget \`yaml:"blob_target"\``. `Validate` refuses a URL whose scheme is not one of `s3`, `sftp`, `smb`, `file`; refuses userinfo with a password (`user:pass@`); refuses `sftp` without `HostKey`; refuses `s3` without both credentials.

- [ ] **Step 1: Failing test** — table over: empty (ok), `s3://b/p` without creds (err), `sftp://u@h/d` without host key (err), `sftp://u:pw@h/d` (err names the secret field), `smb://h/share/dir` (ok), `file:///x` (ok), `ftp://x` (err).
- [ ] **Step 2: Run → compile error.**
- [ ] **Step 3: Implement** the struct, the env overlay lines (pattern from Plan A Task 8), the validation, the example YAML block, and the compose passthrough (`KYNOTES_BLOB_TARGET*` six variables with `:-` empty defaults).
- [ ] **Step 4: Commit** `config: blob mirror target`.

---

### Task 2: Targets from the library

**Landed:** ky-primitives PR #14 (merged 2026-09-05, `8d507fb`) ships `github.com/Busness-app/ky-primitives/offsite` as a nested module with `Put`/`Get`/`Test`, `Parse`, `Key`, `os.Root`-guarded local writes, canonical SMB names and `ErrObjectExists` on SMB overwrite. Tag `offsite/v0.1.0` pending. When that tag exists, this task is `go get github.com/Busness-app/ky-primitives/offsite@offsite/v0.1.0` and `internal/offsite` is not created; `mirror` imports the lib's `Target`, `Parse` and `Key` directly, and the `BlobTarget` config maps onto `offsite.Config`. The steps below are the fallback if the lib package is refused.

### Task 2 (fallback): Targets, copied from KyRecovery

**Files:** `internal/offsite/{target,s3,sftp,smb,local}.go`, `internal/offsite/{s3,sftp,smb,local}_test.go`

- [ ] **Step 1: Copy** `kyrecovery-server/internal/replication/{s3,sftp,smb}.go` and `{sftp,smb}_test.go` into `internal/offsite`, package `offsite`. Keep the file bodies; rename `.kyrecovery-ping` to `.kynotes-ping`. `go get github.com/pkg/sftp@v1.13.11 github.com/hirochachacha/go-smb2@v1.1.0` (the versions kyrecovery pins; check for newer with `go list -m -versions`). Run the copied tests: green before any edit.
- [ ] **Step 2: Failing tests for Get** — in each `_test.go`, after the existing Put round trip, `Get` the same name and compare bytes; `Get` of a missing name is `os.ErrNotExist` (wrap the SFTP/SMB/HTTP-404 cases onto it so `mirror.Fetch` can report "missing on target" uniformly).
- [ ] **Step 3: Implement**
  - `target.go`:
```go
type Target interface {
	Put(ctx context.Context, name string, r io.Reader, size int64) error
	Get(ctx context.Context, name string) (io.ReadCloser, error)
	Test(ctx context.Context) error
}

// Key is the stable identity of a target for blob_replicas rows: the URL
// without credentials, so a changed password does not re-upload everything and
// a changed bucket does.
func Key(cfg config.BlobTarget) string

func Parse(cfg config.BlobTarget) (Target, error) // dispatch on URL scheme
```
  - `s3.go`: adapt `PutObject(ctx, key, data, size, contentType)` to the interface (`Put` sets `application/octet-stream`); add `GetObject` as a signed `GET` with `x-amz-content-sha256: UNSIGNED-PAYLOAD` and the same canonical request shape (`GET\n<uri>\n\n<headers>\n<signed>\nUNSIGNED-PAYLOAD`); 404 → `os.ErrNotExist`. Test the signature with the AWS SigV4 test-suite vector for a GET if the copied tests have none; at minimum, an `httptest` server asserts `Authorization` has the expected `SignedHeaders` and that a 404 maps to `ErrNotExist`.
  - `sftp.go`: `Get` = `dial`, `client.Open(path.Join(Dir, name))`, return a closer that also runs cleanup; `sftp` "no such file" → `os.ErrNotExist`.
  - `smb.go`: `Get` = `mount`, `share.Open`, same closer pattern.
  - `local.go`: `Put` temp+rename at 0600 under the directory, `Get` = `os.Open`.
- [ ] **Step 4: Pass, commit** `offsite: S3, SFTP, SMB and local targets from KyRecovery, with Get`.

---

### Task 3: Mirror and fetch

**Files:** `internal/storage/migrations/0014_blob_replicas.sql`, `internal/mirror/mirror.go`, `internal/mirror/mirror_test.go`

**Interfaces:**
```go
type Stats struct { Uploaded, Skipped, Failed, Fetched, Missing int; FirstError string }
func Sync(ctx context.Context, db *sql.DB, blobs *blobstore.Store, t offsite.Target, targetKey string) (Stats, error)
func Fetch(ctx context.Context, db *sql.DB, blobs *blobstore.Store, t offsite.Target) (Stats, error)
func Pending(db *sql.DB, targetKey string) (int, error)
```
- `Sync`: `SELECT digest,size_bytes FROM blobs WHERE digest NOT IN (SELECT digest FROM blob_replicas WHERE target=?)`; for each, `blobs.Open(digest)` (which verifies the sha256), `t.Put(ctx, "blobs/"+digest, f, size)`, then `INSERT INTO blob_replicas`. A failed `Put` increments `Failed`, records `FirstError` once, continues. Returns an error only when every attempted upload failed and at least one was attempted, so a dead target is one failure row, not silence.
- `Fetch`: for each digest in `blobs` whose `blobs.Stat` says absent: `t.Get`, stream into `blobs.NewTemp(digest)` and `Finalize(digest)`, which refuses a mismatched sha256 (`blobstore.ErrDigestMismatch`). `os.ErrNotExist` from the target increments `Missing` and continues.

- [ ] **Step 1: Failing tests** with the local target in a temp dir:
```go
func TestSyncUploadsOnceAndSkipsNextTime(t *testing.T)   // two blobs → Uploaded 2; again → Skipped 2, Uploaded 0
func TestSyncFailedPutLeavesNoRowAndRetries(t *testing.T) // failing Target fake for one digest → Failed 1, no row; fixed → Uploaded 1
func TestSyncNewTargetKeyReuploads(t *testing.T)
func TestFetchRestoresMissingBlobsAndVerifiesDigest(t *testing.T) // delete local files, Fetch → present, sha256 ok; corrupt the remote copy of one → Failed 1 with ErrDigestMismatch, local file absent
func TestFetchReportsMissingOnTarget(t *testing.T)
func TestGCDeletesReplicaRowByCascade(t *testing.T)      // storage.RunGC on an unreferenced blob removes the blobs row; blob_replicas row goes with it
```
- [ ] **Step 2: Run → compile error.**
- [ ] **Step 3: Implement** as described; `Finalize` already does the digest check, do not duplicate it.
- [ ] **Step 4: Pass, commit** `mirror: content-addressed blob sync and fetch`.

---

### Task 4: CLI, scheduler, route, screen

**Files:** `cmd/kynotes-server/main.go`, `internal/app/serve.go`, `internal/backup/service.go`, `internal/httpapi/backup_routes.go`, `web/src/components/AdminBackup.tsx`, `web/src/api.ts`

- [ ] **Step 1: CLI**
  - `mirror-blobs` → `offsite.Parse`, `mirror.Sync`, print the stats, audit through `storage.RecordAuditOutcome(db, "system", "admin.blob_mirror_run", offsite.Key(cfg), outcome, reason, "cli")`, exit 1 when `Failed > 0`.
  - `fetch-blobs [--config]` → runs against the restored data dir after `restore` (Plan B); `mirror.Fetch`; prints fetched/missing; exit 1 when `Missing > 0` or any failure; then tells the operator to run `consistency-check`.
  - `test-blob-target` → `Target.Test`; for SFTP without a pin, prints the fingerprint from `UnknownHostKeyError` and the exact env var to set, exit 2.
- [ ] **Step 2: Scheduler** — in `runBackupLoop` (Plan B Task 5), after the capsule `Run` returns, when `cfg.Backup.BlobTarget.URL != ""`: `mirror.Sync` on the same `WithoutCancel` context, one audit row. A missing target is a startup log line once (`backup: no blob mirror target; attachments are not backed up`), never a per-tick message.
- [ ] **Step 3: Service and route** — `Service.MirrorNow(ctx) (mirror.Stats, error)`; `POST /api/v1/admin/backup/deposit` calls it after `Run` and includes `mirror` in the response body; `Status()` gains `"mirror": {"configured": bool, "target": offsite.Key(cfg) or "", "pending": n, "last_run": <from the last audit row or a setting "blob_mirror_last">}`. Add `POST /api/v1/admin/backup/mirror` † (step-up) for a mirror-only run. Add it to the hardening test table.
- [ ] **Step 4: Screen** — the attachments line from Plan B Task 6 becomes a fifth fact card "Attachments": target kind and host (never credentials), pending count, last run; warning when not configured; a "Mirror now" button in the action row.
- [ ] **Step 5: Tests** — route test for the new endpoint and the status field; vitest for the card's three states (unconfigured, pending > 0, clean).
- [ ] **Step 6: Gate, dist, commit** `mirror: CLI, scheduler, route and screen`.

---

### Task 5: Docs

- [ ] **Step 1:** `docs/DEPLOYMENT.md`: a "Attachment mirror" section: the six variables, one example per scheme, the SFTP pinning walkthrough (`test-blob-target` prints the fingerprint; compare with `ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub` on the server; set the variable), the SMB caveat paragraph copied from kyrecovery's README, and the sentence that the mirror carries ciphertext blobs only and the whole-directory copy never leaves the host.
- [ ] **Step 2:** `docs/RESTORE.md`: after the capsule restore step, `kynotes-server fetch-blobs`, then `consistency-check`; what `Missing > 0` means (a blob the database names that the mirror never received: the last mirror run before the disaster was earlier than the upload) and that the note still opens with a broken attachment.
- [ ] **Step 3:** `README.md`: one paragraph under backups.
- [ ] **Step 4:** Commit `docs: attachment mirror`.

---

### Task 6: Prove it, PR

- [ ] **Step 1:** Gate (gofmt, vet, `go test -race ./...`, web build and tests, dist committed, the CI probe).
- [ ] **Step 2:** Live with `file://` on a scratch data dir: upload two attachments from the screen, "Mirror now", two files appear under the target, `blob_replicas` has two rows, second run uploads nothing. Delete `blobs/` locally, `fetch-blobs`, both back and `consistency-check` clean.
- [ ] **Step 3:** Live against one real remote in the homelab (whichever Yoshi has: SFTP to a NAS is the likely one). Pin the host key through `test-blob-target`. Mirror, then fetch into an empty data dir restored from a capsule.
- [ ] **Step 4:** PR via the `pull-request` skill; post rounds to `kynotes-kyrecovery-deposit`.

---

## Self-review

- The ask: rename (Plan B T5), send data elsewhere (this plan), steal from KyRecovery (T2 copies four files and two test files verbatim, adding only `Get`). Narrowing: the directory copy itself is not sent elsewhere, because it holds plaintext secrets; the blobs are what need an off-box home and they are ciphertext. Stated in Global Constraints and in the docs task.
- Not built: multiple targets, remote pruning, bandwidth limits. Each is a later folder if wanted.
- Names: `offsite.Target`, `offsite.Parse`, `offsite.Key` (T2 → T3, T4); `mirror.Sync`, `mirror.Fetch`, `mirror.Pending`, `mirror.Stats` (T3 → T4); `storage.RecordAuditOutcome` from Plan B T5; `Service.MirrorNow` (T4).
