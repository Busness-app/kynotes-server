# Logging

KyNotes must emit structured, privacy-safe application logs to standard output
and standard error. It must not build or require a KyNotes-specific log
database, log search system, or long-term retention service.

Operators may route container logs to an existing platform such as Loki,
OpenSearch, Elasticsearch, Graylog, or another OpenTelemetry-compatible
collector.

Log authentication outcomes, device pairing and revocation, sync conflicts,
key-envelope changes, publishing actions, quota events, and administrative
actions. Use request IDs and coarse actor identifiers where useful.

Never log note content, task text, attachment contents, workbook or project
names, encryption keys, session tokens, or raw request bodies. Audit records
must remain content-blind.

Do not add an embedded log database or product-specific log viewer. Operators
should use their existing logging platform for search, alerting, retention, and
access control.
