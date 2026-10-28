package models

import "time"

type StorageDestination struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	IsDefault       bool      `json:"is_default"`
	Endpoint        string    `json:"endpoint"`
	Bucket          string    `json:"bucket"`
	Region          string    `json:"region"`
	AccessKeyID     string    `json:"access_key_id"`
	SecretAccessKey string    `json:"secret_access_key"`
	Prefix          string    `json:"prefix"`
	UseSSL          bool      `json:"use_ssl"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type CreateStorageDestinationInput struct {
	Name            string `json:"name"`
	IsDefault       bool   `json:"is_default"`
	Endpoint        string `json:"endpoint"`
	Bucket          string `json:"bucket"`
	Region          string `json:"region"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	Prefix          string `json:"prefix"`
	UseSSL          bool   `json:"use_ssl"`
}

type DatabaseBackupConfig struct {
	ID                   string              `json:"id"`
	DatabaseID           string              `json:"database_id"`
	StorageDestinationID *string             `json:"storage_destination_id"`
	Enabled              bool                `json:"enabled"`
	CronExpression       string              `json:"cron_expression"`
	RetentionCount       int                 `json:"retention_count"`
	CustomPrefix         *string             `json:"custom_prefix"`
	StorageDestination   *StorageDestination `json:"storage_destination,omitempty"`
	CreatedAt            time.Time           `json:"created_at"`
	UpdatedAt            time.Time           `json:"updated_at"`
}

type DatabaseBackup struct {
	ID                   string              `json:"id"`
	DatabaseID           string              `json:"database_id"`
	StorageDestinationID *string             `json:"storage_destination_id"`
	Filename             string              `json:"filename"`
	FileSize             int64               `json:"file_size"`
	StorageType          string              `json:"storage_type"` // "local" or "s3"
	S3Key                *string             `json:"s3_key"`
	Status               string              `json:"status"` // "pending", "completed", "failed"
	ErrorMessage         *string             `json:"error_message"`
	StorageDestination   *StorageDestination `json:"storage_destination,omitempty"`
	CreatedAt            time.Time           `json:"created_at"`
}
