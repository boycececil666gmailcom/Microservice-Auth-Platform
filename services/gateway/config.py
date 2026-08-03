"""
API Gateway Configuration Module — loads and validates environment variables.
"""

import os


#region Upstream Service URLs
SHORTENER_URL = os.environ.get("SHORTENER_URL", "http://shortener:8001")
AUTH_URL = os.environ.get("AUTH_URL", "http://auth:8002")
ANALYTICS_URL = os.environ.get("ANALYTICS_URL", "http://analytics:8003")
#endregion


#region JWT Public Key Configuration
JWT_ALGORITHM = "RS256"

# Support both JWT_PUBLIC_KEY and RSA_PUBLIC_KEY_PEM for flexibility across setups
JWT_PUBLIC_KEY = os.environ.get("JWT_PUBLIC_KEY") or os.environ.get("RSA_PUBLIC_KEY_PEM")

if not JWT_PUBLIC_KEY:
    # Default fallback key for offline local development and testing
    JWT_PUBLIC_KEY = (
        "-----BEGIN PUBLIC KEY-----\n"
        "MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwQn4lPAu8uCagIaIpxB8\n"
        "3qTZcRGv2wOphDwmDNwJtrk3957znZl0QIvASbXBO/JJ1FcWYJ3E0+Sw6uEXMnyD\n"
        "oBhGJK2HNtColzRnTPYJccNyQrJ3bBoLGSYnNvdBBBZQIRknRf/ZOznfO+MmgguE\n"
        "V4CPchAyklR34tVLPgHByTpKN/ahSJ0eHJ3wwRZEpKojeWFNaGcJ/4NoCKH8YsaY\n"
        "6yiy1k26J+h15OoKdDN6j7Sin1l8KKT1begEYn85631gqY3OktoAjpaaSBJW/fEY\n"
        "l5WDo+KAm79Ru+Bf0ecKk1tdm3oLMgNbpj+4SblzCPV+ItvVew+Gsduc5Y9QZkls\n"
        "kwIDAQAB\n"
        "-----END PUBLIC KEY-----\n"
    )
#endregion
