package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// Token memberships claim structures (unexported, only used for JWT parsing).
type tokenMembershipRole struct {
	ID   int    `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// tokenOrgMembership is one entry in any memberships entity type. Only groups
// populate Name and IsPrimary in ScoutID; higher-level entities carry roles only.
type tokenOrgMembership struct {
	Name      string                `json:"name"`
	Roles     []tokenMembershipRole `json:"roles"`
	IsPrimary bool                  `json:"is_primary"`
}

type tokenTroopMembership struct {
	Name    string                `json:"name"`
	GroupID *int                  `json:"groupId"`
	Roles   []tokenMembershipRole `json:"roles"`
}

type tokenMemberships struct {
	Groups        map[string]tokenOrgMembership   `json:"groups"`
	Districts     map[string]tokenOrgMembership   `json:"districts"`
	Regions       map[string]tokenOrgMembership   `json:"regions"`
	Organisations map[string]tokenOrgMembership   `json:"organisations"`
	Corps         map[string]tokenOrgMembership   `json:"corps"`
	Networks      map[string]tokenOrgMembership   `json:"networks"`
	Projects      map[string]tokenOrgMembership   `json:"projects"`
	Troops        map[string]tokenTroopMembership `json:"troops"`
}

type contextKey string

const claimsKey contextKey = "claims"

// ActiveGroupHeader carries the frontend's active-group-id cookie value
// (set by the group switcher for multi-group members), forwarded by
// hooks.server.ts the same way X-Dev-Role-Override is. Middleware prefers
// this hint when a member matches more than one registered group.
const ActiveGroupHeader = "X-Active-Group-Id"

// AccessLevel constants ordered by privilege.
const (
	AccessView    = "view"
	AccessBook    = "book"
	AccessTrusted = "trusted"
	AccessManager = "manager"
)

// Organisation types mirrored from ScoutID's memberships claim. Scoutrustning
// currently registers groups and districts; the remaining types parse
// generically so a tenant of any type works without further auth changes.
const (
	OrgTypeGroup        = "group"
	OrgTypeDistrict     = "district"
	OrgTypeRegion       = "region"
	OrgTypeOrganisation = "organisation"
	OrgTypeCorps        = "corps"
	OrgTypeNetwork      = "network"
	OrgTypeProject      = "project"
)

// OrgTypes is the canonical order used when flattening the memberships claim.
var OrgTypes = []string{
	OrgTypeGroup,
	OrgTypeDistrict,
	OrgTypeRegion,
	OrgTypeOrganisation,
	OrgTypeCorps,
	OrgTypeNetwork,
	OrgTypeProject,
}

// IsOrgType reports whether t is a known Scoutnet organisation type.
func IsOrgType(t string) bool {
	for _, known := range OrgTypes {
		if t == known {
			return true
		}
	}
	return false
}

// TenantKey maps a Scoutnet entity to the internal groups.id. Groups keep the
// raw Scoutnet group number for backwards compatibility; every other entity
// type is prefixed because its ID namespace is separate from groups.
func TenantKey(orgType, scoutnetID string) string {
	if orgType == "" || orgType == OrgTypeGroup {
		return scoutnetID
	}
	return orgType + ":" + scoutnetID
}

// splitTenantKey reverses TenantKey for dev personas and tooling.
func splitTenantKey(tenantID string) (orgType, scoutnetID string) {
	for _, t := range OrgTypes {
		if t == OrgTypeGroup {
			continue
		}
		prefix := t + ":"
		if strings.HasPrefix(tenantID, prefix) {
			return t, strings.TrimPrefix(tenantID, prefix)
		}
	}
	return OrgTypeGroup, tenantID
}

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
	Picture   string           `json:"picture"`
	Teams     []TeamMembership `json:"teams"`
	MaxAccess string           `json:"max_access"`
	// Orgs is the raw org id/name/role list straight from the JWT memberships
	// claim, independent of DB registration - used by /join so a user with one
	// registered org can still apply for a second, unregistered one.
	Orgs []OrgMembership `json:"orgs,omitempty"`
	// Groups is Orgs filtered to the ones that resolve to a registered DB group -
	// i.e. every group this member can actually switch into. Empty/single-entry
	// for the common case of one registered group.
	Groups []OrgMembership `json:"groups,omitempty"`
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
	// AllowUnmapped, when true, lets requests through even when no memberships
	// claim matches a registered group, instead of returning 403. Used only by
	// routes that operate before a group exists (e.g. group signup). Claims in
	// this case have an empty GroupID and no Teams; use Claims.Orgs instead.
	AllowUnmapped bool
}

// OrgRole is a single role a member holds within an org, read directly from
// the JWT memberships claim.
type OrgRole struct {
	ID   int    `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// OrgMembership is a raw org/name/role tuple read directly from the JWT
// memberships claim. ID is the Scoutnet ID in the entity's own namespace;
// TenantID is the matching internal groups.id (empty when not registered).
type OrgMembership struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id,omitempty"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Roles     []OrgRole `json:"roles"`
	IsPrimary bool      `json:"is_primary"`
}

// parseOrgMemberships flattens the per-entity-type maps in the memberships
// claim into one list. Types are appended in canonical order and IDs within a
// type are sorted so organisation selection stays stable across requests.
func parseOrgMemberships(ms tokenMemberships) []OrgMembership {
	byType := map[string]map[string]tokenOrgMembership{
		OrgTypeGroup:        ms.Groups,
		OrgTypeDistrict:     ms.Districts,
		OrgTypeRegion:       ms.Regions,
		OrgTypeOrganisation: ms.Organisations,
		OrgTypeCorps:        ms.Corps,
		OrgTypeNetwork:      ms.Networks,
		OrgTypeProject:      ms.Projects,
	}

	var out []OrgMembership
	for _, orgType := range OrgTypes {
		members := byType[orgType]
		ids := make([]string, 0, len(members))
		for id := range members {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			m := members[id]
			roles := make([]OrgRole, len(m.Roles))
			for i, role := range m.Roles {
				roles[i] = OrgRole{ID: role.ID, Key: role.Key, Name: role.Name}
			}
			out = append(out, OrgMembership{
				ID: id, Type: orgType, Name: m.Name, Roles: roles, IsPrimary: m.IsPrimary,
			})
		}
	}
	return out
}

// pickActiveGroup chooses which of a member's registered organisations is
// active, preferring the hint (from ActiveGroupHeader) if it matches one of
// them, then the primary org, then a stable first match. Returns "" if
// registered is empty.
func pickActiveGroup(registered []OrgMembership, hint string) string {
	for _, g := range registered {
		if g.TenantID == hint {
			return g.TenantID
		}
	}
	for _, g := range registered {
		if g.IsPrimary {
			return g.TenantID
		}
	}
	if len(registered) > 0 {
		return registered[0].TenantID
	}
	return ""
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
						claims, err := resolveDevPersona(r.Context(), personas, override, cfg.Resolver, r.Header.Get(ActiveGroupHeader))
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
			picture := getStringClaim(mapClaims, "picture")
			preferredUsername := getStringClaim(mapClaims, "preferred_username")

			// Extract member ID from preferred_username ("scoutnet|3169207" → "3169207")
			memberID := preferredUsername
			if parts := strings.SplitN(preferredUsername, "|", 2); len(parts) == 2 {
				memberID = parts[1]
			}

			// Parse memberships claim
			var ms tokenMemberships
			if raw, ok := mapClaims["memberships"]; ok {
				if b, err := json.Marshal(raw); err == nil {
					json.Unmarshal(b, &ms) //nolint:errcheck
				}
			}

			// Raw org memberships from the token, independent of which (if any)
			// resolves to a registered tenant - used by /join so a user with one
			// registered org can still apply for a second, unregistered one.
			orgs := parseOrgMemberships(ms)

			// Determine which registered organisations this member belongs to. A
			// member can match more than one; which one is "active" is resolved
			// below, preferring the ActiveGroupHeader hint (set by the frontend's
			// organisation switcher cookie) over the primary org, over a stable
			// first match. parseOrgMemberships sorts within each entity type.
			var registeredGroups []OrgMembership
			if cfg.Resolver != nil {
				for _, org := range orgs {
					tenantID := TenantKey(org.Type, org.ID)
					exists, _ := cfg.Resolver.GroupExists(r.Context(), tenantID)
					if !exists {
						continue
					}
					org.TenantID = tenantID
					if org.Name == "" {
						// Higher-level Scoutnet entities carry no name; use the
						// locally stored organisation name instead.
						if name, err := cfg.Resolver.GroupName(r.Context(), tenantID); err == nil {
							org.Name = name
						}
					}
					registeredGroups = append(registeredGroups, org)
				}
				sort.Slice(registeredGroups, func(i, j int) bool {
					return registeredGroups[i].TenantID < registeredGroups[j].TenantID
				})
			}

			groupID := pickActiveGroup(registeredGroups, r.Header.Get(ActiveGroupHeader))
			if groupID == "" {
				if cfg.AllowUnmapped {
					claims := Claims{
						MemberID:  memberID,
						Name:      name,
						Email:     email,
						Picture:   picture,
						MaxAccess: AccessView,
						Orgs:      orgs,
					}
					r = r.WithContext(withClaims(r.Context(), claims))
					next.ServeHTTP(w, r)
					return
				}
				slog.Warn("no configured group found in token", "preferred_username", preferredUsername)
				http.Error(w, `{"error":"group_not_found"}`, http.StatusForbidden)
				return
			}

			// Build OIDC claims from memberships
			var oidcClaims []OIDCClaim

			// One claim per role the user holds in the active organisation. The
			// "group" scope means "role in the active tenant" regardless of its
			// Scoutnet entity type, so existing claim mappings keep working.
			for _, org := range registeredGroups {
				if org.TenantID != groupID {
					continue
				}
				for _, role := range org.Roles {
					oidcClaims = append(oidcClaims, OIDCClaim{
						Scope:    "group",
						ID:       role.Key,
						RoleName: role.Key,
						Name:     role.Name,
					})
				}
				break
			}

			// One claim per troop; pass groupId so auto-create can gate on it
			for troopID, tm := range ms.Troops {
				var troopGroupID string
				if tm.GroupID != nil {
					troopGroupID = strconv.Itoa(*tm.GroupID)
				}
				oidcClaims = append(oidcClaims, OIDCClaim{
					Scope:        "troop",
					ID:           troopID,
					Name:         tm.Name,
					TroopGroupID: troopGroupID,
				})
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
				Picture:   picture,
				Teams:     teams,
				MaxAccess: maxAccess,
				Orgs:      orgs,
				Groups:    registeredGroups,
			}

			r = r.WithContext(withClaims(r.Context(), claims))
			next.ServeHTTP(w, r)
		})
	}
}

// resolveDevPersona builds Claims from a dev persona definition. A persona can
// belong to more than one group (map key per group ID) to exercise the
// active-group switcher locally; hint is the same ActiveGroupHeader value
// used for real JWT logins. Uses the resolver to look up team access levels
// and group names from the DB.
func resolveDevPersona(ctx context.Context, personas map[string]devPersona, name string, resolver TeamResolver, hint string) (Claims, error) {
	p, ok := personas[name]
	if !ok {
		return Claims{}, fmt.Errorf("unknown persona: %s", name)
	}

	var registeredGroups []OrgMembership
	for gid := range p.Groups {
		orgType, scoutnetID := splitTenantKey(gid)
		groupName := gid
		if resolver != nil {
			if n, err := resolver.GroupName(ctx, gid); err == nil {
				groupName = n
			}
		}
		registeredGroups = append(registeredGroups, OrgMembership{
			ID: scoutnetID, TenantID: gid, Type: orgType, Name: groupName,
		})
	}
	sort.Slice(registeredGroups, func(i, j int) bool {
		return registeredGroups[i].TenantID < registeredGroups[j].TenantID
	})

	groupID := pickActiveGroup(registeredGroups, hint)
	teamNames := p.Groups[groupID]

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
		Orgs:      registeredGroups,
		Groups:    registeredGroups,
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
