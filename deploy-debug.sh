#!/bin/bash

# Deploy debug version to linht

set -e

REMOTE_HOST="root@10.17.17.17"
REMOTE_DIR="/opt/linht-web"
BINARY_NAME="linht-web"
BUILD_DIR="./build"

GOOS=linux GOARCH=arm64 go build -o "${BUILD_DIR}/${BINARY_NAME}" main.go
ssh "${REMOTE_HOST}" "systemctl stop linht-web || true"

ssh "${REMOTE_HOST}" "rm -rf ${REMOTE_DIR}/*"
ssh "${REMOTE_HOST}" "mkdir -p ${REMOTE_DIR}"

scp "${BUILD_DIR}/${BINARY_NAME}" "${REMOTE_HOST}:${REMOTE_DIR}/"
scp -r web "${REMOTE_HOST}:${REMOTE_DIR}/"
scp config.yaml "${REMOTE_HOST}:${REMOTE_DIR}/"

ssh "${REMOTE_HOST}" "systemctl start linht-web"

echo "Done"