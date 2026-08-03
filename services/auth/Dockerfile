# ── Base image ────────────────────────────────────────────────────────────────
FROM python:3.11-slim

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1

# ── Install uv ────────────────────────────────────────────────────────────────
RUN pip install --no-cache-dir uv

# ── Working directory ─────────────────────────────────────────────────────────
WORKDIR /app

# ── Install dependencies (layer-cache friendly) ───────────────────────────────
# Uses the auth-specific pyproject.toml so only auth dependencies are installed.
COPY services/auth/pyproject.toml services/auth/uv.lock ./
RUN uv sync --frozen --no-dev

# ── Copy application source ───────────────────────────────────────────────────
COPY services/auth/ ./auth/

# ── Copy RSA private key (dev default — override via k8s secret mount) ────────
COPY keys/private_key.pem /app/keys/private_key.pem
ENV JWT_PRIVATE_KEY_PATH=/app/keys/private_key.pem

# ── Run the server ────────────────────────────────────────────────────────────
# Internal service — not exposed to the host, only reachable inside Docker network.
CMD ["uv", "run", "uvicorn", "auth.main:app", \
    "--host", "0.0.0.0", "--port", "8002", "--reload"]
