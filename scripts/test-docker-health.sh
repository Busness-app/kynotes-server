#!/bin/sh
set -eu

image=${1:-kynotes-server:dev}
docker build -t "$image" .

# Bind mounts retain host ownership. Run as the invoking non-root user so the
# fixture models an operator-owned data volume without weakening the image's
# non-root default.
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD/testdata/config-good:/data" "$image" --check-config
if docker run --rm --user "$(id -u):$(id -g)" -v "$PWD/testdata/config-bad:/data" "$image" --check-config; then
	printf '%s\n' 'bad configuration unexpectedly passed' >&2
	exit 1
fi
