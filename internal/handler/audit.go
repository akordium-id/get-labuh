package handler

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/middleware"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/settings"
)

type AuditHandler struct {
	auditRepo *repo.AuditLogRepo
	userRepo  *repo.UserRepo
}

func NewAuditHandler(auditRepo *repo.AuditLogRepo, userRepo *repo.UserRepo) *AuditHandler {
	return &AuditHandler{
		auditRepo: auditRepo,
		userRepo:  userRepo,
	}
}

func (h *AuditHandler) AuditLogsPage(w http.ResponseWriter, r *http.Request) {
	logs, err := h.auditRepo.GetAll(100)
	if err != nil {
		logs = []*models.AuditLog{}
	}

	templ.Handler(layouts.AppLayout(settings.AuditLogsPage(logs))).ServeHTTP(w, r)
}

func LogAudit(auditRepo *repo.AuditLogRepo, r *http.Request, action, resource, resourceID string) {
	middleware.LogAudit(auditRepo, r, action, resource, resourceID)
}

func LogAuditSensitive(auditRepo *repo.AuditLogRepo, r *http.Request, action, resource, resourceID string) {
	middleware.LogAudit(auditRepo, r, action, resource, resourceID)
}

func AuditLoggingMiddleware(auditRepo *repo.AuditLogRepo) func(http.Handler) http.Handler {
	return middleware.AuditLoggingMiddleware(auditRepo)
}