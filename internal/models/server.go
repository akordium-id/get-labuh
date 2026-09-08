package models

import "time"

type ServerStatus string

const (
	ServerStatusOffline ServerStatus = "offline"
	ServerStatusOnline  ServerStatus = "online"
	ServerStatusError   ServerStatus = "error"
)

type Server struct {
	ID         string
	Name       string
	Host       string
	Port       int
	SSHUser    string
	SSHKeyPath string
	Status     ServerStatus
	LastSeen   *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type CreateServerInput struct {
	Name       string
	Host       string
	Port       int
	SSHUser    string
	SSHKeyPath string
}
