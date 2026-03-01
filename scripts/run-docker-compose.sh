#!/usr/bin/env bash
set -euo pipefail

LOCAL_BUILD_SERVICES=(
  "audio_transcript_server"
  "audio_transcript_frontend"
)

CONTAINERS=(
  "ollama"
  "whisper-server"
  "qdrant"
  "audio-transcript-server"
  "audio-transcript-frontend"
)

echo "Stopping and removing existing Docker Compose services..."
docker compose down --remove-orphans

echo "Rebuilding local Docker images without cache..."
docker compose build --no-cache --pull "${LOCAL_BUILD_SERVICES[@]}"

echo "Starting Docker Compose services with forced recreation..."
docker compose up -d --force-recreate --remove-orphans

echo "Docker Compose services started."

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
