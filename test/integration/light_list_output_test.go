package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLightListDefaultOutputIsHumanReadable(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	huectlBinary := mustBuildHuectlBinary(t, repoRoot)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/light" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"light-1","metadata":{"name":"Kitchen"}}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	sum := sha256.Sum256(server.Certificate().Raw)
	fingerprint := strings.ToUpper(hex.EncodeToString(sum[:]))
	configPath := mustWriteCLIConfig(t, strings.TrimPrefix(server.URL, "https://"), fingerprint)

	stdout, stderr, exitCode := runHuectlBinary(
		t,
		repoRoot,
		huectlBinary,
		"--config", configPath,
		"--bridge", "sim",
		"light", "list",
	)
	if exitCode != 0 {
		t.Fatalf("light list failed: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}

	trimmed := strings.TrimSpace(stdout)
	if strings.HasPrefix(trimmed, "{") || strings.Contains(stdout, "\"meta\"") {
		t.Fatalf("expected human-readable output, got JSON-like payload:\n%s", stdout)
	}
	if !strings.Contains(stdout, "BRIDGE") {
		t.Fatalf("expected table header BRIDGE in output:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Kitchen") {
		t.Fatalf("expected resource row for Kitchen in output:\n%s", stdout)
	}
	if strings.Contains(stdout, "resources=") {
		t.Fatalf("expected resource rows, got bridge-summary style output:\n%s", stdout)
	}
}

func TestLightListJSONOutputEnvelope(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	huectlBinary := mustBuildHuectlBinary(t, repoRoot)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/light" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"light-1","metadata":{"name":"Kitchen"}}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	sum := sha256.Sum256(server.Certificate().Raw)
	fingerprint := strings.ToUpper(hex.EncodeToString(sum[:]))
	configPath := mustWriteCLIConfig(t, strings.TrimPrefix(server.URL, "https://"), fingerprint)

	stdout, stderr, exitCode := runHuectlBinary(
		t,
		repoRoot,
		huectlBinary,
		"--config", configPath,
		"--bridge", "sim",
		"--json",
		"light", "list",
	)
	if exitCode != 0 {
		t.Fatalf("light list --json failed: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}

	var envelope map[string]any
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("parse envelope json failed: %v\npayload:\n%s", err, stdout)
	}
	if value, ok := envelope["error"]; !ok {
		t.Fatalf("success json must include top-level error key with null value: %#v", envelope)
	} else if value != nil {
		t.Fatalf("success json error = %#v, want nil", value)
	}
	meta, ok := envelope["meta"].(map[string]any)
	if !ok {
		t.Fatalf("missing meta object in payload:\n%s", stdout)
	}
	if schema := fmt.Sprintf("%v", meta["schema"]); schema != "huectl/v1" {
		t.Fatalf("unexpected schema: %q", schema)
	}
}

func mustRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func mustBuildHuectlBinary(t *testing.T, repoRoot string) string {
	t.Helper()
	outputPath := filepath.Join(t.TempDir(), "huectl")
	cmd := exec.Command("go", "build", "-o", outputPath, "./cmd/huectl")
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build huectl binary failed: %v\nstderr:\n%s", err, stderr.String())
	}
	return outputPath
}

func mustWriteCLIConfig(t *testing.T, address string, fingerprint string) string {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := fmt.Sprintf(`version: 1
default_bridge: sim
bridges:
  - id: sim
    name: Simulator
    address: %s
    username: test-user
    cert_fingerprint: %s
`, address, fingerprint)
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

func runHuectlBinary(t *testing.T, repoRoot string, binaryPath string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = repoRoot
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return stdout.String(), stderr.String(), exitErr.ExitCode()
	}
	return stdout.String(), stderr.String(), -1
}
