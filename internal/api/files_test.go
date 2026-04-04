package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesAPIUpload(t *testing.T) {
	t.Run("sends multipart upload and returns file ID", func(t *testing.T) {
		var receivedContentType string
		var receivedBody []byte

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedContentType = r.Header.Get("Content-Type")
			receivedBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"id": "file-abc123"})
		}))
		defer server.Close()

		client := NewFilesAPIClient(server.URL, "test-oauth-token")
		fileID, err := client.Upload(context.Background(), "test.txt", []byte("hello world"), "text/plain")

		require.NoError(t, err)
		assert.Equal(t, "file-abc123", fileID)

		// Verify multipart encoding
		assert.True(t, strings.HasPrefix(receivedContentType, "multipart/form-data"))
		assert.Contains(t, string(receivedBody), "hello world")
		assert.Contains(t, string(receivedBody), "test.txt")
	})

	t.Run("sets correct auth and API headers", func(t *testing.T) {
		var receivedHeaders http.Header

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedHeaders = r.Header
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"id": "file-xyz"})
		}))
		defer server.Close()

		client := NewFilesAPIClient(server.URL, "my-bearer-token")
		_, err := client.Upload(context.Background(), "doc.pdf", []byte("pdf-data"), "application/pdf")

		require.NoError(t, err)
		assert.Equal(t, "Bearer my-bearer-token", receivedHeaders.Get("Authorization"))
		assert.Equal(t, "2023-06-01", receivedHeaders.Get("anthropic-version"))
		assert.Equal(t, "files-api-2025-04-14,oauth-2025-04-20", receivedHeaders.Get("anthropic-beta"))
	})

	t.Run("retries on 429 then succeeds", func(t *testing.T) {
		var callCount atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := callCount.Add(1)
			if count == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error": "rate limited"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"id": "file-retry-ok"})
		}))
		defer server.Close()

		client := NewFilesAPIClient(server.URL, "token")
		// Use fast retry config for testing
		client.retryConfig = RetryConfig{
			MaxRetries:    3,
			InitialDelay:  1,
			MaxDelay:      10,
			BackoffFactor: 1.0,
			Jitter:        false,
		}
		fileID, err := client.Upload(context.Background(), "f.txt", []byte("data"), "text/plain")

		require.NoError(t, err)
		assert.Equal(t, "file-retry-ok", fileID)
		assert.Equal(t, int32(2), callCount.Load())
	})

	t.Run("retries on 500+ status codes", func(t *testing.T) {
		var callCount atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := callCount.Add(1)
			if count <= 2 {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error": "server error"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"id": "file-500-ok"})
		}))
		defer server.Close()

		client := NewFilesAPIClient(server.URL, "token")
		client.retryConfig = RetryConfig{
			MaxRetries:    3,
			InitialDelay:  1,
			MaxDelay:      10,
			BackoffFactor: 1.0,
			Jitter:        false,
		}
		fileID, err := client.Upload(context.Background(), "f.txt", []byte("data"), "text/plain")

		require.NoError(t, err)
		assert.Equal(t, "file-500-ok", fileID)
		assert.Equal(t, int32(3), callCount.Load())
	})
}

func TestFilesAPIDownload(t *testing.T) {
	t.Run("downloads file content by ID", func(t *testing.T) {
		fileContent := []byte("downloaded file content here")

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/files/file-abc123/content", r.URL.Path)
			assert.Equal(t, "GET", r.Method)
			w.WriteHeader(http.StatusOK)
			w.Write(fileContent)
		}))
		defer server.Close()

		client := NewFilesAPIClient(server.URL, "token")
		data, err := client.Download(context.Background(), "file-abc123")

		require.NoError(t, err)
		assert.Equal(t, fileContent, data)
	})

	t.Run("sets correct auth headers on download", func(t *testing.T) {
		var receivedHeaders http.Header

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedHeaders = r.Header
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("content"))
		}))
		defer server.Close()

		client := NewFilesAPIClient(server.URL, "download-token")
		_, err := client.Download(context.Background(), "file-xyz")

		require.NoError(t, err)
		assert.Equal(t, "Bearer download-token", receivedHeaders.Get("Authorization"))
		assert.Equal(t, "2023-06-01", receivedHeaders.Get("anthropic-version"))
		assert.Equal(t, "files-api-2025-04-14,oauth-2025-04-20", receivedHeaders.Get("anthropic-beta"))
	})

	t.Run("returns error on 404", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "file not found"}`))
		}))
		defer server.Close()

		client := NewFilesAPIClient(server.URL, "token")
		data, err := client.Download(context.Background(), "file-nonexistent")

		assert.Error(t, err)
		assert.Nil(t, data)
		assert.Contains(t, err.Error(), "404")
	})
}

func TestFilesAPIDelete(t *testing.T) {
	t.Run("deletes file by ID", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/files/file-del123", r.URL.Path)
			assert.Equal(t, "DELETE", r.Method)
			assert.Equal(t, "Bearer del-token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewFilesAPIClient(server.URL, "del-token")
		err := client.Delete(context.Background(), "file-del123")

		require.NoError(t, err)
	})

	t.Run("returns error on non-200 status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()

		client := NewFilesAPIClient(server.URL, "token")
		err := client.Delete(context.Background(), "file-nope")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "403")
	})
}
