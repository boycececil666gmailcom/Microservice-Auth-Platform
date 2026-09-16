package auth

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/boycececil666gmailcom/Microservice-Auth-Platform/internal/httpjson"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	cfg            Config
	db             *pgxpool.Pool
	redis          *redis.Client
	privateKey     *rsa.PrivateKey
	httpClient     *http.Client
	googleVerifier *GoogleVerifier
}

var rotateRefreshTokenScript = redis.NewScript(`
local current = redis.call("GET", KEYS[1])
if not current or current ~= ARGV[1] then
  return false
end
redis.call("DEL", KEYS[1])
redis.call("SET", KEYS[2], current, "EX", ARGV[2])
return current
`)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type googleCallbackRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

type sessionData struct {
	Email       string `json:"email"`
	SSOProvider string `json:"sso_provider"`
}

func NewServer(ctx context.Context, cfg Config) (*Server, error) {
	privateKey, err := ParsePrivateKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, err
	}
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}
	redisOptions, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		db.Close()
		return nil, err
	}
	redisClient := redis.NewClient(redisOptions)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		db.Close()
		_ = redisClient.Close()
		return nil, err
	}
	if _, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			email VARCHAR(255) PRIMARY KEY,
			password_hash VARCHAR(255) NOT NULL,
			sso_provider VARCHAR(50) DEFAULT 'local',
			google_sub VARCHAR(255),
			created_at TIMESTAMPTZ DEFAULT NOW()
		);
		CREATE UNIQUE INDEX IF NOT EXISTS users_google_sub_unique
		ON users (google_sub) WHERE google_sub IS NOT NULL`); err != nil {
		db.Close()
		_ = redisClient.Close()
		return nil, err
	}
	httpClient := &http.Client{Timeout: 10 * time.Second}
	return &Server{
		cfg: cfg, db: db, redis: redisClient, privateKey: privateKey,
		httpClient:     httpClient,
		googleVerifier: NewGoogleVerifier(httpClient, cfg.GoogleClientID, cfg.GoogleJWKSURL),
	}, nil
}

func (s *Server) Close() {
	s.db.Close()
	_ = s.redis.Close()
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /ready", s.ready)
	mux.HandleFunc("GET /.well-known/jwks.json", s.jwks)
	mux.HandleFunc("GET /auth/.well-known/jwks.json", s.jwks)
	mux.HandleFunc("POST /auth/login", s.login)
	mux.HandleFunc("POST /auth/refresh", s.refresh)
	mux.HandleFunc("POST /auth/logout", s.logout)
	mux.HandleFunc("POST /auth/google/callback", s.googleCallback)
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.Ping(ctx); err != nil {
		httpjson.Error(w, http.StatusServiceUnavailable, "Database unavailable")
		return
	}
	if err := s.redis.Ping(ctx).Err(); err != nil {
		httpjson.Error(w, http.StatusServiceUnavailable, "Session store unavailable")
		return
	}
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) jwks(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	httpjson.Write(w, http.StatusOK, PublicJWKS(&s.privateKey.PublicKey))
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body loginRequest
	if err := httpjson.Decode(w, r, &body); err != nil {
		httpjson.Error(w, http.StatusUnprocessableEntity, "Invalid request body")
		return
	}
	email, valid := normalizeEmail(body.Email)
	if !valid || len(body.Password) < 8 || len([]byte(body.Password)) > 72 {
		httpjson.Error(w, http.StatusUnprocessableEntity, "Email must be valid and password must be 8-72 bytes")
		return
	}
	var passwordHash, provider string
	err := s.db.QueryRow(r.Context(),
		"SELECT password_hash, COALESCE(sso_provider, 'local') FROM users WHERE email = $1",
		email,
	).Scan(&passwordHash, &provider)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		passwordHash, err = HashPassword(body.Password)
		if err == nil {
			result, insertErr := s.db.Exec(r.Context(),
				"INSERT INTO users (email, password_hash, sso_provider) VALUES ($1, $2, 'local') ON CONFLICT (email) DO NOTHING",
				email, passwordHash,
			)
			err = insertErr
			if err == nil && result.RowsAffected() == 0 {
				err = s.db.QueryRow(r.Context(),
					"SELECT password_hash, COALESCE(sso_provider, 'local') FROM users WHERE email = $1",
					email,
				).Scan(&passwordHash, &provider)
				if err == nil && !VerifyPassword(body.Password, passwordHash) {
					httpjson.Error(w, http.StatusUnauthorized, "Invalid credentials")
					return
				}
			}
		}
		if provider == "" {
			provider = "local"
		}
	case err == nil:
		if !VerifyPassword(body.Password, passwordHash) {
			httpjson.Error(w, http.StatusUnauthorized, "Invalid credentials")
			return
		}
	}
	if err != nil {
		slog.Error("login database operation failed", "error", err)
		httpjson.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	s.issueTokens(w, r, email, provider)
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil || cookie.Value == "" {
		httpjson.Error(w, http.StatusUnauthorized, "Missing refresh token")
		return
	}
	oldKey := refreshTokenKey(cookie.Value)
	encodedSession, err := s.redis.Get(r.Context(), oldKey).Result()
	if errors.Is(err, redis.Nil) {
		httpjson.Error(w, http.StatusUnauthorized, "Invalid or expired refresh token")
		return
	}
	if err != nil {
		slog.Error("refresh token lookup failed", "error", err)
		httpjson.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	var session sessionData
	if err := json.Unmarshal([]byte(encodedSession), &session); err != nil || session.Email == "" || session.SSOProvider == "" {
		_ = s.redis.Del(r.Context(), oldKey).Err()
		httpjson.Error(w, http.StatusUnauthorized, "Invalid or expired refresh token")
		return
	}
	var userExists bool
	if err := s.db.QueryRow(r.Context(),
		"SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", session.Email,
	).Scan(&userExists); err != nil {
		slog.Error("refresh user lookup failed", "error", err)
		httpjson.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if !userExists {
		httpjson.Error(w, http.StatusUnauthorized, "User no longer exists")
		return
	}
	accessToken, err := CreateAccessToken(s.privateKey, s.cfg.AccessTokenTTL, session.Email, session.SSOProvider, time.Now().UTC())
	if err != nil {
		httpjson.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	newRefreshToken, err := GenerateRefreshToken()
	if err != nil {
		httpjson.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	rotatedEmail, err := rotateRefreshTokenScript.Run(
		r.Context(), s.redis,
		[]string{oldKey, refreshTokenKey(newRefreshToken)},
		encodedSession, int64(s.cfg.RefreshTokenTTL.Seconds()),
	).Text()
	if errors.Is(err, redis.Nil) || (err == nil && rotatedEmail != encodedSession) {
		httpjson.Error(w, http.StatusUnauthorized, "Invalid or expired refresh token")
		return
	}
	if err != nil {
		slog.Error("refresh token rotation failed", "error", err)
		httpjson.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	s.setRefreshCookie(w, newRefreshToken)
	httpjson.Write(w, http.StatusOK, map[string]string{"access_token": accessToken})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("refresh_token"); err == nil && cookie.Value != "" {
		if err := s.redis.Del(r.Context(), refreshTokenKey(cookie.Value)).Err(); err != nil {
			slog.Warn("refresh token deletion failed", "error", err)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name: "refresh_token", Value: "", Path: "/auth", HttpOnly: true,
		Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
		MaxAge: -1, Expires: time.Unix(1, 0),
	})
	httpjson.Write(w, http.StatusOK, map[string]string{"detail": "Logged out"})
}

func (s *Server) googleCallback(w http.ResponseWriter, r *http.Request) {
	var body googleCallbackRequest
	if err := httpjson.Decode(w, r, &body); err != nil || body.Code == "" {
		httpjson.Error(w, http.StatusUnprocessableEntity, "Invalid request body")
		return
	}
	if !isMockGoogleConfig(s.cfg) && (s.cfg.GoogleClientID == "" || s.cfg.GoogleClientSecret == "") {
		httpjson.Error(w, http.StatusServiceUnavailable, "Google OIDC is not configured")
		return
	}
	idToken, err := ExchangeGoogleCode(r.Context(), s.httpClient, s.cfg, body.Code, s.cfg.GoogleCallbackURL)
	if err != nil {
		httpjson.Error(w, http.StatusBadRequest, "Google Code Exchange failed: "+err.Error())
		return
	}
	var user GoogleUser
	if isMockGoogleConfig(s.cfg) {
		user, err = ParseMockGoogleIDToken(idToken)
	} else {
		user, err = s.googleVerifier.Verify(r.Context(), idToken)
	}
	if err != nil {
		httpjson.Error(w, http.StatusBadRequest, "Invalid Google ID Token: "+err.Error())
		return
	}
	normalizedEmail, valid := normalizeEmail(user.Email)
	if !valid {
		httpjson.Error(w, http.StatusBadRequest, "Invalid Google ID Token: invalid email claim")
		return
	}
	user.Email = normalizedEmail
	user.Email, err = s.resolveGoogleIdentity(r.Context(), user.Email, user.GoogleSub)
	if err != nil {
		slog.Error("Google user provisioning failed", "error", err)
		httpjson.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	s.issueTokens(w, r, user.Email, "google_oidc")
}

func (s *Server) issueTokens(w http.ResponseWriter, r *http.Request, email, provider string) {
	accessToken, err := CreateAccessToken(s.privateKey, s.cfg.AccessTokenTTL, email, provider, time.Now().UTC())
	if err != nil {
		httpjson.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		httpjson.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	encodedSession, err := json.Marshal(sessionData{Email: email, SSOProvider: provider})
	if err != nil {
		httpjson.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if err := s.redis.Set(r.Context(), refreshTokenKey(refreshToken), encodedSession, s.cfg.RefreshTokenTTL).Err(); err != nil {
		slog.Error("refresh token storage failed", "error", err)
		httpjson.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	s.setRefreshCookie(w, refreshToken)
	httpjson.Write(w, http.StatusOK, map[string]string{"access_token": accessToken})
}

func (s *Server) resolveGoogleIdentity(ctx context.Context, email, googleSub string) (string, error) {
	var existingEmail string
	err := s.db.QueryRow(ctx, "SELECT email FROM users WHERE google_sub = $1", googleSub).Scan(&existingEmail)
	if err == nil {
		return existingEmail, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	var existingSub *string
	err = s.db.QueryRow(ctx, "SELECT google_sub FROM users WHERE email = $1", email).Scan(&existingSub)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = s.db.Exec(ctx, `
			INSERT INTO users (email, password_hash, sso_provider, google_sub)
			VALUES ($1, 'GOOGLE_OIDC_USER_NO_PASSWORD', 'google_oidc', $2)`, email, googleSub)
		return email, err
	}
	if err != nil {
		return "", err
	}
	if existingSub != nil && *existingSub != googleSub {
		return "", errors.New("email is already linked to another Google identity")
	}
	if existingSub == nil {
		_, err = s.db.Exec(ctx, "UPDATE users SET google_sub = $1 WHERE email = $2 AND google_sub IS NULL", googleSub, email)
	}
	return email, err
}

func (s *Server) setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: "refresh_token", Value: token, Path: "/auth", HttpOnly: true,
		Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
		MaxAge: int(s.cfg.RefreshTokenTTL.Seconds()),
	})
}

func normalizeEmail(raw string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if len(email) == 0 || len(email) > 254 {
		return "", false
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || !strings.Contains(email, "@") {
		return "", false
	}
	return email, true
}
