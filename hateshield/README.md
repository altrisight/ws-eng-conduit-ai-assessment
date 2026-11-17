# HateShield

Multimodal hate-speech detection: social posts, audio, and video.

## Layout

| Path | Stack | Role |
|---|---|---|
| `api-gateway` | Go | Routing, auth (TODO), rate limiting |
| `ingestion-service` | Go | Accepts content, calls ml-inference |
| `moderation-service` | Go | Review queue + moderator verdicts |
| `feedback-service` | Go | Collects labels for retraining |
| `ml-inference` | Python (FastAPI) | Text/audio/video scoring; Whisper ASR |
| `app` | Flutter | Client + moderator review UI |

## Dev

Copy `.env.example` to `.env` and set `POSTGRES_PASSWORD`, `MINIO_ROOT_USER`,
`MINIO_ROOT_PASSWORD` (compose refuses to start without them). Optional:
`MODERATION_TOKEN` / `FEEDBACK_TOKEN` add bearer auth to those services.

```bash
docker compose up --build
# gateway on :8080
curl -X POST localhost:8080/v1/analyze \
  -H 'Content-Type: application/json' \
  -d '{"content_type":"text","text":"hello world"}'
```

`ml-inference` currently ships a stub classifier for pipeline bring-up —
swap `classify_text` for a fine-tuned transformer and `transcribe` for
Whisper once models are chosen.
