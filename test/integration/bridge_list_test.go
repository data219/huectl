package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/data219/huectl/internal/app"
	"github.com/data219/huectl/internal/domain"
)

func TestBridgeListReadsConfiguredBridges(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`version: 1
bridges:
  - id: alpha
    name: alpha
    address: 192.168.1.10
  - id: beta
    name: beta
    address: 192.168.1.11
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	svc := app.NewService(configPath, 2*time.Second)
	result, partial, err := svc.Execute(context.Background(), domain.CommandContext{}, "bridge", "list", app.ActionInput{})
	if err != nil {
		t.Fatalf("execute bridge list: %v", err)
	}
	if partial {
		t.Fatal("partial must be false")
	}
	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	bridges, ok := payload["bridges"].([]domain.Bridge)
	if !ok {
		t.Fatalf("unexpected bridges payload type: %T", payload["bridges"])
	}
	if len(bridges) != 2 {
		t.Fatalf("expected 2 bridges, got %d", len(bridges))
	}
}
