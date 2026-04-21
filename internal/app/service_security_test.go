package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/data219/huectl/internal/domain"
	storeconfig "github.com/data219/huectl/internal/store/config"
)

func TestExecuteLightListRequiresStoredFingerprint(t *testing.T) {
	configPath := writeServiceTestConfig(t, `version: 1
default_bridge: alpha
bridges:
  - id: alpha
    name: Alpha
    address: 127.0.0.1:443
    username: test-user
`)

	service := NewService(configPath, time.Second)
	_, _, appErr := service.Execute(context.Background(), domain.CommandContext{Bridge: "alpha"}, "light", "list", ActionInput{})
	if appErr == nil {
		t.Fatal("expected fingerprint requirement error")
	}
	if appErr.Code != "FINGERPRINT_REQUIRED" {
		t.Fatalf("unexpected error code: %s", appErr.Code)
	}
}

func TestBridgeHealthRequiresStoredFingerprint(t *testing.T) {
	configPath := writeServiceTestConfig(t, `version: 1
default_bridge: alpha
bridges:
  - id: alpha
    name: Alpha
    address: 127.0.0.1:443
    username: test-user
`)

	service := NewService(configPath, time.Second)
	_, _, appErr := service.Execute(context.Background(), domain.CommandContext{Bridge: "alpha"}, "bridge", "health", ActionInput{})
	if appErr == nil {
		t.Fatal("expected fingerprint requirement error")
	}
	if appErr.Code != "FINGERPRINT_REQUIRED" {
		t.Fatalf("unexpected error code: %s", appErr.Code)
	}
}

func TestRemoveBridgeByNameUpdatesDefaultBridge(t *testing.T) {
	configPath := writeServiceTestConfig(t, `version: 1
default_bridge: alpha
bridges:
  - id: alpha
    name: Alpha
    address: 192.168.1.10
  - id: beta
    name: Beta
    address: 192.168.1.11
`)

	service := NewService(configPath, time.Second)
	_, _, appErr := service.removeBridge("Alpha")
	if appErr != nil {
		t.Fatalf("removeBridge error: %v", appErr)
	}

	store := storeconfig.NewFileStore(configPath)
	cfg, loadErr := store.Load()
	if loadErr != nil {
		t.Fatalf("load config after removal: %v", loadErr)
	}
	if cfg.DefaultBridge != "beta" {
		t.Fatalf("expected default bridge to move to beta, got %q", cfg.DefaultBridge)
	}
	if len(cfg.Bridges) != 1 {
		t.Fatalf("expected one remaining bridge, got %d", len(cfg.Bridges))
	}
	if !strings.EqualFold(cfg.Bridges[0].ID, "beta") {
		t.Fatalf("expected remaining bridge beta, got %q", cfg.Bridges[0].ID)
	}
}

func writeServiceTestConfig(t *testing.T, content string) string {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}
	return configPath
}
