package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRejectsInsecurePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("version: 1\nbridges: []\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	store := NewFileStore(path)
	_, err := store.Load()
	if err == nil {
		t.Fatal("expected permission error")
	}
	if err.Code != "CONFIG_PERMISSIONS" {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestLoadAcceptsStrictPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("version: 1\nbridges: []\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	store := NewFileStore(path)
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Version != 1 {
		t.Fatalf("unexpected version: %d", cfg.Version)
	}
}
