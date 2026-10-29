package models

import "time"

type CronJobStatus string

const (
	CronJobStatusSuccess CronJobStatus = "success"
	CronJobStatusFailed  CronJobStatus = "failed"
	CronJobStatusRunning CronJobStatus = "running"
)

type ApplicationCronJob struct {
	ID             string         `json:"id"`
	ApplicationID  string         `json:"application_id"`
	Name           string         `json:"name"`
	Command        string         `json:"command"`
	CronExpression string         `json:"cron_expression"`
	IsEnabled      bool           `json:"is_enabled"`
	LastRunAt      *time.Time     `json:"last_run_at"`
	LastStatus     *CronJobStatus `json:"last_status"`
	LastOutput     *string        `json:"last_output"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type CreateCronJobInput struct {
	ApplicationID  string
	Name           string
	Command        string
	CronExpression string
}

type UpdateCronJobInput struct {
	Name           string
	Command        string
	CronExpression string
	IsEnabled      bool
}
