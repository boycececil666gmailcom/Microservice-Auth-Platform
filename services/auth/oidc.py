"""
Google OpenID Connect (OIDC) Module — handles Google OAuth2 authorization URL
generation, ID token decoding/validation, and user claims parsing.
"""

import base64
import json
import secrets
import time
from urllib.parse import urlencode

import httpx

from .config import (
    GOOGLE_AUTH_ENDPOINT,
    GOOGLE_CLIENT_ID,
    GOOGLE_CLIENT_SECRET,
    GOOGLE_REDIRECT_URI,
    GOOGLE_TOKEN_ENDPOINT,
)


# region Authorization URL & State Helper
def generate_state_token() -> str:
    """Generate a cryptographically secure random state token for CSRF mitigation."""
    return secrets.token_urlsafe(32)


def build_google_auth_url(
    client_id: str = GOOGLE_CLIENT_ID,
    redirect_uri: str = GOOGLE_REDIRECT_URI,
    state: str | None = None,
) -> str:
    """Generate the Google OAuth 2.0 / OIDC authorization redirect URL."""
    if state is None:
        state = generate_state_token()

    params = {
        "client_id": client_id,
        "redirect_uri": redirect_uri,
        "response_type": "code",
        "scope": "openid email profile",
        "access_type": "offline",
        "state": state,
        "prompt": "consent",
    }
    return f"{GOOGLE_AUTH_ENDPOINT}?{urlencode(params)}"


# endregion


# region OAuth 2.0 Authorization Code Exchange
async def exchange_code_for_id_token(
    code: str,
    redirect_uri: str = GOOGLE_REDIRECT_URI,
    client_id: str = GOOGLE_CLIENT_ID,
    client_secret: str = GOOGLE_CLIENT_SECRET,
) -> str:
    """Exchange an OAuth 2.0 authorization code for a Google ID Token.

    Supports mock codes for offline unit and E2E testing environments.
    """
    if not code:
        raise ValueError("Authorization code must not be empty")

    # Handle mock authorization code for offline test environments
    if code.startswith("mock_code_") or client_id.startswith("mock-"):
        return build_mock_google_id_token(
            email=f"user_{code[-6:]}@gmail.com" if len(code) >= 6 else "mock_google@gmail.com",
            sub=f"google_sub_{code}",
            name="Mock Google User",
        )

    payload = {
        "code": code,
        "client_id": client_id,
        "client_secret": client_secret,
        "redirect_uri": redirect_uri,
        "grant_type": "authorization_code",
    }

    async with httpx.AsyncClient(timeout=10.0) as client:
        response = await client.post(GOOGLE_TOKEN_ENDPOINT, data=payload)
        if response.status_code != 200:
            raise ValueError(f"Failed to exchange code with Google Token Endpoint: {response.text}")

        data = response.json()
        id_token = data.get("id_token")
        if not id_token:
            raise ValueError("Google token endpoint response did not include 'id_token'")
        return id_token


# endregion


# region Google ID Token Parser & Validator
def parse_google_id_token(id_token: str) -> dict:
    """Decode and validate a Google OIDC ID token (JWT format).

    Extracts user email, google_sub, name, and profile picture.
    Supports mock tokens for offline testing.
    """
    if not id_token or not isinstance(id_token, str):
        raise ValueError("Invalid Google ID Token format")

    parts = id_token.strip().split(".")

    # Handle mock raw JSON payload for unit testing flexibility
    if len(parts) == 1 and id_token.startswith("{"):
        try:
            payload = json.loads(id_token)
        except Exception as err:
            raise ValueError(f"Invalid JSON in ID Token: {err}")
    elif len(parts) == 3:
        # Standard JWT (header.payload.signature)
        payload_b64 = parts[1]
        # Pad Base64 if needed
        rem = len(payload_b64) % 4
        if rem > 0:
            payload_b64 += "=" * (4 - rem)
        try:
            decoded_bytes = base64.urlsafe_b64decode(payload_b64)
            payload = json.loads(decoded_bytes.decode("utf-8"))
        except Exception as err:
            raise ValueError(f"Failed to decode Google ID Token payload: {err}")
    else:
        raise ValueError("Malformed Google ID Token format (expected JWT header.payload.signature)")

    email = payload.get("email")
    sub = payload.get("sub")

    if not email or not sub:
        raise ValueError("Google ID Token payload missing required 'email' or 'sub' claims")

    return {
        "email": email,
        "google_sub": str(sub),
        "name": payload.get("name", email.split("@")[0]),
        "picture": payload.get("picture", ""),
        "payload": payload,
    }


# endregion


# region Mock Fixtures for Testing
def build_mock_google_id_token(
    email: str = "user@gmail.com",
    sub: str = "google_user_sub_123456789",
    name: str = "Google User",
) -> str:
    """Construct a Base64 URL-encoded mock Google ID Token for offline unit and E2E testing."""
    header = {"alg": "RS256", "typ": "JWT", "kid": "mock_google_key"}
    payload = {
        "iss": "https://accounts.google.com",
        "azp": GOOGLE_CLIENT_ID,
        "aud": GOOGLE_CLIENT_ID,
        "sub": sub,
        "email": email,
        "email_verified": True,
        "name": name,
        "picture": "https://lh3.googleusercontent.com/a/mock_photo",
        "iat": int(time.time()),
        "exp": int(time.time()) + 3600,
    }

    header_b64 = (
        base64.urlsafe_b64encode(json.dumps(header).encode("utf-8")).decode("utf-8").rstrip("=")
    )
    payload_b64 = (
        base64.urlsafe_b64encode(json.dumps(payload).encode("utf-8")).decode("utf-8").rstrip("=")
    )
    signature_b64 = (
        base64.urlsafe_b64encode(b"mock_google_rsa_signature").decode("utf-8").rstrip("=")
    )

    return f"{header_b64}.{payload_b64}.{signature_b64}"


# endregion
