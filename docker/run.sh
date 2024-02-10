#!/usr/bin/env bash

SCRIPT_NAME=$(basename "${0}")
SCRIPT_DIRECTORY_PATH=$(dirname "${0}")

cd "${SCRIPT_DIRECTORY_PATH}"

version=latest

IMAGE=${1-jddwoody/album:$version}
CONTAINER=${2-album}
COMMAND="docker"

${COMMAND} stop ${CONTAINER} || true && ${COMMAND} rm -f ${CONTAINER} || true
${COMMAND} run \
    --hostname ${CONTAINER} \
    --name ${CONTAINER} \
    -p9000:9000 \
    -v /home/jdw/testalbum:/albums \
    -d \
    ${IMAGE}

