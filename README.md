# Ventra Audio Search

Ventra Audio Search is a containerized audio ingestion and semantic search platform.
It processes audio files into transcripts, generates embeddings and AI metadata, and enables hybrid retrieval using keyword + vector search.

## Features

- Import audio via direct file URL or base64 payload
- Asynchronous processing pipeline (persist, transcribe, embed, AI enrichment)
- Whisper-based transcription
- Ollama-based embeddings and LLM summarization/keywords
- Hybrid search: full-text candidate selection + Qdrant reranking
- Gradio frontend with Import, Search, Statistics, and Config routes
- Persistent data volumes for Postgres, Qdrant, models, and app data

## Use Cases
- Meeting recording search and summarization
- Podcast indexing and discovery
- Lecture capture retrieval for education
- Other audio content management scenarios where semantic search can enhance retrieval and insights


## Architecture

Core services are orchestrated with Docker Compose:

- `audio_transcript_frontend` (Gradio UI)
- `audio_transcript_server` (Go REST API + workers)
- `postgres` (metadata, transcripts, segments)
- `qdrant` (vector search index)
- `whisper-nvidia` (speech-to-text)
- `ollama` (embedding + LLM inference)

Processing flow:

1. Import request is validated and queued in Postgres.
2. Worker persists or downloads audio and normalizes metadata.
3. Audio is transcribed into full transcript + segments.
4. Segment embeddings are generated and stored in Qdrant.
5. AI summary and keywords are generated and stored.
6. Search combines lexical candidates with semantic reranking.

## Tech Stack

- Backend: Go (`net/http`, `pgx`, Qdrant client)
- Frontend: Python + Gradio
- Databases: PostgreSQL + Qdrant
- AI services: Whisper Server + Ollama
- Orchestration: Docker Compose

## Prerequisites

- Docker Engine
- NVIDIA Container Toolkit (required for GPU services in this stack)
- Linux environment with permission to run Docker

## Hardware

- GPU: NVIDIA GPU with at least `12GB VRAM` for default Ollama configuration (nomic-embed-text, dolphin-mistral:7b) (for dev used a 3060 with 12GB VRAM)
- CPU: Modern multi-core CPU for backend processing
- Disk: Audio files, models, and databases can consume significant disk space, especially with large models and many audio files. Ensure sufficient storage is available.

## Quick Start

```bash
git clone https://github.com/DeanPrangenberg/Ventra-Audio-Search.git
cd Ventra-Audio-Search
chmod +x scripts/run-docker-compose.sh
./scripts/run-docker-compose.sh
```

The setup script:

- rebuilds local backend/frontend images,
- starts all services,
- waits for readiness,
- pulls required Ollama models (`nomic-embed-text`, `dolphin-mistral:7b`).
    - Note: Ollama model pull may take time on first run due to large model sizes.
    - Models are modular and can be customized by changing `OLLAMA_MODELS` in `scripts/run-docker-compose.sh` and `docker-compose.yml`.
## Service Endpoints

After startup, the default endpoints are:

- Frontend UI: `http://localhost:7860`
- Backend API: `http://localhost:8880`
- Qdrant REST/UI: `http://localhost:6333`
- Whisper API: `http://localhost:9001`
- Whisper Docs: `http://localhost:9002`
- Ollama API: `http://localhost:8884`
- Postgres: `localhost:5432`

## API Reference

### Health

```bash
curl -s http://localhost:8880/health
```

### Import

```bash
curl -X POST http://localhost:8880/import \
  -H "Content-Type: application/json" \
  -d '[
    {
      "title": "Sprint Planning",
      "recording_date": "2026-03-01T09:00:00Z",
      "category": "Engineering",
      "audio_type": "meeting",
      "duration_in_sec": 1254,
      "user_summary": "Weekly sprint planning call",
      "file_url": "https://example.com/audio.mp3"
    }
  ]'
```

Required fields per item:

- `title`
- `user_summary`
- one of: `file_url` or `base64_data`

### Search

```bash
curl -X GET http://localhost:8880/search \
  -H "Content-Type: application/json" \
  -d '{
    "ts_query": "deadline AND release",
    "semantic_search_query": "When is the release deadline?",
    "category": "Engineering",
    "start_time_period_iso": "2026-01-01T00:00:00Z",
    "end_time_period_iso": "2026-12-31T23:59:59Z",
    "max_segment_return": 10
  }'
```

## Configuration

Key backend environment variables (defined in `docker-compose.yml`):

- `WHISPER_API_URL`
- `OLLAMA_API_URL`
- `QDRANT_API_HOST`
- `QDRANT_API_PORT_GRPC`
- `POSTGRES_URL`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`
- `LLM_MODEL`
- `EMBEDDING_MODEL`
- `EMBEDDING_MODEL_DIM`
- `LOG_LEVEL`

Key frontend variables:

- `PORT`
- `AUDIO_TRANSCRIPT_SERVER_URL`
- `FRONTEND_SERVER_URL`
- `POSTGRES_URL`
- `DATA_DIR`
- `FILE_CLEAN_UP`

## Repository Structure

```text
backend/     Go API, workers, DB and vector integrations
frontend/    Gradio application and API client
scripts/     Operational helper scripts
charts/      Architecture/flow diagrams (.drawio)
.data/       Local persistent runtime data
```
