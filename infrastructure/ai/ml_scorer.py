"""
ml_scorer.py — Local XGBoost pre-scorer for BTC-PILOT signals.

Serves two endpoints:
  GET  /health   → {"model_loaded": bool}
  POST /predict  → {"probability_win": float}

Feature order must match FeatureVector.ToSlice() in engine/internal/ml/prescorer.go:
  RSI14, MACDHist, EMA9vsEMA21, BBPosition, ATRRatio, VolumeRatio,
  RegimeNum, FundingRate, OITrendNum, ETFScore, DominanceScore,
  MacroScore, SentimentScore, TemporalMod

Start: uvicorn ml_scorer:app --host 0.0.0.0 --port 8002
Or via Docker Compose (see docker-compose.grafana.yml ml-scorer service).
"""

import os
import logging
from pathlib import Path
from typing import Optional

import numpy as np
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger(__name__)

app = FastAPI(title="BTC-PILOT ML Pre-scorer", version="1.0.0")

# ---------------------------------------------------------------------------
# Model loading
# ---------------------------------------------------------------------------

MODEL_PATH = os.getenv("ML_MODEL_PATH", "models/btc_pilot_xgb.json")
FEATURES = [
    "RSI14", "MACDHist", "EMA9vsEMA21", "BBPosition",
    "ATRRatio", "VolumeRatio", "RegimeNum", "FundingRate",
    "OITrendNum", "ETFScore", "DominanceScore",
    "MacroScore", "SentimentScore", "TemporalMod",
]
NUM_FEATURES = len(FEATURES)

_model: Optional[object] = None


def _load_model() -> Optional[object]:
    global _model
    if _model is not None:
        return _model
    model_path = Path(MODEL_PATH)
    if not model_path.exists():
        log.warning("Model file not found at %s — running in stub mode", MODEL_PATH)
        return None
    try:
        import xgboost as xgb  # type: ignore
        booster = xgb.Booster()
        booster.load_model(str(model_path))
        log.info("XGBoost model loaded from %s", MODEL_PATH)
        _model = booster
        return _model
    except Exception as exc:
        log.error("Failed to load XGBoost model: %s", exc)
        return None


# Attempt load at startup; engine will poll /health until model_loaded=true.
_load_model()


# ---------------------------------------------------------------------------
# Schemas
# ---------------------------------------------------------------------------

class PredictRequest(BaseModel):
    features: list[float]


class PredictResponse(BaseModel):
    probability_win: float


class HealthResponse(BaseModel):
    model_loaded: bool
    num_features: int


# ---------------------------------------------------------------------------
# Endpoints
# ---------------------------------------------------------------------------

@app.get("/health", response_model=HealthResponse)
def health() -> HealthResponse:
    model = _load_model()
    return HealthResponse(model_loaded=model is not None, num_features=NUM_FEATURES)


@app.post("/predict", response_model=PredictResponse)
def predict(req: PredictRequest) -> PredictResponse:
    if len(req.features) != NUM_FEATURES:
        raise HTTPException(
            status_code=422,
            detail=f"Expected {NUM_FEATURES} features, got {len(req.features)}",
        )

    model = _load_model()
    if model is None:
        # No model: pass-through (never block the engine in stub mode).
        return PredictResponse(probability_win=1.0)

    try:
        import xgboost as xgb  # type: ignore
        arr = np.array([req.features], dtype=np.float32)
        dmat = xgb.DMatrix(arr, feature_names=FEATURES)
        prob = float(model.predict(dmat)[0])
        return PredictResponse(probability_win=prob)
    except Exception as exc:
        log.error("Prediction error: %s", exc)
        # On error return pass-through — never block the engine.
        return PredictResponse(probability_win=1.0)
