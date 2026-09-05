CREATE TABLE blob_replicas (
  digest TEXT PRIMARY KEY REFERENCES blobs(digest) ON DELETE CASCADE,
  target TEXT NOT NULL,
  uploaded_at TEXT NOT NULL
);
CREATE INDEX blob_replicas_target ON blob_replicas(target);
