package auth

import (
	"reflect"
	"testing"
)

func TestPickActiveGroup(t *testing.T) {
	primary := OrgMembership{ID: "766", TenantID: "766", IsPrimary: true}
	other := OrgMembership{ID: "900", TenantID: "900", IsPrimary: false}
	district := OrgMembership{ID: "111", TenantID: "district:111", Type: OrgTypeDistrict}

	tests := []struct {
		name       string
		registered []OrgMembership
		hint       string
		want       string
	}{
		{"no registered groups", nil, "766", ""},
		{"single group, no hint", []OrgMembership{primary}, "", "766"},
		{"hint matches a registered group", []OrgMembership{primary, other}, "900", "900"},
		{"hint matches nothing, falls back to primary", []OrgMembership{primary, other}, "999", "766"},
		{"no hint, falls back to primary", []OrgMembership{other, primary}, "", "766"},
		{"no hint, no primary, falls back to first (stable order)", []OrgMembership{other, {ID: "800", TenantID: "800"}}, "", "900"},
		{"hint matches a prefixed district tenant", []OrgMembership{primary, district}, "district:111", "district:111"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickActiveGroup(tt.registered, tt.hint)
			if got != tt.want {
				t.Errorf("pickActiveGroup(%v, %q) = %q, want %q", tt.registered, tt.hint, got, tt.want)
			}
		})
	}
}

func TestTenantKey(t *testing.T) {
	tests := []struct {
		orgType    string
		scoutnetID string
		want       string
	}{
		{OrgTypeGroup, "766", "766"},
		{"", "766", "766"},
		{OrgTypeDistrict, "111", "district:111"},
		{OrgTypeRegion, "5", "region:5"},
		{OrgTypeOrganisation, "1", "organisation:1"},
	}

	for _, tt := range tests {
		if got := TenantKey(tt.orgType, tt.scoutnetID); got != tt.want {
			t.Errorf("TenantKey(%q, %q) = %q, want %q", tt.orgType, tt.scoutnetID, got, tt.want)
		}
	}
}

func TestSplitTenantKey(t *testing.T) {
	tests := []struct {
		tenantID string
		orgType  string
		wantID   string
	}{
		{"766", OrgTypeGroup, "766"},
		{"district:111", OrgTypeDistrict, "111"},
		{"project:abc", OrgTypeProject, "abc"},
	}

	for _, tt := range tests {
		orgType, scoutnetID := splitTenantKey(tt.tenantID)
		if orgType != tt.orgType || scoutnetID != tt.wantID {
			t.Errorf("splitTenantKey(%q) = (%q, %q), want (%q, %q)", tt.tenantID, orgType, scoutnetID, tt.orgType, tt.wantID)
		}
	}
}

func TestParseOrgMemberships(t *testing.T) {
	ms := tokenMemberships{
		Groups: map[string]tokenOrgMembership{
			"766": {Name: "Mälarscouterna", IsPrimary: true, Roles: []tokenMembershipRole{{ID: 1, Key: "it_manager", Name: "IT Manager"}}},
		},
		Districts: map[string]tokenOrgMembership{
			"111": {Roles: []tokenMembershipRole{{ID: 68, Key: "district_board"}}},
		},
		Projects: map[string]tokenOrgMembership{},
	}

	got := parseOrgMemberships(ms)
	want := []OrgMembership{
		{ID: "766", Type: OrgTypeGroup, Name: "Mälarscouterna", IsPrimary: true, Roles: []OrgRole{{ID: 1, Key: "it_manager", Name: "IT Manager"}}},
		{ID: "111", Type: OrgTypeDistrict, Roles: []OrgRole{{ID: 68, Key: "district_board"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseOrgMemberships() = %#v, want %#v", got, want)
	}
}
