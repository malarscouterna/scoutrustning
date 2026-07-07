package auth

import "testing"

func TestPickActiveGroup(t *testing.T) {
	primary := OrgMembership{ID: "766", IsPrimary: true}
	other := OrgMembership{ID: "900", IsPrimary: false}

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
		{"no hint, no primary, falls back to first (stable order)", []OrgMembership{other, {ID: "800"}}, "", "900"},
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
