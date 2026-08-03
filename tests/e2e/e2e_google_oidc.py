"""
End-to-End Tests for Google OpenID Connect (OIDC) Integration (e2e_google_oidc.py).

Tests:
  - GET /auth/google/login returns valid OAuth2 authorization URL via Gateway
  - POST /auth/google/callback auto-provisions Google user, returns RS256 JWT & refresh cookie
  - Accessing protected /api/v1/shorten using Google-issued RS256 JWT succeeds
"""

import uuid
import pytest
import requests
from services.auth.oidc import build_mock_google_id_token

GATEWAY_URL = "http://localhost:8000"


def test_google_login_auth_url_endpoint():
    """Verify GET /auth/google/login returns Google OAuth 2.0 authorization URL via Gateway."""
    resp = requests.get(f"{GATEWAY_URL}/auth/google/login")
    assert resp.status_code == 200
    data = resp.json()
    assert "auth_url" in data
    assert "https://accounts.google.com/o/oauth2/v2/auth" in data["auth_url"]


def test_google_oidc_callback_and_protected_access():
    """Verify POST /auth/google/callback processes Google ID Token, auto-provisions account,
    issues RS256 JWT, and allows shortening URLs via Gateway.
    """
    google_email = f"google_user_{uuid.uuid4().hex[:8]}@gmail.com"
    google_sub = f"google_sub_{uuid.uuid4().hex[:12]}"
    mock_id_token = build_mock_google_id_token(email=google_email, sub=google_sub)

    session = requests.Session()

    # 1. Post Google ID token to OIDC Callback endpoint
    cb_resp = session.post(
        f"{GATEWAY_URL}/auth/google/callback",
        json={"id_token": mock_id_token},
    )
    assert cb_resp.status_code == 200, f"Google OIDC Callback failed: {cb_resp.text}"
    cb_data = cb_resp.json()
    assert "access_token" in cb_data
    access_token = cb_data["access_token"]

    # Verify refresh token cookie set
    assert "refresh_token" in session.cookies

    # 2. Use Google-issued RS256 JWT access token to call protected /api/v1/shorten endpoint
    shorten_resp = requests.post(
        f"{GATEWAY_URL}/api/v1/shorten",
        json={"long_url": f"https://google.com/search?q={uuid.uuid4()}"},
        headers={"Authorization": f"Bearer {access_token}"},
    )
    assert shorten_resp.status_code == 201, f"Shorten URL failed: {shorten_resp.text}"
    shorten_data = shorten_resp.json()
    assert "short_url" in shorten_data
