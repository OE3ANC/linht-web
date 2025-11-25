package plugins

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ServiceInfo represents information about a systemd service
type ServiceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ActiveState string `json:"active_state"`
	UnitState   string `json:"unit_state"`
	IsActive    bool   `json:"is_active"`
	IsEnabled   bool   `json:"is_enabled"`
}

type ServicesPlugin struct {
	prefix          string
	defaultLogLines string
}

func NewServicesPlugin(prefix string, defaultLogLines string) (*ServicesPlugin, error) {
	if prefix == "" {
		prefix = "linht-"
	}
	if defaultLogLines == "" {
		defaultLogLines = "100"
	}
	return &ServicesPlugin{
		prefix:          prefix,
		defaultLogLines: defaultLogLines,
	}, nil
}

func (p *ServicesPlugin) Name() string {
	return "services"
}

func (p *ServicesPlugin) Shutdown() error {
	return nil
}

func (p *ServicesPlugin) RegisterRoutes(app *fiber.App) {
	api := app.Group("/api/services")

	api.Get("/", p.listServices)
	api.Post("/:name/start", p.startService)
	api.Post("/:name/stop", p.stopService)
	api.Post("/:name/enable", p.enableService)
	api.Post("/:name/disable", p.disableService)
	api.Get("/:name/logs", p.streamLogs)
}

// validateServiceName ensures the service name is safe and has the correct prefix
func (p *ServicesPlugin) validateServiceName(name string) error {
	// Check for valid characters (alphanumeric, dash, underscore, @)
	validName := regexp.MustCompile(`^[a-zA-Z0-9_@-]+$`)
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid service name: contains invalid characters")
	}

	// Ensure the service has the required prefix
	if !strings.HasPrefix(name, p.prefix) {
		return fmt.Errorf("service must start with prefix '%s'", p.prefix)
	}

	return nil
}

// listServices returns all services matching the prefix
func (p *ServicesPlugin) listServices(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// List all units matching the prefix
	pattern := p.prefix + "*"
	cmd := exec.CommandContext(ctx, "systemctl", "list-units", "--type=service", "--all", "--no-legend", "--no-pager", pattern)
	output, err := cmd.Output()
	if err != nil {
		// If no services found, return empty list
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return SendSuccess(c, []ServiceInfo{}, "")
		}
		return SendError(c, 500, fmt.Errorf("failed to list services: %w", err))
	}

	services := []ServiceInfo{}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Parse the systemctl output
		// Format: UNIT LOAD ACTIVE SUB DESCRIPTION
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		unitName := fields[0]
		// Remove .service suffix for cleaner display
		serviceName := strings.TrimSuffix(unitName, ".service")

		// Get detailed info for this service
		info, err := p.getServiceInfo(ctx, serviceName)
		if err != nil {
			// Skip services we can't get info for
			continue
		}

		services = append(services, info)
	}

	return SendSuccess(c, services, "")
}

// getServiceInfo retrieves detailed information about a service
func (p *ServicesPlugin) getServiceInfo(ctx context.Context, name string) (ServiceInfo, error) {
	info := ServiceInfo{Name: name}

	// Get service properties
	cmd := exec.CommandContext(ctx, "systemctl", "show", "-p", "ActiveState,UnitFileState,Description", name+".service")
	output, err := cmd.Output()
	if err != nil {
		return info, err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "ActiveState":
			info.ActiveState = value
			info.IsActive = value == "active"
		case "UnitFileState":
			info.UnitState = value
			info.IsEnabled = value == "enabled"
		case "Description":
			info.Description = value
		}
	}

	return info, nil
}

// startService starts a systemd service
func (p *ServicesPlugin) startService(c *fiber.Ctx) error {
	name := c.Params("name")

	if err := p.validateServiceName(name); err != nil {
		return SendErrorMessage(c, 400, err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "systemctl", "start", name+".service")
	if output, err := cmd.CombinedOutput(); err != nil {
		return SendErrorMessage(c, 500, fmt.Sprintf("failed to start service: %s", string(output)))
	}

	return SendSuccess(c, nil, "Service started")
}

// stopService stops a systemd service
func (p *ServicesPlugin) stopService(c *fiber.Ctx) error {
	name := c.Params("name")

	if err := p.validateServiceName(name); err != nil {
		return SendErrorMessage(c, 400, err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "systemctl", "stop", name+".service")
	if output, err := cmd.CombinedOutput(); err != nil {
		return SendErrorMessage(c, 500, fmt.Sprintf("failed to stop service: %s", string(output)))
	}

	return SendSuccess(c, nil, "Service stopped")
}

// enableService enables a systemd service to start at boot
func (p *ServicesPlugin) enableService(c *fiber.Ctx) error {
	name := c.Params("name")

	if err := p.validateServiceName(name); err != nil {
		return SendErrorMessage(c, 400, err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "systemctl", "enable", name+".service")
	if output, err := cmd.CombinedOutput(); err != nil {
		return SendErrorMessage(c, 500, fmt.Sprintf("failed to enable service: %s", string(output)))
	}

	return SendSuccess(c, nil, "Service enabled")
}

// disableService disables a systemd service from starting at boot
func (p *ServicesPlugin) disableService(c *fiber.Ctx) error {
	name := c.Params("name")

	if err := p.validateServiceName(name); err != nil {
		return SendErrorMessage(c, 400, err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "systemctl", "disable", name+".service")
	if output, err := cmd.CombinedOutput(); err != nil {
		return SendErrorMessage(c, 500, fmt.Sprintf("failed to disable service: %s", string(output)))
	}

	return SendSuccess(c, nil, "Service disabled")
}

// streamLogs streams service logs via SSE
func (p *ServicesPlugin) streamLogs(c *fiber.Ctx) error {
	name := c.Params("name")

	if err := p.validateServiceName(name); err != nil {
		return SendErrorMessage(c, 400, err.Error())
	}

	// Set SSE headers
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	// Create a context that will be cancelled when the client disconnects
	ctx := c.Context()

	// Start journalctl with follow mode
	cmd := exec.Command("journalctl", "-u", name+".service", "-f", "-n", p.defaultLogLines, "--no-pager", "-o", "short-iso")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return SendError(c, 500, fmt.Errorf("failed to create pipe: %w", err))
	}

	if err := cmd.Start(); err != nil {
		return SendError(c, 500, fmt.Errorf("failed to start journalctl: %w", err))
	}

	// Stream logs
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer cmd.Process.Kill()
		defer stdout.Close()

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			// Check if client disconnected
			if ctx.Err() != nil {
				return
			}

			line := scanner.Text()
			fmt.Fprintf(w, "data: %s\n\n", line)
			w.Flush()
		}
	})

	return nil
}

// Register the plugin
func init() {
	Register("services", func(config interface{}) (Plugin, error) {
		prefix := "linht-"
		defaultLogLines := "100"

		if cfg, ok := config.(map[string]interface{}); ok {
			if p, ok := cfg["prefix"].(string); ok && p != "" {
				prefix = p
			}
			if lines, ok := cfg["default_log_lines"].(string); ok && lines != "" {
				defaultLogLines = lines
			}
		}
		return NewServicesPlugin(prefix, defaultLogLines)
	})
}