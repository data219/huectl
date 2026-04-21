package output

import (
	"testing"
	"time"

	"github.com/data219/huectl/internal/domain"
)

func TestBuildViewRoomListDefaultOmitsVerboseFields(t *testing.T) {
	view := BuildView(
		BuildSuccess("room list", []string{"bridge-a"}, time.Now(), sampleRoomAggregate(), false),
		RenderOptions{Verbosity: 0, JSON: false},
	)

	rows, ok := view.Data.(AggregateView)
	if !ok {
		t.Fatalf("view.Data type = %T, want AggregateView", view.Data)
	}
	if len(rows.ResourceRows) != 1 {
		t.Fatalf("resource rows = %d, want 1", len(rows.ResourceRows))
	}

	row := rows.ResourceRows[0]
	if _, ok := row["id"]; ok {
		t.Fatal("default room list must omit id")
	}
	if _, ok := row["details"]; ok {
		t.Fatal("default room list must omit details")
	}
	if _, ok := row["type"]; ok {
		t.Fatal("default room list must omit redundant type")
	}
	if _, ok := row["status"]; ok {
		t.Fatal("default room list must omit meaningless status")
	}
	if view.Meta.RequestID != "" {
		t.Fatal("default view must omit request_id")
	}
}

func TestBuildViewRoomListVerboseIncludesIDAndDetails(t *testing.T) {
	view := BuildView(
		BuildSuccess("room list", []string{"bridge-a"}, time.Now(), sampleRoomAggregate(), false),
		RenderOptions{Verbosity: 1, JSON: false},
	)

	rows, ok := view.Data.(AggregateView)
	if !ok {
		t.Fatalf("view.Data type = %T, want AggregateView", view.Data)
	}

	row := rows.ResourceRows[0]
	if row["id"] == "" {
		t.Fatal("verbose room list must expose id")
	}
	if row["details"] == "" {
		t.Fatal("verbose room list must expose details")
	}
	if _, ok := row["type"]; ok {
		t.Fatal("verbose room list must still omit redundant type")
	}
	if view.Meta.RequestID != "" {
		t.Fatal("verbosity 1 must still omit diagnostic request_id")
	}
}

func TestBuildViewPartialSuccessKeepsBridgeErrorsAtDefault(t *testing.T) {
	view := BuildView(
		BuildSuccess("bridge health", []string{"*"}, time.Now(), samplePartialAggregate(), true),
		RenderOptions{Verbosity: 0, JSON: false},
	)

	rows, ok := view.Data.(AggregateView)
	if !ok {
		t.Fatalf("view.Data type = %T, want AggregateView", view.Data)
	}
	if !view.Meta.PartialSuccess {
		t.Fatal("partial success must remain visible at default verbosity")
	}
	if len(rows.BridgeErrors) == 0 {
		t.Fatal("bridge errors must remain visible at default verbosity")
	}
	if rows.Summary != nil {
		t.Fatalf("summary = %#v, want nil below -vv", rows.Summary)
	}
}

func sampleRoomAggregate() domain.AggregateResult {
	return domain.AggregateResult{
		Items: []domain.BridgeResult{
			{
				BridgeID:   "bridge-a",
				BridgeName: "Bridge A",
				Success:    true,
				StatusCode: 200,
				Data: map[string]any{
					"resources": []map[string]any{
						{
							"id":   "room-1",
							"type": "room",
							"metadata": map[string]any{
								"name":      "Bathroom",
								"archetype": "bathroom",
							},
							"children": []any{"light-1"},
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
	}
}

func samplePartialAggregate() domain.AggregateResult {
	return domain.AggregateResult{
		Items: []domain.BridgeResult{
			{
				BridgeID:   "bridge-a",
				BridgeName: "Bridge A",
				Success:    true,
				StatusCode: 200,
				Data: map[string]any{
					"response": map[string]any{"ok": true},
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
	}
}
