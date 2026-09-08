package models

import "time"

type DeployKey struct {
	ID          string
	ProjectID   string
	Name        string
	PublicKey   string
	PrivateKey  string
	Fingerprint string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CreateDeployKeyInput struct {
	ProjectID   string
	Name        string
	PublicKey   string
	PrivateKey  string
	Fingerprint string
}
