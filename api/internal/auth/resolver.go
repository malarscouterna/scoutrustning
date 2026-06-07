package auth

import "context"

// OIDCClaim represents a parsed membership entry from the token.
type OIDCClaim struct {
	Scope        string // "group" or "troop"
	ID           string // role key for group scope; troop ID for troop scope
	RoleName     string // role key (same as ID for group scope)
	Name         string // display name from token (e.g. "IT Manager", "Yggdrasil"); may be empty
	TroopGroupID string // group ID the troop belongs to (troop scope only; empty if unknown)
}

// TeamResolver resolves team names or OIDC claims to memberships.
// Implemented by the handler layer to avoid circular imports between auth and db packages.
type TeamResolver interface {
	ResolveTeamsByNames(ctx context.Context, groupID string, names []string) ([]TeamMembership, error)
	ResolveTeamsByClaims(ctx context.Context, groupID string, claims []OIDCClaim) ([]TeamMembership, error)
	AutoCreateTeams(ctx context.Context, groupID string, claims []OIDCClaim) ([]TeamMembership, error)
	DefaultAccessForUnknown(ctx context.Context, groupID string) (string, error)
	GroupExists(ctx context.Context, groupID string) (bool, error)
	CountManagerTeams(ctx context.Context, groupID string) (int, error)
}
