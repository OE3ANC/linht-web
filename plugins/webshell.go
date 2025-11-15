package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
)

// Session types
const (
	SessionTypeHost      = "host"
	SessionTypeContainer = "container"
)

// WebShellPlugin provides terminal access to host and containers
type WebShellPlugin struct {
	dockerClient *client.Client
	sessions     map[string]*Session
	sessionsMu   sync.RWMutex
	defaultShell string
}

// Session represents an active terminal session
type Session struct {
	ID           string
	Type         string
	ContainerID  string
	PTY          *os.File
	Cmd          *exec.Cmd
	ExecID       string
	HijackedResp types.HijackedResponse
	Closed       bool
	mu           sync.Mutex
}

// ResizeMessage represents a terminal resize request
type ResizeMessage struct {
	Type string `json:"type"`
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

// NewWebShellPlugin creates a new WebShell plugin instance
func NewWebShellPlugin(dockerClient *client.Client, defaultShell string) (*WebShellPlugin, error) {
	if dockerClient == nil {
		return nil, fmt.Errorf("docker client cannot be nil")
	}

	if defaultShell == "" {
		defaultShell = "/bin/sh"
	}

	return &WebShellPlugin{
		dockerClient: dockerClient,
		sessions:     make(map[string]*Session),
		defaultShell: defaultShell,
	}, nil
}

// Name returns the plugin identifier
func (p *WebShellPlugin) Name() string {
	return "webshell"
}

// RegisterRoutes adds the plugin's HTTP routes
func (p *WebShellPlugin) RegisterRoutes(app *fiber.App) {
	api := app.Group("/api/webshell")

	// WebSocket endpoint for terminal
	api.Get("/ws", websocket.New(p.handleWebSocket))

	// REST endpoint to list running containers
	api.Get("/containers", p.listContainers)
}

// Shutdown performs cleanup
func (p *WebShellPlugin) Shutdown() error {
	p.sessionsMu.Lock()
	defer p.sessionsMu.Unlock()

	// Close all sessions
	for id := range p.sessions {
		p.closeSessionUnsafe(id)
	}

	// Docker client is shared, so we don't close it here
	return nil
}

// handleWebSocket handles WebSocket connections for terminal I/O
func (p *WebShellPlugin) handleWebSocket(c *websocket.Conn) {
	sessionType := c.Query("type")
	containerID := c.Query("container")

	var session *Session
	var err error

	// Create appropriate session
	switch sessionType {
	case SessionTypeHost:
		session, err = p.createHostSession()
	case SessionTypeContainer:
		if containerID == "" {
			c.WriteJSON(fiber.Map{"error": "Container ID required"})
			return
		}
		session, err = p.createContainerSession(containerID)
	default:
		c.WriteJSON(fiber.Map{"error": "Invalid session type. Use 'host' or 'container'"})
		return
	}

	if err != nil {
		c.WriteJSON(fiber.Map{"error": err.Error()})
		return
	}

	defer p.CloseSession(session.ID)

	// Handle I/O
	if session.Type == SessionTypeHost {
		p.handleHostSession(c, session)
	} else {
		p.handleContainerSession(c, session)
	}
}

// createHostSession creates a new host shell session
func (p *WebShellPlugin) createHostSession() (*Session, error) {
	sessionID := uuid.New().String()

	// Start shell with PTY
	cmd := exec.Command(p.defaultShell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	// Set initial directory to home directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		cmd.Dir = homeDir
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start PTY: %w", err)
	}

	session := &Session{
		ID:   sessionID,
		Type: SessionTypeHost,
		PTY:  ptmx,
		Cmd:  cmd,
	}

	p.sessionsMu.Lock()
	p.sessions[sessionID] = session
	p.sessionsMu.Unlock()

	return session, nil
}

// createContainerSession creates a new container shell session
func (p *WebShellPlugin) createContainerSession(containerID string) (*Session, error) {
	ctx := context.Background()
	sessionID := uuid.New().String()

	// Create exec instance
	execConfig := container.ExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          []string{"/bin/sh"},
	}

	execIDResp, err := p.dockerClient.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach to exec
	resp, err := p.dockerClient.ContainerExecAttach(ctx, execIDResp.ID, container.ExecStartOptions{
		Tty: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach to exec: %w", err)
	}

	session := &Session{
		ID:           sessionID,
		Type:         SessionTypeContainer,
		ContainerID:  containerID,
		ExecID:       execIDResp.ID,
		HijackedResp: resp,
	}

	p.sessionsMu.Lock()
	p.sessions[sessionID] = session
	p.sessionsMu.Unlock()

	return session, nil
}

// handleHostSession handles I/O for host shell sessions
func (p *WebShellPlugin) handleHostSession(c *websocket.Conn, session *Session) {
	// Goroutine: Read from PTY and send to WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := session.PTY.Read(buf)
			if err != nil {
				return
			}
			if err := c.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
				return
			}
		}
	}()

	// Read from WebSocket and write to PTY
	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			return
		}

		// Check if this is a resize message
		var resizeMsg ResizeMessage
		if err := json.Unmarshal(msg, &resizeMsg); err == nil && resizeMsg.Type == "resize" {
			pty.Setsize(session.PTY, &pty.Winsize{
				Rows: resizeMsg.Rows,
				Cols: resizeMsg.Cols,
			})
			continue
		}

		// Regular input - write to PTY
		if _, err := session.PTY.Write(msg); err != nil {
			return
		}
	}
}

// handleContainerSession handles I/O for container shell sessions
func (p *WebShellPlugin) handleContainerSession(c *websocket.Conn, session *Session) {
	// Goroutine: Read from container and send to WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := session.HijackedResp.Reader.Read(buf)
			if err != nil {
				return
			}
			if err := c.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
				return
			}
		}
	}()

	// Read from WebSocket and write to container
	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			return
		}

		// Check if this is a resize message
		var resizeMsg ResizeMessage
		if err := json.Unmarshal(msg, &resizeMsg); err == nil && resizeMsg.Type == "resize" {
			p.dockerClient.ContainerExecResize(context.Background(), session.ExecID, container.ResizeOptions{
				Height: uint(resizeMsg.Rows),
				Width:  uint(resizeMsg.Cols),
			})
			continue
		}

		// Regular input - write to container
		if _, err := session.HijackedResp.Conn.Write(msg); err != nil {
			return
		}
	}
}

// CloseSession closes a session and cleans up resources
func (p *WebShellPlugin) CloseSession(sessionID string) error {
	p.sessionsMu.Lock()
	defer p.sessionsMu.Unlock()
	return p.closeSessionUnsafe(sessionID)
}

// closeSessionUnsafe closes a session without locking (internal use)
func (p *WebShellPlugin) closeSessionUnsafe(sessionID string) error {
	session, exists := p.sessions[sessionID]
	if !exists {
		return nil
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Closed {
		return nil
	}

	session.Closed = true

	switch session.Type {
	case SessionTypeHost:
		if session.PTY != nil {
			session.PTY.Close()
		}
		if session.Cmd != nil && session.Cmd.Process != nil {
			session.Cmd.Process.Kill()
		}
	case SessionTypeContainer:
		session.HijackedResp.Close()
	}

	delete(p.sessions, sessionID)
	return nil
}

// listContainers returns running containers for shell access
func (p *WebShellPlugin) listContainers(c *fiber.Ctx) error {
	ctx := context.Background()
	containers, err := p.dockerClient.ContainerList(ctx, container.ListOptions{
		All: false, // Only running containers
	})
	if err != nil {
		return SendError(c, 500, err)
	}

	result := make([]fiber.Map, len(containers))
	for i, cont := range containers {
		name := "unnamed"
		if len(cont.Names) > 0 {
			name = cont.Names[0]
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
		}

		result[i] = fiber.Map{
			"id":    cont.ID,
			"name":  name,
			"image": cont.Image,
			"state": cont.State,
		}
	}

	return SendSuccess(c, result, "")
}

// Register the plugin
func init() {
	Register("webshell", func(config interface{}) (Plugin, error) {
		configMap, ok := config.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid config for webshell plugin: expected map[string]interface{}")
		}

		dockerClient, ok := configMap["client"].(*client.Client)
		if !ok {
			return nil, fmt.Errorf("invalid config for webshell plugin: client must be *client.Client")
		}

		shell, _ := configMap["shell"].(string)

		return NewWebShellPlugin(dockerClient, shell)
	})
}
