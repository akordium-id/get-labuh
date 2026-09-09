package middleware

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
	"time"
)

func CacheHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isStaticAsset(r.URL.Path) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			next.ServeHTTP(w, r)
			return
		}

		if isAPIListEndpoint(r.URL.Path, r.Method) {
			w.Header().Set("Cache-Control", "private, max-age=60")
			next.ServeHTTP(w, r)
			return
		}

		etag := generateETag(r.URL.Path)
		w.Header().Set("ETag", etag)

		if match := r.Header.Get("If-None-Match"); match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isStaticAsset(path string) bool {
	ext := strings.ToLower(path[strings.LastIndex(path, ".")+1:])
	switch ext {
	case "css", "js", "png", "jpg", "jpeg", "gif", "svg", "ico", "woff", "woff2", "ttf", "eot":
		return true
	}
	return false
}

func isAPIListEndpoint(path, method string) bool {
	if method != http.MethodGet {
		return false
	}
	return strings.Contains(path, "/api/")
}

func generateETag(input string) string {
	hasher := md5.New()
	hasher.Write([]byte(input))
	return hex.EncodeToString(hasher.Sum(nil))
}

func getCacheEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func cacheKeyFromRequest(r *http.Request) string {
	return r.Method + ":" + r.URL.Path
}

func parseCacheDuration(durationStr string) (time.Duration, error) {
	return time.ParseDuration(durationStr)
}

func matchesCacheControl(cacheControl string, directive string) bool {
	parts := strings.Split(cacheControl, ",")
	for _, part := range parts {
		if strings.TrimSpace(strings.ToLower(part)) == directive {
			return true
		}
	}
	return false
}

var CacheMiddleware = CacheHeadersMiddleware
