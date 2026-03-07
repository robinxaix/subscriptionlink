package main

import (
	"os"
	"path/filepath"
	"testing"

	"subscriptionlink/internal/store"
)

func TestResolveAdminTokenCreatesDataDirWhenTokenProvided(t *testing.T) {
	base := t.TempDir()
	dataDir := filepath.Join(base, "runtime-data")
	store.SetDataDir(dataDir)

	token, generated, err := resolveAdminToken("provided-token")
	if err != nil {
		t.Fatalf("resolveAdminToken returned error: %v", err)
	}
	if generated {
		t.Fatalf("expected generated=false when token is provided")
	}
	if token != "provided-token" {
		t.Fatalf("expected provided token, got %q", token)
	}
	if _, err := os.Stat(dataDir); err != nil {
		t.Fatalf("expected data dir to be created, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "admin.key")); !os.IsNotExist(err) {
		t.Fatalf("expected admin.key not to be written when token is provided")
	}
}
