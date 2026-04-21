package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/data219/huectl/internal/domain"
	"github.com/data219/huectl/internal/hue/common"
	v1client "github.com/data219/huectl/internal/hue/v1"
	v2client "github.com/data219/huectl/internal/hue/v2"
)

func TestListForBridgeFallbacksToV1WhenV2ReturnsNotFound(t *testing.T) {
	var v2Calls atomic.Int32
	var v1Calls atomic.Int32

	v2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v2Calls.Add(1)
		if r.URL.Path == "/clip/v2/resource/light" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.NotFound(w, r)
	}))
	defer v2.Close()

	v1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v1Calls.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/api/test-user/lights" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"1":{"name":"Kitchen"}}`))
	}))
	defer v1.Close()

	service := newContractTestService(t)
	bridge := domain.Bridge{
		ID:        "bridge-1",
		Name:      "Bridge One",
		Username:  "test-user",
		APIBaseV2: v2.URL + "/clip/v2",
		APIBaseV1: v1.URL + "/api/test-user",
	}

	items, status, err := service.listForBridge(context.Background(), bridge, resourceSpecs()["light"])
	if err != nil {
		t.Fatalf("listForBridge error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("unexpected status: %d", status)
	}
	if v2Calls.Load() == 0 {
		t.Fatal("expected v2 to be called")
	}
	if v1Calls.Load() == 0 {
		t.Fatal("expected v1 fallback to be called")
	}
	if len(items) != 1 {
		t.Fatalf("unexpected item count: %d", len(items))
	}
	if got := toString(items[0]["id"]); got != "1" {
		t.Fatalf("unexpected id: %q", got)
	}
	if got := toString(items[0]["name"]); got != "Kitchen" {
		t.Fatalf("unexpected name: %q", got)
	}
}

func TestWriteResourceActionDoesNotFallbackWhenV2WriteSucceeds(t *testing.T) {
	var v1Calls atomic.Int32

	v2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/light":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"light-1","metadata":{"name":"Kitchen"}}]}`))
			return
		case r.Method == http.MethodPut && r.URL.Path == "/clip/v2/resource/light/light-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"rid":"light-1"}]}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer v2.Close()

	v1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v1Calls.Add(1)
		http.NotFound(w, r)
	}))
	defer v1.Close()

	service := newContractTestService(t)
	bridge := domain.Bridge{
		ID:        "bridge-1",
		Name:      "Bridge One",
		Username:  "test-user",
		APIBaseV2: v2.URL + "/clip/v2",
		APIBaseV1: v1.URL + "/api/test-user",
	}

	result, partial, err := service.writeResourceAction(
		context.Background(),
		domain.CommandContext{},
		[]domain.Bridge{bridge},
		"light",
		"on",
		resourceSpecs()["light"],
		ActionInput{ID: "light-1"},
	)
	if err != nil {
		t.Fatalf("writeResourceAction error: %v", err)
	}
	if partial {
		t.Fatal("did not expect partial success")
	}
	if v1Calls.Load() != 0 {
		t.Fatalf("v1 fallback should not be used on v2 success (calls=%d)", v1Calls.Load())
	}

	aggregate, ok := result.(domain.AggregateResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	if len(aggregate.Items) != 1 {
		t.Fatalf("unexpected item count: %d", len(aggregate.Items))
	}
	source := toString(aggregate.Items[0].Data["source"])
	if source != "v2" {
		t.Fatalf("expected v2 source, got %q", source)
	}
}

func TestWriteResourceActionUsesActionPayloadForV1Fallback(t *testing.T) {
	var v1Payload map[string]any

	v2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/light":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"light-1","metadata":{"name":"Kitchen"}}]}`))
			return
		case r.Method == http.MethodPut && r.URL.Path == "/clip/v2/resource/light/light-1":
			http.Error(w, "not found", http.StatusNotFound)
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer v2.Close()

	v1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/test-user/lights/light-1" {
			http.NotFound(w, r)
			return
		}
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&v1Payload); err != nil {
			t.Fatalf("decode v1 payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"success":true}]`))
	}))
	defer v1.Close()

	service := newContractTestService(t)
	bridge := domain.Bridge{
		ID:        "bridge-1",
		Name:      "Bridge One",
		Username:  "test-user",
		APIBaseV2: v2.URL + "/clip/v2",
		APIBaseV1: v1.URL + "/api/test-user",
	}

	result, partial, err := service.writeResourceAction(
		context.Background(),
		domain.CommandContext{},
		[]domain.Bridge{bridge},
		"light",
		"on",
		resourceSpecs()["light"],
		ActionInput{ID: "light-1"},
	)
	if err != nil {
		t.Fatalf("writeResourceAction error: %v", err)
	}
	if partial {
		t.Fatal("did not expect partial success")
	}
	if v1Payload == nil {
		t.Fatal("expected v1 fallback payload to be sent")
	}
	if got, ok := v1Payload["on"].(bool); !ok || !got {
		t.Fatalf("expected v1 payload to contain on=true, got %#v", v1Payload["on"])
	}

	aggregate, ok := result.(domain.AggregateResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	if len(aggregate.Items) != 1 {
		t.Fatalf("unexpected item count: %d", len(aggregate.Items))
	}
	source := toString(aggregate.Items[0].Data["source"])
	if source != "v1" {
		t.Fatalf("expected v1 source, got %q", source)
	}
}

func TestWriteResourceActionUsesV1ResourceIDForFallbackWrites(t *testing.T) {
	var v1Path string

	v2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/light":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"v2-light-1","id_v1":"/lights/7","metadata":{"name":"Kitchen"}}]}`))
			return
		case r.Method == http.MethodPut && r.URL.Path == "/clip/v2/resource/light/v2-light-1":
			http.Error(w, "not found", http.StatusNotFound)
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer v2.Close()

	v1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v1Path = r.URL.Path
		if r.Method != http.MethodPut || r.URL.Path != "/api/test-user/lights/7" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"success":true}]`))
	}))
	defer v1.Close()

	service := newContractTestService(t)
	bridge := domain.Bridge{
		ID:        "bridge-1",
		Name:      "Bridge One",
		Username:  "test-user",
		APIBaseV2: v2.URL + "/clip/v2",
		APIBaseV1: v1.URL + "/api/test-user",
	}

	result, partial, err := service.writeResourceAction(
		context.Background(),
		domain.CommandContext{},
		[]domain.Bridge{bridge},
		"light",
		"on",
		resourceSpecs()["light"],
		ActionInput{ID: "7"},
	)
	if err != nil {
		t.Fatalf("writeResourceAction error: %v", err)
	}
	if partial {
		t.Fatal("did not expect partial success")
	}
	if v1Path != "/api/test-user/lights/7" {
		t.Fatalf("expected v1 fallback to use v1 id path, got %q", v1Path)
	}

	aggregate, ok := result.(domain.AggregateResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	if len(aggregate.Items) != 1 {
		t.Fatalf("unexpected item count: %d", len(aggregate.Items))
	}
	source := toString(aggregate.Items[0].Data["source"])
	if source != "v1" {
		t.Fatalf("expected v1 source, got %q", source)
	}
}

func TestWriteResourceActionUsesSensorPayloadForV1Fallback(t *testing.T) {
	var v1Payload map[string]any

	v2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/motion":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"sensor-1","metadata":{"name":"Hall Motion"}}]}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/temperature":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/light_level":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/button":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/contact":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/device_power":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		case r.Method == http.MethodPut && r.URL.Path == "/clip/v2/resource/motion/sensor-1":
			http.Error(w, "not found", http.StatusNotFound)
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer v2.Close()

	v1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/test-user/sensors/sensor-1" {
			http.NotFound(w, r)
			return
		}
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&v1Payload); err != nil {
			t.Fatalf("decode v1 payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"success":true}]`))
	}))
	defer v1.Close()

	service := newContractTestService(t)
	bridge := domain.Bridge{
		ID:        "bridge-1",
		Name:      "Bridge One",
		Username:  "test-user",
		APIBaseV2: v2.URL + "/clip/v2",
		APIBaseV1: v1.URL + "/api/test-user",
	}

	sensitivity := 2
	enabled := true
	result, partial, err := service.writeResourceAction(
		context.Background(),
		domain.CommandContext{},
		[]domain.Bridge{bridge},
		"sensor",
		"sensitivity",
		resourceSpecs()["sensor"],
		ActionInput{
			ID: "sensor-1",
			Sensor: &SensorInput{
				Sensitivity: &sensitivity,
				Enabled:     &enabled,
			},
		},
	)
	if err != nil {
		t.Fatalf("writeResourceAction error: %v", err)
	}
	if partial {
		t.Fatal("did not expect partial success")
	}
	if v1Payload == nil {
		t.Fatal("expected v1 fallback payload to be sent")
	}
	if got, ok := v1Payload["sensitivity"].(float64); !ok || int(got) != 2 {
		t.Fatalf("expected v1 payload sensitivity=2, got %#v", v1Payload["sensitivity"])
	}

	aggregate, ok := result.(domain.AggregateResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	if len(aggregate.Items) != 1 {
		t.Fatalf("unexpected item count: %d", len(aggregate.Items))
	}
	source := toString(aggregate.Items[0].Data["source"])
	if source != "v1" {
		t.Fatalf("expected v1 source, got %q", source)
	}
}

func newContractTestService(t *testing.T) *Service {
	t.Helper()
	client := common.NewHTTPClient(common.HTTPClientConfig{
		Timeout:    2 * time.Second,
		MaxRetries: 0,
	})
	return &Service{
		httpClient: client,
		v2:         v2client.NewClient(client),
		v1:         v1client.NewClient(client),
	}
}

func TestWriteResourceActionFallbackStatusAndExitCodeRemainConsistent(t *testing.T) {
	v2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/light":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"light-1","metadata":{"name":"Kitchen"}}]}`))
			return
		case r.Method == http.MethodPut && r.URL.Path == "/clip/v2/resource/light/light-1":
			http.Error(w, "not found", http.StatusNotFound)
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer v2.Close()

	v1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/test-user/lights/") {
			http.Error(w, "bridge error", http.StatusServiceUnavailable)
			return
		}
		http.NotFound(w, r)
	}))
	defer v1.Close()

	service := newContractTestService(t)
	bridge := domain.Bridge{
		ID:        "bridge-1",
		Name:      "Bridge One",
		Username:  "test-user",
		APIBaseV2: v2.URL + "/clip/v2",
		APIBaseV1: v1.URL + "/api/test-user",
	}

	result, partial, err := service.writeResourceAction(
		context.Background(),
		domain.CommandContext{},
		[]domain.Bridge{bridge},
		"light",
		"on",
		resourceSpecs()["light"],
		ActionInput{ID: "light-1"},
	)
	if err != nil {
		t.Fatalf("writeResourceAction error: %v", err)
	}
	if !partial {
		t.Fatal("expected partial=true on fallback failure")
	}

	aggregate, ok := result.(domain.AggregateResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	if len(aggregate.Items) != 1 {
		t.Fatalf("unexpected item count: %d", len(aggregate.Items))
	}
	item := aggregate.Items[0]
	if item.Success {
		t.Fatal("expected failed item")
	}
	if item.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status code: %d", item.StatusCode)
	}
	if !strings.Contains(item.Error, "BRIDGE_REQUEST") {
		t.Fatalf("expected bridge http error, got %q", item.Error)
	}
}
