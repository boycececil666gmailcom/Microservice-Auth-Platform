# ── Base image ────────────────────────────────────────────────────────────────
FROM python:3.11-slim

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1

# ── Install uv ────────────────────────────────────────────────────────────────
RUN pip install --no-cache-dir uv

# ── Working directory ─────────────────────────────────────────────────────────
WORKDIR /app

# ── Install dependencies (layer-cache friendly) ───────────────────────────────
COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev

# ── Copy application source ───────────────────────────────────────────────────
COPY services/gateway/ ./gateway/

# ── Copy RSA public key (dev default — override via k8s secret mount) ─────────
COPY keys/public_key.pem /app/keys/public_key.pem
ENV JWT_PUBLIC_KEY_PATH=/app/keys/public_key.pem

# ── Run the server ────────────────────────────────────────────────────────────
CMD ["uv", "run", "uvicorn", "gateway.main:app", \
     "--host", "0.0.0.0", "--port", "8000", "--reload"]
