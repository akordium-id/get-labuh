package runner

import (
	"encoding/json"
	"time"
)

type MessageType string

const (
	// Downlink: SaaS -> Runner
	MsgTypeAuthAck         MessageType = "auth_ack"
	MsgTypeAuthReject      MessageType = "auth_reject"
	MsgTypeDeployJob       MessageType = "deploy_job"
	MsgTypeContainerAction MessageType = "container_action"
	MsgTypeProxySync       MessageType = "proxy_sync"
	MsgTypePing            MessageType = "ping"

	// Uplink: Runner -> SaaS
	MsgTypeHello     MessageType = "hello"
	MsgTypeLog       MessageType = "log"
	MsgTypeJobStatus MessageType = "job_status"
	MsgTypeHeartbeat MessageType = "heartbeat"
	MsgTypePong      MessageType = "pong"
)

// Envelope wraps all messages passed over the WebSocket tunnel.
type Envelope struct {
	Type      MessageType     `json:"type"`
	Timestamp int64           `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

func NewEnvelope(msgType MessageType, payload any) (*Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Envelope{
		Type:      msgType,
		Timestamp: time.Now().Unix(),
		Payload:   raw,
	}, nil
}

// AgentHelloPayload is sent by the runner upon connection.
type AgentHelloPayload struct {
	Token    string `json:"token"`
	Version  string `json:"version"`
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

// DeployJobPayload is sent by the SaaS Hub to trigger a deployment.
type DeployJobPayload struct {
	JobID          string            `json:"job_id"`
	ApplicationID  string            `json:"application_id"`
	SourceType     string            `json:"source_type"` // "git", "dockerfile", "image"
	RepositoryURL  string            `json:"repository_url,omitempty"`
	Branch         string            `json:"branch,omitempty"`
	CommitSHA      string            `json:"commit_sha,omitempty"`
	DeployKey      string            `json:"deploy_key,omitempty"`
	DockerfilePath string            `json:"dockerfile_path,omitempty"`
	BuildPath      string            `json:"build_path,omitempty"`
	DockerImage    string            `json:"docker_image,omitempty"`
	AppPort        int               `json:"app_port"`
	EnvVars        map[string]string `json:"env_vars,omitempty"`
	Domains        []string          `json:"domains,omitempty"`
	LogPath        string            `json:"log_path,omitempty"`
	Priority       int               `json:"priority,omitempty"`
}

// ContainerActionPayload requests a container lifecycle action.
type ContainerActionPayload struct {
	Action        string `json:"action"` // "start", "stop", "restart"
	ApplicationID string `json:"application_id"`
	ContainerID   string `json:"container_id,omitempty"`
}

// ProxySyncPayload instructs the runner to update local reverse proxy routing (Caddy).
type ProxySyncPayload struct {
	ApplicationID string   `json:"application_id"`
	Domains       []string `json:"domains"`
	TargetPort    int      `json:"target_port"`
}

// LogChunkPayload streams real-time logs from runner to SaaS Hub.
type LogChunkPayload struct {
	JobID         string `json:"job_id"`
	ApplicationID string `json:"application_id"`
	Stream        string `json:"stream"` // "stdout", "stderr", "system"
	Chunk         string `json:"chunk"`
	Timestamp     int64  `json:"timestamp"`
}

// JobStatusPayload reports status changes during deployment execution.
type JobStatusPayload struct {
	JobID         string `json:"job_id"`
	ApplicationID string `json:"application_id"`
	Step          string `json:"step"`   // "clone", "build", "run", "proxy", "done"
	Status        string `json:"status"` // "queued", "building", "success", "failed"
	ContainerID   string `json:"container_id,omitempty"`
	Error         string `json:"error,omitempty"`
	DurationMs    int64  `json:"duration_ms,omitempty"`
}

// HeartbeatPayload sends periodic system and resource stats to SaaS Hub.
type HeartbeatPayload struct {
	Timestamp        int64   `json:"timestamp"`
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryUsedMB     uint64  `json:"memory_used_mb"`
	MemoryTotalMB    uint64  `json:"memory_total_mb"`
	DiskUsedPercent  float64 `json:"disk_used_percent"`
	DockerVersion    string  `json:"docker_version"`
	ActiveContainers int     `json:"active_containers"`
}
