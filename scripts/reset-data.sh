#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "$(realpath -- "$0")")" && pwd -P)"
PARENT_DIR="$(dirname -- "$SCRIPT_DIR")"
DATA_DIR="$PARENT_DIR/.data"

echo "script dir:  $SCRIPT_DIR"
echo "parent dir:  $PARENT_DIR"
echo "data dir:    $DATA_DIR"

echo "Removing qdrant data"
sudo rm -rf -- "$DATA_DIR/qdrant"

echo "Removing SQLite Data (backend/sqlite)"
sudo rm -rf -- "$DATA_DIR/backend/sqlite"

echo "Removing Uploaded Frontend Files"
sudo rm -rf -- "$DATA_DIR/frontend/uploads"

echo "Removing Uploaded Backend Files"
sudo rm -rf -- "$DATA_DIR/backend/downloaded_audios"

