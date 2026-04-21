package simulator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/data219/huectl/internal/hue/common"
)

const (
	defaultSimHTTPBase  = "http://127.0.0.1:18080"
	defaultSimHTTPSBase = "https://127.0.0.1:18443"
	defaultSimAddress   = "127.0.0.1:18443"
	defaultSimService   = "diyhue"
)

var (
	repoRootPath string
	huectlBinary string
)

type simulatorEnv struct {
	HTTPBase   string
	HTTPSBase  string
	Address    string
	Compose    string
	Service    string
	HardFail   bool
	BridgeID   string
	BridgeName string
}

func TestMain(m *testing.M) {
	root, err := detectRepoRoot()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "detect repo root: %v\n", err)
		os.Exit(2)
	}
	repoRootPath = root

	tmpDir, err := os.MkdirTemp("", "huectl-e2e-bin-*")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "create temp dir: %v\n", err)
		os.Exit(2)
	}
	defer os.RemoveAll(tmpDir)

	huectlBinary = filepath.Join(tmpDir, "huectl")
	build := exec.Command("go", "build", "-o", huectlBinary, "./cmd/huectl")
	build.Dir = repoRootPath
	if output, err := build.CombinedOutput(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "build huectl binary failed: %v\n%s\n", err, string(output))
		os.Exit(2)
	}

	os.Exit(m.Run())
}

func TestSimulatorSmoke(t *testing.T) {
	env := loadSimulatorEnv()
	ensureSimulatorReachable(t, env)

	username := provisionSimulatorUser(t, env)
	fingerprint := fetchSimulatorFingerprint(t, env)
	configPath := writeSimulatorConfig(t, env, username, fingerprint)

	cases := []struct {
		name string
		args []string
	}{
		{name: "bridge-list", args: []string{"bridge", "list"}},
		{name: "light-list", args: []string{"light", "list"}},
		{name: "scene-list", args: []string{"scene", "list"}},
		{name: "api-get-light", args: []string{"api", "get", "--path", "/resource/light"}},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			stdout, stderr, exitCode := runHuectl(t, configPath, testCase.args...)
			if exitCode != 0 {
				t.Fatalf("command failed: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
			}
			assertEnvelopeMeta(t, stdout)
		})
	}

	t.Run("light-list-human-default", func(t *testing.T) {
		stdout, stderr, exitCode := runHuectlHuman(t, configPath, "light", "list")
		if exitCode != 0 {
			t.Fatalf("command failed: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
		}
		trimmed := strings.TrimSpace(stdout)
		if strings.HasPrefix(trimmed, "{") || strings.Contains(stdout, "\"meta\"") {
			t.Fatalf("expected human-readable default output, got JSON-like payload:\n%s", stdout)
		}
		if !strings.Contains(stdout, "BRIDGE") || !strings.Contains(stdout, "NAME") {
			if !strings.Contains(stdout, "No resources.") {
				t.Fatalf("expected resource-table human output, got:\n%s", stdout)
			}
		}
		if strings.Contains(stdout, "resources=") {
			t.Fatalf("expected resource rows or explicit empty state, got bridge-summary style output:\n%s", stdout)
		}
	})
}

func loadSimulatorEnv() simulatorEnv {
	httpBase := firstNonEmpty(os.Getenv("HUECTL_SIM_HTTP_BASE"), defaultSimHTTPBase)
	httpsBase := firstNonEmpty(os.Getenv("HUECTL_SIM_HTTPS_BASE"), defaultSimHTTPSBase)
	address := firstNonEmpty(os.Getenv("HUECTL_SIM_ADDRESS"), defaultSimAddress)
	service := firstNonEmpty(os.Getenv("HUECTL_SIM_SERVICE"), defaultSimService)
	composePath := firstNonEmpty(os.Getenv("HUECTL_SIM_COMPOSE_FILE"), filepath.Join(repoRootPath, "test/simulator/compose.yml"))

	return simulatorEnv{
		HTTPBase:   strings.TrimSuffix(httpBase, "/"),
		HTTPSBase:  strings.TrimSuffix(httpsBase, "/"),
		Address:    address,
		Compose:    composePath,
		Service:    service,
		HardFail:   isTruthy(os.Getenv("HUECTL_SIM_REQUIRED")),
		BridgeID:   "sim",
		BridgeName: "simulator",
	}
}

func ensureSimulatorReachable(t *testing.T, env simulatorEnv) {
	t.Helper()

	client := &http.Client{Timeout: 2 * time.Second}
	var lastErr error
	for range 20 {
		resp, err := client.Get(env.HTTPBase + "/api/config")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
			lastErr = fmt.Errorf("unexpected status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}

	msg := fmt.Sprintf("simulator is not reachable at %s/api/config: %v", env.HTTPBase, lastErr)
	if env.HardFail {
		t.Fatal(msg)
	}
	t.Skip(msg)
}

func provisionSimulatorUser(t *testing.T, env simulatorEnv) string {
	t.Helper()

	pressAndCreateScript := fmt.Sprintf(
		"set -e\n"+
			"curl -fsS -X PUT -H 'Content-Type: application/json' -d '{\"linkbutton\":{\"lastlinkbuttonpushed\":%d}}' http://127.0.0.1/api/internal/config >/dev/null\n"+
			"curl -fsS -X POST -H 'Content-Type: application/json' -d '{\"devicetype\":\"huectl#sim\",\"generateclientkey\":true}' http://127.0.0.1/api\n",
		time.Now().Unix(),
	)

	cmd := exec.Command("docker", "compose", "-f", env.Compose, "exec", "-T", env.Service, "sh", "-lc", pressAndCreateScript)
	cmd.Dir = repoRootPath
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("provision simulator user failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	var payload []struct {
		Success struct {
			Username string `json:"username"`
		} `json:"success"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("parse simulator user response failed: %v\npayload:\n%s", err, stdout.String())
	}
	if len(payload) == 0 || strings.TrimSpace(payload[0].Success.Username) == "" {
		t.Fatalf("simulator did not return a username:\n%s", stdout.String())
	}
	return payload[0].Success.Username
}

func writeSimulatorConfig(t *testing.T, env simulatorEnv, username string, fingerprint string) string {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := fmt.Sprintf(`version: 1
default_bridge: %s
bridges:
  - id: %s
    name: %s
    address: %s
    username: %s
    cert_fingerprint: %s
`, env.BridgeID, env.BridgeID, env.BridgeName, env.Address, username, fingerprint)

	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write simulator config failed: %v", err)
	}
	return configPath
}

func fetchSimulatorFingerprint(t *testing.T, env simulatorEnv) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fingerprint, err := common.FetchCertFingerprint(ctx, env.Address, 5*time.Second)
	if err != nil {
		t.Fatalf("fetch simulator certificate fingerprint failed: %v", err)
	}
	return fingerprint
}

func runHuectl(t *testing.T, configPath string, args ...string) (string, string, int) {
	t.Helper()

	cmdArgs := []string{"--config", configPath, "--bridge", "sim", "--json"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(huectlBinary, cmdArgs...)
	cmd.Dir = repoRootPath

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), exitCodeFromError(err)
}

func runHuectlHuman(t *testing.T, configPath string, args ...string) (string, string, int) {
	t.Helper()

	cmdArgs := []string{"--config", configPath, "--bridge", "sim"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(huectlBinary, cmdArgs...)
	cmd.Dir = repoRootPath

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), exitCodeFromError(err)
}

func assertEnvelopeMeta(t *testing.T, payload string) {
	t.Helper()

	var envelope map[string]any
	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		t.Fatalf("expected JSON output, got parse error: %v\npayload:\n%s", err, payload)
	}
	metaRaw, ok := envelope["meta"].(map[string]any)
	if !ok {
		t.Fatalf("missing meta object in payload: %s", payload)
	}
	if got := fmt.Sprintf("%v", metaRaw["schema"]); got != "huectl/v1" {
		t.Fatalf("unexpected schema: %q", got)
	}
}

func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func detectRepoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../..")), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
