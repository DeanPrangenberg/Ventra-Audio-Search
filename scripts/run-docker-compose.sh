#!/usr/bin/env bash
set -euo pipefail

set -a
source .env
set +a

: "${WHISPER_REPLICAS:=1}"
: "${EMBEDDING_MODEL:=nomic-embed-text-v2-moe}"
: "${LLM_MODEL:=dolphin-mistral:7b}"
: "${DEACTIVATE_LLM:=true}"

INITIAL_WHISPER_REPLICAS=1
TARGET_WHISPER_REPLICAS="${WHISPER_REPLICAS}"

LOCAL_BUILD_SERVICES=(
  "api"
  "frontend"
)

echo "Stopping and removing existing Docker Compose services..."
docker compose down --remove-orphans

echo "Rebuilding local Docker images without cache..."
docker compose build --no-cache --pull "${LOCAL_BUILD_SERVICES[@]}"

echo "Starting base services with a single whisper replica for model initialization..."
docker compose up -d \
  --force-recreate \
  --remove-orphans \
  --scale whisper="${INITIAL_WHISPER_REPLICAS}" \
  postgres qdrant ollama whisper

echo "Checking initial whisper replica..."
ACTUAL_WHISPER_REPLICAS="$(docker compose ps -q whisper | wc -l | tr -d ' ')"
if [[ "${ACTUAL_WHISPER_REPLICAS}" != "${INITIAL_WHISPER_REPLICAS}" ]]; then
  echo "ERROR: Expected ${INITIAL_WHISPER_REPLICAS} initial whisper replica, but found ${ACTUAL_WHISPER_REPLICAS}."
  docker compose ps
  exit 1
fi

echo "Waiting for whisper to be ready..."
for i in {1..600}; do
  WHISPER_CID="$(docker compose ps -q whisper | head -n1)"

  if [[ -z "${WHISPER_CID}" ]]; then
    echo "Waiting for whisper container to appear..."
    sleep 2
    continue
  fi

  WHISPER_STATE="$(docker inspect -f '{{.State.Status}}' "${WHISPER_CID}" 2>/dev/null || true)"
  if [[ "${WHISPER_STATE}" != "running" ]]; then
    echo "Waiting for whisper container to stay running..."
    sleep 2
    continue
  fi

  if docker compose exec -T whisper sh -lc 'ffmpeg -version >/dev/null 2>&1 && test -s /app/models/ggml-*.bin' >/dev/null 2>&1; then
    echo "Whisper model file exists."
    break
  fi

  echo "Waiting for whisper model download / initialization..."
  sleep 2

  if [[ "$i" -eq 600 ]]; then
    echo "ERROR: Whisper did not become ready in time."
    docker compose logs --tail=200 whisper
    exit 1
  fi
done

echo "Checking whisper logs for model load errors..."
if docker compose logs whisper 2>&1 | grep -q "invalid model data (bad magic)"; then
  echo "ERROR: Whisper model file is corrupted (bad magic)."
  echo "Delete the whisper volume and rerun:"
  echo "  docker compose down"
  echo "  docker volume rm \$(docker volume ls -q | grep whisper-models || true)"
  exit 1
fi

if docker compose logs whisper 2>&1 | grep -q "failed to initialize whisper context"; then
  echo "ERROR: Whisper failed to initialize."
  docker compose logs --tail=200 whisper
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

if [[ "${TARGET_WHISPER_REPLICAS}" != "${INITIAL_WHISPER_REPLICAS}" ]]; then
  echo "Scaling whisper from ${INITIAL_WHISPER_REPLICAS} to ${TARGET_WHISPER_REPLICAS}..."
  docker compose up -d --scale whisper="${TARGET_WHISPER_REPLICAS}" whisper

  ACTUAL_WHISPER_REPLICAS="$(docker compose ps -q whisper | wc -l | tr -d ' ')"
  if [[ "${ACTUAL_WHISPER_REPLICAS}" != "${TARGET_WHISPER_REPLICAS}" ]]; then
    echo "ERROR: Expected ${TARGET_WHISPER_REPLICAS} whisper replicas, but found ${ACTUAL_WHISPER_REPLICAS}."
    docker compose ps
    exit 1
  fi
fi

echo "Starting api and frontend after base services are ready..."
docker compose up -d api frontend

echo ""
echo "===================================================="
echo "= Setup complete. All services are up and running. ="
echo "===================================================="
echo ""