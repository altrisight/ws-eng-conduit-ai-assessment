"""ML inference service: text/audio/video hate-speech scoring.

Baseline implementation: a pluggable text classifier interface. Swap
`classify_text` for a fine-tuned transformer (e.g. XLM-R) once trained.
Audio/video are transcribed (Whisper) then scored as text.
"""
import os
from typing import Optional

from fastapi import FastAPI
from pydantic import BaseModel

app = FastAPI(title="ml-inference")

# Placeholder lexicon — replace with a real model in classify_text().
HATE_TERMS = {"hate", "kill", "subhuman"}  # stub, for pipeline bring-up only


class AnalyzeRequest(BaseModel):
    content_type: str  # text | audio | video
    text: Optional[str] = None
    media_url: Optional[str] = None
    post_id: Optional[str] = None


def classify_text(text: str) -> dict:
    """Return per-category scores in [0,1]. Replace with model inference."""
    tokens = set(text.lower().split())
    hits = len(tokens & HATE_TERMS)
    score = min(1.0, hits * 0.4)
    return {"hate": score}


def transcribe(media_url: str) -> str:
    """Transcribe audio/video via Whisper. Stub until model is wired in."""
    raise NotImplementedError("Whisper transcription not yet configured")


@app.get("/healthz")
def healthz():
    return {"status": "ok"}


@app.post("/analyze")
def analyze(req: AnalyzeRequest):
    transcript = None
    if req.content_type == "text":
        if not req.text:
            return {"error": "text required"}, 400
        text = req.text
    elif req.content_type in ("audio", "video"):
        if not req.media_url:
            return {"error": "media_url required"}, 400
        transcript = transcribe(req.media_url)
        text = transcript
    else:
        return {"error": f"unknown content_type {req.content_type}"}, 400

    scores = classify_text(text)
    threshold = float(os.getenv("FLAG_THRESHOLD", "0.5"))
    return {
        "post_id": req.post_id,
        "scores": scores,
        "flagged": scores["hate"] >= threshold,
        "transcript": transcript,
    }
