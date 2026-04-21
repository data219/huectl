package simulator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type matrixCase struct {
	ID   string
	Args []string
}

type unsupportedAllowlist struct {
	Unsupported []string `yaml:"unsupported"`
}

func TestSimulatorCoverageMatrix(t *testing.T) {
	env := loadSimulatorEnv()
	ensureSimulatorReachable(t, env)

	username := provisionSimulatorUser(t, env)
	fingerprint := fetchSimulatorFingerprint(t, env)
	configPath := writeSimulatorConfig(t, env, username, fingerprint)

	allowlist := loadUnsupportedAllowlist(t)
	cases := []matrixCase{
		{ID: "bridge.list", Args: []string{"bridge", "list"}},
		{ID: "bridge.show", Args: []string{"bridge", "show", "--id", "sim"}},
		{ID: "bridge.health", Args: []string{"bridge", "health"}},
		{ID: "bridge.capabilities", Args: []string{"bridge", "capabilities"}},
		{ID: "device.list", Args: []string{"device", "list"}},
		{ID: "light.list", Args: []string{"light", "list"}},
		{ID: "room.list", Args: []string{"room", "list"}},
		{ID: "zone.list", Args: []string{"zone", "list"}},
		{ID: "scene.list", Args: []string{"scene", "list"}},
		{ID: "automation.list", Args: []string{"automation", "list"}},
		{ID: "sensor.list", Args: []string{"sensor", "list"}},
		{ID: "update.list", Args: []string{"update", "list"}},
		{ID: "diagnose.ping", Args: []string{"diagnose", "ping"}},
		{ID: "diagnose.latency", Args: []string{"diagnose", "latency"}},
		{ID: "diagnose.events", Args: []string{"diagnose", "events", "--duration", "1s"}},
		{ID: "api.get.light", Args: []string{"api", "get", "--path", "/resource/light"}},
		{ID: "api.get.device", Args: []string{"api", "get", "--path", "/resource/device"}},
	}

	unexpectedUnsupported := make([]string, 0)
	nowSupported := make([]string, 0)

	for _, testCase := range cases {
		stdout, stderr, exitCode := runHuectl(t, configPath, testCase.Args...)
		_, isAllowlisted := allowlist[testCase.ID]

		if exitCode == 0 {
			assertEnvelopeMeta(t, stdout)
			if isAllowlisted {
				nowSupported = append(nowSupported, testCase.ID)
			}
			continue
		}

		if !isAllowlisted {
			unexpectedUnsupported = append(
				unexpectedUnsupported,
				fmt.Sprintf("%s failed with exit=%d\nstdout:\n%s\nstderr:\n%s", testCase.ID, exitCode, stdout, stderr),
			)
		}
	}

	if len(nowSupported) > 0 {
		sort.Strings(nowSupported)
		t.Fatalf("remove commands from unsupported_allowlist.yaml; they now pass: %s", strings.Join(nowSupported, ", "))
	}

	if len(unexpectedUnsupported) > 0 {
		sort.Strings(unexpectedUnsupported)
		t.Fatalf("new simulator gaps detected:\n%s", strings.Join(unexpectedUnsupported, "\n\n"))
	}
}

func loadUnsupportedAllowlist(t *testing.T) map[string]struct{} {
	t.Helper()

	path := filepath.Join(repoRootPath, "test/e2e/simulator/unsupported_allowlist.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read unsupported allowlist failed: %v", err)
	}

	var raw unsupportedAllowlist
	if err := yaml.Unmarshal(content, &raw); err != nil {
		t.Fatalf("parse unsupported allowlist failed: %v", err)
	}

	allowlist := make(map[string]struct{}, len(raw.Unsupported))
	for _, item := range raw.Unsupported {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		allowlist[trimmed] = struct{}{}
	}
	return allowlist
}
