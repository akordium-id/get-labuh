package handler

import (
	"encoding/json"
	"net/http"
)

// ToastType mendefinisikan tipe visual dari toast notification
type ToastType string

const (
	ToastSuccess ToastType = "success"
	ToastError   ToastType = "error"
	ToastInfo    ToastType = "info"
	ToastWarning ToastType = "warning"
)

// ToastEvent merepresentasikan payload event showToast untuk HTMX HX-Trigger
type ToastEvent struct {
	Message  string `json:"message"`
	Type     string `json:"type"`
	Duration int    `json:"duration,omitempty"` // durasi dalam milidetik, default 4000ms di client
}

// SetToast menyetel header HX-Trigger dengan event showToast
// Format: HX-Trigger: {"showToast": {"message": "...", "type": "success"}}
func SetToast(w http.ResponseWriter, message string, toastType ToastType) {
	if toastType == "" {
		toastType = ToastInfo
	}

	payload := map[string]ToastEvent{
		"showToast": {
			Message: message,
			Type:    string(toastType),
		},
	}

	data, err := json.Marshal(payload)
	if err == nil {
		w.Header().Set("HX-Trigger", string(data))
	}
}

func parseInt(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	var val int
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return defaultValue
		}
		val = val*10 + int(ch-'0')
	}
	if val < 1 {
		return defaultValue
	}
	return val
}
