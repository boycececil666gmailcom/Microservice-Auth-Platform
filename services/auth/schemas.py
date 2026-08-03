"""
Auth Service Pydantic Schemas — request and response data models.
"""

from pydantic import BaseModel


#region Password Authentication Schemas
class LoginRequest(BaseModel):
    email: str
    password: str


class TokenResponse(BaseModel):
    access_token: str
#endregion


#region Google OIDC Schemas
class GoogleCallbackRequest(BaseModel):
    id_token: str | None = None
    code: str | None = None
    state: str | None = None
    redirect_uri: str | None = None
#endregion

