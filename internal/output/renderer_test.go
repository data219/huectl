package output

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/data219/huectl/internal/domain"
)

func TestWriteHumanSuccessGolden(t *testing.T) {
	env := Envelope{
		Meta: Meta{
			Schema:         SchemaVersion,
			Command:        "light set",
			RequestID:      "req-1",
			Timestamp:      "2026-03-03T10:00:00Z",
			BridgeScope:    []string{"bridge-a"},
			DurationMS:     12,
			PartialSuccess: false,
		},
		Data: map[string]any{
			"bridge": "alpha",
			"result": "ok",
		},
	}

	var buf bytes.Buffer
	if err := Write(&buf, false, env); err != nil {
		t.Fatalf("write human success: %v", err)
	}

	expected := readGolden(t, "human_success.golden")
	if buf.String() != expected {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", buf.String(), expected)
	}
}

func TestWriteHumanErrorGolden(t *testing.T) {
	env := Envelope{
		Meta: Meta{
			Schema:         SchemaVersion,
			Command:        "light set",
			RequestID:      "req-2",
			Timestamp:      "2026-03-03T10:00:00Z",
			BridgeScope:    []string{"bridge-a"},
			DurationMS:     23,
			PartialSuccess: true,
		},
		Error: &ErrorData{
			Code:    "TARGET_AMBIGUOUS",
			Message: "Target name 'Kitchen' matches multiple resources.",
			Hints:   []string{"Use --bridge", "Use composite id bridge/resource"},
			Details: map[string]any{"matches": 2},
		},
	}

	var buf bytes.Buffer
	if err := Write(&buf, false, env); err != nil {
		t.Fatalf("write human error: %v", err)
	}

	expected := readGolden(t, "human_error.golden")
	if buf.String() != expected {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", buf.String(), expected)
	}
}

func TestWriteHumanErrorDiagnosticGolden(t *testing.T) {
	env := Envelope{
		Meta: Meta{
			Schema:         SchemaVersion,
			Command:        "light set",
			RequestID:      "req-2",
			Timestamp:      "2026-03-03T10:00:00Z",
			BridgeScope:    []string{"bridge-a"},
			DurationMS:     23,
			PartialSuccess: true,
		},
		Error: &ErrorData{
			Code:    "TARGET_AMBIGUOUS",
			Message: "Target name 'Kitchen' matches multiple resources.",
			Hints:   []string{"Use --bridge", "Use composite id bridge/resource"},
			Details: map[string]any{"matches": 2},
		},
	}

	var buf bytes.Buffer
	if err := WriteWithVerbosity(&buf, false, 2, env); err != nil {
		t.Fatalf("write human error diagnostic: %v", err)
	}

	expected := readGolden(t, "human_error_vv.golden")
	if buf.String() != expected {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", buf.String(), expected)
	}
}

func TestWriteHumanLightListGolden(t *testing.T) {
	env := Envelope{
		Meta: Meta{
			Schema:         SchemaVersion,
			Command:        "light list",
			RequestID:      "req-3",
			Timestamp:      "2026-03-03T10:00:00Z",
			BridgeScope:    []string{"bridge-a"},
			DurationMS:     17,
			PartialSuccess: false,
		},
		Data: domain.AggregateResult{
			Items: []domain.BridgeResult{
				{
					BridgeID:   "bridge-a",
					BridgeName: "Bridge A",
					Success:    true,
					StatusCode: 200,
					Data: map[string]any{
						"resources": []map[string]any{
							{
								"id":   "light-1",
								"type": "light",
								"metadata": map[string]any{
									"name": "Kitchen",
								},
								"on": map[string]any{
									"on": true,
								},
							},
							{
								"id":   "light-2",
								"type": "light",
								"metadata": map[string]any{
									"name": "Desk",
								},
								"on": map[string]any{
									"on": false,
								},
							},
						},
					},
				},
			},
			Summary: map[string]any{
				"bridges_total":   1,
				"bridges_success": 1,
				"bridges_failed":  0,
			},
		},
	}

	var buf bytes.Buffer
	if err := Write(&buf, false, env); err != nil {
		t.Fatalf("write human light list: %v", err)
	}

	expected := readGolden(t, "human_light_list.golden")
	if buf.String() != expected {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", buf.String(), expected)
	}
}

func TestWriteHumanLightListEmptyGolden(t *testing.T) {
	env := Envelope{
		Meta: Meta{
			Schema:         SchemaVersion,
			Command:        "light list",
			RequestID:      "req-empty",
			Timestamp:      "2026-03-03T10:00:00Z",
			BridgeScope:    []string{"bridge-a"},
			DurationMS:     8,
			PartialSuccess: false,
		},
		Data: domain.AggregateResult{
			Items: []domain.BridgeResult{
				{
					BridgeID:   "bridge-a",
					BridgeName: "Bridge A",
					Success:    true,
					StatusCode: 200,
					Data: map[string]any{
						"resources": []map[string]any{},
					},
				},
			},
			Summary: map[string]any{
				"bridges_total":   1,
				"bridges_success": 1,
				"bridges_failed":  0,
			},
		},
	}

	var buf bytes.Buffer
	if err := Write(&buf, false, env); err != nil {
		t.Fatalf("write human light list empty: %v", err)
	}

	expected := readGolden(t, "human_light_list_empty.golden")
	if buf.String() != expected {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", buf.String(), expected)
	}
}

func TestWriteHumanBridgeHealthGolden(t *testing.T) {
	env := Envelope{
		Meta: Meta{
			Schema:         SchemaVersion,
			Command:        "bridge health",
			RequestID:      "req-4",
			Timestamp:      "2026-03-03T10:00:00Z",
			BridgeScope:    []string{"bridge-a", "bridge-b"},
			DurationMS:     22,
			PartialSuccess: true,
		},
		Data: domain.AggregateResult{
			Items: []domain.BridgeResult{
				{
					BridgeID:   "bridge-a",
					BridgeName: "Bridge A",
					Success:    true,
					StatusCode: 200,
					Data: map[string]any{
						"response": map[string]any{"ok": true},
						"source":   "v2",
					},
				},
				{
					BridgeID:   "bridge-b",
					BridgeName: "Bridge B",
					Success:    false,
					StatusCode: 503,
					Error:      "BRIDGE_REQUEST: timeout",
				},
			},
			Summary: map[string]any{
				"bridges_total":   2,
				"bridges_success": 1,
				"bridges_failed":  1,
			},
		},
	}

	var buf bytes.Buffer
	if err := Write(&buf, false, env); err != nil {
		t.Fatalf("write human bridge health: %v", err)
	}

	expected := readGolden(t, "human_bridge_health.golden")
	if buf.String() != expected {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", buf.String(), expected)
	}
}

func TestWriteHumanRoomListDefaultGolden(t *testing.T) {
	env := sampleRoomListEnvelope()

	var buf bytes.Buffer
	if err := Write(&buf, false, env); err != nil {
		t.Fatalf("write human room list default: %v", err)
	}

	expected := readGolden(t, "human_room_list_default.golden")
	if buf.String() != expected {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", buf.String(), expected)
	}
}

func TestWriteHumanRoomListVerboseGolden(t *testing.T) {
	env := sampleRoomListEnvelope()

	var buf bytes.Buffer
	if err := WriteWithVerbosity(&buf, false, 1, env); err != nil {
		t.Fatalf("write human room list verbose: %v", err)
	}

	expected := readGolden(t, "human_room_list_v.golden")
	if buf.String() != expected {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", buf.String(), expected)
	}
}

func TestWriteHumanRoomListDiagnosticGolden(t *testing.T) {
	env := sampleRoomListEnvelope()

	var buf bytes.Buffer
	if err := WriteWithVerbosity(&buf, false, 2, env); err != nil {
		t.Fatalf("write human room list diagnostic: %v", err)
	}

	expected := readGolden(t, "human_room_list_vv.golden")
	if buf.String() != expected {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", buf.String(), expected)
	}
}

func TestWriteHumanListRendersResourceRows(t *testing.T) {
	env := Envelope{
		Meta: Meta{
			Schema:         SchemaVersion,
			Command:        "light list",
			RequestID:      "req-list-resources",
			Timestamp:      "2026-03-03T10:00:00Z",
			BridgeScope:    []string{"bridge-a"},
			DurationMS:     17,
			PartialSuccess: false,
		},
		Data: domain.AggregateResult{
			Items: []domain.BridgeResult{
				{
					BridgeID:   "bridge-a",
					BridgeName: "Bridge A",
					Success:    true,
					StatusCode: 200,
					Data: map[string]any{
						"resources": []map[string]any{
							{
								"id":   "light-1",
								"type": "light",
								"metadata": map[string]any{
									"name": "Kitchen",
								},
							},
						},
					},
				},
			},
			Summary: map[string]any{
				"bridges_total":   1,
				"bridges_success": 1,
				"bridges_failed":  0,
			},
		},
	}

	var buf bytes.Buffer
	if err := Write(&buf, false, env); err != nil {
		t.Fatalf("write human list: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "NAME") {
		t.Fatalf("expected resource table with NAME header, got:\n%s", output)
	}
	if !strings.Contains(output, "Kitchen") {
		t.Fatalf("expected resource row with Kitchen, got:\n%s", output)
	}
	headerFields := strings.Fields(strings.Split(strings.TrimSpace(output), "\n")[0])
	if len(headerFields) != 2 || headerFields[0] != "BRIDGE" || headerFields[1] != "NAME" {
		t.Fatalf("expected compact default header BRIDGE NAME, got:\n%s", output)
	}
}

func TestWriteHumanListIncludesBridgeErrorsSection(t *testing.T) {
	env := Envelope{
		Meta: Meta{
			Schema:         SchemaVersion,
			Command:        "scene list",
			RequestID:      "req-list-partial",
			Timestamp:      "2026-03-03T10:00:00Z",
			BridgeScope:    []string{"bridge-a", "bridge-b"},
			DurationMS:     31,
			PartialSuccess: true,
		},
		Data: domain.AggregateResult{
			Items: []domain.BridgeResult{
				{
					BridgeID:   "bridge-a",
					BridgeName: "Bridge A",
					Success:    true,
					StatusCode: 200,
					Data: map[string]any{
						"resources": []map[string]any{
							{
								"id":   "scene-1",
								"type": "scene",
								"metadata": map[string]any{
									"name": "Evening",
								},
							},
						},
					},
				},
				{
					BridgeID:   "bridge-b",
					BridgeName: "Bridge B",
					Success:    false,
					StatusCode: 503,
					Error:      "BRIDGE_REQUEST: timeout",
				},
			},
			Summary: map[string]any{
				"bridges_total":   2,
				"bridges_success": 1,
				"bridges_failed":  1,
			},
		},
	}

	var buf bytes.Buffer
	if err := Write(&buf, false, env); err != nil {
		t.Fatalf("write human list partial: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "bridge errors:") {
		t.Fatalf("expected bridge errors section, got:\n%s", output)
	}
	if !strings.Contains(output, "Bridge B") {
		t.Fatalf("expected failing bridge row, got:\n%s", output)
	}
}

func TestWriteHumanPartialSuccessMetaVisibleByDefault(t *testing.T) {
	env := Envelope{
		Meta: Meta{
			Schema:         SchemaVersion,
			Command:        "light set",
			RequestID:      "req-partial-default",
			Timestamp:      "2026-03-05T21:15:00Z",
			BridgeScope:    []string{"bridge-a"},
			DurationMS:     14,
			PartialSuccess: true,
		},
		Data: map[string]any{
			"result": "ok",
		},
	}

	var buf bytes.Buffer
	if err := Write(&buf, false, env); err != nil {
		t.Fatalf("write human partial-success default: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "meta:") {
		t.Fatalf("expected meta line for partial success, got:\n%s", output)
	}
	if !strings.Contains(output, "partial_success=true") {
		t.Fatalf("expected partial_success=true in meta line, got:\n%s", output)
	}
}

func TestWriteJSONRoomListDefaultOmitsVerboseFields(t *testing.T) {
	env := sampleRoomListEnvelope()

	var buf bytes.Buffer
	if err := WriteWithVerbosity(&buf, true, 0, env); err != nil {
		t.Fatalf("write json room list default: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\npayload:\n%s", err, buf.String())
	}
	if value, ok := payload["error"]; !ok {
		t.Fatalf("success json must include top-level error key with null value: %#v", payload)
	} else if value != nil {
		t.Fatalf("success json error = %#v, want nil", value)
	}

	meta := payload["meta"].(map[string]any)
	if got := meta["schema"]; got != SchemaVersion {
		t.Fatalf("schema = %#v, want %q", got, SchemaVersion)
	}
	if got := meta["request_id"]; got != "" {
		t.Fatalf("request_id = %#v, want empty string", got)
	}
	if got := meta["timestamp"]; got != "" {
		t.Fatalf("timestamp = %#v, want empty string", got)
	}
	if got := meta["duration_ms"]; got != float64(0) {
		t.Fatalf("duration_ms = %#v, want 0", got)
	}

	data := payload["data"].(map[string]any)
	rows := data["resource_rows"].([]any)
	row := rows[0].(map[string]any)
	if _, ok := row["id"]; ok {
		t.Fatalf("default json room list must omit id: %#v", row)
	}
	if _, ok := row["details"]; ok {
		t.Fatalf("default json room list must omit details: %#v", row)
	}
	if _, ok := row["type"]; ok {
		t.Fatalf("default json room list must omit type: %#v", row)
	}
}

func TestWriteJSONRoomListVerboseIncludesIDAndDetails(t *testing.T) {
	env := sampleRoomListEnvelope()

	var buf bytes.Buffer
	if err := WriteWithVerbosity(&buf, true, 1, env); err != nil {
		t.Fatalf("write json room list verbose: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\npayload:\n%s", err, buf.String())
	}
	if value, ok := payload["error"]; !ok {
		t.Fatalf("success json must include top-level error key with null value: %#v", payload)
	} else if value != nil {
		t.Fatalf("success json error = %#v, want nil", value)
	}

	meta := payload["meta"].(map[string]any)
	if got := meta["request_id"]; got != "" {
		t.Fatalf("request_id = %#v, want empty string below -vv", got)
	}

	data := payload["data"].(map[string]any)
	rows := data["resource_rows"].([]any)
	row := rows[0].(map[string]any)
	if got := row["id"]; got != "room-1" {
		t.Fatalf("id = %#v, want room-1", got)
	}
	if got := row["details"]; got != "children=1, archetype=bathroom" {
		t.Fatalf("details = %#v, want children/archetype", got)
	}
}

func TestWriteJSONRoomListDiagnosticIncludesMeta(t *testing.T) {
	env := sampleRoomListEnvelope()

	var buf bytes.Buffer
	if err := WriteWithVerbosity(&buf, true, 2, env); err != nil {
		t.Fatalf("write json room list diagnostic: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v\npayload:\n%s", err, buf.String())
	}
	if value, ok := payload["error"]; !ok {
		t.Fatalf("success json must include top-level error key with null value: %#v", payload)
	} else if value != nil {
		t.Fatalf("success json error = %#v, want nil", value)
	}

	meta := payload["meta"].(map[string]any)
	if got := meta["request_id"]; got != "req-room-1" {
		t.Fatalf("request_id = %#v, want req-room-1", got)
	}
	if got := meta["timestamp"]; got != "2026-03-05T21:00:00Z" {
		t.Fatalf("timestamp = %#v, want fixed timestamp", got)
	}
	bridgeScope := meta["bridge_scope"].([]any)
	if len(bridgeScope) != 1 || bridgeScope[0] != "bridge-a" {
		t.Fatalf("bridge_scope = %#v, want [bridge-a]", bridgeScope)
	}
	if got := meta["duration_ms"]; got != float64(19) {
		t.Fatalf("duration_ms = %#v, want 19", got)
	}
}

func readGolden(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name)
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return string(payload)
}

func sampleRoomListEnvelope() Envelope {
	return Envelope{
		Meta: Meta{
			Schema:         SchemaVersion,
			Command:        "room list",
			RequestID:      "req-room-1",
			Timestamp:      "2026-03-05T21:00:00Z",
			BridgeScope:    []string{"bridge-a"},
			DurationMS:     19,
			PartialSuccess: false,
		},
		Data: sampleRoomAggregate(),
	}
}
