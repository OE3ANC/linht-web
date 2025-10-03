package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/gofiber/fiber/v2"
	fiberLogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/linht/web-manager/plugins"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

// Configuration constants
const (
	// Server timeouts
	ServerReadTimeout  = 120 * time.Second
	ServerWriteTimeout = 120 * time.Second

	// Upload limits
	MaxBodySize = 10 * 1024 * 1024 * 1024 // 10 GB

	// Session management (24-hour expiry)
	SessionDuration = 24 * time.Hour
	TokenBytes      = 32
)

type Config struct {
	Server struct {
		Port string `yaml:"port"`
		Host string `yaml:"host"`
	} `yaml:"server"`
	Auth struct {
		PasswordHash string `yaml:"password_hash"`
	} `yaml:"auth"`
	Docker struct {
		Socket string `yaml:"socket"`
	} `yaml:"docker"`
	WebShell struct {
		Shell    string `yaml:"shell"`
		Terminal struct {
			Rows int `yaml:"rows"`
			Cols int `yaml:"cols"`
		} `yaml:"terminal"`
	} `yaml:"webshell"`
	FileManager struct {
		MaxUploadSize int64 `yaml:"max_upload_size"`
	} `yaml:"filemanager"`
	Plugins []string `yaml:"plugins"`
}

// Session represents a simple authenticated session for local use
type Session struct {
	Token     string
	ExpiresAt time.Time
}

var (
	config         Config
	currentSession *Session
	sessionMu      sync.RWMutex
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	if err := loadConfig("config.yaml"); err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}
	slog.Info("Configuration loaded")

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:  ServerReadTimeout,
		WriteTimeout: ServerWriteTimeout,
		AppName:      "Linht Web Manager",
		BodyLimit:    MaxBodySize,
	})

	// Add logger middleware
	app.Use(fiberLogger.New(fiberLogger.Config{
		Format: "[${time}] ${status} - ${method} ${path} (${latency})\n",
	}))

	// Serve static files
	app.Static("/", "./web")

	// Login/logout endpoints (no auth required for login)
	app.Post("/login", handleLogin)
	app.Post("/logout", handleLogout)

	// Auth middleware for all other API routes
	app.Use("/api", authMiddleware)

	// Create shared Docker client
	dockerClient, err := createDockerClient(config.Docker.Socket)
	if err != nil {
		slog.Error("Failed to create Docker client", "error", err, "socket", config.Docker.Socket)
		os.Exit(1)
	}
	defer dockerClient.Close()
	slog.Info("Docker client created", "socket", config.Docker.Socket)

	// Initialize and register plugins
	if err := initPlugins(app, dockerClient); err != nil {
		slog.Error("Failed to initialize plugins", "error", err)
		os.Exit(1)
	}

	// Start server with graceful shutdown
	addr := config.Server.Host + ":" + config.Server.Port

	// Setup graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		slog.Info("Shutting down server...")
		if err := app.ShutdownWithContext(context.Background()); err != nil {
			slog.Error("Server shutdown error", "error", err)
		}
	}()

	slog.Info("Starting Linht Web Manager", "address", addr)
	if err := app.Listen(addr); err != nil {
		slog.Error("Failed to start server", "error", err, "address", addr)
		os.Exit(1)
	}
}

func loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, &config)
}

func handleLogin(c *fiber.Ctx) error {
	var req struct {
		Password string `json:"password"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(config.Auth.PasswordHash), []byte(req.Password)); err != nil {
		slog.Warn("Failed login attempt", "ip", c.IP())
		return c.Status(401).JSON(fiber.Map{"error": "Invalid password"})
	}

	slog.Info("Successful login", "ip", c.IP())

	// Generate new session (replaces any existing session for local-only use)
	sessionMu.Lock()
	currentSession = &Session{
		Token:     generateToken(),
		ExpiresAt: time.Now().Add(SessionDuration),
	}
	sessionMu.Unlock()

	return c.JSON(fiber.Map{
		"success": true,
		"token":   currentSession.Token,
		"expires": currentSession.ExpiresAt.Unix(),
	})
}

func handleLogout(c *fiber.Ctx) error {
	sessionMu.Lock()
	currentSession = nil
	sessionMu.Unlock()
	slog.Info("User logged out", "ip", c.IP())
	return c.JSON(fiber.Map{"success": true})
}

func authMiddleware(c *fiber.Ctx) error {
	// Check for token in header first, fallback to query parameter (for WebSocket/SSE)
	token := c.Get("X-Auth-Token")
	if token == "" {
		token = c.Query("token")
	}

	if !validateToken(token) {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}
	return c.Next()
}

func validateToken(token string) bool {
	if token == "" {
		return false
	}

	sessionMu.RLock()
	defer sessionMu.RUnlock()

	if currentSession == nil {
		return false
	}

	// Check token match and expiration
	if currentSession.Token != token {
		return false
	}

	if time.Now().After(currentSession.ExpiresAt) {
		return false
	}

	return true
}

func generateToken() string {
	b := make([]byte, TokenBytes)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func createDockerClient(socket string) (*client.Client, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost(socket),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	return cli, nil
}

func initPlugins(app *fiber.App, dockerClient *client.Client) error {
	for _, name := range config.Plugins {
		factory, exists := plugins.Get(name)
		if !exists {
			slog.Warn("Unknown plugin", "name", name)
			continue
		}

		// Get plugin-specific config
		var pluginConfig interface{}
		switch name {
		case "docker":
			pluginConfig = dockerClient
		case "webshell":
			pluginConfig = map[string]interface{}{
				"client": dockerClient,
				"shell":  config.WebShell.Shell,
			}
		case "filemanager":
			pluginConfig = map[string]interface{}{
				"max_upload_size": config.FileManager.MaxUploadSize,
			}
		}

		plugin, err := factory(pluginConfig)
		if err != nil {
			return err
		}

		// Set token validator for plugins
		if dockerPlugin, ok := plugin.(*plugins.DockerPlugin); ok {
			dockerPlugin.SetTokenValidator(validateToken)
		}
		if webshellPlugin, ok := plugin.(*plugins.WebShellPlugin); ok {
			webshellPlugin.SetTokenValidator(validateToken)
		}
		if fileManagerPlugin, ok := plugin.(*plugins.FileManagerPlugin); ok {
			fileManagerPlugin.SetTokenValidator(validateToken)
		}

		plugin.RegisterRoutes(app)
		slog.Info("Plugin loaded", "name", plugin.Name())
	}
	return nil
}
