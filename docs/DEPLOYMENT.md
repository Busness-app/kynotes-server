# KyNotes Server deployment

Run the container behind an HTTPS reverse proxy on the same Docker network. The
embedded web client is served at `/`; `/api/` is served by the same process.
The compose service does not publish a host port. Set `server.behind_proxy`
and list only the proxy networks in `trusted_proxies`; the proxy must forward
the client address through `X-Forwarded-For`.

For local/LAN access without a reverse proxy, use the development override:

```bash
docker compose -f docker-compose.yml -f docker-compose.local.yml up -d --build
```

It publishes `8081` on the host and forwards it to the server's internal
port `8080`. Do not use this override for an internet-facing deployment.

The default compose file uses the named `kynotes-data` volume for `/data`.
Restart without `--volumes` to preserve notes:

```bash
docker compose -f docker-compose.yml -f docker-compose.local.yml down
docker compose -f docker-compose.yml -f docker-compose.local.yml up -d --build
```

Do not run `docker compose down --volumes` unless you intentionally want to
delete the KyNotes database and encrypted blobs. Confirm the mount before a
maintenance restart:

```bash
docker inspect "$(docker compose -f docker-compose.yml -f docker-compose.local.yml ps -q kynotes)" --format '{{range .Mounts}}{{.Destination}} <- {{.Name}}{{"\n"}}{{end}}'
```

The server applies read-header, read, write, idle, and graceful-shutdown
timeouts from the configuration. Keep the default non-root container,
read-only root filesystem, dropped capabilities, and `/data`/`/tmp` writable
mounts.

## Accounts

`kynotes-server user add --username <name> --password <pass> [--admin]` creates an
account and prints its recovery code once:

```
recovery code: xxxx-xxxx-xxxx
```

Store it offline. It is single-use, it unlocks the account without the password,
and the server keeps only an Argon2id hash of it. `POST /api/v1/auth/recover`
consumes it and returns a fresh one in the response.

Passwords are stored as Argon2id (64 MiB, t=3, p=4). Databases created before this
change hold scrypt verifiers, which are refused; recreate them.

## Backup settings

| Variable | Default | Meaning |
|---|---|---|
| `KYNOTES_BACKUP_DIR` | empty | Directory for sealed `.kycap` copies. The container is read-only, so mount one (`./backups:/backups`) and point this at it. Empty disables local copies. |
| `KYNOTES_BACKUP_KEEP` | `7` | Newest N local copies kept. Must be at least 1. |
| `KYNOTES_BACKUP_DEPOSIT_INTERVAL` | `24h` | Default schedule. `0` disables; the floor is `15m`. The admin UI setting overrides this. |
| `KYNOTES_BACKUP_ALLOW_PRIVATE_RECOVERY` | `false` | Admit a KyRecovery on a private or carrier-grade NAT address. HTTPS is still required; loopback, link-local and reserved ranges stay refused. |
| `KYNOTES_DNS` | unset | Only with `docker-compose.lan-dns.yml`: the LAN resolver the container uses, for a KyRecovery that resolves only there. A value in `.env` alone does nothing; pass it on the command line and recreate the container. |

```bash
KYNOTES_DNS=192.168.1.1 docker compose -f docker-compose.yml -f docker-compose.lan-dns.yml up -d
docker inspect KyNotes-Server --format '{{.HostConfig.Dns}}'
```

Sealed capsule backups and the attachment mirror land in a following release; the
settings above are read today and the plaintext copy below remains the backup path.

## Plaintext copy

Back up with the server stopped: stop the container, copy `/data` as one unit,
then start it. Restore only while stopped, run `integrity-check` and
`consistency-check`, and start the server after both checks pass.

Migrations run at startup. Make a backup before upgrades. To roll back, stop
the server, restore the backup directory, run both integrity checks, and start
again. The server holds a data-directory lock while running, so backup and
restore commands must be run against a stopped service.
