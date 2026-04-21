package common

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/data219/huectl/internal/domain"
)

func TestDoWithFallbackUsesV1WhenV2NotFound(t *testing.T) {
	v2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer v2.Close()

	v1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer v1.Close()

	client := NewHTTPClient(HTTPClientConfig{
		Timeout:    2 * time.Second,
		MaxRetries: 0,
	})

	result, status, usedFallback, err := client.DoWithFallback(
		context.Background(),
		http.MethodGet,
		v2.URL,
		http.MethodGet,
		v1.URL,
		nil,
	)
	if err != nil {
		t.Fatalf("do with fallback: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("unexpected status: %d", status)
	}
	if !usedFallback {
		t.Fatal("expected fallback to be used")
	}
	if string(result) != "{\"ok\":true}" {
		t.Fatalf("unexpected payload: %s", string(result))
	}
}

func TestBridgeBaseV2PreservesAddressScheme(t *testing.T) {
	bridge := domain.Bridge{Address: "http://127.0.0.1:9080"}
	got := BridgeBaseV2(bridge)
	want := "http://127.0.0.1:9080/clip/v2"
	if got != want {
		t.Fatalf("unexpected v2 base url: got=%q want=%q", got, want)
	}
}

func TestBridgeBaseV2AvoidsDoublePrefixForHTTPSAddress(t *testing.T) {
	bridge := domain.Bridge{Address: "https://bridge.local"}
	got := BridgeBaseV2(bridge)
	want := "https://bridge.local/clip/v2"
	if got != want {
		t.Fatalf("unexpected v2 base url: got=%q want=%q", got, want)
	}
}

func TestBridgeBaseV1PreservesAddressSchemeAndUsername(t *testing.T) {
	bridge := domain.Bridge{
		Address:  "http://127.0.0.1:9080",
		Username: "dev-user",
	}
	got := BridgeBaseV1(bridge)
	want := "http://127.0.0.1:9080/api/dev-user"
	if got != want {
		t.Fatalf("unexpected v1 base url: got=%q want=%q", got, want)
	}
}
