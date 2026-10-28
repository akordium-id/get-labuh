package backup

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	stdtesting "testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/models"
)

func TestS3Client_PutAndDeleteObject(t *stdtesting.T) {
	uploadedFiles := make(map[string][]byte)

	// Mock S3 HTTP Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validasi keberadaan header SigV4
		authHeader := r.Header.Get("Authorization")
		assert.True(t, strings.HasPrefix(authHeader, "AWS4-HMAC-SHA256"))
		assert.NotEmpty(t, r.Header.Get("x-amz-date"))
		assert.NotEmpty(t, r.Header.Get("x-amz-content-sha256"))

		switch r.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			uploadedFiles[r.URL.Path] = body
			w.WriteHeader(http.StatusOK)

		case http.MethodDelete:
			delete(uploadedFiles, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	dest := &models.StorageDestination{
		Name:            "Test R2",
		Endpoint:        server.URL,
		Bucket:          "my-bucket",
		Region:          "auto",
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret-key-12345",
		UseSSL:          false,
	}

	client, err := NewS3ClientFromDestination(dest)
	require.NoError(t, err)

	ctx := context.Background()
	payload := []byte("database backup gzip stream content")

	// 1. Put Object
	err = client.PutObject(ctx, "backups/pg-test.sql.gz", bytes.NewReader(payload), int64(len(payload)), "application/gzip")
	require.NoError(t, err)
	assert.Equal(t, payload, uploadedFiles["/my-bucket/backups/pg-test.sql.gz"])

	// 2. Delete Object
	err = client.DeleteObject(ctx, "backups/pg-test.sql.gz")
	require.NoError(t, err)
	_, exists := uploadedFiles["/my-bucket/backups/pg-test.sql.gz"]
	assert.False(t, exists)
}

func TestS3Client_TestConnection(t *stdtesting.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut || r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	dest := &models.StorageDestination{
		Name:            "Test S3",
		Endpoint:        server.URL,
		Bucket:          "labuh",
		Region:          "us-east-1",
		AccessKeyID:     "akid",
		SecretAccessKey: "secret",
		UseSSL:          false,
	}

	client, err := NewS3ClientFromDestination(dest)
	require.NoError(t, err)

	err = client.TestConnection(context.Background())
	assert.NoError(t, err)
}
