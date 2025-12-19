// Package connection provides connection management for tokmesh-cli.
package connection

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPClient provides HTTP communication with the server.
type HTTPClient struct {
	baseURL  string
	client   *http.Client
	apiKeyID string
	apiKey   string
}

// NewHTTPClient creates a new HTTP client.
func NewHTTPClient(server, apiKeyID, apiKey string) *HTTPClient {
	// Ensure baseURL has http:// prefix
	baseURL := server
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}

	return &HTTPClient{
		baseURL:  baseURL,
		apiKeyID: apiKeyID,
		apiKey:   apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Get performs a GET request.
func (c *HTTPClient) Get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.addHeaders(req)
	return c.client.Do(req)
}

// Post performs a POST request with JSON body.
func (c *HTTPClient) Post(ctx context.Context, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.addHeaders(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.client.Do(req)
}

// addHeaders adds authentication and common headers.
func (c *HTTPClient) addHeaders(req *http.Request) {
	if c.apiKeyID != "" && c.apiKey != "" {
		req.Header.Set("X-API-Key-ID", c.apiKeyID)
		req.Header.Set("X-API-Key", c.apiKey)
	}
	req.Header.Set("User-Agent", "tokmesh-cli/1.0")
}

// BaseURL returns the base URL of the client.
func (c *HTTPClient) BaseURL() string {
	return c.baseURL
}

// ParseResponse parses a JSON response body into the target struct.
func ParseResponse(resp *http.Response, target any) error {
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// Try to parse error response
		var errResp struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp.Message != "" {
			return fmt.Errorf("[%s] %s", errResp.Code, errResp.Message)
		}
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
	}

	return nil
}

// NOTE: TokMesh 对外 HTTP API 当前仅使用 GET/POST；如未来引入其他方法，需先更新 specs/ 并同步调整该客户端。
