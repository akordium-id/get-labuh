package middleware

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"strings"
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

		// Rute HTML dinamis jangan diberi ETag statis berbasis URL path saja,
		// agar perubahan state session/cookie langsung merender konten baru.
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
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

var CacheMiddleware = CacheHeadersMiddleware
