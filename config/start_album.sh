#!/bin/sh

SCRIPT_NAME=$(basename "${0}")
SCRIPT_DIRECTORY_PATH=$(dirname "${0}")

cd "${SCRIPT_DIRECTORY_PATH}"

printf 'album container user: %s (uid=%s gid=%s)\n' "$(id -un)" "$(id -u)" "$(id -g)"

exec ./album
