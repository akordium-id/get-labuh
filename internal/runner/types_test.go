package runner

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvelopeSerialization(t *testing.T) {
	deployJob := DeployJobPayload{
		JobID:         "job-123",
		ApplicationID: "app-abc",
		SourceType:    "git",
		RepositoryURL: "https://github.com/example/repo.git",
		Branch:        "main",
		AppPort:       8080,
		EnvVars: map[string]string{
			"PORT": "8080",
			"ENV":  "production",
		},
		Domains: []string{"example.com", "app.example.com"},
	}

	env, err := NewEnvelope(MsgTypeDeployJob, deployJob)
	require.NoError(t, err)
	assert.Equal(t, MsgTypeDeployJob, env.Type)
	assert.True(t, env.Timestamp > 0)

	// Serialize
	data, err := json.Marshal(env)
	require.NoError(t, err)

	// Deserialize envelope
	var receivedEnv Envelope
	err = json.Unmarshal(data, &receivedEnv)
	require.NoError(t, err)
	assert.Equal(t, MsgTypeDeployJob, receivedEnv.Type)

	// Deserialize payload
	var receivedPayload DeployJobPayload
	err = json.Unmarshal(receivedEnv.Payload, &receivedPayload)
	require.NoError(t, err)
	assert.Equal(t, "job-123", receivedPayload.JobID)
	assert.Equal(t, "app-abc", receivedPayload.ApplicationID)
	assert.Equal(t, 8080, receivedPayload.AppPort)
	assert.Equal(t, "8080", receivedPayload.EnvVars["PORT"])
	assert.Equal(t, []string{"example.com", "app.example.com"}, receivedPayload.Domains)
}

func TestHeartbeatPayload(t *testing.T) {
	hb := HeartbeatPayload{
		Timestamp:        time.Now().Unix(),
		CPUPercent:       12.5,
		MemoryUsedMB:     512,
		MemoryTotalMB:    2048,
		DiskUsedPercent:  35.2,
		DockerVersion:    "27.5.1",
		ActiveContainers: 4,
	}

	env, err := NewEnvelope(MsgTypeHeartbeat, hb)
	require.NoError(t, err)

	var receivedHb HeartbeatPayload
	err = json.Unmarshal(env.Payload, &receivedHb)
	require.NoError(t, err)
	assert.Equal(t, 12.5, receivedHb.CPUPercent)
	assert.Equal(t, 4, receivedHb.ActiveContainers)
}
