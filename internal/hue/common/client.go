package common

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"time"
)

type HTTPClientConfig struct {
	Timeout    time.Duration
	MaxRetries int
	BaseDelay  time.Duration
}

type HTTPClient struct {
	httpClient *http.Client
	maxRetries int
	baseDelay  time.Duration
}

func NewHTTPClient(cfg HTTPClientConfig) *HTTPClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	baseDelay := cfg.BaseDelay
	if baseDelay <= 0 {
		baseDelay = 250 * time.Millisecond
	}
	maxRetries := cfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}

	return &HTTPClient{
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				DialContext:         (&net.Dialer{Timeout: timeout}).DialContext,
				TLSHandshakeTimeout: timeout,
				TLSClientConfig: &tls.Config{
					MinVersion:         tls.VersionTLS12,
					InsecureSkipVerify: true,
				},
			},
		},
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
	}
}

func (c *HTTPClient) Do(
	ctx context.Context,
	method string,
	url string,
	body []byte,
	headers map[string]string,
) ([]byte, int, error) {
	attempts := c.maxRetries + 1
	var lastErr error

	for i := range attempts {
		payload, status, err := c.doOnce(ctx, method, url, body, headers)
		if err == nil && status < 429 {
			return payload, status, nil
		}
		if err == nil && status >= 500 {
			lastErr = fmt.Errorf("server returned %d", status)
		} else if err == nil && status == 429 {
			lastErr = fmt.Errorf("rate limited with status %d", status)
		} else {
			lastErr = err
		}

		if i == attempts-1 {
			return payload, status, lastErr
		}

		backoff := time.Duration(math.Pow(2, float64(i))) * c.baseDelay
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-time.After(backoff):
		}
	}

	return nil, 0, lastErr
}

func (c *HTTPClient) DoWithFallback(
	ctx context.Context,
	primaryMethod string,
	primaryURL string,
	fallbackMethod string,
	fallbackURL string,
	body []byte,
) ([]byte, int, bool, error) {
	payload, status, err := c.Do(ctx, primaryMethod, primaryURL, body, nil)
	if err == nil && status != http.StatusNotFound {
		return payload, status, false, nil
	}

	fallbackPayload, fallbackStatus, fallbackErr := c.Do(ctx, fallbackMethod, fallbackURL, body, nil)
	if fallbackErr != nil {
		if err != nil {
			return nil, 0, false, fmt.Errorf("primary error: %v; fallback error: %w", err, fallbackErr)
		}
		return nil, 0, false, fallbackErr
	}
	return fallbackPayload, fallbackStatus, true, nil
}

func (c *HTTPClient) doOnce(
	ctx context.Context,
	method string,
	url string,
	body []byte,
	headers map[string]string,
) ([]byte, int, error) {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	return payload, resp.StatusCode, nil
}
