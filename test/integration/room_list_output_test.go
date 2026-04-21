package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoomListDefaultOutputOmitsVerboseColumns(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	huectlBinary := mustBuildHuectlBinary(t, repoRoot)

	server := newRoomListServer()
	defer server.Close()

	configPath := mustWriteCLIConfig(t, roomServerAddress(server), roomServerFingerprint(server))

	stdout, stderr, exitCode := runHuectlBinary(
		t,
		repoRoot,
		huectlBinary,
		"--config", configPath,
		"--bridge", "sim",
		"room", "list",
	)
	if exitCode != 0 {
		t.Fatalf("room list failed: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected room table output, got:\n%s", stdout)
	}
	header := strings.Fields(lines[0])
	if len(header) != 2 || header[0] != "BRIDGE" || header[1] != "NAME" {
		t.Fatalf("expected compact BRIDGE NAME header, got %q", lines[0])
	}
	if strings.Contains(lines[0], "DETAILS") || strings.Contains(lines[0], "TYPE") || strings.Contains(lines[0], " ID") {
		t.Fatalf("default room list must omit verbose columns:\n%s", stdout)
	}
	if strings.Contains(stdout, "summary:") || strings.Contains(stdout, "\nmeta: ") {
		t.Fatalf("default room list must omit routine summary/meta:\n%s", stdout)
	}
}

func TestRoomListJSONDefaultOmitsVerboseFields(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	huectlBinary := mustBuildHuectlBinary(t, repoRoot)

	server := newRoomListServer()
	defer server.Close()

	configPath := mustWriteCLIConfig(t, roomServerAddress(server), roomServerFingerprint(server))

	stdout, stderr, exitCode := runHuectlBinary(
		t,
		repoRoot,
		huectlBinary,
		"--config", configPath,
		"--bridge", "sim",
		"--json",
		"room", "list",
	)
	if exitCode != 0 {
		t.Fatalf("room list --json failed: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
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

	meta := envelope["meta"].(map[string]any)
	if got := meta["schema"]; got != "huectl/v1" {
		t.Fatalf("schema = %#v, want huectl/v1", got)
	}
	if got := meta["request_id"]; got != "" {
		t.Fatalf("request_id = %#v, want empty string below -vv", got)
	}

	data := envelope["data"].(map[string]any)
	rowsValue, ok := data["resource_rows"]
	if !ok {
		t.Fatalf("default json room list must expose resource_rows view data: %#v", data)
	}
	rows, ok := rowsValue.([]any)
	if !ok {
		t.Fatalf("resource_rows has unexpected type %T", rowsValue)
	}
	row := rows[0].(map[string]any)
	if _, ok := row["id"]; ok {
		t.Fatalf("default json room list must omit id: %#v", row)
	}
	if _, ok := row["details"]; ok {
		t.Fatalf("default json room list must omit details: %#v", row)
	}
}

func TestRoomListJSONVerboseIncludesIDAndDetails(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	huectlBinary := mustBuildHuectlBinary(t, repoRoot)

	server := newRoomListServer()
	defer server.Close()

	configPath := mustWriteCLIConfig(t, roomServerAddress(server), roomServerFingerprint(server))

	stdout, stderr, exitCode := runHuectlBinary(
		t,
		repoRoot,
		huectlBinary,
		"--config", configPath,
		"--bridge", "sim",
		"--json",
		"-v",
		"room", "list",
	)
	if exitCode != 0 {
		t.Fatalf("room list --json -v failed: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
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

	data := envelope["data"].(map[string]any)
	rowsValue, ok := data["resource_rows"]
	if !ok {
		t.Fatalf("verbose json room list must expose resource_rows view data: %#v", data)
	}
	rows, ok := rowsValue.([]any)
	if !ok {
		t.Fatalf("resource_rows has unexpected type %T", rowsValue)
	}
	row := rows[0].(map[string]any)
	if got := row["id"]; got != "room-1" {
		t.Fatalf("id = %#v, want room-1", got)
	}
	if got := row["details"]; got != "children=1, archetype=bathroom" {
		t.Fatalf("details = %#v, want verbose details", got)
	}
}

func TestRoomListJSONDiagnosticIncludesMetaFields(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	huectlBinary := mustBuildHuectlBinary(t, repoRoot)

	server := newRoomListServer()
	defer server.Close()

	configPath := mustWriteCLIConfig(t, roomServerAddress(server), roomServerFingerprint(server))

	stdout, stderr, exitCode := runHuectlBinary(
		t,
		repoRoot,
		huectlBinary,
		"--config", configPath,
		"--bridge", "sim",
		"--json",
		"-vv",
		"room", "list",
	)
	if exitCode != 0 {
		t.Fatalf("room list --json -vv failed: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
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

	meta := envelope["meta"].(map[string]any)
	if meta["request_id"] == "" {
		t.Fatalf("expected request_id in diagnostic json: %#v", meta)
	}
	if meta["timestamp"] == "" {
		t.Fatalf("expected timestamp in diagnostic json: %#v", meta)
	}
	bridgeScope := meta["bridge_scope"].([]any)
	if len(bridgeScope) != 1 || bridgeScope[0] != "sim" {
		t.Fatalf("bridge_scope = %#v, want [sim]", bridgeScope)
	}
	if got := meta["duration_ms"]; got == float64(0) {
		t.Fatalf("expected non-zero duration_ms in diagnostic json: %#v", meta)
	}
}

func newRoomListServer() *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/room" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"room-1","type":"room","metadata":{"name":"Bathroom","archetype":"bathroom"},"children":["light-1"]}]}`))
			return
		}
		http.NotFound(w, r)
	}))
}

func roomServerFingerprint(server *httptest.Server) string {
	sum := sha256.Sum256(server.Certificate().Raw)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func roomServerAddress(server *httptest.Server) string {
	return strings.TrimPrefix(server.URL, "https://")
}
