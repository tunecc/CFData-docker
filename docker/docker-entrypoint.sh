#!/bin/sh
set -eu

SOURCE=/app/cfdata
TARGET=/data/cfdata
APP_UID="${PUID:-10001}"
APP_GID="${PGID:-10001}"

if [ "$(id -u)" = "0" ] && [ "$APP_UID" != "0" ]; then
    if [ ! -d /data ]; then
        echo "cfdata: /data 不存在或未正确挂载" >&2
        exit 1
    fi
    if [ "$(stat -c '%u:%g' /data)" != "$APP_UID:$APP_GID" ]; then
        if ! chown -R "$APP_UID:$APP_GID" /data; then
            echo "cfdata: 无法修正 /data 属主为 $APP_UID:$APP_GID（只读挂载？）" >&2
            exit 1
        fi
    fi
    exec su-exec "$APP_UID:$APP_GID" "$0" "$@"
fi

if [ ! -w /data ]; then
    echo "cfdata: /data 不可写（当前 UID $(id -u):$(id -g)）；请调整挂载目录权限，或用 PUID/PGID 指定可写用户" >&2
    exit 1
fi

if [ ! -f "$TARGET" ] || [ ! -x "$TARGET" ] || ! cmp -s "$SOURCE" "$TARGET"; then
    if ! tmp="$(mktemp "${TARGET}.tmp.XXXXXX" 2>/dev/null)"; then
        echo "cfdata: 无法在 /data 创建临时文件，请检查目录是否可写" >&2
        exit 1
    fi
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
