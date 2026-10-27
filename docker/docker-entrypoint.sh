#!/bin/sh
set -eu

SOURCE=/app/cfdata
TARGET=/data/cfdata

if [ ! -w /data ]; then
    echo "cfdata: /data 不可写，请检查卷挂载属主（需要 UID/GID 10001）" >&2
    exit 1
fi

if [ ! -f "$TARGET" ] || [ ! -x "$TARGET" ] || ! cmp -s "$SOURCE" "$TARGET"; then
    tmp="$(mktemp "${TARGET}.tmp.XXXXXX")"
    trap 'rm -f "$tmp"' EXIT
    if ! cp "$SOURCE" "$tmp"; then
        echo "cfdata: 无法同步二进制到 $TARGET" >&2
        exit 1
    fi
    chmod 0755 "$tmp"
    mv -f "$tmp" "$TARGET"
    trap - EXIT
fi

cd /data
exec "$TARGET" "$@"
