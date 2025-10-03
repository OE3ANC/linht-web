package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// File operation constants
const (
	DefaultMaxUploadSize = 1 * 1024 * 1024 * 1024 // 1GB
)

// FileManagerPlugin provides simple file management functionality
type FileManagerPlugin struct {
	tokenValidator TokenValidator
	maxUploadSize  int64
}

// FileItem represents a file or directory
type FileItem struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	IsDir    bool      `json:"isDir"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

// DirectoryListing represents the contents of a directory
type DirectoryListing struct {
	Path   string     `json:"path"`
	Parent string     `json:"parent"`
	Items  []FileItem `json:"items"`
}

// NewFileManagerPlugin creates a new FileManager plugin instance
func NewFileManagerPlugin(maxUploadSize int64) (*FileManagerPlugin, error) {
	if maxUploadSize <= 0 {
		maxUploadSize = DefaultMaxUploadSize
	}

	return &FileManagerPlugin{
		maxUploadSize: maxUploadSize,
	}, nil
}

// SetTokenValidator sets the token validation function
func (p *FileManagerPlugin) SetTokenValidator(validator TokenValidator) {
	p.tokenValidator = validator
}

// Name returns the plugin identifier
func (p *FileManagerPlugin) Name() string {
	return "filemanager"
}

// RegisterRoutes adds the plugin's HTTP routes
func (p *FileManagerPlugin) RegisterRoutes(app *fiber.App) {
	api := app.Group("/api/filemanager")

	api.Get("/list", p.listDirectory)
	api.Post("/upload", p.uploadFile)
	api.Get("/download", p.downloadFile)
	api.Delete("/delete", p.deleteItem)
	api.Post("/mkdir", p.createFolder)
}

// Shutdown performs cleanup
func (p *FileManagerPlugin) Shutdown() error {
	return nil
}

// sanitizePath validates and cleans the path to prevent directory traversal
func sanitizePath(path string) (string, error) {
	if path == "" {
		return "/", nil
	}

	// Clean the path
	clean := filepath.Clean(path)

	// Prevent directory traversal
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("invalid path: directory traversal not allowed")
	}

	// Convert to absolute path
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	return abs, nil
}

// listDirectory handles GET /api/filemanager/list?path=/path/to/dir
func (p *FileManagerPlugin) listDirectory(c *fiber.Ctx) error {
	pathParam := c.Query("path", "/")

	// Sanitize path
	dirPath, err := sanitizePath(pathParam)
	if err != nil {
		return SendErrorMessage(c, 400, err.Error())
	}

	// Check if path exists and is a directory
	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return SendErrorMessage(c, 404, "Directory not found")
		}
		return SendError(c, 500, err)
	}

	if !info.IsDir() {
		return SendErrorMessage(c, 400, "Path is not a directory")
	}

	// Read directory contents
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return SendError(c, 500, err)
	}

	// Build file items list
	items := make([]FileItem, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())
		items = append(items, FileItem{
			Name:     entry.Name(),
			Path:     fullPath,
			IsDir:    entry.IsDir(),
			Size:     info.Size(),
			Modified: info.ModTime(),
		})
	}

	// Get parent directory
	parent := filepath.Dir(dirPath)
	if parent == dirPath {
		parent = ""
	}

	listing := DirectoryListing{
		Path:   dirPath,
		Parent: parent,
		Items:  items,
	}

	return SendSuccess(c, listing, "")
}

// uploadFile handles POST /api/filemanager/upload
func (p *FileManagerPlugin) uploadFile(c *fiber.Ctx) error {
	// Get destination path
	destPath := c.FormValue("path")
	if destPath == "" {
		return SendErrorMessage(c, 400, "Destination path required")
	}

	// Sanitize path
	dirPath, err := sanitizePath(destPath)
	if err != nil {
		return SendErrorMessage(c, 400, err.Error())
	}

	// Check if destination is a directory
	info, err := os.Stat(dirPath)
	if err != nil {
		return SendErrorMessage(c, 400, "Destination path does not exist")
	}
	if !info.IsDir() {
		return SendErrorMessage(c, 400, "Destination path is not a directory")
	}

	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		return SendErrorMessage(c, 400, "No file provided")
	}

	// Check file size
	if file.Size > p.maxUploadSize {
		return SendErrorMessage(c, 413, fmt.Sprintf("File too large (max %d bytes)", p.maxUploadSize))
	}

	// Sanitize filename
	filename := filepath.Base(file.Filename)
	if filename == "" || filename == "." || filename == ".." {
		return SendErrorMessage(c, 400, "Invalid filename")
	}

	// Build destination file path
	destFile := filepath.Join(dirPath, filename)

	// Save file
	if err := c.SaveFile(file, destFile); err != nil {
		return SendError(c, 500, err)
	}

	return SendSuccess(c, nil, "File uploaded successfully")
}

// downloadFile handles GET /api/filemanager/download?path=/path/to/file
func (p *FileManagerPlugin) downloadFile(c *fiber.Ctx) error {
	pathParam := c.Query("path")
	if pathParam == "" {
		return SendErrorMessage(c, 400, "File path required")
	}

	// Sanitize path
	filePath, err := sanitizePath(pathParam)
	if err != nil {
		return SendErrorMessage(c, 400, err.Error())
	}

	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return SendErrorMessage(c, 404, "File not found")
		}
		return SendError(c, 500, err)
	}

	// Check if it's a file
	if info.IsDir() {
		return SendErrorMessage(c, 400, "Cannot download a directory")
	}

	// Set headers
	filename := filepath.Base(filePath)
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	// Send file
	return c.SendFile(filePath)
}

// deleteItem handles DELETE /api/filemanager/delete
func (p *FileManagerPlugin) deleteItem(c *fiber.Ctx) error {
	var req struct {
		Path string `json:"path"`
	}

	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	if req.Path == "" {
		return SendErrorMessage(c, 400, "Path required")
	}

	// Sanitize path
	itemPath, err := sanitizePath(req.Path)
	if err != nil {
		return SendErrorMessage(c, 400, err.Error())
	}

	// Prevent deleting root
	if itemPath == "/" {
		return SendErrorMessage(c, 400, "Cannot delete root directory")
	}

	// Check if path exists
	_, err = os.Stat(itemPath)
	if err != nil {
		if os.IsNotExist(err) {
			return SendErrorMessage(c, 404, "Item not found")
		}
		return SendError(c, 500, err)
	}

	// Delete file or directory
	if err := os.RemoveAll(itemPath); err != nil {
		return SendError(c, 500, err)
	}

	return SendSuccess(c, nil, "Deleted successfully")
}

// createFolder handles POST /api/filemanager/mkdir
func (p *FileManagerPlugin) createFolder(c *fiber.Ctx) error {
	var req struct {
		Path string `json:"path"`
	}

	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	if req.Path == "" {
		return SendErrorMessage(c, 400, "Path required")
	}

	// Sanitize path
	folderPath, err := sanitizePath(req.Path)
	if err != nil {
		return SendErrorMessage(c, 400, err.Error())
	}

	// Check if already exists
	if _, err := os.Stat(folderPath); err == nil {
		return SendErrorMessage(c, 400, "Path already exists")
	}

	// Create folder
	if err := os.MkdirAll(folderPath, 0755); err != nil {
		return SendError(c, 500, err)
	}

	return SendSuccess(c, nil, "Folder created successfully")
}

// Register the plugin
func init() {
	Register("filemanager", func(config interface{}) (Plugin, error) {
		configMap, ok := config.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid config for filemanager plugin: expected map[string]interface{}")
		}

		maxUploadSize, _ := configMap["max_upload_size"].(int64)

		return NewFileManagerPlugin(maxUploadSize)
	})
}