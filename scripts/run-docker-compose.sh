#!/usr/bin/env bash
set -euo pipefail

echo "Starting Docker Compose services..."
docker compose up -d --build --force-recreate
echo "Docker Compose services are running."

CONTAINERS=("ollama" "whisper-server" "qdrant" "audio-transcript-server" "audio-transcript-frontend")

echo "Waiting for containers to be running..."
for C in "${CONTAINERS[@]}"; do
  while true; do
    STATUS="$(docker inspect -f '{{.State.Status}}' "$C" 2>/dev/null || true)"
    if [[ "$STATUS" == "running" ]]; then
      echo "$C is running."
      break
    fi
    echo "Waiting for $C to start..."
    sleep 2
  done
done

echo "Waiting for Ollama to be ready..."
for i in {1..600}; do
  # Wenn der Server bereit ist, geht "ollama list" ohne Fehler durch.
  if docker exec ollama sh -lc 'ollama list >/dev/null 2>&1'; then
    echo "Ollama is ready."
    break
  fi

  echo "Waiting for Ollama (this can take a while)..."
  sleep 2

  if [[ "$i" -eq 600 ]]; then
    echo "ERROR: Ollama did not become ready in time."
    exit 1
  fi
done

echo "Downloading necessary Ollama models..."
docker exec ollama sh -lc 'ollama pull nomic-embed-text && ollama pull dolphin-mistral:7b'
echo "Ollama models downloaded."

echo ""
echo "===================================================="
echo "= Setup complete. All services are up and running. ="
echo "===================================================="
echo ""