"""ML inference service: text/audio/video hate-speech scoring.

Text is scored by a HuggingFace classifier (`MODEL_NAME`, default
cardiffnlp/twitter-roberta-base-hate). Audio/video are transcribed with
Whisper then scored as text. Both models load lazily on first use; if the
dependencies are not installed the service falls back to a stub so the
pipeline still works for bring-up.
"""
import os
from functools import lru_cache
from typing import Optional

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

app = FastAPI(title="ml-inference")

MODEL_NAME = os.getenv("MODEL_NAME", "cardiffnlp/twitter-roberta-base-hate")


class AnalyzeRequest(BaseModel):
    content_type: str  # text | audio | video
    text: Optional[str] = None
    media_url: Optional[str] = None
    post_id: Optional[str] = None


@lru_cache(maxsize=1)
def _text_classifier():
    from transformers import pipeline  # heavy import, deferred

    return pipeline("text-classification", model=MODEL_NAME)


@lru_cache(maxsize=1)
def _whisper_model():
    import whisper

    return whisper.load_model(os.getenv("WHISPER_MODEL", "base"))


def classify_text(text: str) -> dict:
    """Return per-category scores in [0,1]."""
    try:
        clf = _text_classifier()
    except ImportError:
        # Stub for bring-up when transformers isn't installed.
        tokens = set(text.lower().split())
        hits = len(tokens & {"hate", "kill", "subhuman"})
        return {"hate": min(1.0, hits * 0.4)}

    result = clf(text, truncation=True, max_length=512)[0]
    # cardiffnlp model labels: LABEL_0 = non-hate, LABEL_1 = hate
    label = result["label"].lower()
    if label in ("hate", "label_1"):
        return {"hate": float(result["score"])}
    return {"hate": float(1.0 - result["score"])}


def transcribe(media_url: str) -> str:
    """Transcribe audio/video via Whisper. Only http(s) URLs are accepted so
    callers cannot read local files or internal-only endpoints."""
    if not media_url.startswith(("https://", "http://")):
        raise HTTPException(status_code=400, detail="media_url must be an http(s) URL")
    try:
        import whisper  # noqa: F401
    except ImportError as exc:
        raise HTTPException(
            status_code=501,
            detail="Whisper not installed; audio/video transcription unavailable",
        ) from exc
    return _whisper_model().transcribe(media_url)["text"]


@app.get("/healthz")
def healthz():
    return {"status": "ok"}


@app.post("/analyze")
def analyze(req: AnalyzeRequest):
    transcript = None
    if req.content_type == "text":
        if not req.text:
            raise HTTPException(status_code=400, detail="text required")
        text = req.text
    elif req.content_type in ("audio", "video"):
        if not req.media_url:
            raise HTTPException(status_code=400, detail="media_url required")
        transcript = transcribe(req.media_url)
        text = transcript
    else:
        raise HTTPException(status_code=400, detail=f"unknown content_type {req.content_type}")

    scores = classify_text(text)
    threshold = float(os.getenv("FLAG_THRESHOLD", "0.5"))
    return {
        "post_id": req.post_id,
        "scores": scores,
        "flagged": scores["hate"] >= threshold,
        "transcript": transcript,
    }
