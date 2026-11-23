package models

import "time"

type DockerRegistry struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	ServerAddress string    `json:"server_address"`
	Username      string    `json:"username"`
	Password      string    `json:"-"` // Terenkripsi di DB, disembunyikan dari JSON
	Email         string    `json:"email"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CreateDockerRegistryInput struct {
	Name          string `json:"name"`
	ServerAddress string `json:"server_address"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	Email         string `json:"email"`
}

type UpdateDockerRegistryInput struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	ServerAddress string  `json:"server_address"`
	Username      string  `json:"username"`
	Password      *string `json:"password,omitempty"`
	Email         string  `json:"email"`
}
