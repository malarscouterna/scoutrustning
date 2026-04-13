package auth

import (
	"context"
	"encoding/json"
	"fmt"
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

// AccessLevel constants ordered by privilege.
const (
	AccessView    = "view"
	AccessBook    = "book"
	AccessTrusted = "trusted"
	AccessManager = "manager"
)

var accessOrder = map[string]int{
	AccessView:    0,
	AccessBook:    1,
	AccessTrusted: 2,
	AccessManager: 3,
}

// AccessAtLeast returns true if level >= required.
func AccessAtLeast(level, required string) bool {
	return accessOrder[level] >= accessOrder[required]
}

type TeamMembership struct {
	TeamID      string `json:"team_id"`
	TeamName    string `json:"team_name"`
	TeamType    string `json:"team_type"`
	AccessLevel string `json:"access_level"`
}

type Claims struct {
	MemberID  string           `json:"member_id"`
	GroupID   string           `json:"group_id"`
	Name      string           `json:"name"`
	Email     string           `json:"email"`
	Teams     []TeamMembership `json:"teams"`
	MaxAccess string           `json:"max_access"`
}

func (c Claims) IsManager() bool {
	return c.MaxAccess == AccessManager
}

func (c Claims) CanBook() bool {
	return AccessAtLeast(c.MaxAccess, AccessBook)
}

func (c Claims) AccessForTeam(teamID string) string {
	for _, t := range c.Teams {
		if t.TeamID == teamID {
			return t.AccessLevel
		}
	}
	return AccessBook // personal booking fallback
}

// TeamNames returns a list of team names for query filtering.
func (c Claims) TeamNames() []string {
	names := make([]string, len(c.Teams))
	for i, t := range c.Teams {
		names[i] = t.TeamName
	}
	return names
}

// HasRole is a compatibility shim during migration. Maps old role names to access checks.
// TODO: Remove once all call sites are migrated.
func (c Claims) HasRole(role string) bool {
	switch role {
	case "equipment_manager":
		return c.IsManager()
	case "project_leader":
		return AccessAtLeast(c.MaxAccess, AccessTrusted)
	case "leader":
		return AccessAtLeast(c.MaxAccess, AccessBook)
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

// Dev persona file format
type devPersonasFile struct {
	Personas map[string]devPersona `json:"personas"`
}

type devPersona struct {
	MemberID string              `json:"member_id"`
	Name     string              `json:"name"`
	Email    string              `json:"email"`
	Groups   map[string][]string `json:"groups"` // group_id → team names
}

func loadDevPersonas(path string) map[string]devPersona {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("could not load dev personas", "path", path, "error", err)
		return nil
	}
	var pf devPersonasFile
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
	Resolver     TeamResolver
}

// Middleware returns auth middleware. In dev mode, it supports X-Dev-Role-Override.
// In production, it validates JWTs against the JWKS endpoint and maps claims.
func Middleware(cfg MiddlewareConfig) func(http.Handler) http.Handler {
	var personas map[string]devPersona
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
			// Dev mode: check for raw claims header or persona override
			if cfg.DevMode {
				if raw := r.Header.Get("X-Dev-Claims"); raw != "" {
					var c Claims
					if err := json.Unmarshal([]byte(raw), &c); err != nil {
						http.Error(w, `{"error":"invalid dev claims"}`, http.StatusBadRequest)
						return
					}
					r = r.WithContext(withClaims(r.Context(), c))
					next.ServeHTTP(w, r)
					return
				}
				if personas != nil {
					if override := r.Header.Get("X-Dev-Role-Override"); override != "" {
						claims, err := resolveDevPersona(r.Context(), personas, override, cfg.Resolver)
						if err != nil {
							http.Error(w, `{"error":"unknown dev persona"}`, http.StatusBadRequest)
							return
						}
						r = r.WithContext(withClaims(r.Context(), claims))
						next.ServeHTTP(w, r)
						return
					}
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
			name := getStringClaim(mapClaims, "name")
			email := getStringClaim(mapClaims, "email")
			preferredUsername := getStringClaim(mapClaims, "preferred_username")
			tokenRoles := getStringSliceClaim(mapClaims, "roles")

			// Extract member ID from preferred_username ("scoutnet|3169207" → "3169207")
			memberID := preferredUsername
			if parts := strings.SplitN(preferredUsername, "|", 2); len(parts) == 2 {
				memberID = parts[1]
			}

			// Parse OIDC claims and determine group
			var oidcClaims []OIDCClaim
			groupIDs := map[string]bool{}
			for _, role := range tokenRoles {
				parts := strings.SplitN(role, ":", 3)
				if len(parts) != 3 {
					continue
				}
				scope, id, roleName := parts[0], parts[1], parts[2]
				if scope == "group" || scope == "troop" {
					// For group claims, claim_id is the role name (identifies the team).
					// For troop claims, claim_id is the troop ID.
					claimID := id
					if scope == "group" {
						claimID = roleName
					}
					oidcClaims = append(oidcClaims, OIDCClaim{Scope: scope, ID: claimID, RoleName: roleName})
				}
				if scope == "group" {
					groupIDs[id] = true
				}
			}

			// Determine which group to use
			var groupID string
			if cfg.Resolver != nil {
				for gid := range groupIDs {
					exists, _ := cfg.Resolver.GroupExists(r.Context(), gid)
					if exists {
						groupID = gid
						break
					}
				}
			}
			if groupID == "" {
				slog.Warn("no configured group found in token", "preferred_username", preferredUsername, "group_ids", groupIDs)
				http.Error(w, `{"error":"group_not_found"}`, http.StatusForbidden)
				return
			}

			// Resolve teams from claim mappings
			var teams []TeamMembership
			maxAccess := AccessView
			if cfg.Resolver != nil {
				resolved, err := cfg.Resolver.ResolveTeamsByClaims(r.Context(), groupID, oidcClaims)
				if err != nil {
					slog.Error("failed to resolve teams from claims", "error", err)
				} else {
					teams = resolved
				}

				// Auto-create teams for unrecognized claims
				autoCreated, err := cfg.Resolver.AutoCreateTeams(r.Context(), groupID, oidcClaims)
				if err != nil {
					slog.Warn("failed to auto-create teams", "error", err)
				} else {
					teams = append(teams, autoCreated...)
				}
			}

			for _, t := range teams {
				if accessOrder[t.AccessLevel] > accessOrder[maxAccess] {
					maxAccess = t.AccessLevel
				}
			}
			if len(teams) == 0 && cfg.Resolver != nil {
				if def, err := cfg.Resolver.DefaultAccessForUnknown(r.Context(), groupID); err == nil {
					maxAccess = def
				}
			}

			claims := Claims{
				MemberID:  memberID,
				GroupID:   groupID,
				Name:      name,
				Email:     email,
				Teams:     teams,
				MaxAccess: maxAccess,
			}

			r = r.WithContext(withClaims(r.Context(), claims))
			next.ServeHTTP(w, r)
		})
	}
}

// resolveDevPersona builds Claims from a dev persona definition.
// Uses the resolver to look up team access levels from the DB.
func resolveDevPersona(ctx context.Context, personas map[string]devPersona, name string, resolver TeamResolver) (Claims, error) {
	p, ok := personas[name]
	if !ok {
		return Claims{}, fmt.Errorf("unknown persona: %s", name)
	}

	// Use first group
	var groupID string
	var teamNames []string
	for gid, names := range p.Groups {
		groupID = gid
		teamNames = names
		break
	}

	var teams []TeamMembership
	maxAccess := AccessView

	if resolver != nil && len(teamNames) > 0 {
		resolved, err := resolver.ResolveTeamsByNames(ctx, groupID, teamNames)
		if err != nil {
			slog.Warn("failed to resolve teams for persona", "persona", name, "error", err)
		} else {
			teams = resolved
		}
	}

	// Fallback: if resolver unavailable or returned nothing, build placeholder teams
	if len(teams) == 0 && len(teamNames) > 0 {
		for _, tn := range teamNames {
			teams = append(teams, TeamMembership{
				TeamID: tn, TeamName: tn, TeamType: "troop", AccessLevel: AccessBook,
			})
		}
	}

	for _, t := range teams {
		if accessOrder[t.AccessLevel] > accessOrder[maxAccess] {
			maxAccess = t.AccessLevel
		}
	}

	if len(teams) == 0 {
		// No teams — use group default for unknown users
		if resolver != nil {
			if def, err := resolver.DefaultAccessForUnknown(ctx, groupID); err == nil {
				maxAccess = def
			}
		}
	}

	return Claims{
		MemberID:  p.MemberID,
		GroupID:   groupID,
		Name:      p.Name,
		Email:     p.Email,
		Teams:     teams,
		MaxAccess: maxAccess,
	}, nil
}

// RequireAccess returns middleware that checks the user has at least the given access level.
func RequireAccess(level string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			if !AccessAtLeast(claims.MaxAccess, level) {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole is kept for backward compatibility during migration.
// It maps old role names to access level checks.
func RequireRole(role string) func(http.Handler) http.Handler {
	switch role {
	case "equipment_manager":
		return RequireAccess(AccessManager)
	case "project_leader":
		return RequireAccess(AccessTrusted)
	case "leader":
		return RequireAccess(AccessBook)
	default:
		return RequireAccess(AccessManager)
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
