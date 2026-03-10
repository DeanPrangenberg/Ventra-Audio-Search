#!/usr/bin/env bash
set -euo pipefail

set -a
source .env
set +a

: "${WHISPER_REPLICAS:=1}"
: "${EMBEDDING_MODEL:=nomic-embed-text-v2-moe}"
: "${LLM_MODEL:=dolphin-mistral:7b}"
: "${DEACTIVATE_LLM:=true}"

LOCAL_BUILD_SERVICES=(
  "api"
  "frontend"
)

echo "Stopping and removing existing Docker Compose services..."
docker compose down --remove-orphans

echo "Rebuilding local Docker images without cache..."
docker compose build --no-cache --pull "${LOCAL_BUILD_SERVICES[@]}"

echo "Starting Docker Compose services with forced recreation..."
docker compose up -d \
  --force-recreate \
  --remove-orphans \
  --scale whisper="${WHISPER_REPLICAS}"

echo "Checking running whisper replicas..."
ACTUAL_WHISPER_REPLICAS="$(docker compose ps -q whisper | wc -l | tr -d ' ')"
if [[ "${ACTUAL_WHISPER_REPLICAS}" != "${WHISPER_REPLICAS}" ]]; then
  echo "ERROR: Expected ${WHISPER_REPLICAS} whisper replicas, but found ${ACTUAL_WHISPER_REPLICAS}."
  docker compose ps
  exit 1
fi

echo "Waiting for Ollama to be ready..."
for i in {1..600}; do
  if docker compose exec -T ollama ollama list >/dev/null 2>&1; then
    echo "Ollama is ready."
    break
  fi

  echo "Waiting for Ollama..."
  sleep 2

  if [[ "$i" -eq 600 ]]; then
    echo "ERROR: Ollama did not become ready in time."
    exit 1
  fi
done

echo "Downloading required Ollama embedding model..."
docker compose exec -T ollama ollama pull "${EMBEDDING_MODEL}"

if [[ "${DEACTIVATE_LLM,,}" != "true" ]]; then
  echo "Downloading LLM model..."
  docker compose exec -T ollama ollama pull "${LLM_MODEL}"
else
  echo "Skipping LLM model pull because DEACTIVATE_LLM=true"
fi

echo ""
echo "===================================================="
echo "= Setup complete. All services are up and running. ="
echo "===================================================="
echo ""