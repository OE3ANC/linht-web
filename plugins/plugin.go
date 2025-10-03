package plugins

import "github.com/gofiber/fiber/v2"

// Plugin interface that all plugins must implement
type Plugin interface {
	// Name returns the plugin identifier
	Name() string

	// RegisterRoutes adds the plugin's HTTP routes to the app
	RegisterRoutes(app *fiber.App)

	// Shutdown performs cleanup when the plugin is stopped
	Shutdown() error
}

// PluginFactory creates a new plugin instance
type PluginFactory func(config interface{}) (Plugin, error)

var registry = make(map[string]PluginFactory)

// Register adds a plugin factory to the registry
func Register(name string, factory PluginFactory) {
	registry[name] = factory
}

// Get retrieves a plugin factory by name
func Get(name string) (PluginFactory, bool) {
	factory, exists := registry[name]
	return factory, exists
}

// TokenValidator is a function type for validating authentication tokens
type TokenValidator func(token string) bool