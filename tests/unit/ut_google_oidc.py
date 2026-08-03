"""
Unit Tests for Google OpenID Connect (OIDC) functionality (ut_google_oidc.py).

Tests:
  - Google OAuth 2.0 Authorization URL generation
  - Google ID Token decoding & user claim extraction
  - Malformed Google ID Token rejection
  - Missing claim validation
"""

import pytest
from services.auth import oidc


#region Unit Tests
def test_build_google_auth_url():
    """Verify Google OAuth 2.0 authorization URL contains expected parameters."""
    url = oidc.build_google_auth_url(state="custom_state_456")
    assert "https://accounts.google.com/o/oauth2/v2/auth" in url
    assert "client_id=" in url
    assert "response_type=code" in url
    assert "scope=openid+email+profile" in url or "scope=openid" in url
    assert "state=custom_state_456" in url


def test_generate_state_token():
    """Verify random state token is unique and non-empty."""
    state1 = oidc.generate_state_token()
    state2 = oidc.generate_state_token()
    assert len(state1) > 10
    assert state1 != state2


@pytest.mark.asyncio
async def test_exchange_code_for_id_token():
    """Verify exchange_code_for_id_token handles mock codes correctly."""
    id_token = await oidc.exchange_code_for_id_token("mock_code_12345")
    assert id_token is not None
    user_info = oidc.parse_google_id_token(id_token)
    assert "google_sub" in user_info
    assert "email" in user_info


def test_parse_valid_mock_google_id_token():
    """Verify standard Base64 JWT mock Google ID Token correctly extracts user claims."""
    mock_token = oidc.build_mock_google_id_token(
        email="alice@gmail.com",
        sub="google_sub_987654321",
        name="Alice Smith",
    )
    user_info = oidc.parse_google_id_token(mock_token)

    assert user_info["email"] == "alice@gmail.com"
    assert user_info["google_sub"] == "google_sub_987654321"
    assert user_info["name"] == "Alice Smith"


def test_parse_malformed_google_id_token_raises_value_error():
    """Verify invalid token string raises ValueError."""
    with pytest.raises(ValueError, match="Malformed Google ID Token|Invalid"):
        oidc.parse_google_id_token("invalid.jwt.string.with.too.many.parts")


def test_parse_missing_claims_raises_value_error():
    """Verify token payload missing required 'email' or 'sub' claims raises ValueError."""
    invalid_token = '{"name": "No Sub User"}'
    with pytest.raises(ValueError, match="missing required 'email' or 'sub' claims"):
        oidc.parse_google_id_token(invalid_token)
#endregion

