package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestSetToast(t *testing.T) {
	rec := httptest.NewRecorder()

	SetToast(rec, "Project berhasil dibuat", ToastSuccess)

	trigger := rec.Header().Get("HX-Trigger")
	if trigger == "" {
		t.Fatal("expected HX-Trigger header to be set, got empty")
	}

	var data map[string]ToastEvent
	if err := json.Unmarshal([]byte(trigger), &data); err != nil {
		t.Fatalf("failed to parse HX-Trigger as JSON: %v", err)
	}

	event, exists := data["showToast"]
	if !exists {
		t.Fatalf("expected 'showToast' key in trigger, got: %s", trigger)
	}

	if event.Message != "Project berhasil dibuat" {
		t.Errorf("expected message 'Project berhasil dibuat', got '%s'", event.Message)
	}

	if event.Type != "success" {
		t.Errorf("expected type 'success', got '%s'", event.Type)
	}
}

func TestSetToastDefaultType(t *testing.T) {
	rec := httptest.NewRecorder()

	SetToast(rec, "Info notification", "")

	trigger := rec.Header().Get("HX-Trigger")
	var data map[string]ToastEvent
	if err := json.Unmarshal([]byte(trigger), &data); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if data["showToast"].Type != "info" {
		t.Errorf("expected default type 'info', got '%s'", data["showToast"].Type)
	}
}
