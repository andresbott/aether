#!/bin/sh
# Set up the data directory and register the service. Runs on install and on
# upgrade, so everything here must be idempotent.
set -e

# DataDir from the shipped config: the SQLite DB, cached artwork and task logs.
# 0750 keeps it private to the aether user and group.
mkdir -p /var/lib/aether
chown aether:aether /var/lib/aether
chmod 0750 /var/lib/aether

# The config dir is world-readable, but API-key files referenced with "@<path>"
# are not — restrict group access to aether so it can still read them.
chown root:aether /etc/aether
chmod 0750 /etc/aether
if [ -f /etc/aether/config.yaml ]; then
    chown root:aether /etc/aether/config.yaml
    chmod 0640 /etc/aether/config.yaml
fi

if [ -d /run/systemd/system ]; then
    systemctl daemon-reload || true
    if [ "$1" = "configure" ] && [ -z "$2" ]; then
        # First install: enable and start, per Debian convention.
        systemctl enable --now aether.service || true
    elif systemctl is-active --quiet aether.service; then
        # Upgrade: respect the admin's enable/disable choice, restart into the
        # new binary only if it was already running.
        systemctl restart aether.service || true
    fi
fi
