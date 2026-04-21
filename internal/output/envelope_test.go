package output

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildEnvelopeUsesStableSchema(t *testing.T) {
	started := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	env := BuildSuccess(
		"light set",
		[]string{"bridge-a", "bridge-b"},
		started,
		map[string]any{"ok": true},
		false,
	)

	if env.Meta.Schema != "huectl/v1" {
		t.Fatalf("unexpected schema: %s", env.Meta.Schema)
	}
	if env.Meta.Command != "light set" {
		t.Fatalf("unexpected command: %s", env.Meta.Command)
	}
	if env.Meta.PartialSuccess {
		t.Fatal("partial_success must be false")
	}

	payload, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	meta, ok := got["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta missing or invalid: %#v", got["meta"])
	}
	if meta["schema"] != "huectl/v1" {
		t.Fatalf("unexpected schema in JSON: %#v", meta["schema"])
	}
	if got["error"] != nil {
		t.Fatalf("expected null error, got %#v", got["error"])
	}
}

func TestBuildErrorEnvelope(t *testing.T) {
	started := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	errData := ErrorData{
		Code:    "TARGET_AMBIGUOUS",
		Message: "Target name 'Kitchen' matches multiple resources.",
		Hints:   []string{"Use --bridge", "Use composite id bridge/resource"},
		Details: map[string]any{"matches": 2},
	}
	env := BuildError("light set", []string{"bridge-a", "bridge-b"}, started, errData, true)

	if !env.Meta.PartialSuccess {
		t.Fatal("partial_success must be true")
	}
	if env.Error == nil {
		t.Fatal("error must be set")
	}
	if env.Error.Code != "TARGET_AMBIGUOUS" {
		t.Fatalf("unexpected error code: %s", env.Error.Code)
	}
}

func TestMetaFilteredOmitsDiagnosticsBelowVV(t *testing.T) {
	meta := Meta{
		Schema:         SchemaVersion,
		Command:        "room list",
		RequestID:      "req-123",
		Timestamp:      "2026-03-05T20:00:00Z",
		BridgeScope:    []string{"bridge-a"},
		DurationMS:     42,
		PartialSuccess: true,
	}

	got := meta.filtered(1)

	if got.Schema != SchemaVersion {
		t.Fatalf("schema = %q, want %q", got.Schema, SchemaVersion)
	}
	if got.Command != "room list" {
		t.Fatalf("command = %q, want room list", got.Command)
	}
	if !got.PartialSuccess {
		t.Fatal("partial success must stay visible below -vv")
	}
	if got.RequestID != "" {
		t.Fatalf("request_id = %q, want empty", got.RequestID)
	}
	if got.Timestamp != "" {
		t.Fatalf("timestamp = %q, want empty", got.Timestamp)
	}
	if got.DurationMS != 0 {
		t.Fatalf("duration_ms = %d, want 0", got.DurationMS)
	}
	if len(got.BridgeScope) != 0 {
		t.Fatalf("bridge_scope = %#v, want empty", got.BridgeScope)
	}
}

func TestMetaFilteredKeepsDiagnosticsAtVV(t *testing.T) {
	meta := Meta{
		Schema:         SchemaVersion,
		Command:        "room list",
		RequestID:      "req-123",
		Timestamp:      "2026-03-05T20:00:00Z",
		BridgeScope:    []string{"bridge-a"},
		DurationMS:     42,
		PartialSuccess: true,
	}

	got := meta.filtered(2)

	if got.RequestID != "req-123" {
		t.Fatalf("request_id = %q, want req-123", got.RequestID)
	}
	if got.Timestamp != "2026-03-05T20:00:00Z" {
		t.Fatalf("timestamp = %q, want preserved", got.Timestamp)
	}
	if got.DurationMS != 42 {
		t.Fatalf("duration_ms = %d, want 42", got.DurationMS)
	}
	if len(got.BridgeScope) != 1 || got.BridgeScope[0] != "bridge-a" {
		t.Fatalf("bridge_scope = %#v, want preserved", got.BridgeScope)
	}
}

func TestErrorDataFilteredOmitsDetailsBelowVV(t *testing.T) {
	errData := ErrorData{
		Code:    "TARGET_AMBIGUOUS",
		Message: "Target name 'Kitchen' matches multiple resources.",
		Hints:   []string{"Use --bridge", "Use composite id bridge/resource"},
		Details: map[string]any{"matches": 2},
	}

	got := errData.filtered(1)

	if got.Code != "TARGET_AMBIGUOUS" {
		t.Fatalf("code = %q, want TARGET_AMBIGUOUS", got.Code)
	}
	if got.Message != "Target name 'Kitchen' matches multiple resources." {
		t.Fatalf("message = %q, want preserved", got.Message)
	}
	if len(got.Hints) != 2 {
		t.Fatalf("hints = %#v, want preserved", got.Hints)
	}
	if got.Hints[0] != "Use --bridge" || got.Hints[1] != "Use composite id bridge/resource" {
		t.Fatalf("hints = %#v, want preserved", got.Hints)
	}
	if got.Details != nil {
		t.Fatalf("details = %#v, want nil below -vv", got.Details)
	}
}

func TestErrorDataFilteredKeepsDetailsAtVV(t *testing.T) {
	errData := ErrorData{
		Code:    "TARGET_AMBIGUOUS",
		Message: "Target name 'Kitchen' matches multiple resources.",
		Hints:   []string{"Use --bridge", "Use composite id bridge/resource"},
		Details: map[string]any{"matches": 2},
	}

	got := errData.filtered(2)

	if len(got.Hints) != 2 {
		t.Fatalf("hints = %#v, want preserved", got.Hints)
	}
	if got.Details == nil {
		t.Fatal("details must be preserved at -vv")
	}
	if got.Details["matches"] != 2 {
		t.Fatalf("details = %#v, want matches=2", got.Details)
	}
}
