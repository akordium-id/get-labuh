package applications

import "github.com/akordium-id/get-labuh/internal/models"

// BuildDeploymentJob mengonversi data aplikasi dan environment variables menjadi DeploymentJob.
func BuildDeploymentJob(app *models.Application, deploymentID string, envVars []*models.AppEnvVar) models.DeploymentJob {
	var envList []models.AppEnvVar
	for _, ev := range envVars {
		if ev != nil {
			envList = append(envList, *ev)
		}
	}
	var repoURL, branch, dockerfilePath, buildPath, dockerImage string
	if app.RepositoryURL != nil {
		repoURL = *app.RepositoryURL
	}
	if app.Branch != nil {
		branch = *app.Branch
	}
	if app.DockerfilePath != nil {
		dockerfilePath = *app.DockerfilePath
	}
	if app.BuildPath != nil {
		buildPath = *app.BuildPath
	}
	if app.DockerImage != nil {
		dockerImage = *app.DockerImage
	}

	return models.DeploymentJob{
		DeploymentID:   deploymentID,
		ApplicationID:  app.ID,
		SourceType:     string(app.SourceType),
		RepositoryURL:  repoURL,
		Branch:         branch,
		DockerfilePath: dockerfilePath,
		BuildPath:      buildPath,
		DockerImage:    dockerImage,
		AppPort:        app.AppPort,
		EnvVars:        envList,
		LogPath:        "/tmp/labuh-logs/" + deploymentID + ".log",
		Priority:       1,
		ServerID:       app.ServerID,
	}
}
