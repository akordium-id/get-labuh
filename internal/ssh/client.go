package ssh

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

type Client struct {
	client *ssh.Client
	config *ssh.ClientConfig
}

func NewClient(host, user, keyPath string) (*Client, error) {
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := host
	if !strings.Contains(host, ":") {
		addr = fmt.Sprintf("%s:22", host)
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH: %w", err)
	}

	return &Client{
		client: client,
		config: config,
	}, nil
}

func (c *Client) Execute(command string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), err
	}

	return string(output), nil
}

func (c *Client) CopyFile(localPath, remotePath string) error {
	session, err := c.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	localFile, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer stdin.Close()
		io.Copy(stdin, localFile)
	}()

	escapedRemotePath := "'" + strings.ReplaceAll(remotePath, "'", "'\\''") + "'"
	return session.Run(fmt.Sprintf("cat > %s", escapedRemotePath))
}

func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func TestConnection(host, user, keyPath string) error {
	client, err := NewClient(host, user, keyPath)
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = client.Execute("echo 'connected'")
	return err
}

func ParseAuthorizedKey(keyPath string) (string, error) {
	file, err := os.Open(keyPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			return parts[1], nil
		}
	}

	return "", fmt.Errorf("no valid public key found in %s", keyPath)
}
