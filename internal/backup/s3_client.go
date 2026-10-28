package backup

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
)

type S3Client struct {
	endpoint        string
	bucket          string
	region          string
	accessKeyID     string
	secretAccessKey string
	useSSL          bool
	httpClient      *http.Client
}

func NewS3ClientFromDestination(dest *models.StorageDestination) (*S3Client, error) {
	if dest == nil {
		return nil, fmt.Errorf("storage destination is nil")
	}

	endpoint := strings.TrimSpace(dest.Endpoint)
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		if dest.UseSSL {
			endpoint = "https://" + endpoint
		} else {
			endpoint = "http://" + endpoint
		}
	}

	region := dest.Region
	if region == "" {
		region = "us-east-1"
	}

	return &S3Client{
		endpoint:        endpoint,
		bucket:          dest.Bucket,
		region:          region,
		accessKeyID:     dest.AccessKeyID,
		secretAccessKey: dest.SecretAccessKey,
		useSSL:          dest.UseSSL,
		httpClient:      &http.Client{Timeout: 30 * time.Minute},
	}, nil
}

func (s *S3Client) buildURL(key string) (string, error) {
	u, err := url.Parse(s.endpoint)
	if err != nil {
		return "", err
	}

	key = strings.TrimPrefix(key, "/")
	// Path-style: endpoint/bucket/key
	// S3 Compatible (MinIO, R2, AWS standard)
	cleanPath := strings.TrimSuffix(u.Path, "/")
	u.Path = fmt.Sprintf("%s/%s/%s", cleanPath, s.bucket, key)
	return u.String(), nil
}

func (s *S3Client) PutObject(ctx context.Context, key string, data io.Reader, size int64, contentType string) error {
	reqURL, err := s.buildURL(key)
	if err != nil {
		return fmt.Errorf("invalid S3 URL: %w", err)
	}

	if contentType == "" {
		contentType = "application/gzip"
	}

	// Payload hashing untuk SigV4
	var bodyBytes []byte
	if data != nil {
		bodyBytes, err = io.ReadAll(data)
		if err != nil {
			return fmt.Errorf("failed to read object payload: %w", err)
		}
	}
	payloadHash := sha256Hex(bodyBytes)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(bodyBytes)))
	s.signRequest(req, now, payloadHash)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("S3 PutObject request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("S3 PutObject failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (s *S3Client) DeleteObject(ctx context.Context, key string) error {
	reqURL, err := s.buildURL(key)
	if err != nil {
		return err
	}

	payloadHash := sha256Hex(nil)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	s.signRequest(req, now, payloadHash)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("S3 DeleteObject request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("S3 DeleteObject failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (s *S3Client) TestConnection(ctx context.Context) error {
	testKey := fmt.Sprintf(".labuh-test-%d.tmp", time.Now().Unix())
	content := []byte("labuh-connection-test")

	// 1. Put
	if err := s.PutObject(ctx, testKey, bytes.NewReader(content), int64(len(content)), "text/plain"); err != nil {
		return fmt.Errorf("upload test failed: %w", err)
	}

	// 2. Cleanup Delete
	_ = s.DeleteObject(ctx, testKey)
	return nil
}

// AWS Signature Version 4 implementation
func (s *S3Client) signRequest(req *http.Request, t time.Time, payloadHash string) {
	dateStamp := t.Format("20060102")
	amzDate := t.Format("20060102T150405Z")

	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("Host", req.URL.Host)

	// Canonical headers
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		req.URL.Host,
		payloadHash,
		amzDate,
	)

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		req.URL.EscapedPath(),
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	)

	algorithm := "AWS4-HMAC-SHA256"
	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, s.region)
	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s",
		algorithm,
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	)

	signingKey := getSignatureKey(s.secretAccessKey, dateStamp, s.region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm,
		s.accessKeyID,
		credentialScope,
		signedHeaders,
		signature,
	)

	req.Header.Set("Authorization", authHeader)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func getSignatureKey(key, dateStamp, regionName, serviceName string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+key), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(regionName))
	kService := hmacSHA256(kRegion, []byte(serviceName))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}
