package simulator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/data219/huectl/internal/store/config"
)

func TestBridgeLinkStoresUsernameAndFingerprint(t *testing.T) {
	env := loadSimulatorEnv()
	ensureSimulatorReachable(t, env)

	configPath := writeSimulatorConfigWithoutCredentials(t, env)
	setSimulatorLinkButtonTimestamp(t, env, time.Now().Unix())

	stdout, stderr, exitCode := runHuectl(t, configPath, "bridge", "link")
	if exitCode != 0 {
		t.Fatalf("bridge link failed: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	assertEnvelopeMeta(t, stdout)

	envelope := decodeEnvelope(t, stdout)
	data, _ := envelope["data"].(map[string]any)
	if strings.TrimSpace(fmt.Sprintf("%v", data["cert_fingerprint"])) == "" {
		t.Fatalf("expected cert_fingerprint in response, payload:\n%s", stdout)
	}
	if _, ok := data["username"]; ok {
		t.Fatalf("link response must redact username, payload:\n%s", stdout)
	}
	if _, ok := data["client_key"]; ok {
		t.Fatalf("link response must redact client_key, payload:\n%s", stdout)
	}

	store := config.NewFileStore(configPath)
	cfg, appErr := store.Load()
	if appErr != nil {
		t.Fatalf("load linked config failed: %v", appErr)
	}

	var bridge configBridgeView
	found := false
	for _, candidate := range cfg.Bridges {
		if candidate.ID == env.BridgeID {
			bridge = configBridgeView{
				Username:        candidate.Username,
				ClientKey:       candidate.ClientKey,
				CertFingerprint: candidate.CertFingerprint,
				APIBaseV1:       candidate.APIBaseV1,
				APIBaseV2:       candidate.APIBaseV2,
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("bridge %q not found in linked config", env.BridgeID)
	}
	if strings.TrimSpace(bridge.Username) == "" {
		t.Fatal("expected username to be persisted after link")
	}
	if strings.TrimSpace(bridge.ClientKey) == "" {
		t.Fatal("expected client_key to be persisted after link")
	}
	if strings.TrimSpace(bridge.CertFingerprint) == "" {
		t.Fatal("expected cert_fingerprint to be persisted after link")
	}
	if strings.TrimSpace(bridge.APIBaseV1) == "" || strings.TrimSpace(bridge.APIBaseV2) == "" {
		t.Fatalf("expected API base URLs to be persisted, got v1=%q v2=%q", bridge.APIBaseV1, bridge.APIBaseV2)
	}
}

func TestBridgeLinkFailsWhenLinkButtonIsNotPressed(t *testing.T) {
	env := loadSimulatorEnv()
	ensureSimulatorReachable(t, env)

	configPath := writeSimulatorConfigWithoutCredentials(t, env)
	setSimulatorLinkButtonTimestamp(t, env, time.Now().Add(-2*time.Hour).Unix())

	stdout, stderr, exitCode := runHuectl(t, configPath, "bridge", "link")
	if exitCode == 0 {
		t.Fatalf("expected bridge link to fail without link button\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}

	envelope := decodeEnvelope(t, stdout)
	errorObject, ok := envelope["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected JSON error object, payload:\n%s", stdout)
	}
	if code := strings.TrimSpace(fmt.Sprintf("%v", errorObject["code"])); code != "LINK_FAILED" {
		t.Fatalf("unexpected error code: got=%q want=%q", code, "LINK_FAILED")
	}
}

type configBridgeView struct {
	Username        string
	ClientKey       string
	CertFingerprint string
	APIBaseV1       string
	APIBaseV2       string
}

func writeSimulatorConfigWithoutCredentials(t *testing.T, env simulatorEnv) string {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := fmt.Sprintf(`version: 1
default_bridge: %s
bridges:
  - id: %s
    name: %s
    address: %s
`, env.BridgeID, env.BridgeID, env.BridgeName, env.Address)

	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write simulator config failed: %v", err)
	}
	return configPath
}

func setSimulatorLinkButtonTimestamp(t *testing.T, env simulatorEnv, timestamp int64) {
	t.Helper()

	script := fmt.Sprintf(
		"set -e\n"+
			"curl -fsS -X PUT -H 'Content-Type: application/json' -d '{\"linkbutton\":{\"lastlinkbuttonpushed\":%d}}' http://127.0.0.1/api/internal/config >/dev/null\n",
		timestamp,
	)
	cmd := exec.Command("docker", "compose", "-f", env.Compose, "exec", "-T", env.Service, "sh", "-lc", script)
	cmd.Dir = repoRootPath
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("set simulator link button timestamp failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
}

func decodeEnvelope(t *testing.T, payload string) map[string]any {
	t.Helper()

	var envelope map[string]any
	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		t.Fatalf("expected JSON output, got parse error: %v\npayload:\n%s", err, payload)
	}
	return envelope
}
