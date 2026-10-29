package applications

import (
	"context"

	"github.com/akordium-id/get-labuh/internal/models"
)

// EnvironmentProvider mengabstraksi pembacaan project & environment.
type EnvironmentProvider interface {
	GetEnvironmentByID(id string) (*models.Environment, error)
	GetByID(id string) (*models.Project, error)
}

// ProjectEnvironmentProvider mengabstraksi pembuatan dan pembacaan project & environment untuk Dropzone.
type ProjectEnvironmentProvider interface {
	GetAll() ([]*models.Project, error)
	Create(input models.CreateProjectInput) (*models.Project, error)
	GetEnvironmentsByProjectID(projectID string) ([]*models.Environment, error)
	CreateEnvironment(projectID, name, slug string) (*models.Environment, error)
}

// DeploymentCreator mengabstraksi pembuatan dan pembacaan deployment record.
type DeploymentCreator interface {
	Create(input models.CreateDeploymentInput) (*models.Deployment, error)
	CreateWithContext(ctx context.Context, input models.CreateDeploymentInput) (*models.Deployment, error)
	GetByApplicationIDWithContext(ctx context.Context, appID string) ([]*models.Deployment, error)
	MarkStarted(id string) error
	UpdateStatus(id string, status models.DeployStatus) error
}

// SettingProvider mengabstraksi pembacaan setting aplikasi.
type SettingProvider interface {
	Get(ctx context.Context, key string) (string, error)
}

// DeployEnqueuer mengabstraksi worker antrean deploy.
type DeployEnqueuer interface {
	Enqueue(job models.DeploymentJob) error
}
