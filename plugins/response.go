package plugins

import "github.com/gofiber/fiber/v2"

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// SendSuccess sends a successful response
func SendSuccess(c *fiber.Ctx, data interface{}, message string) error {
	return c.JSON(APIResponse{
		Success: true,
		Data:    data,
		Message: message,
	})
}

// SendError sends an error response
func SendError(c *fiber.Ctx, status int, err error) error {
	return c.Status(status).JSON(APIResponse{
		Success: false,
		Error:   err.Error(),
	})
}

// SendErrorMessage sends an error response with a custom message
func SendErrorMessage(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(APIResponse{
		Success: false,
		Error:   message,
	})
}