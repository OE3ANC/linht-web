package plugins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
	"gopkg.in/yaml.v3"
)


// OrderedMap represents a map that preserves insertion order
// It implements json.Marshaler to output keys in order
type OrderedMap struct {
	Keys   []string
	Values map[string]interface{}
}

// MarshalJSON implements json.Marshaler for OrderedMap
func (om *OrderedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{")
	for i, key := range om.Keys {
		if i > 0 {
			buf.WriteString(",")
		}
		// Marshal the key
		keyBytes, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		buf.Write(keyBytes)
		buf.WriteString(":")
		// Marshal the value
		valBytes, err := json.Marshal(om.Values[key])
		if err != nil {
			return nil, err
		}
		buf.Write(valBytes)
	}
	buf.WriteString("}")
	return buf.Bytes(), nil
}

// yamlNodeToOrderedJSON converts a yaml.Node to an ordered JSON-compatible structure
func yamlNodeToOrderedJSON(node *yaml.Node) interface{} {
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) > 0 {
			return yamlNodeToOrderedJSON(node.Content[0])
		}
		return nil

	case yaml.MappingNode:
		om := &OrderedMap{
			Keys:   make([]string, 0, len(node.Content)/2),
			Values: make(map[string]interface{}),
		}
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]
			key := keyNode.Value
			om.Keys = append(om.Keys, key)
			om.Values[key] = yamlNodeToOrderedJSON(valueNode)
		}
		return om

	case yaml.SequenceNode:
		result := make([]interface{}, len(node.Content))
		for i, item := range node.Content {
			result[i] = yamlNodeToOrderedJSON(item)
		}
		return result

	case yaml.ScalarNode:
		// Parse scalar values based on their tag
		switch node.Tag {
		case "!!null":
			return nil
		case "!!bool":
			return node.Value == "true"
		case "!!int":
			var v int64
			if err := node.Decode(&v); err == nil {
				return v
			}
			return node.Value
		case "!!float":
			var v float64
			if err := node.Decode(&v); err == nil {
				return v
			}
			return node.Value
		default:
			// For strings and other types
			return node.Value
		}

	case yaml.AliasNode:
		if node.Alias != nil {
			return yamlNodeToOrderedJSON(node.Alias)
		}
		return nil

	default:
		return node.Value
	}
}

// updateYAMLNodeWithValues updates a yaml.Node tree with values from a map while preserving structure
func updateYAMLNodeWithValues(node *yaml.Node, values map[string]interface{}) {
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) > 0 {
			updateYAMLNodeWithValues(node.Content[0], values)
		}

	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]
			key := keyNode.Value

			if newValue, exists := values[key]; exists {
				// Update the value node based on the new value type
				switch v := newValue.(type) {
				case map[string]interface{}:
					// Recursively update nested maps
					if valueNode.Kind == yaml.MappingNode {
						updateYAMLNodeWithValues(valueNode, v)
					}
				case []interface{}:
					// Handle arrays - rebuild the sequence
					valueNode.Kind = yaml.SequenceNode
					valueNode.Content = nil
					for _, item := range v {
						valueNode.Content = append(valueNode.Content, createYAMLNode(item))
					}
				default:
					// Update scalar value
					updateScalarNode(valueNode, v)
				}
			}
		}
	}
}

// createYAMLNode creates a yaml.Node from an interface value
func createYAMLNode(value interface{}) *yaml.Node {
	switch v := value.(type) {
	case map[string]interface{}:
		node := &yaml.Node{Kind: yaml.MappingNode}
		for key, val := range v {
			keyNode := &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: key,
				Tag:   "!!str",
			}
			node.Content = append(node.Content, keyNode, createYAMLNode(val))
		}
		return node

	case []interface{}:
		node := &yaml.Node{Kind: yaml.SequenceNode}
		for _, item := range v {
			node.Content = append(node.Content, createYAMLNode(item))
		}
		return node

	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: v, Tag: "!!str"}

	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%t", v), Tag: "!!bool"}

	case float64:
		if v == float64(int64(v)) {
			return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d", int64(v)), Tag: "!!int"}
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%g", v), Tag: "!!float"}

	case int64:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d", v), Tag: "!!int"}

	case int:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d", v), Tag: "!!int"}

	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: "null", Tag: "!!null"}

	default:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", v)}
	}
}

// updateScalarNode updates a scalar node with a new value
func updateScalarNode(node *yaml.Node, value interface{}) {
	switch v := value.(type) {
	case string:
		node.Value = v
		node.Tag = "!!str"
	case bool:
		node.Value = fmt.Sprintf("%t", v)
		node.Tag = "!!bool"
	case float64:
		if v == float64(int64(v)) {
			node.Value = fmt.Sprintf("%d", int64(v))
			node.Tag = "!!int"
		} else {
			node.Value = fmt.Sprintf("%g", v)
			node.Tag = "!!float"
		}
	case int64:
		node.Value = fmt.Sprintf("%d", v)
		node.Tag = "!!int"
	case int:
		node.Value = fmt.Sprintf("%d", v)
		node.Tag = "!!int"
	case nil:
		node.Value = "null"
		node.Tag = "!!null"
	default:
		node.Value = fmt.Sprintf("%v", v)
	}
}

// CPSPlugin provides Customer Programming Software functionality for editing settings
type CPSPlugin struct {
	settingsPath string
}

// NewCPSPlugin creates a new CPS plugin instance
func NewCPSPlugin(settingsPath string) (*CPSPlugin, error) {
	if settingsPath == "" {
		return nil, fmt.Errorf("settings_path is required in cps plugin configuration")
	}

	return &CPSPlugin{
		settingsPath: settingsPath,
	}, nil
}

// Name returns the plugin identifier
func (p *CPSPlugin) Name() string {
	return "cps"
}

// RegisterRoutes adds the plugin's HTTP routes
func (p *CPSPlugin) RegisterRoutes(app *fiber.App) {
	api := app.Group("/api/cps")

	api.Get("/load", p.loadSettings)
	api.Post("/save", p.saveSettings)
}

// Shutdown performs cleanup
func (p *CPSPlugin) Shutdown() error {
	return nil
}

// loadSettings handles GET /api/cps/load
func (p *CPSPlugin) loadSettings(c *fiber.Ctx) error {
	// Read the settings file
	data, err := os.ReadFile(p.settingsPath)
	if err != nil {
		return SendError(c, 500, fmt.Errorf("failed to read settings file: %w", err))
	}

	// Parse YAML into yaml.Node to preserve key order
	var rootNode yaml.Node
	if err := yaml.Unmarshal(data, &rootNode); err != nil {
		return SendError(c, 500, fmt.Errorf("failed to parse settings file: %w", err))
	}

	// Convert to ordered JSON structure
	orderedData := yamlNodeToOrderedJSON(&rootNode)

	return SendSuccess(c, orderedData, "Settings loaded successfully")
}

// saveSettings handles POST /api/cps/save
func (p *CPSPlugin) saveSettings(c *fiber.Ctx) error {
	// Parse the request body into a generic structure
	var newSettings map[string]interface{}
	if err := c.BodyParser(&newSettings); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	// Read the original YAML file to preserve structure and key order
	originalData, err := os.ReadFile(p.settingsPath)
	if err != nil {
		return SendError(c, 500, fmt.Errorf("failed to read original settings file: %w", err))
	}

	// Parse original YAML into yaml.Node to preserve structure
	var rootNode yaml.Node
	if err := yaml.Unmarshal(originalData, &rootNode); err != nil {
		return SendError(c, 500, fmt.Errorf("failed to parse original settings file: %w", err))
	}

	// Update the yaml.Node tree with new values while preserving structure
	updateYAMLNodeWithValues(&rootNode, newSettings)

	// Marshal back to YAML
	data, err := yaml.Marshal(&rootNode)
	if err != nil {
		return SendError(c, 500, fmt.Errorf("failed to serialize settings: %w", err))
	}

	// Write to file
	if err := os.WriteFile(p.settingsPath, data, 0644); err != nil {
		return SendError(c, 500, fmt.Errorf("failed to write settings file: %w", err))
	}

	return SendSuccess(c, nil, "Settings saved successfully")
}

// Register the plugin
func init() {
	Register("cps", func(config interface{}) (Plugin, error) {
		var settingsPath string

		if configMap, ok := config.(map[string]interface{}); ok {
			if path, ok := configMap["settings_path"].(string); ok && path != "" {
				settingsPath = path
			}
		}

		return NewCPSPlugin(settingsPath)
	})
}
