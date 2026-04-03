package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
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

// Middleware returns auth middleware. In dev mode, it supports X-Dev-Role-Override.
// jwksURL is used for production JWT validation (not yet implemented).
func Middleware(jwksURL string, devMode bool, personasPath string) func(http.Handler) http.Handler {
	var personas map[string]Claims
	if devMode {
		personas = loadDevPersonas(personasPath)
		if personas != nil {
			slog.Info("dev mode: loaded personas", "count", len(personas))
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Dev mode: check for persona override header
			if devMode && personas != nil {
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
			auth := r.Header.Get("Authorization")
			if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			// TODO: Validate JWT signature using JWKS endpoint
			// For now in dev mode, we require the X-Dev-Role-Override header
			// This will be replaced with real JWT validation when Keycloak is wired up
			if devMode {
				http.Error(w, `{"error":"JWT validation not yet implemented, use X-Dev-Role-Override header in dev mode"}`, http.StatusUnauthorized)
				return
			}

			http.Error(w, `{"error":"JWT validation not configured"}`, http.StatusUnauthorized)
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
