import pytest
import jwt
from pathlib import Path
from services.auth.passwords import hash_password, verify_password

from services.auth.config import JWT_PRIVATE_KEY
from services.gateway.config import JWT_PUBLIC_KEY




def test_hash_password_and_verification_success():
    """Verify that password hashing creates a valid bcrypt hash and verifies correctly."""
    # Arrange
    raw_password = "SecurePassword123!"

    # Act
    hashed = hash_password(raw_password)

    # Assert
    assert hashed != raw_password
    assert verify_password(raw_password, hashed) is True


def test_verify_password_invalid_credentials():
    """Verify that verifying an incorrect password returns False."""
    # Arrange
    raw_password = "SecurePassword123!"
    wrong_password = "WrongPassword321!"
    hashed = hash_password(raw_password)

    # Act
    result = verify_password(wrong_password, hashed)

    # Assert
    assert result is False


#region Password and Token Unit Tests
def test_rs256_token_creation_and_decoding():
    """Verify RS256 JWT encoding with private key and decoding with public key."""
    # Arrange
    from services.auth.tokens import create_access_token
    test_email = "alice@example.com"

    # Act
    token = create_access_token(email=test_email, sso_provider="google_oidc")
    decoded = jwt.decode(token, JWT_PUBLIC_KEY, algorithms=["RS256"])

    # Assert
    assert token is not None
    assert decoded["sub"] == test_email
    assert decoded["email"] == test_email
    assert decoded["sso_provider"] == "google_oidc"
#endregion


def test_invalid_jwt_signature_rejection():
    """Verify that decoding a JWT with an invalid signature raises PyJWTError."""
    # Arrange
    token = jwt.encode({"sub": "100"}, JWT_PRIVATE_KEY, algorithm="RS256")
    tampered_token = token[:-5] + "XXXXX"

    # Act & Assert
    with pytest.raises(jwt.PyJWTError):
        jwt.decode(tampered_token, JWT_PUBLIC_KEY, algorithms=["RS256"])

