package resolve

import "testing"

func TestResolveTargetReturnsAmbiguousError(t *testing.T) {
	candidates := []Candidate{
		{BridgeID: "b1", BridgeName: "alpha", ResourceID: "r1", ResourceName: "Kitchen", ResourceType: "room"},
		{BridgeID: "b2", BridgeName: "beta", ResourceID: "r2", ResourceName: "Kitchen", ResourceType: "room"},
	}

	resolved, err := ResolveTarget("Kitchen", candidates, false)
	if err == nil {
		t.Fatal("expected ambiguous error")
	}
	if err.Code != "TARGET_AMBIGUOUS" {
		t.Fatalf("unexpected code: %s", err.Code)
	}
	if len(resolved) != 0 {
		t.Fatalf("expected no resolved candidates, got %d", len(resolved))
	}
}

func TestResolveTargetReturnsAllMatchesInBroadcastMode(t *testing.T) {
	candidates := []Candidate{
		{BridgeID: "b1", BridgeName: "alpha", ResourceID: "r1", ResourceName: "Kitchen", ResourceType: "room"},
		{BridgeID: "b2", BridgeName: "beta", ResourceID: "r2", ResourceName: "Kitchen", ResourceType: "room"},
	}

	resolved, err := ResolveTarget("Kitchen", candidates, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("expected 2 resolved candidates, got %d", len(resolved))
	}
}

func TestResolveTargetMatchesV1ResourceID(t *testing.T) {
	candidates := []Candidate{
		{BridgeID: "b1", BridgeName: "alpha", ResourceID: "v2-r1", V1ResourceID: "7", ResourceName: "Kitchen", ResourceType: "light"},
	}

	resolved, err := ResolveTarget("7", candidates, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved candidate, got %d", len(resolved))
	}
	if resolved[0].V1ResourceID != "7" {
		t.Fatalf("unexpected v1 resource id: %q", resolved[0].V1ResourceID)
	}
}
