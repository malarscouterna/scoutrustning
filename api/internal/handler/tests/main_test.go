package tests

import (
	"os"
	"testing"

	"github.com/malarscouterna/ms-utrustning/api/internal/testutil"
)

func TestMain(m *testing.M) {
	// Fixed 32-byte hex key for tests — never used in production.
	os.Setenv("SETTINGS_ENCRYPTION_KEY", "0000000000000000000000000000000000000000000000000000000000000000")
	testutil.SetupShared()
	code := m.Run()
	testutil.TeardownShared()
	os.Exit(code)
}
