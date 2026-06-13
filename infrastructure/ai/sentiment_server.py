#!/usr/bin/env python3
"""
sentiment_server.py — FastAPI FinBERT news sentiment server.

Fetches BTC headlines from CryptoPanic (public API, no key required).
Scores with ProsusAI/finbert loaded at startup.
Provides /sentiment and /health endpoints.

Start: uvicorn sentiment_server:app --port 8001
"""
import re
import json
import logging
from datetime import datetime, timezone
from typing import List

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

try:
    from fastapi import FastAPI  # type: ignore
    import uvicorn               # type: ignore
    import httpx                 # type: ignore
    from transformers import pipeline  # type: ignore
except ImportError as e:
    raise SystemExit(f"Missing dependency: {e}. Run: pip install fastapi uvicorn httpx transformers torch")

app = FastAPI(title="BTC Sentiment Server", version="1.0.0")

# ── Load FinBERT once at startup ──────────────────────────────────────────────
_scorer = None

def get_scorer():
    global _scorer
    if _scorer is None:
        logger.info("[sentiment] Loading ProsusAI/finbert model...")
        _scorer = pipeline("text-classification", model="ProsusAI/finbert", truncation=True)
        logger.info("[sentiment] FinBERT model loaded.")
    return _scorer

# ── Prompt injection protection ───────────────────────────────────────────────
INJECT_PATTERNS = re.compile(
    r"(ignore|system:|assistant:|<\|im_start\|>|\{|\}|\[|\])",
    re.IGNORECASE,
)

def sanitize(text: str, max_len: int = 120) -> str:
    text = re.sub(r"[^\w\s\.\,\!\?\-\'\"]", " ", text)
    text = text[:max_len].strip()
    if INJECT_PATTERNS.search(text):
        return ""
    return text

HOT_KEYWORDS = ["etf", "halving", "hack", "ban", "sec", "fed", "approval", "crash", "exploit", "rug"]

def extract_keywords(text: str) -> List[str]:
    text_lower = text.lower()
    return [k for k in HOT_KEYWORDS if k in text_lower]

CRYPTOPANIC_URL = "https://cryptopanic.com/api/v1/posts/?auth_token=&public=true&currencies=BTC&kind=news"

_cached_result = None
_cached_at = None
CACHE_TTL_SECONDS = 1800  # 30 minutes


@app.get("/health")
def health():
    return {"status": "ok", "model": "ProsusAI/finbert"}


@app.get("/sentiment")
def sentiment():
    global _cached_result, _cached_at
    now = datetime.now(timezone.utc)
    if _cached_result and _cached_at and (now - _cached_at).total_seconds() < CACHE_TTL_SECONDS:
        return _cached_result

    # Fetch headlines.
    try:
        resp = httpx.get(CRYPTOPANIC_URL, timeout=15)
        posts = resp.json().get("results", [])[:20]
    except Exception as e:
        logger.warning(f"[sentiment] CryptoPanic fetch failed: {e}")
        posts = []

    headlines = []
    for p in posts:
        raw = p.get("title", "")
        cleaned = sanitize(raw)
        if cleaned:
            headlines.append(cleaned)

    if not headlines:
        result = {
            "sentiment_score": 0.0,
            "sentiment_label": "NEUTRAL",
            "hot_keywords": [],
            "news_velocity": 0,
            "top_headlines": [],
            "scored_at": now.isoformat(),
        }
        _cached_result = result
        _cached_at = now
        return result

    # Score with FinBERT.
    scorer = get_scorer()
    scored = scorer(headlines, batch_size=8)

    label_map = {"positive": 1.0, "negative": -1.0, "neutral": 0.0}
    weighted_scores = []
    for i, s in enumerate(scored):
        label = s["label"].lower()
        confidence = s["score"]
        # Recency boost: earlier headlines are more recent (CryptoPanic sorts desc).
        recency_weight = max(0.5, 1.0 - i * 0.04)
        weighted_scores.append(label_map.get(label, 0.0) * confidence * recency_weight)

    total_weight = sum(max(0.5, 1.0 - i * 0.04) for i in range(len(weighted_scores)))
    avg_score = sum(weighted_scores) / total_weight if total_weight > 0 else 0.0

    aggregate_label = "NEUTRAL"
    if avg_score > 0.15:
        aggregate_label = "BULLISH"
    elif avg_score < -0.15:
        aggregate_label = "BEARISH"

    all_text = " ".join(headlines).lower()
    hot = [k for k in HOT_KEYWORDS if k in all_text]

    # News velocity: approximate from published timestamps.
    try:
        recent_count = sum(
            1 for p in posts
            if p.get("published_at") and
            (now - datetime.fromisoformat(p["published_at"].replace("Z", "+00:00"))).total_seconds() < 3600
        )
    except Exception:
        recent_count = len(posts)

    result = {
        "sentiment_score": round(avg_score, 4),
        "sentiment_label": aggregate_label,
        "hot_keywords": hot,
        "news_velocity": recent_count,
        "top_headlines": headlines[:3],
        "scored_at": now.isoformat(),
    }
    _cached_result = result
    _cached_at = now
    return result


if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8001)
