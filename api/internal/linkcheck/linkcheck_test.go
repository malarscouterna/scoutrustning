package linkcheck

import (
	"context"
	"testing"
)

func TestCheckRejectsUnsafeTargets(t *testing.T) {
	checker := New()
	tests := []struct {
		name string
		url  string
	}{
		{"non http scheme", "file:///etc/passwd"},
		{"ftp scheme", "ftp://example.com/file"},
		{"loopback literal", "http://127.0.0.1:8080/"},
		{"ipv6 loopback literal", "http://[::1]:8080/"},
		{"private literal", "http://192.168.1.10/"},
		{"link local literal", "http://169.254.169.254/latest/meta-data/"},
		{"credentials in url", "http://user:pass@example.com/"},
		{"missing host", "http:///path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.Check(context.Background(), tt.url)
			if result.OK {
				t.Fatalf("expected %q to be rejected", tt.url)
			}
			if result.FailureKind != "invalid" {
				t.Errorf("expected failure kind invalid, got %q (%s)", result.FailureKind, result.Error)
			}
		})
	}
}

func TestCheckRejectsPrivateHostnameResolution(t *testing.T) {
	checker := New()
	// localhost resolves to a loopback address; the dial-time check must stop it
	// even though the literal above is not used.
	result := checker.Check(context.Background(), "http://localhost:9/")
	if result.OK {
		t.Fatal("expected localhost to be rejected")
	}
	if result.FailureKind != "blocked" {
		t.Errorf("expected blocked, got %q (%s)", result.FailureKind, result.Error)
	}
}
