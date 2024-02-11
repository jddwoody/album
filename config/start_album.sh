#!/usr/bin/env bash

SCRIPT_NAME=$(basename "${0}")
SCRIPT_DIRECTORY_PATH=$(dirname "${0}")

cd "${SCRIPT_DIRECTORY_PATH}"

./album 2>&1 > logs/albums.log
