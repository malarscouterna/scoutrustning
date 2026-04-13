package handler

import (
	"context"
	"fmt"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
)

// DBTeamResolver implements auth.TeamResolver using database queries.
type DBTeamResolver struct {
	Q *db.Queries
}

func (r *DBTeamResolver) ResolveTeamsByNames(ctx context.Context, groupID string, names []string) ([]auth.TeamMembership, error) {
	teams, err := r.Q.ListTeamsByNames(ctx, db.ListTeamsByNamesParams{
		GroupID: groupID,
		Names:   names,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve teams: %w", err)
	}
	var memberships []auth.TeamMembership
	for _, t := range teams {
		memberships = append(memberships, auth.TeamMembership{
			TeamID:      t.ID.String(),
			TeamName:    t.Name,
			TeamType:    t.Type,
			AccessLevel: t.AccessLevel,
		})
	}
	return memberships, nil
}

func (r *DBTeamResolver) ResolveTeamsByClaims(ctx context.Context, groupID string, claims []auth.OIDCClaim) ([]auth.TeamMembership, error) {
	// Get all claim mappings for this group
	mappings, err := r.Q.GetTeamClaimMappingsByClaims(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("get claim mappings: %w", err)
	}

	// Build lookup: "scope:id" → team membership
	lookup := make(map[string]auth.TeamMembership, len(mappings))
	for _, m := range mappings {
		key := m.ClaimScope + ":" + m.ClaimID
		lookup[key] = auth.TeamMembership{
			TeamID:      m.TeamID.String(),
			TeamName:    m.TeamName,
			TeamType:    m.TeamType,
			AccessLevel: m.AccessLevel,
		}
	}

	// Match token claims to mappings
	seen := make(map[string]bool)
	var memberships []auth.TeamMembership
	for _, c := range claims {
		key := c.Scope + ":" + c.ID
		if tm, ok := lookup[key]; ok && !seen[tm.TeamID] {
			memberships = append(memberships, tm)
			seen[tm.TeamID] = true
		}
	}
	return memberships, nil
}

func (r *DBTeamResolver) DefaultAccessForUnknown(ctx context.Context, groupID string) (string, error) {
	settings, err := r.Q.GetGroupSettings(ctx, groupID)
	if err != nil {
		return auth.AccessView, nil
	}
	return settings.DefaultAccessUnknown, nil
}

func (r *DBTeamResolver) GroupExists(ctx context.Context, groupID string) (bool, error) {
	_, err := r.Q.GetGroup(ctx, groupID)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (r *DBTeamResolver) AutoCreateTeams(ctx context.Context, groupID string, claims []auth.OIDCClaim) ([]auth.TeamMembership, error) {
	// Get existing mappings to find unmatched claims
	mappings, err := r.Q.GetTeamClaimMappingsByClaims(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("get claim mappings: %w", err)
	}
	existing := make(map[string]bool, len(mappings))
	for _, m := range mappings {
		existing[m.ClaimScope+":"+m.ClaimID] = true
	}

	// Get group defaults
	settings, _ := r.Q.GetGroupSettings(ctx, groupID)
	defaultTroop := auth.AccessBook
	defaultRole := auth.AccessBook
	if settings.GroupID != "" {
		defaultTroop = settings.DefaultAccessTroop
		defaultRole = settings.DefaultAccessRole
	}

	var created []auth.TeamMembership
	for _, c := range claims {
		key := c.Scope + ":" + c.ID
		if existing[key] {
			continue
		}

		// Determine team type, name, and access level
		var teamType, teamName, accessLevel string
		switch c.Scope {
		case "troop":
			teamType = "troop"
			teamName = "Avdelning " + c.ID
			accessLevel = defaultTroop
		case "group":
			teamType = "role"
			teamName = "Roll " + c.RoleName
			accessLevel = defaultRole
		default:
			continue
		}

		// Create team
		team, err := r.Q.CreateTeam(ctx, db.CreateTeamParams{
			GroupID: groupID, Name: teamName, Type: teamType, AccessLevel: accessLevel,
		})
		if err != nil {
			// Might conflict on name — skip
			continue
		}

		// Create claim mapping
		r.Q.CreateTeamClaimMapping(ctx, db.CreateTeamClaimMappingParams{
			GroupID: groupID, TeamID: team.ID, ClaimScope: c.Scope, ClaimID: c.ID,
		})

		created = append(created, auth.TeamMembership{
			TeamID: team.ID.String(), TeamName: teamName, TeamType: teamType, AccessLevel: accessLevel,
		})
		existing[key] = true
	}
	return created, nil
}

func (r *DBTeamResolver) CountManagerTeams(ctx context.Context, groupID string) (int, error) {
	count, err := r.Q.CountManagerTeams(ctx, groupID)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}
