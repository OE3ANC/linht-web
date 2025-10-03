package plugins

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/gofiber/fiber/v2"
)

// Docker operation constants
const (
	ContainerStopTimeout = 10    // seconds
	DefaultLogLines      = "100" // default number of log lines
)

type DockerPlugin struct {
	client         *client.Client
	tokenValidator TokenValidator
}

func NewDockerPlugin(cli *client.Client) (*DockerPlugin, error) {
	if cli == nil {
		return nil, fmt.Errorf("docker client cannot be nil")
	}
	return &DockerPlugin{client: cli}, nil
}

// SetTokenValidator sets the token validation function
func (p *DockerPlugin) SetTokenValidator(validator TokenValidator) {
	p.tokenValidator = validator
}

// Shutdown implements the Plugin interface
// Note: Docker client is shared, so we don't close it here
func (p *DockerPlugin) Shutdown() error {
	return nil
}

func (p *DockerPlugin) Name() string {
	return "docker"
}

func (p *DockerPlugin) RegisterRoutes(app *fiber.App) {
	api := app.Group("/api")

	// Images
	api.Get("/images", p.listImages)
	api.Post("/images/import", p.importImage)
	api.Get("/images/:id/export", p.exportImage)
	api.Delete("/images/:id", p.deleteImage)

	// Containers
	api.Get("/containers", p.listContainers)
	api.Post("/containers", p.createContainer)
	api.Post("/containers/:id/start", p.startContainer)
	api.Post("/containers/:id/stop", p.stopContainer)
	api.Delete("/containers/:id", p.deleteContainer)
	api.Get("/containers/:id/logs", p.streamLogs)
}

// Image handlers

func (p *DockerPlugin) listImages(c *fiber.Ctx) error {
	ctx := context.Background()
	images, err := p.client.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return SendError(c, 500, err)
	}

	result := make([]fiber.Map, len(images))
	for i, img := range images {
		tags := img.RepoTags
		if len(tags) == 0 {
			tags = []string{"<none>"}
		}

		result[i] = fiber.Map{
			"id":      img.ID,
			"tags":    tags,
			"size":    img.Size,
			"created": time.Unix(img.Created, 0).Format(time.RFC3339),
		}
	}

	return SendSuccess(c, result, "")
}

func (p *DockerPlugin) importImage(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return SendErrorMessage(c, 400, "No file provided")
	}

	// Validate file type (basic check on extension)
	filename := file.Filename
	if len(filename) > 0 {
		validExtensions := []string{".tar", ".tar.gz", ".tgz"}
		isValid := false
		for _, ext := range validExtensions {
			if len(filename) >= len(ext) && filename[len(filename)-len(ext):] == ext {
				isValid = true
				break
			}
		}
		if !isValid {
			return SendErrorMessage(c, 400, "Invalid file type. Only .tar, .tar.gz, or .tgz files are accepted")
		}
	}

	src, err := file.Open()
	if err != nil {
		return SendErrorMessage(c, 500, "Failed to open file")
	}
	defer src.Close()

	ctx := context.Background()
	resp, err := p.client.ImageLoad(ctx, src, true)
	if err != nil {
		return SendError(c, 500, err)
	}
	defer resp.Body.Close()

	// Read response to ensure completion
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		return SendErrorMessage(c, 500, fmt.Sprintf("Failed to process response: %v", err))
	}

	return SendSuccess(c, nil, "Image imported successfully")
}

func (p *DockerPlugin) exportImage(c *fiber.Ctx) error {
	imageID := c.Params("id")
	ctx := context.Background()

	reader, err := p.client.ImageSave(ctx, []string{imageID})
	if err != nil {
		slog.Error("Failed to export image", "imageID", imageID[:12], "error", err)
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	c.Set("Content-Type", "application/x-tar")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.tar", imageID[:12]))

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer reader.Close()

		buf := make([]byte, 32*1024) // 32KB buffer
		for {
			n, readErr := reader.Read(buf)
			if n > 0 {
				if _, writeErr := w.Write(buf[:n]); writeErr != nil {
					return
				}
				w.Flush()
			}

			if readErr != nil {
				if readErr == io.EOF {
					w.Flush()
				}
				return
			}
		}
	})

	return nil
}

func (p *DockerPlugin) deleteImage(c *fiber.Ctx) error {
	imageID := c.Params("id")
	ctx := context.Background()

	_, err := p.client.ImageRemove(ctx, imageID, image.RemoveOptions{
		Force:         true,
		PruneChildren: true,
	})
	if err != nil {
		return SendError(c, 500, err)
	}

	return SendSuccess(c, nil, "Image deleted")
}

// Container handlers

func (p *DockerPlugin) listContainers(c *fiber.Ctx) error {
	ctx := context.Background()
	containers, err := p.client.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return SendError(c, 500, err)
	}

	result := make([]fiber.Map, len(containers))
	for i, cont := range containers {
		result[i] = fiber.Map{
			"id":      cont.ID,
			"names":   cont.Names,
			"image":   cont.Image,
			"state":   cont.State,
			"status":  cont.Status,
			"created": time.Unix(cont.Created, 0).Format(time.RFC3339),
		}
	}

	return SendSuccess(c, result, "")
}

func (p *DockerPlugin) createContainer(c *fiber.Ctx) error {
	var req struct {
		Image string   `json:"image"`
		Name  string   `json:"name"`
		Env   []string `json:"env"`
		Cmd   []string `json:"cmd"`
	}

	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	// Validate required fields
	if req.Image == "" {
		return SendErrorMessage(c, 400, "Image is required")
	}

	// Validate image format (basic check)
	if len(req.Image) > 255 {
		return SendErrorMessage(c, 400, "Image name too long")
	}

	ctx := context.Background()

	// Create container config
	config := &container.Config{
		Image: req.Image,
		Env:   req.Env,
		Cmd:   req.Cmd,
	}

	// Create container
	resp, err := p.client.ContainerCreate(ctx, config, nil, nil, nil, req.Name)
	if err != nil {
		return SendError(c, 500, err)
	}

	return SendSuccess(c, fiber.Map{
		"id":       resp.ID,
		"warnings": resp.Warnings,
	}, "")
}

func (p *DockerPlugin) startContainer(c *fiber.Ctx) error {
	containerID := c.Params("id")
	ctx := context.Background()

	if err := p.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return SendError(c, 500, err)
	}

	return SendSuccess(c, nil, "Container started")
}

func (p *DockerPlugin) stopContainer(c *fiber.Ctx) error {
	containerID := c.Params("id")
	ctx := context.Background()

	timeout := ContainerStopTimeout
	if err := p.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return SendError(c, 500, err)
	}

	return SendSuccess(c, nil, "Container stopped")
}

func (p *DockerPlugin) deleteContainer(c *fiber.Ctx) error {
	containerID := c.Params("id")
	ctx := context.Background()

	if err := p.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return SendError(c, 500, err)
	}

	return SendSuccess(c, nil, "Container deleted")
}

func (p *DockerPlugin) streamLogs(c *fiber.Ctx) error {
	containerID := c.Params("id")

	// Validate token from query parameter (EventSource can't use headers)
	token := c.Query("token")
	if p.tokenValidator != nil && !p.tokenValidator(token) {
		return c.Status(401).JSON(APIResponse{
			Success: false,
			Error:   "Unauthorized",
		})
	}

	ctx := context.Background()

	// Set SSE headers
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	// Get container logs
	logs, err := p.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       DefaultLogLines,
	})
	if err != nil {
		return c.Status(500).JSON(APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}
	defer logs.Close()

	// Stream logs
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		scanner := bufio.NewScanner(logs)
		for scanner.Scan() {
			line := scanner.Text()
			// Remove Docker log header (8 bytes)
			if len(line) > 8 {
				line = line[8:]
			}
			fmt.Fprintf(w, "data: %s\n\n", line)
			w.Flush()
		}
	})

	return nil
}

// Register the plugin
func init() {
	Register("docker", func(config interface{}) (Plugin, error) {
		cli, ok := config.(*client.Client)
		if !ok {
			return nil, fmt.Errorf("invalid config for docker plugin: expected *client.Client")
		}
		return NewDockerPlugin(cli)
	})
}
