import os
import time
import uuid
import pytest
import requests

# Base URL for the API Gateway
GATEWAY_URL = os.environ.get("GATEWAY_URL", "http://localhost:8000")


@pytest.fixture(scope="module")
def http_session():
    """Module-level HTTP session fixture for maintaining cookies across requests."""
    session = requests.Session()
    yield session
    session.close()


@pytest.fixture(scope="module")
def user_credentials():
    """Generate unique user credentials for the test module."""
    return {
        "user": f"testuser_{uuid.uuid4()}",
        "password": "testpassword123",
    }


@pytest.fixture(scope="module")
def authenticated_session(http_session, user_credentials):
    """Fixture that logs in and yields (session, access_token)."""
    # Arrange
    login_url = f"{GATEWAY_URL}/auth/login"

    # Act
    response = http_session.post(login_url, json=user_credentials)

    # Assert
    assert response.status_code == 200, f"Login failed: {response.text}"
    token_data = response.json()
    assert "access_token" in token_data, f"Response missing access_token: {token_data}"
    assert "refresh_token" in http_session.cookies.get_dict(), "No refresh_token cookie found"

    yield http_session, token_data["access_token"]


def test_gateway_health():
    """Verify that the API Gateway health endpoint returns 200 OK."""
    # Arrange
    health_url = f"{GATEWAY_URL}/health"

    # Act
    response = requests.get(health_url)

    # Assert
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_unauthenticated_shorten_access_denied():
    """Verify that shortening a URL without an Authorization header returns 401 Unauthorized."""
    # Arrange
    shorten_url = f"{GATEWAY_URL}/api/v1/shorten"
    payload = {"long_url": f"https://example.com/unauth-{uuid.uuid4()}"}

    # Act
    response = requests.post(shorten_url, json=payload)

    # Assert
    assert response.status_code == 401
    assert "detail" in response.json()


def test_authenticated_shorten_url_success(authenticated_session):
    """Verify creating a short URL with valid Bearer token returns 201 Created."""
    # Arrange
    session, access_token = authenticated_session
    shorten_url = f"{GATEWAY_URL}/api/v1/shorten"
    long_url = f"https://example.com/e2e-{uuid.uuid4()}"
    headers = {"Authorization": f"Bearer {access_token}"}

    # Act
    response = session.post(shorten_url, json={"long_url": long_url}, headers=headers)

    # Assert
    assert response.status_code == 201, f"Expected 201, got {response.status_code}: {response.text}"
    data = response.json()
    assert "short_url" in data
    assert data["long_url"] == long_url


def test_reissue_access_token(authenticated_session):
    """Verify exchanging a refresh token cookie for a new access token."""
    # Arrange
    session, old_token = authenticated_session
    refresh_url = f"{GATEWAY_URL}/auth/refresh"
    time.sleep(1)  # Ensure distinct iat timestamp

    # Act
    response = session.post(refresh_url)

    # Assert
    assert response.status_code == 200, f"Refresh failed: {response.text}"
    new_data = response.json()
    assert "access_token" in new_data
    assert new_data["access_token"] != old_token


def test_retrieve_long_url(authenticated_session):
    """Verify retrieving original long URL details via the short URL ID."""
    # Arrange
    session, access_token = authenticated_session
    long_url = f"https://example.com/retrieve-{uuid.uuid4()}"
    headers = {"Authorization": f"Bearer {access_token}"}
    create_resp = session.post(f"{GATEWAY_URL}/api/v1/shorten", json={"long_url": long_url}, headers=headers)
    short_url_id = create_resp.json()["short_url"]

    # Act
    get_resp = session.get(f"{GATEWAY_URL}/api/v1/urls/{short_url_id}", headers=headers)

    # Assert
    assert get_resp.status_code == 200
    data = get_resp.json()
    assert data["long_url"] == long_url
    assert data["short_url"] == short_url_id


def test_public_redirect_and_analytics(authenticated_session):
    """Verify public redirect endpoint (302) and async analytics tracking."""
    # Arrange
    session, access_token = authenticated_session
    long_url = f"https://example.com/redirect-{uuid.uuid4()}"
    headers = {"Authorization": f"Bearer {access_token}"}
    stats_url = f"{GATEWAY_URL}/api/v1/analytics/stats"

    # Act 1: Get initial stats if available
    try:
        initial_resp = requests.get(stats_url, headers=headers)
        if initial_resp.status_code in (500, 502):
            pytest.skip("Analytics service unavailable in environment")
        initial_stats = initial_resp.json()
    except requests.RequestException:
        pytest.skip("Analytics service unreachable in environment")

    create_resp = session.post(f"{GATEWAY_URL}/api/v1/shorten", json={"long_url": long_url}, headers=headers)
    short_url_id = create_resp.json()["short_url"]

    # Act 2: Perform 302 redirect
    redirect_url = f"{GATEWAY_URL}/r/{short_url_id}"
    redirect_resp = requests.get(redirect_url, allow_redirects=False)

    # Assert
    assert redirect_resp.status_code == 302
    assert redirect_resp.headers.get("Location") == long_url

    # Act 3: Check updated stats after Kafka event consumption
    time.sleep(2)
    updated_resp = requests.get(stats_url, headers=headers)

    # Assert stats count increased
    assert updated_resp.status_code == 200
    updated_stats = updated_resp.json()
    assert updated_stats.get("total_redirects", 0) >= initial_stats.get("total_redirects", 0) + 1


def test_logout_clears_refresh_token(http_session, user_credentials):
    """Verify logging out invalidates session and clears refresh token cookie."""
    # Arrange
    logout_url = f"{GATEWAY_URL}/auth/logout"

    # Act
    response = http_session.post(logout_url)

    # Assert
    assert response.status_code == 200
    cookies = http_session.cookies.get_dict()
    assert "refresh_token" not in cookies or cookies["refresh_token"] in ('""', '')
