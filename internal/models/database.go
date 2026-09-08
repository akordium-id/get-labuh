package models

import "time"

type DatabaseEngine string

const (
	DatabaseEnginePostgres DatabaseEngine = "postgres"
	DatabaseEngineMySQL    DatabaseEngine = "mysql"
	DatabaseEngineRedis    DatabaseEngine = "redis"
	DatabaseEngineMongoDB  DatabaseEngine = "mongodb"
)

type DatabaseStatus string

const (
	DatabaseStatusIdle     DatabaseStatus = "idle"
	DatabaseStatusRunning  DatabaseStatus = "running"
	DatabaseStatusStopped  DatabaseStatus = "stopped"
	DatabaseStatusFailed   DatabaseStatus = "failed"
)

type DatabaseService struct {
	ID              string
	EnvironmentID   string
	Name            string
	Slug            string
	Engine          DatabaseEngine
	Version         string
	Username        string
	Password        string
	DatabaseName    string
	Port            int
	ContainerID     *string
	ContainerName   *string
	Status          DatabaseStatus
	ConnectionString *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateDatabaseServiceInput struct {
	EnvironmentID   string
	Name            string
	Slug            string
	Engine          DatabaseEngine
	Version         string
	Username        string
	Password        string
	DatabaseName    string
	Port            int
}

type ContainerStats struct {
	CPUPercent    float64
	MemoryUsage   uint64
	MemoryLimit   uint64
	NetworkRx     uint64
	NetworkTx     uint64
}
