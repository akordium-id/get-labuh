package models

import (
	stdtesting "testing"

	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string {
	return &s
}

func TestDeployStepConstants(t *stdtesting.T) {
	assert.Equal(t, DeployStep("clone"), DeployStepClone)
	assert.Equal(t, DeployStep("build"), DeployStepBuild)
	assert.Equal(t, DeployStep("push"), DeployStepPush)
	assert.Equal(t, DeployStep("deploy"), DeployStepDeploy)
	assert.Equal(t, DeployStep("cleanup"), DeployStepCleanup)
}

func TestDeploymentStruct(t *stdtesting.T) {
	deployment := &Deployment{
		ID:            "deploy-1",
		ApplicationID: "app-1",
		Status:        DeployStatusQueued,
		Step:          strPtr("clone"),
		Output:        strPtr("cloning repo..."),
	}

	assert.Equal(t, "deploy-1", deployment.ID)
	assert.Equal(t, "app-1", deployment.ApplicationID)
	assert.Equal(t, DeployStatusQueued, deployment.Status)
	assert.Equal(t, "clone", *deployment.Step)
	assert.Equal(t, "cloning repo...", *deployment.Output)
}

func TestCreateDeploymentInput(t *stdtesting.T) {
	input := CreateDeploymentInput{
		ApplicationID:  "app-1",
		CommitHash:     strPtr("abc123"),
		CommitMessage:  strPtr("fix: bug"),
		LogPath:        strPtr("/tmp/logs"),
		Step:           strPtr("build"),
		Output:         strPtr("building..."),
	}

	assert.Equal(t, "app-1", input.ApplicationID)
	assert.Equal(t, "abc123", *input.CommitHash)
	assert.Equal(t, "fix: bug", *input.CommitMessage)
	assert.Equal(t, "/tmp/logs", *input.LogPath)
	assert.Equal(t, "build", *input.Step)
	assert.Equal(t, "building...", *input.Output)
}
