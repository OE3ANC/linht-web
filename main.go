package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/gofiber/fiber/v2"
	fiberLogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/linht/web-manager/plugins"
	"gopkg.in/yaml.v3"
)

// Configuration constants
const (
	// Server timeouts (increased for large file transfers on embedded devices)
	ServerReadTimeout  = 600 * time.Second // 10 minutes
	ServerWriteTimeout = 600 * time.Second // 10 minutes

	// Upload limits
	MaxBodySize = 10 * 1024 * 1024 * 1024 // 10 GB
)

type Config struct {
	Server struct {
		Port string `yaml:"port"`
		Host string `yaml:"host"`
	} `yaml:"server"`
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
	Hardware struct {
		SX1255 struct {
			SPIDevice string `yaml:"spi_device"`
			SPISpeed  uint32 `yaml:"spi_speed"`
			GPIOChip  string `yaml:"gpio_chip"`
			ResetPin  int    `yaml:"reset_pin"`
			TxRxPin   int    `yaml:"tx_rx_pin"`
			ClockFreq uint32 `yaml:"clock_freq"`
		} `yaml:"sx1255"`
	} `yaml:"hardware"`
	Plugins []string `yaml:"plugins"`
}

var (
	config Config
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

	// Log server configuration
	slog.Info("Server configuration",
		"read_timeout", ServerReadTimeout,
		"write_timeout", ServerWriteTimeout,
		"max_body_size", MaxBodySize,
		"filemanager_max_upload", config.FileManager.MaxUploadSize)

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

	// Add memory tracking middleware for large file operations
	app.Use(func(c *fiber.Ctx) error {
		// Track memory for upload and import endpoints
		if c.Path() == "/api/filemanager/upload" || c.Path() == "/api/images/import" {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			slog.Info("Request started",
				"path", c.Path(),
				"method", c.Method(),
				"content_length", c.Get("Content-Length"),
				"alloc_mb", m.Alloc/1024/1024,
				"sys_mb", m.Sys/1024/1024)
		}
		return c.Next()
	})

	// Serve static files
	app.Static("/", "./web")

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
		case "hardware":
			pluginConfig = map[string]interface{}{
				"sx1255": map[string]interface{}{
					"spi_device": config.Hardware.SX1255.SPIDevice,
					"spi_speed":  config.Hardware.SX1255.SPISpeed,
					"gpio_chip":  config.Hardware.SX1255.GPIOChip,
					"reset_pin":  config.Hardware.SX1255.ResetPin,
					"tx_rx_pin":  config.Hardware.SX1255.TxRxPin,
					"clock_freq": config.Hardware.SX1255.ClockFreq,
				},
			}
		}

		plugin, err := factory(pluginConfig)
		if err != nil {
			return err
		}

		plugin.RegisterRoutes(app)
		slog.Info("Plugin loaded", "name", plugin.Name())
	}
	return nil
}
