#!/bin/sh
# Purge removes configuration and the system user; plain remove keeps both, and
# /var/lib/aether (the music database) is never deleted automatically — the
# admin can drop it by hand.
set -e

if [ -d /run/systemd/system ]; then
    systemctl daemon-reload || true
fi

if [ "$1" = "purge" ]; then
    rm -f /etc/aether/config.yaml
    rmdir --ignore-fail-on-non-empty /etc/aether 2>/dev/null || true

    if getent passwd aether >/dev/null; then
        deluser --system aether || true
    fi
    if getent group aether >/dev/null; then
        delgroup --system aether || true
    fi

    echo "aether: /var/lib/aether was kept; remove it manually to delete the database and cached artwork."
fi
