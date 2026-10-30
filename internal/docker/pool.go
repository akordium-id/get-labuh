package docker

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/client"
	"golang.org/x/crypto/ssh"

	"github.com/akordium-id/get-labuh/internal/models"
)

// ServerProvider mengabstraksi pengambilan data server untuk koneksi remote.
type ServerProvider interface {
	GetByID(id string) (*models.Server, error)
}

// ClientPool mengelola pool koneksi docker client lokal dan remote.
type ClientPool struct {
	mu             sync.RWMutex
	defaultClient  *Client
	serverProvider ServerProvider
	remoteClients  map[string]*Client
	sshClients     map[string]*ssh.Client
}

func NewClientPool(defaultClient *Client, provider ServerProvider) *ClientPool {
	return &ClientPool{
		defaultClient:  defaultClient,
		serverProvider: provider,
		remoteClients:  make(map[string]*Client),
		sshClients:     make(map[string]*ssh.Client),
	}
}

// GetClient mengembalikan docker client yang sesuai untuk serverID.
// Jika serverID kosong atau "local", akan mengembalikan defaultClient (lokal).
func (p *ClientPool) GetClient(ctx context.Context, serverID *string) (*Client, error) {
	if serverID == nil || *serverID == "" || *serverID == "local" {
		return p.defaultClient, nil
	}

	p.mu.RLock()
	if c, ok := p.remoteClients[*serverID]; ok {
		p.mu.RUnlock()
		return c, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double check
	if c, ok := p.remoteClients[*serverID]; ok {
		return c, nil
	}

	if p.serverProvider == nil {
		return nil, fmt.Errorf("server provider not configured")
	}

	server, err := p.serverProvider.GetByID(*serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to load server %s: %w", *serverID, err)
	}

	remoteCli, sshCli, err := DialRemoteDocker(server)
	if err != nil {
		return nil, fmt.Errorf("failed to dial remote docker on %s (%s): %w", server.Name, server.Host, err)
	}

	remoteDockerClient := &Client{cli: remoteCli}
	p.remoteClients[*serverID] = remoteDockerClient
	p.sshClients[*serverID] = sshCli

	return remoteDockerClient, nil
}

// DialRemoteDocker membangun koneksi Docker API melalui SSH tunnel ke remote Unix socket /var/run/docker.sock
func DialRemoteDocker(server *models.Server) (*client.Client, *ssh.Client, error) {
	var keyBytes []byte
	if server.SSHKeyContent != nil && *server.SSHKeyContent != "" {
		keyBytes = []byte(*server.SSHKeyContent)
	} else if server.SSHKeyPath != "" {
		var err error
		keyBytes, err = os.ReadFile(server.SSHKeyPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read ssh key file: %w", err)
		}
	} else {
		return nil, nil, fmt.Errorf("no ssh key provided for server %s", server.Name)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse ssh private key: %w", err)
	}

	sshUser := server.SSHUser
	if sshUser == "" {
		sshUser = "root"
	}
	sshPort := server.Port
	if sshPort == 0 {
		sshPort = 22
	}

	config := &ssh.ClientConfig{
		User: sshUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", server.Host, sshPort)
	if strings.Contains(server.Host, ":") {
		addr = server.Host
	}

	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, nil, fmt.Errorf("ssh dial failed: %w", err)
	}

	// Custom HTTP transport yang me-route traffic Docker API langsung via SSH Dial ke remote /var/run/docker.sock
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return sshClient.Dial("unix", "/var/run/docker.sock")
			},
		},
	}

	cli, err := client.NewClientWithOpts(
		client.WithHTTPClient(httpClient),
		client.WithHost("http://docker"),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		_ = sshClient.Close()
		return nil, nil, fmt.Errorf("failed to create docker client over ssh: %w", err)
	}

	return cli, sshClient, nil
}

// CloseAll menutup semua koneksi remote Docker dan SSH client.
func (p *ClientPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, sshCli := range p.sshClients {
		_ = sshCli.Close()
	}
	p.remoteClients = make(map[string]*Client)
	p.sshClients = make(map[string]*ssh.Client)
}
