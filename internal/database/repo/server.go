package repo

import (
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

type ServerRepo struct {
	db *sql.DB
}

func NewServerRepo(db *sql.DB) *ServerRepo {
	return &ServerRepo{db: db}
}

func (r *ServerRepo) Create(input models.CreateServerInput) (*models.Server, error) {
	server := &models.Server{
		ID:         uuid.New().String(),
		Name:       input.Name,
		Host:       input.Host,
		Port:       input.Port,
		SSHUser:    input.SSHUser,
		SSHKeyPath: input.SSHKeyPath,
		Status:     models.ServerStatusOffline,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if server.Port == 0 {
		server.Port = 22
	}
	if server.SSHUser == "" {
		server.SSHUser = "root"
	}

	_, err := r.db.Exec(
		`INSERT INTO servers (id, name, host, port, ssh_user, ssh_key_path, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		server.ID, server.Name, server.Host, server.Port, server.SSHUser, server.SSHKeyPath, server.Status, server.CreatedAt, server.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return server, nil
}

func (r *ServerRepo) GetByID(id string) (*models.Server, error) {
	server := &models.Server{}
	err := r.db.QueryRow(
		`SELECT id, name, host, port, ssh_user, ssh_key_path, status, last_seen, created_at, updated_at
		 FROM servers WHERE id = ?`,
		id,
	).Scan(&server.ID, &server.Name, &server.Host, &server.Port, &server.SSHUser, &server.SSHKeyPath, &server.Status, &server.LastSeen, &server.CreatedAt, &server.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return server, nil
}

func (r *ServerRepo) GetAll() ([]*models.Server, error) {
	rows, err := r.db.Query(
		`SELECT id, name, host, port, ssh_user, ssh_key_path, status, last_seen, created_at, updated_at
		 FROM servers ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*models.Server
	for rows.Next() {
		server := &models.Server{}
		err := rows.Scan(&server.ID, &server.Name, &server.Host, &server.Port, &server.SSHUser, &server.SSHKeyPath, &server.Status, &server.LastSeen, &server.CreatedAt, &server.UpdatedAt)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}

	return servers, nil
}

func (r *ServerRepo) UpdateStatus(id string, status models.ServerStatus) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE servers SET status = ?, last_seen = ?, updated_at = ? WHERE id = ?`,
		status, now, now, id,
	)
	return err
}

func (r *ServerRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM servers WHERE id = ?", id)
	return err
}
