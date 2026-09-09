package repo

import (
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

type AuditLogRepo struct {
	db *sql.DB
}

func NewAuditLogRepo(db *sql.DB) *AuditLogRepo {
	return &AuditLogRepo{db: db}
}

func (r *AuditLogRepo) Create(input models.CreateAuditLogInput) (*models.AuditLog, error) {
	log := &models.AuditLog{
		ID:         uuid.New().String(),
		UserID:     input.UserID,
		Action:     input.Action,
		Resource:   input.Resource,
		ResourceID: input.ResourceID,
		IPAddress:  input.IPAddress,
		UserAgent:  input.UserAgent,
		CreatedAt:  time.Now(),
	}

	_, err := r.db.Exec(
		`INSERT INTO audit_logs (id, user_id, action, resource, resource_id, ip_address, user_agent, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		log.ID, log.UserID, log.Action, log.Resource, log.ResourceID, log.IPAddress, log.UserAgent, log.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return log, nil
}

func (r *AuditLogRepo) GetByUserID(userID string, limit int) ([]*models.AuditLog, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, action, resource, resource_id, ip_address, user_agent, created_at
		 FROM audit_logs WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		log := &models.AuditLog{}
		err := rows.Scan(&log.ID, &log.UserID, &log.Action, &log.Resource, &log.ResourceID, &log.IPAddress, &log.UserAgent, &log.CreatedAt)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, nil
}

func (r *AuditLogRepo) GetAll(limit int) ([]*models.AuditLog, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, action, resource, resource_id, ip_address, user_agent, created_at
		 FROM audit_logs ORDER BY created_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		log := &models.AuditLog{}
		err := rows.Scan(&log.ID, &log.UserID, &log.Action, &log.Resource, &log.ResourceID, &log.IPAddress, &log.UserAgent, &log.CreatedAt)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, nil
}

func (r *AuditLogRepo) GetByResource(resource, resourceID string, limit int) ([]*models.AuditLog, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, action, resource, resource_id, ip_address, user_agent, created_at
		 FROM audit_logs WHERE resource = ? AND resource_id = ? ORDER BY created_at DESC LIMIT ?`,
		resource, resourceID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		log := &models.AuditLog{}
		err := rows.Scan(&log.ID, &log.UserID, &log.Action, &log.Resource, &log.ResourceID, &log.IPAddress, &log.UserAgent, &log.CreatedAt)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, nil
}
