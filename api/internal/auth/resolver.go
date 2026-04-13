package auth

import "context"

// OIDCClaim represents a parsed scope:id:role claim from the token.
type OIDCClaim struct {
	Scope    string // "group" or "troop"
	ID       string // e.g. "766", "17443"
	RoleName string // e.g. "it_manager", "leader"
}

// TeamResolver resolves team names or OIDC claims to memberships.
// Implemented by the handler layer to avoid circular imports between auth and db packages.
type TeamResolver interface {
	ResolveTeamsByNames(ctx context.Context, groupID string, names []string) ([]TeamMembership, error)
	ResolveTeamsByClaims(ctx context.Context, groupID string, claims []OIDCClaim) ([]TeamMembership, error)
	DefaultAccessForUnknown(ctx context.Context, groupID string) (string, error)
	GroupExists(ctx context.Context, groupID string) (bool, error)
}
