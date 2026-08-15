# KyNotes Server deployment

Run the container behind an HTTPS reverse proxy on the same Docker network. The
compose service does not publish a host port. Set `server.behind_proxy` and
list only the proxy networks in `trusted_proxies`; the proxy must forward the
client address through `X-Forwarded-For`.

The server applies read-header, read, write, idle, and graceful-shutdown
timeouts from the configuration. Keep the default non-root container,
read-only root filesystem, dropped capabilities, and `/data`/`/tmp` writable
mounts.

Back up with the server stopped: stop the container, copy `/data` as one unit,
then start it. Restore only while stopped, run `integrity-check` and
`consistency-check`, and start the server after both checks pass.

Migrations run at startup. Make a backup before upgrades. To roll back, stop
the server, restore the backup directory, run both integrity checks, and start
again. The server holds a data-directory lock while running, so backup and
restore commands must be run against a stopped service.
