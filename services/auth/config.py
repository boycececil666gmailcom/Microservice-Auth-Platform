"""
Auth Service Configuration Module — loads, validates, and manages environment variables.
"""

import os


#region Database & Redis Configuration
DATABASE_URL = os.environ.get("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/auth")
REDIS_URL = os.environ.get("REDIS_URL", "redis://localhost:6379")
#endregion


#region JWT Token Configuration
JWT_ALGORITHM = "RS256"
JWT_EXPIRATION_MINUTES = int(os.environ.get("JWT_EXPIRATION_MINUTES", "15"))
REFRESH_TOKEN_TTL_SECONDS = int(os.environ.get("REFRESH_TOKEN_TTL_SECONDS", str(60 * 60 * 24 * 30)))

# Support both JWT_PRIVATE_KEY and RSA_PRIVATE_KEY_PEM for flexibility across setups
JWT_PRIVATE_KEY = os.environ.get("JWT_PRIVATE_KEY") or os.environ.get("RSA_PRIVATE_KEY_PEM")

if not JWT_PRIVATE_KEY:
    # Default fallback key for offline local development and testing
    JWT_PRIVATE_KEY = (
        "-----BEGIN PRIVATE KEY-----\n"
        "MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDBCfiU8C7y4JqA\n"
        "hoinEHzepNlxEa/bA6mEPCYM3Am2uTf3nvOdmXRAi8BJtcE78knUVxZgncTT5LDq\n"
        "4RcyfIOgGEYkrYc20KiXNGdM9glxw3JCsndsGgsZJic290EEFlAhGSdF/9k7Od87\n"
        "4yaCC4RXgI9yEDKSVHfi1Us+AcHJOko39qFInR4cnfDBFkSkqiN5YU1oZwn/g2gI\n"
        "ofxixpjrKLLWTbon6HXk6gp0M3qPtKKfWXwopPVt6ARifznrfWCpjc6S2gCOlppI\n"
        "Elb98RiXlYOj4oCbv1G74F/R5wqTW12begsyA1umP7hJuXMI9X4i29V7D4ax25zl\n"
        "j1BmSWyTAgMBAAECggEAFSkSpsDtZJJaXVxh/m5AQeewLkTSEiAEpQoy4ZX9Oppz\n"
        "GCHEcrIvnCO1oF7cH8YfcbdaLJ0exlt7SUQDVvVvOE1w4vRirg+Ra4HDERynTGEw\n"
        "VT9a9+6i6M7V3aCc7+XCQt6O/41cMrHVVqs/vWGl0DG3h7le0cuQmLzo0pM+uuAI\n"
        "JgqOnZXBsqk6CVuBvrjFQ4x9ZbGMe1qJwcXWsEBL1eNos3iwBacM1Kl271EgBcDH\n"
        "re9l/G7/QZRsX4DdBI3SFX7wGZg+fr0/G9YSNrscnOQEErY9waIp8Zq2VDV35YzH\n"
        "krh/AvV4uGJ2hWsh11612WDJVKJVKKIaXnGO0I8iAQKBgQDen8jHhuWr3kmlQ3Qd\n"
        "DC36XoNve3NK5sUHzDHcszl/MUiIB47vpO7Fcg9Cki9gewIUGFMjwwuuZpiF/Xw2\n"
        "EoVsarwzI9KAMqRmjEVBn/D3+mj9/b6jwJvYgBH+O9FqVn90Lv3NjwRnHTiPnUzW\n"
        "JJK+r3bLvW7T6eGWViJBRmngYwKBgQDd+rg7iIuR30tKQiPBLFnPJscG5iXXuUcH\n"
        "/k4u3Oos5BgXv000nqk1cqFaGZqlhiFCLB4m8Rv/Kxlm4NWGylQVMj0qzxabelDs\n"
        "teo7HIcinALdyI1TKvaWv1ileSgCe2jXxElvZvhEbvjxqF/Pi/3t6lIgWpM8Xj0y\n"
        "cVpXdXhCEQKBgCHs0YjuWqOFPU3M6K3ghEUqD/d2JYydfBsDF/oc6b8jQH1SQYrt\n"
        "ZGF8Ty0C3+tg82Eij9DcUTRjeAy7IymOSvzJiyJz7AkTLpBeAdPNTshLRaKm/10u\n"
        "5dDpO1S1wuTkh4mp+41OpQodntfrzaC4dBBQ5taHaJMsie8B8zhlRY8nAoGAK9Qx\n"
        "RC/1vtuj9gmRHbcwFGLHsWkH18xRZhakQUSFSE/RIf83s0gQiOkVSsD7c+tD7djg\n"
        "Kzg4Gu3bmiCSiIayi2zb/vPctt4z1Ekm8nzzgbXkKv5KST2WarVlP2boq3TKgq/T\n"
        "ABgItRpkNPLV2BkADlXR2WmI4MaKtscC23nqQMECgYEAor4IREPa8b70qNln2q2d\n"
        "Dxv6NR/j580mU/Q4Ymz8un4P7FOYivpB89OTSd/jjSp966Tx1C5bXjI+WCGJJG9R\n"
        "3v+fbOu2ZuoQO9sOnrGJ4+r3VmUfBtlz0XmFxb5trXzNV04AfHn4OIQsN/JUBP5t\n"
        "zGrvBAmJ/R/G2UVqyd9d5NE=\n"
        "-----END PRIVATE KEY-----\n"
    )
#endregion


#region Google OpenID Connect (OIDC) Configuration
GOOGLE_CLIENT_ID = os.environ.get("GOOGLE_CLIENT_ID", "mock-google-client-id.apps.googleusercontent.com")
GOOGLE_CLIENT_SECRET = os.environ.get("GOOGLE_CLIENT_SECRET", "mock-google-client-secret")
GOOGLE_REDIRECT_URI = os.environ.get("GOOGLE_REDIRECT_URI", "http://localhost/auth/google/callback")

GOOGLE_AUTH_ENDPOINT = "https://accounts.google.com/o/oauth2/v2/auth"
GOOGLE_TOKEN_ENDPOINT = "https://oauth2.googleapis.com/token"
#endregion
