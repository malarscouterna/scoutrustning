package tests

import (
	"os"
	"testing"

	"github.com/malarscouterna/ms-utrustning/api/internal/testutil"
)

func TestMain(m *testing.M) {
	testutil.SetupShared()
	code := m.Run()
	testutil.TeardownShared()
	os.Exit(code)
}
