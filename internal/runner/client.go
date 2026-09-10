package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type Config struct {
	HubURL            string
	NodeToken         string
	Version           string
	HeartbeatInterval time.Duration
}

type Client struct {
	config     Config
	executor   *Executor
	collector  *MetricsCollector
	conn       *websocket.Conn
	connMu     sync.Mutex
	writeMu    sync.Mutex
	isClosed   bool
	closeChan  chan struct{}
}

func NewClient(cfg Config, executor *Executor, collector *MetricsCollector) *Client {
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 15 * time.Second
	}
	if cfg.Version == "" {
		cfg.Version = "0.1.0"
	}
	return &Client{
		config:    cfg,
		executor:  executor,
		collector: collector,
		closeChan: make(chan struct{}),
	}
}

func (c *Client) Run(ctx context.Context) error {
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	hostname, _ := os.Hostname()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		slog.Info("connecting to SaaS Hub", "url", c.config.HubURL)
		err := c.connectAndServe(ctx, hostname)
		if err != nil {
			slog.Warn("connection to SaaS Hub lost or failed", "error", err, "retry_in", backoff)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (c *Client) connectAndServe(ctx context.Context, hostname string) error {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+c.config.NodeToken)
	headers.Set("X-Agent-Version", c.config.Version)

	dialCtx, dialCancel := context.WithTimeout(ctx, 15*time.Second)
	defer dialCancel()

	conn, _, err := websocket.Dial(dialCtx, c.config.HubURL, &websocket.DialOptions{
		HTTPHeader: headers,
	})
	if err != nil {
		return fmt.Errorf("websocket dial error: %w", err)
	}
	defer conn.CloseNow()

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	slog.Info("successfully connected to SaaS Hub")

	// Send Agent Hello
	hello := AgentHelloPayload{
		Token:    c.config.NodeToken,
		Version:  c.config.Version,
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
	}
	if err := c.Send(ctx, MsgTypeHello, hello); err != nil {
		return fmt.Errorf("failed to send hello handshake: %w", err)
	}

	sessionCtx, sessionCancel := context.WithCancel(ctx)
	defer sessionCancel()

	// Heartbeat loop
	go c.runHeartbeat(sessionCtx)

	// Reader loop
	for {
		var env Envelope
		err := wsjson.Read(sessionCtx, conn, &env)
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}

		go c.handleMessage(sessionCtx, env)
	}
}

func (c *Client) handleMessage(ctx context.Context, env Envelope) {
	slog.Debug("received message from hub", "type", env.Type)

	switch env.Type {
	case MsgTypePing:
		_ = c.Send(ctx, MsgTypePong, map[string]string{"reply": "pong"})

	case MsgTypeDeployJob:
		var payload DeployJobPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			slog.Error("failed to unmarshal deploy job payload", "error", err)
			return
		}
		go c.runDeployJob(ctx, payload)

	case MsgTypeContainerAction:
		var payload ContainerActionPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			slog.Error("failed to unmarshal container action payload", "error", err)
			return
		}
		if err := c.executor.ExecuteContainerAction(ctx, payload); err != nil {
			slog.Error("container action failed", "action", payload.Action, "error", err)
		}

	case MsgTypeProxySync:
		var payload ProxySyncPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			slog.Error("failed to unmarshal proxy sync payload", "error", err)
			return
		}
		if err := c.executor.ExecuteProxySync(ctx, payload); err != nil {
			slog.Error("proxy sync failed", "error", err)
		}

	default:
		slog.Warn("unhandled message type from hub", "type", env.Type)
	}
}

func (c *Client) runDeployJob(ctx context.Context, payload DeployJobPayload) {
	startTime := time.Now()
	slog.Info("executing deploy job from SaaS Hub", "job_id", payload.JobID, "app_id", payload.ApplicationID)

	_ = c.SendJobStatus(ctx, JobStatusPayload{
		JobID:         payload.JobID,
		ApplicationID: payload.ApplicationID,
		Step:          "init",
		Status:        "building",
	})

	logWriter := &wsLogWriter{
		client:        c,
		ctx:           ctx,
		jobID:         payload.JobID,
		applicationID: payload.ApplicationID,
	}

	statusReporter := func(step string, status string, err error) {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		_ = c.SendJobStatus(ctx, JobStatusPayload{
			JobID:         payload.JobID,
			ApplicationID: payload.ApplicationID,
			Step:          step,
			Status:        status,
			Error:         errMsg,
		})
	}

	err := c.executor.ExecuteDeploy(ctx, payload, logWriter, statusReporter)
	durationMs := time.Since(startTime).Milliseconds()

	if err != nil {
		slog.Error("deployment job failed", "job_id", payload.JobID, "error", err)
		_ = c.SendJobStatus(ctx, JobStatusPayload{
			JobID:         payload.JobID,
			ApplicationID: payload.ApplicationID,
			Step:          "done",
			Status:        "failed",
			Error:         err.Error(),
			DurationMs:    durationMs,
		})
	} else {
		slog.Info("deployment job completed successfully", "job_id", payload.JobID)
		_ = c.SendJobStatus(ctx, JobStatusPayload{
			JobID:         payload.JobID,
			ApplicationID: payload.ApplicationID,
			Step:          "done",
			Status:        "success",
			DurationMs:    durationMs,
		})
	}
}

func (c *Client) runHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if c.collector != nil {
				stats := c.collector.Collect(ctx)
				_ = c.Send(ctx, MsgTypeHeartbeat, stats)
			}
		}
	}
}

func (c *Client) Send(ctx context.Context, msgType MessageType, payload any) error {
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()

	if conn == nil {
		return fmt.Errorf("websocket connection is not active")
	}

	env, err := NewEnvelope(msgType, payload)
	if err != nil {
		return fmt.Errorf("failed to encode envelope: %w", err)
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return wsjson.Write(writeCtx, conn, env)
}

func (c *Client) SendLog(ctx context.Context, jobID, appID, stream, chunk string) error {
	payload := LogChunkPayload{
		JobID:         jobID,
		ApplicationID: appID,
		Stream:        stream,
		Chunk:         chunk,
		Timestamp:     time.Now().Unix(),
	}
	return c.Send(ctx, MsgTypeLog, payload)
}

func (c *Client) SendJobStatus(ctx context.Context, status JobStatusPayload) error {
	return c.Send(ctx, MsgTypeJobStatus, status)
}

type wsLogWriter struct {
	client        *Client
	ctx           context.Context
	jobID         string
	applicationID string
}

func (w *wsLogWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	_ = w.client.SendLog(w.ctx, w.jobID, w.applicationID, "stdout", string(p))
	return len(p), nil
}
