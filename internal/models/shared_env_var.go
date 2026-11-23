package models

import "time"

type SharedEnvVarScope string

const (
	ScopeProject SharedEnvVarScope = "project"
	ScopeTeam    SharedEnvVarScope = "team"
)

type SharedEnvVar struct {
	ID          string            `json:"id"`
	Scope       SharedEnvVarScope `json:"scope"`
	ScopeID     string            `json:"scope_id"`
	Key         string            `json:"key"`
	Value       string            `json:"value"`
	IsSecret    bool              `json:"is_secret"`
	IsEncrypted bool              `json:"is_encrypted"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type ResolvedEnvVar struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret"`
	Source   string `json:"source"` // "team", "project", atau "app"
}
