package v2

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

type Response struct {
	Data   []map[string]any `json:"data"`
	Errors []map[string]any `json:"errors"`
}

func NewClient(httpClient *common.HTTPClient) *Client {
	return &Client{httpClient: httpClient}
}

func (c *Client) List(ctx context.Context, bridge domain.Bridge, resourceType string) ([]map[string]any, int, *domain.AppError) {
	url := fmt.Sprintf("%s/resource/%s", common.BridgeBaseV2(bridge), strings.Trim(resourceType, "/"))
	payload, status, err := c.httpClient.Do(
		ctx,
		http.MethodGet,
		url,
		nil,
		map[string]string{"hue-application-key": bridge.Username},
	)
	if err != nil {
		return nil, status, domain.WrapError("BRIDGE_REQUEST", "v2 list request failed", domain.ExitNetwork, err)
	}
	if status >= 400 {
		return nil, status, &domain.AppError{
			Code:     "BRIDGE_HTTP",
			Message:  "v2 list request returned error status",
			ExitCode: exitCodeFromHTTP(status),
			Details: map[string]any{
				"status": status,
				"body":   string(payload),
				"url":    url,
			},
		}
	}
	resp := Response{}
	if unmarshalErr := json.Unmarshal(payload, &resp); unmarshalErr != nil {
		return nil, status, domain.WrapError("BRIDGE_PARSE", "failed to parse v2 response", domain.ExitInternal, unmarshalErr)
	}
	return resp.Data, status, nil
}

func (c *Client) Get(ctx context.Context, bridge domain.Bridge, resourceType string, resourceID string) (map[string]any, int, *domain.AppError) {
	url := fmt.Sprintf("%s/resource/%s/%s", common.BridgeBaseV2(bridge), strings.Trim(resourceType, "/"), strings.Trim(resourceID, "/"))
	payload, status, err := c.httpClient.Do(
		ctx,
		http.MethodGet,
		url,
		nil,
		map[string]string{"hue-application-key": bridge.Username},
	)
	if err != nil {
		return nil, status, domain.WrapError("BRIDGE_REQUEST", "v2 get request failed", domain.ExitNetwork, err)
	}
	if status >= 400 {
		return nil, status, &domain.AppError{
			Code:     "BRIDGE_HTTP",
			Message:  "v2 get request returned error status",
			ExitCode: exitCodeFromHTTP(status),
			Details: map[string]any{
				"status": status,
				"body":   string(payload),
				"url":    url,
			},
		}
	}
	resp := Response{}
	if unmarshalErr := json.Unmarshal(payload, &resp); unmarshalErr != nil {
		return nil, status, domain.WrapError("BRIDGE_PARSE", "failed to parse v2 response", domain.ExitInternal, unmarshalErr)
	}
	if len(resp.Data) == 0 {
		return nil, status, domain.NewError("TARGET_NOT_FOUND", "resource not found", domain.ExitTarget)
	}
	return resp.Data[0], status, nil
}

func (c *Client) Write(
	ctx context.Context,
	bridge domain.Bridge,
	method string,
	resourceType string,
	resourceID string,
	payloadBody map[string]any,
) (map[string]any, int, *domain.AppError) {
	url := fmt.Sprintf("%s/resource/%s/%s", common.BridgeBaseV2(bridge), strings.Trim(resourceType, "/"), strings.Trim(resourceID, "/"))
	encoded, marshalErr := json.Marshal(payloadBody)
	if marshalErr != nil {
		return nil, 0, domain.WrapError("PAYLOAD_SERIALIZE", "failed to serialize payload", domain.ExitUsage, marshalErr)
	}

	payload, status, err := c.httpClient.Do(
		ctx,
		method,
		url,
		encoded,
		map[string]string{
			"hue-application-key": bridge.Username,
			"Content-Type":        "application/json",
		},
	)
	if err != nil {
		return nil, status, domain.WrapError("BRIDGE_REQUEST", "v2 write request failed", domain.ExitNetwork, err)
	}
	if status >= 400 {
		return nil, status, &domain.AppError{
			Code:     "BRIDGE_HTTP",
			Message:  "v2 write request returned error status",
			ExitCode: exitCodeFromHTTP(status),
			Details: map[string]any{
				"status": status,
				"body":   string(payload),
				"url":    url,
			},
		}
	}

	resp := Response{}
	if unmarshalErr := json.Unmarshal(payload, &resp); unmarshalErr != nil {
		return nil, status, domain.WrapError("BRIDGE_PARSE", "failed to parse v2 response", domain.ExitInternal, unmarshalErr)
	}
	if len(resp.Data) > 0 {
		return resp.Data[0], status, nil
	}
	return map[string]any{"status": "ok"}, status, nil
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
	url := common.BridgeBaseV2(bridge) + trimmed
	payload, status, err := c.httpClient.Do(ctx, method, url, body, map[string]string{"hue-application-key": bridge.Username})
	if err != nil {
		return nil, status, domain.WrapError("BRIDGE_REQUEST", "raw v2 request failed", domain.ExitNetwork, err)
	}
	if status >= 400 {
		return payload, status, &domain.AppError{
			Code:     "BRIDGE_HTTP",
			Message:  "raw v2 request returned error status",
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
