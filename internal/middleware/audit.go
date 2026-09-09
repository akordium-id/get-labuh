package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/akordium-id/get-labuh/internal/auth"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
)

const csrfTokenCookieName = "csrf_token"
const csrfTokenHeaderName = "X-CSRF-Token"
const csrfTokenLifetime = 1 * time.Hour

func CSRFProtectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			token := generateCSRFToken()
			setCSRFCookie(w, token)
			w.Header().Set("X-CSRF-Token", token)
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(csrfTokenCookieName)
		if err != nil {
			http.Error(w, "CSRF token missing", http.StatusForbidden)
			return
		}

		headerToken := r.Header.Get(csrfTokenHeaderName)
		if headerToken == "" {
			headerToken = r.FormValue(csrfTokenHeaderName)
		}

		if !validateCSRFToken(cookie.Value, headerToken) {
			http.Error(w, "CSRF token invalid", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isSafeMethod(method string) bool {
	switch method {
	case "GET", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}

func generateCSRFToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return base64.StdEncoding.EncodeToString(bytes)
}

func setCSRFCookie(w http.ResponseWriter, token string) {
	cookie := &http.Cookie{
		Name:     csrfTokenCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(csrfTokenLifetime.Seconds()),
	}
	http.SetCookie(w, cookie)
}

func validateCSRFToken(cookieToken, headerToken string) bool {
	return subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) == 1
}

func GetCSRFToken(r *http.Request) string {
	cookie, err := r.Cookie(csrfTokenCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func ExtractUserAgent(r *http.Request) string {
	return r.Header.Get("User-Agent")
}

func AuditLoggingMiddleware(auditRepo *repo.AuditLogRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isMutationMethod(r.Method) && !isExcludedPath(r.URL.Path) {
				defer func() {
					resource, resourceID := parseResource(r.URL.Path)
					LogAudit(auditRepo, r, r.Method+"_"+strings.ToLower(r.Method), resource, resourceID)
				}()
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isMutationMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isExcludedPath(path string) bool {
	excluded := []string{
		"/health",
		"/metrics",
		"/auth/login",
		"/auth/logout",
	}

	for _, ex := range excluded {
		if path == ex || strings.HasPrefix(path, ex) {
			return true
		}
	}

	return false
}

func parseResource(path string) (string, string) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) == 0 {
		return "unknown", ""
	}

	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1], parts[len(parts)-1]
	}

	return parts[0], ""
}

func LogAudit(auditRepo *repo.AuditLogRepo, r *http.Request, action, resource, resourceID string) {
	if auditRepo == nil {
		return
	}

	var userID *string
	user, ok := auth.UserFromContext(r.Context())
	if ok && user != nil {
		userID = &user.ID
	}

	_, _ = auditRepo.Create(models.CreateAuditLogInput{
		UserID:     userID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		IPAddress:  auth.ExtractIP(r),
		UserAgent:  ExtractUserAgent(r),
	})
}
