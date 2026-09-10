package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunnerClientEndToEnd(t *testing.T) {
	var mu sync.Mutex
	receivedEnvelopes := make([]Envelope, 0)
	authHeaderReceived := ""

	// Setup Mock SaaS Hub WebSocket Server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaderReceived = r.Header.Get("Authorization")

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("websocket accept error: %v", err)
			return
		}
		defer conn.CloseNow()

		ctx := r.Context()

		// Read hello message from agent
		var helloEnv Envelope
		if err := wsjson.Read(ctx, conn, &helloEnv); err != nil {
			t.Logf("read hello error: %v", err)
			return
		}
		mu.Lock()
		receivedEnvelopes = append(receivedEnvelopes, helloEnv)
		mu.Unlock()

		// Send Deploy Job to Agent
		deployPayload := DeployJobPayload{
			JobID:         "job-test-01",
			ApplicationID: "app-test-01",
			SourceType:    "dockerfile",
			AppPort:       8080,
			LogPath:       "/tmp/labuh-test-logs/job.log",
		}
		deployEnv, _ := NewEnvelope(MsgTypeDeployJob, deployPayload)
		if err := wsjson.Write(ctx, conn, deployEnv); err != nil {
			return
		}

		// Read responses from agent (job status, logs, heartbeats)
		for {
			var env Envelope
			if err := wsjson.Read(ctx, conn, &env); err != nil {
				return
			}
			mu.Lock()
			receivedEnvelopes = append(receivedEnvelopes, env)
			mu.Unlock()
		}
	}))
	defer mockServer.Close()

	wsURL := "ws" + strings.TrimPrefix(mockServer.URL, "http")

	dockerClient, err := docker.NewClient()
	require.NoError(t, err)

	executor := NewExecutor(dockerClient, nil)
	collector := NewMetricsCollector(dockerClient)

	cfg := Config{
		HubURL:            wsURL,
		NodeToken:         "secret-node-token-123",
		Version:           "1.0.0-test",
		HeartbeatInterval: 100 * time.Millisecond,
	}

	client := NewClient(cfg, executor, collector)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		_ = client.Run(ctx)
	}()

	// Wait for messages to be processed
	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		hasHello := false
		hasJobStatus := false
		for _, env := range receivedEnvelopes {
			if env.Type == MsgTypeHello {
				hasHello = true
			}
			if env.Type == MsgTypeJobStatus {
				hasJobStatus = true
			}
		}
		return hasHello && hasJobStatus
	}, 1500*time.Millisecond, 50*time.Millisecond)

	assert.Equal(t, "Bearer secret-node-token-123", authHeaderReceived)

	mu.Lock()
	defer mu.Unlock()

	var helloPayload AgentHelloPayload
	for _, env := range receivedEnvelopes {
		if env.Type == MsgTypeHello {
			_ = json.Unmarshal(env.Payload, &helloPayload)
		}
	}
	assert.Equal(t, "secret-node-token-123", helloPayload.Token)
	assert.Equal(t, "1.0.0-test", helloPayload.Version)
}
