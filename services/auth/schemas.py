"""
Auth Service Pydantic Schemas — request and response data models.
"""

from pydantic import BaseModel


#region Password Authentication Schemas
class LoginRequest(BaseModel):
    user: str
    password: str


class TokenResponse(BaseModel):
    access_token: str
#endregion


#region Google OIDC Schemas
class GoogleCallbackRequest(BaseModel):
    id_token: str
#endregion
