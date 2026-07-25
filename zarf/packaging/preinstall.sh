#!/bin/sh
# Create the system user before any files land, so postinstall can chown to it.
set -e

if ! getent group aether >/dev/null; then
    addgroup --system aether
fi

if ! getent passwd aether >/dev/null; then
    # The home dir must exist before adduser or it warns; postinstall fixes up
    # ownership and mode afterwards.
    mkdir -p /var/lib/aether
    adduser --system --ingroup aether --no-create-home \
        --home /var/lib/aether --shell /usr/sbin/nologin \
        --gecos "Aether music server" aether
fi
