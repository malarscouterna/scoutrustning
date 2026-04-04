package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/MicahParks/keyfunc/v3"
)

type contextKey string

const claimsKey contextKey = "claims"

type Claims struct {
	MemberID string   `json:"member_id"`
	GroupID  string   `json:"group_id"`
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	Units    []string `json:"units"`
}

func (c Claims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(claimsKey).(Claims)
	return c, ok
}

func withClaims(ctx context.Context, c Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

type personasFile struct {
	Personas map[string]Claims `json:"personas"`
}

func loadDevPersonas(path string) map[string]Claims {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("could not load dev personas", "path", path, "error", err)
		return nil
	}
	var pf personasFile
	if err := json.Unmarshal(data, &pf); err != nil {
		slog.Warn("could not parse dev personas", "error", err)
		return nil
	}
	return pf.Personas
}

// MiddlewareConfig holds configuration for the auth middleware.
type MiddlewareConfig struct {
	JWKSURL      string
	DevMode      bool
	PersonasPath string
	RoleMapping  *RoleMapping
}

// Middleware returns auth middleware. In dev mode, it supports X-Dev-Role-Override.
// In production, it validates JWTs against the JWKS endpoint and maps claims.
func Middleware(cfg MiddlewareConfig) func(http.Handler) http.Handler {
	var personas map[string]Claims
	if cfg.DevMode {
		personas = loadDevPersonas(cfg.PersonasPath)
		if personas != nil {
			slog.Info("dev mode: loaded personas", "count", len(personas))
		}
	}

	// Set up JWKS for real JWT validation
	var jwks keyfunc.Keyfunc
	if cfg.JWKSURL != "" {
		var err error
		jwks, err = keyfunc.NewDefault([]string{cfg.JWKSURL})
		if err != nil {
			slog.Error("failed to create JWKS keyfunc", "error", err, "url", cfg.JWKSURL)
		} else {
			slog.Info("JWKS initialized", "url", cfg.JWKSURL)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Dev mode: check for persona override header
			if cfg.DevMode && personas != nil {
				if override := r.Header.Get("X-Dev-Role-Override"); override != "" {
					if p, ok := personas[override]; ok {
						r = r.WithContext(withClaims(r.Context(), p))
						next.ServeHTTP(w, r)
						return
					}
					http.Error(w, `{"error":"unknown dev persona"}`, http.StatusBadRequest)
					return
				}
			}

			// Extract Bearer token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			if jwks == nil {
				http.Error(w, `{"error":"JWT validation not configured"}`, http.StatusUnauthorized)
				return
			}

			// Parse and validate the JWT
			token, err := jwt.Parse(tokenStr, jwks.KeyfuncCtx(r.Context()),
				jwt.WithExpirationRequired(),
				jwt.WithIssuedAt(),
				jwt.WithLeeway(10*time.Second),
			)
			if err != nil || !token.Valid {
				slog.Debug("JWT validation failed", "error", err)
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}

			mapClaims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, `{"error":"invalid token claims"}`, http.StatusUnauthorized)
				return
			}

			// Extract raw claims from the token
			tc := TokenClaims{
				Name:              getStringClaim(mapClaims, "name"),
				Email:             getStringClaim(mapClaims, "email"),
				PreferredUsername: getStringClaim(mapClaims, "preferred_username"),
				Roles:             getStringSliceClaim(mapClaims, "roles"),
			}

			if cfg.RoleMapping == nil {
				http.Error(w, `{"error":"role mapping not configured"}`, http.StatusInternalServerError)
				return
			}

			claims, err := ParseClaims(tc, cfg.RoleMapping)
			if err != nil {
				slog.Warn("failed to parse claims", "error", err, "preferred_username", tc.PreferredUsername)
				http.Error(w, `{"error":"could not determine group from token"}`, http.StatusForbidden)
				return
			}

			r = r.WithContext(withClaims(r.Context(), claims))
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole returns middleware that checks the authenticated user has the given role.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			if !claims.HasRole(role) {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func getStringClaim(m jwt.MapClaims, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getStringSliceClaim(m jwt.MapClaims, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch val := v.(type) {
	case []interface{}:
		var result []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return val
	}
	return nil
}
