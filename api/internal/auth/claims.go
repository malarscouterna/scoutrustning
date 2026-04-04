package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// RoleMapping defines how Scoutnet token roles map to application roles.
type RoleMapping struct {
	Groups map[string]GroupMapping `json:"groups"`
}

type GroupMapping struct {
	Name         string            `json:"name"`
	AdminRoles   map[string]string `json:"admin_roles"`
	ProjectRoles map[string]string `json:"project_roles"`
	Troops       map[string]string `json:"troops"`
}

func LoadRoleMapping(path string) (*RoleMapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read role mapping: %w", err)
	}
	var rm RoleMapping
	if err := json.Unmarshal(data, &rm); err != nil {
		return nil, fmt.Errorf("parse role mapping: %w", err)
	}
	return &rm, nil
}

// TokenClaims represents the raw claims from a Keycloak access token.
type TokenClaims struct {
	Sub               string   `json:"sub"`
	Name              string   `json:"name"`
	Email             string   `json:"email"`
	PreferredUsername string   `json:"preferred_username"`
	Roles             []string `json:"roles"`
}

// ParseClaims converts raw Keycloak token claims into application Claims
// using the role mapping config.
func ParseClaims(tc TokenClaims, rm *RoleMapping) (Claims, error) {
	// Extract member ID from preferred_username ("scoutnet|3169207" → "3169207")
	memberID := tc.PreferredUsername
	if parts := strings.SplitN(tc.PreferredUsername, "|", 2); len(parts) == 2 {
		memberID = parts[1]
	}

	// Parse roles to extract group ID, app roles, and units
	var groupID string
	appRoles := map[string]bool{}
	units := map[string]bool{}

	for _, role := range tc.Roles {
		parts := strings.SplitN(role, ":", 3)
		if len(parts) != 3 {
			continue
		}
		scope, id, roleName := parts[0], parts[1], parts[2]

		switch scope {
		case "group":
			// First group:XXX role determines the group ID
			if groupID == "" {
				groupID = id
			}
			gm, ok := rm.Groups[id]
			if !ok {
				continue
			}
			if _, isAdmin := gm.AdminRoles[roleName]; isAdmin {
				appRoles["equipment_manager"] = true
				units[gm.AdminRoles[roleName]] = true
			}
			if _, isProject := gm.ProjectRoles[roleName]; isProject {
				appRoles["project_leader"] = true
				units[gm.ProjectRoles[roleName]] = true
			}

		case "troop":
			appRoles["leader"] = true
			// Look up troop name from any group mapping
			for gid, gm := range rm.Groups {
				if name, ok := gm.Troops[id]; ok {
					units[name] = true
					if groupID == "" {
						groupID = gid
					}
					break
				}
			}

		case "organisation":
			// Organisation roles don't map to app roles currently
		}
	}

	if groupID == "" {
		return Claims{}, fmt.Errorf("no group found in token roles")
	}

	var roleList []string
	for r := range appRoles {
		roleList = append(roleList, r)
	}
	var unitList []string
	for u := range units {
		unitList = append(unitList, u)
	}

	return Claims{
		MemberID: memberID,
		GroupID:  groupID,
		Name:     tc.Name,
		Email:    tc.Email,
		Roles:    roleList,
		Units:    unitList,
	}, nil
}
