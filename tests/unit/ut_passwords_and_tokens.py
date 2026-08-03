import pytest
import jwt
from pathlib import Path
from services.auth.passwords import hash_password, verify_password

# Read dev RSA keys for testing RS256 token creation & decoding
PRIVATE_KEY_PATH = Path("keys/private_key.pem")
PUBLIC_KEY_PATH = Path("keys/public_key.pem")


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


def test_rs256_token_creation_and_decoding():
    """Verify RS256 JWT encoding with private key and decoding with public key."""
    # Arrange
    private_key = PRIVATE_KEY_PATH.read_text()
    public_key = PUBLIC_KEY_PATH.read_text()
    user_id = 42
    payload = {"sub": str(user_id)}

    # Act
    token = jwt.encode(payload, private_key, algorithm="RS256")
    decoded = jwt.decode(token, public_key, algorithms=["RS256"])

    # Assert
    assert token is not None
    assert decoded["sub"] == "42"


def test_invalid_jwt_signature_rejection():
    """Verify that decoding a JWT with an invalid signature raises PyJWTError."""
    # Arrange
    private_key = PRIVATE_KEY_PATH.read_text()
    wrong_public_key_rsa = jwt.generate_jwt if hasattr(jwt, "generate_jwt") else None
    token = jwt.encode({"sub": "100"}, private_key, algorithm="RS256")
    tampered_token = token[:-5] + "XXXXX"

    # Act & Assert
    with pytest.raises(jwt.PyJWTError):
        jwt.decode(tampered_token, PUBLIC_KEY_PATH.read_text(), algorithms=["RS256"])
