package v1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/data219/huectl/internal/domain"
	"github.com/data219/huectl/internal/hue/common"
)

func TestLinkParsesNumericErrorTypePayload(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"error":{"type":101,"address":"/api/","description":"link button not pressed"}}]`))
	}))
	defer server.Close()

	httpClient := common.NewHTTPClient(common.HTTPClientConfig{
		Timeout:    2 * time.Second,
		MaxRetries: 0,
	})
	client := NewClient(httpClient)

	address := strings.TrimPrefix(server.URL, "https://")
	_, _, appErr := client.Link(context.Background(), domain.Bridge{Address: address}, "huectl#test")
	if appErr == nil {
		t.Fatal("expected link error")
	}
	if appErr.Code != "LINK_FAILED" {
		t.Fatalf("unexpected error code: got=%q want=%q", appErr.Code, "LINK_FAILED")
	}
	if appErr.ExitCode != domain.ExitAuth {
		t.Fatalf("unexpected exit code: got=%d want=%d", appErr.ExitCode, domain.ExitAuth)
	}
}
