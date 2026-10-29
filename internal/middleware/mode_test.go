package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestModeMiddleware(t *testing.T) {
	tests := []struct {
		name         string
		cookieValue  string
		hasCookie    bool
		expectedMode string
		isLite       bool
	}{
		{
			name:         "Default without cookie is Lite",
			hasCookie:    false,
			expectedMode: ModeLite,
			isLite:       true,
		},
		{
			name:         "Cookie pro activates Pro mode",
			hasCookie:    true,
			cookieValue:  ModePro,
			expectedMode: ModePro,
			isLite:       false,
		},
		{
			name:         "Cookie lite activates Lite mode",
			hasCookie:    true,
			cookieValue:  ModeLite,
			expectedMode: ModeLite,
			isLite:       true,
		},
		{
			name:         "Invalid cookie value falls back to Lite mode",
			hasCookie:    true,
			cookieValue:  "unknown",
			expectedMode: ModeLite,
			isLite:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := ModeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mode := GetUserMode(r.Context())
				if mode != tt.expectedMode {
					t.Errorf("expected mode %s, got %s", tt.expectedMode, mode)
				}
				if IsLiteMode(r.Context()) != tt.isLite {
					t.Errorf("expected IsLiteMode %v, got %v", tt.isLite, IsLiteMode(r.Context()))
				}
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.hasCookie {
				req.AddCookie(&http.Cookie{
					Name:  CookieModeName,
					Value: tt.cookieValue,
				})
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		})
	}
}
