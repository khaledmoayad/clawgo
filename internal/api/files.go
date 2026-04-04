package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// FilesAPIClient provides upload and download operations for the Anthropic
// Files API. Used for file attachments in conversations.
// Mirrors the TS services/api/filesApi.ts implementation.
type FilesAPIClient struct {
	baseURL     string
	oauthToken  string
	httpClient  *http.Client
	retryConfig RetryConfig
}

// NewFilesAPIClient creates a FilesAPIClient with proxy-aware transport.
func NewFilesAPIClient(baseURL, oauthToken string) *FilesAPIClient {
	return &FilesAPIClient{
		baseURL:    baseURL,
		oauthToken: oauthToken,
		httpClient: &http.Client{
			Transport: NewProxyTransport(),
		},
		retryConfig: DefaultRetryConfig(),
	}
}

// setAuthHeaders sets the standard Anthropic auth and API headers on a request.
func (c *FilesAPIClient) setAuthHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.oauthToken)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "files-api-2025-04-14,oauth-2025-04-20")
}

// Upload sends a file to the Anthropic Files API via multipart form encoding.
// Returns the file ID from the API response.
// Retries on transient errors (429, 500+).
func (c *FilesAPIClient) Upload(ctx context.Context, filename string, content []byte, contentType string) (string, error) {
	type uploadResponse struct {
		ID string `json:"id"`
	}

	result, err := WithRetry(ctx, c.retryConfig, func(ctx context.Context) (string, error) {
		// Build multipart form body
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		// Add "purpose" field
		if err := writer.WriteField("purpose", "tool_result"); err != nil {
			return "", fmt.Errorf("files api: write purpose field: %w", err)
		}

		// Add file field
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			return "", fmt.Errorf("files api: create form file: %w", err)
		}
		if _, err := part.Write(content); err != nil {
			return "", fmt.Errorf("files api: write file content: %w", err)
		}

		if err := writer.Close(); err != nil {
			return "", fmt.Errorf("files api: close multipart writer: %w", err)
		}

		url := c.baseURL + "/v1/files"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
		if err != nil {
			return "", fmt.Errorf("files api: create request: %w", err)
		}

		c.setAuthHeaders(req)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("files api upload: %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			return "", &HTTPError{
				StatusCode: resp.StatusCode,
				Body:       string(body),
			}
		}

		var respData uploadResponse
		if err := json.Unmarshal(body, &respData); err != nil {
			return "", fmt.Errorf("files api: parse response: %w", err)
		}

		return respData.ID, nil
	})

	return result, err
}

// Download retrieves file content by ID from the Anthropic Files API.
// Returns the raw file bytes.
// Retries on transient errors (429, 500+).
func (c *FilesAPIClient) Download(ctx context.Context, fileID string) ([]byte, error) {
	result, err := WithRetry(ctx, c.retryConfig, func(ctx context.Context) ([]byte, error) {
		url := fmt.Sprintf("%s/v1/files/%s/content", c.baseURL, fileID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("files api: create request: %w", err)
		}

		c.setAuthHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("files api download: %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			return nil, &HTTPError{
				StatusCode: resp.StatusCode,
				Body:       string(body),
			}
		}

		return body, nil
	})

	return result, err
}

// Delete removes a file by ID from the Anthropic Files API.
// Retries on transient errors (429, 500+).
func (c *FilesAPIClient) Delete(ctx context.Context, fileID string) error {
	_, err := WithRetry(ctx, c.retryConfig, func(ctx context.Context) (struct{}, error) {
		url := fmt.Sprintf("%s/v1/files/%s", c.baseURL, fileID)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		if err != nil {
			return struct{}{}, fmt.Errorf("files api: create request: %w", err)
		}

		c.setAuthHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return struct{}{}, fmt.Errorf("files api delete: %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			return struct{}{}, &HTTPError{
				StatusCode: resp.StatusCode,
				Body:       string(body),
			}
		}

		return struct{}{}, nil
	})

	return err
}

// HTTPError represents an HTTP error response with status code and body.
// It implements IsRetryable() by checking for transient status codes (429, 500+).
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("http %d: %s", e.StatusCode, e.Body)
}
