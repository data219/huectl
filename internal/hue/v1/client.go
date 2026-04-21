package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/data219/huectl/internal/domain"
	"github.com/data219/huectl/internal/hue/common"
)

type Client struct {
	httpClient *common.HTTPClient
}

func NewClient(httpClient *common.HTTPClient) *Client {
	return &Client{httpClient: httpClient}
}

func (c *Client) Link(ctx context.Context, bridge domain.Bridge, deviceType string) (string, string, *domain.AppError) {
	url := common.BridgeBaseV1(domain.Bridge{Address: bridge.Address})
	if !strings.HasSuffix(url, "/api") {
		url = fmt.Sprintf("https://%s/api", strings.TrimSuffix(bridge.Address, "/"))
	}
	payload, marshalErr := json.Marshal(map[string]any{
		"devicetype":        deviceType,
		"generateclientkey": true,
	})
	if marshalErr != nil {
		return "", "", domain.WrapError("PAYLOAD_SERIALIZE", "failed to serialize link payload", domain.ExitInternal, marshalErr)
	}

	body, status, err := c.httpClient.Do(ctx, http.MethodPost, url, payload, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return "", "", domain.WrapError("BRIDGE_REQUEST", "v1 link request failed", domain.ExitNetwork, err)
	}
	if status >= 400 {
		return "", "", &domain.AppError{
			Code:     "BRIDGE_HTTP",
			Message:  "v1 link request returned error status",
			ExitCode: domain.ExitAuth,
			Details:  map[string]any{"status": status, "body": string(body), "url": url},
		}
	}

	var result []map[string]any
	if unmarshalErr := json.Unmarshal(body, &result); unmarshalErr != nil {
		return "", "", domain.WrapError("BRIDGE_PARSE", "failed to parse v1 link response", domain.ExitInternal, unmarshalErr)
	}

	for _, item := range result {
		if successRaw, ok := item["success"].(map[string]any); ok {
			username := stringValue(successRaw["username"])
			if username != "" {
				return username, stringValue(successRaw["clientkey"]), nil
			}
		}
		if failureRaw, ok := item["error"].(map[string]any); ok {
			description := stringValue(failureRaw["description"])
			if description == "" {
				description = "link failed"
			}
			return "", "", &domain.AppError{
				Code:     "LINK_FAILED",
				Message:  description,
				ExitCode: domain.ExitAuth,
				Details:  map[string]any{"type": failureRaw["type"]},
				Hints:    []string{"Press the physical link button on the bridge", "Retry within 30 seconds"},
			}
		}
	}

	return "", "", domain.NewError("LINK_FAILED", "unexpected v1 link response", domain.ExitAuth)
}

func (c *Client) List(ctx context.Context, bridge domain.Bridge, resource string) (map[string]any, int, *domain.AppError) {
	url := fmt.Sprintf("%s/%s", common.BridgeBaseV1(bridge), strings.Trim(resource, "/"))
	payload, status, err := c.httpClient.Do(ctx, http.MethodGet, url, nil, nil)
	if err != nil {
		return nil, status, domain.WrapError("BRIDGE_REQUEST", "v1 list request failed", domain.ExitNetwork, err)
	}
	if status >= 400 {
		return nil, status, &domain.AppError{
			Code:     "BRIDGE_HTTP",
			Message:  "v1 list request returned error status",
			ExitCode: exitCodeFromHTTP(status),
			Details:  map[string]any{"status": status, "body": string(payload), "url": url},
		}
	}
	result := map[string]any{}
	if unmarshalErr := json.Unmarshal(payload, &result); unmarshalErr != nil {
		return nil, status, domain.WrapError("BRIDGE_PARSE", "failed to parse v1 response", domain.ExitInternal, unmarshalErr)
	}
	return result, status, nil
}

func (c *Client) Get(ctx context.Context, bridge domain.Bridge, resource string, id string) (map[string]any, int, *domain.AppError) {
	url := fmt.Sprintf("%s/%s/%s", common.BridgeBaseV1(bridge), strings.Trim(resource, "/"), strings.Trim(id, "/"))
	payload, status, err := c.httpClient.Do(ctx, http.MethodGet, url, nil, nil)
	if err != nil {
		return nil, status, domain.WrapError("BRIDGE_REQUEST", "v1 get request failed", domain.ExitNetwork, err)
	}
	if status >= 400 {
		return nil, status, &domain.AppError{
			Code:     "BRIDGE_HTTP",
			Message:  "v1 get request returned error status",
			ExitCode: exitCodeFromHTTP(status),
			Details:  map[string]any{"status": status, "body": string(payload), "url": url},
		}
	}
	result := map[string]any{}
	if unmarshalErr := json.Unmarshal(payload, &result); unmarshalErr != nil {
		return nil, status, domain.WrapError("BRIDGE_PARSE", "failed to parse v1 response", domain.ExitInternal, unmarshalErr)
	}
	return result, status, nil
}

func (c *Client) Write(
	ctx context.Context,
	bridge domain.Bridge,
	method string,
	resource string,
	id string,
	payloadMap map[string]any,
) (map[string]any, int, *domain.AppError) {
	url := fmt.Sprintf("%s/%s/%s", common.BridgeBaseV1(bridge), strings.Trim(resource, "/"), strings.Trim(id, "/"))
	payload, marshalErr := json.Marshal(payloadMap)
	if marshalErr != nil {
		return nil, 0, domain.WrapError("PAYLOAD_SERIALIZE", "failed to serialize payload", domain.ExitUsage, marshalErr)
	}
	body, status, err := c.httpClient.Do(ctx, method, url, payload, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return nil, status, domain.WrapError("BRIDGE_REQUEST", "v1 write request failed", domain.ExitNetwork, err)
	}
	if status >= 400 {
		return nil, status, &domain.AppError{
			Code:     "BRIDGE_HTTP",
			Message:  "v1 write request returned error status",
			ExitCode: exitCodeFromHTTP(status),
			Details:  map[string]any{"status": status, "body": string(body), "url": url},
		}
	}
	var result any
	if unmarshalErr := json.Unmarshal(body, &result); unmarshalErr != nil {
		return nil, status, domain.WrapError("BRIDGE_PARSE", "failed to parse v1 response", domain.ExitInternal, unmarshalErr)
	}
	return map[string]any{"response": result}, status, nil
}

func (c *Client) Raw(
	ctx context.Context,
	bridge domain.Bridge,
	method string,
	path string,
	body []byte,
) ([]byte, int, *domain.AppError) {
	trimmed := strings.TrimSpace(path)
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	url := common.BridgeBaseV1(bridge) + trimmed
	payload, status, err := c.httpClient.Do(ctx, method, url, body, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return nil, status, domain.WrapError("BRIDGE_REQUEST", "raw v1 request failed", domain.ExitNetwork, err)
	}
	if status >= 400 {
		return payload, status, &domain.AppError{
			Code:     "BRIDGE_HTTP",
			Message:  "raw v1 request returned error status",
			ExitCode: exitCodeFromHTTP(status),
			Details:  map[string]any{"status": status, "body": string(payload), "url": url},
		}
	}
	return payload, status, nil
}

func exitCodeFromHTTP(status int) int {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return domain.ExitAuth
	case status == http.StatusNotFound || status == http.StatusConflict:
		return domain.ExitTarget
	case status == http.StatusTooManyRequests:
		return domain.ExitRetry
	case status >= 500:
		return domain.ExitNetwork
	default:
		return domain.ExitUsage
	}
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return fmt.Sprintf("%v", value)
}
