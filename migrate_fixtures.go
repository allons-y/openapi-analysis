// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func convertSwagger2ToOpenAPI3(data map[string]interface{}) map[string]interface{} {
	// Check if already OpenAPI 3
	if _, ok := data["openapi"]; ok {
		return data
	}

	// Check if Swagger 2.0
	if swagger, ok := data["swagger"].(string); !ok || swagger != "2.0" {
		return data
	}

	// Create new OpenAPI 3 structure
	result := make(map[string]interface{})
	result["openapi"] = "3.0.0"

	// Copy basic fields
	for _, field := range []string{"info", "externalDocs", "tags", "security", "paths"} {
		if val, ok := data[field]; ok {
			result[field] = val
		}
	}

	// Create components object
	components := make(map[string]interface{})

	if defs, ok := data["definitions"]; ok {
		components["schemas"] = defs
	}

	if params, ok := data["parameters"]; ok {
		components["parameters"] = params
	}

	if responses, ok := data["responses"]; ok {
		components["responses"] = responses
	}

	if secDefs, ok := data["securityDefinitions"]; ok {
		components["securitySchemes"] = secDefs
	}

	if len(components) > 0 {
		result["components"] = components
	}

	// Convert servers from host/basePath/schemes
	if host, hasHost := data["host"]; hasHost {
		servers := make([]map[string]interface{}, 0)
		schemes := []string{"http"}
		if s, ok := data["schemes"].([]interface{}); ok {
			schemes = nil
			for _, scheme := range s {
				if str, ok := scheme.(string); ok {
					schemes = append(schemes, str)
				}
			}
		}

		basePath := ""
		if bp, ok := data["basePath"].(string); ok {
			basePath = bp
		}

		for _, scheme := range schemes {
			server := map[string]interface{}{
				"url": fmt.Sprintf("%s://%s%s", scheme, host, basePath),
			}
			servers = append(servers, server)
		}

		if len(servers) > 0 {
			result["servers"] = servers
		}
	}

	// Copy extension fields
	for key, val := range data {
		if strings.HasPrefix(key, "x-") {
			result[key] = val
		}
	}

	return result
}

func processFile(path string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	var content map[string]interface{}

	// Try to determine format and parse
	ext := strings.ToLower(filepath.Ext(path))
	isYAML := ext == ".yaml" || ext == ".yml"
	isJSON := ext == ".json"

	if isYAML {
		if err := yaml.Unmarshal(data, &content); err != nil {
			return fmt.Errorf("yaml unmarshal: %w", err)
		}
	} else if isJSON {
		if err := json.Unmarshal(data, &content); err != nil {
			return fmt.Errorf("json unmarshal: %w", err)
		}
	} else {
		return nil // Skip unknown file types
	}

	// Check if it's a Swagger spec (not all files are specs)
	if _, hasSwagger := content["swagger"]; !hasSwagger {
		if _, hasOpenAPI := content["openapi"]; !hasOpenAPI {
			return nil // Not a spec file, skip
		}
	}

	// Convert the spec
	converted := convertSwagger2ToOpenAPI3(content)

	// Check if anything changed
	if fmt.Sprintf("%v", content) == fmt.Sprintf("%v", converted) {
		return nil
	}

	// Write back
	var output []byte
	if isYAML {
		output, err = yaml.Marshal(converted)
		if err != nil {
			return fmt.Errorf("yaml marshal: %w", err)
		}
	} else {
		output, err = json.MarshalIndent(converted, "", "  ")
		if err != nil {
			return fmt.Errorf("json marshal: %w", err)
		}
		output = append(output, '\n')
	}

	if err := ioutil.WriteFile(path, output, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	fmt.Printf("✓ Converted %s\n", path)
	return nil
}

func main() {
	fixturesDir := "fixtures"

	if len(os.Args) > 1 {
		fixturesDir = os.Args[1]
	}

	count := 0
	err := filepath.Walk(fixturesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" || ext == ".json" {
			if err := processFile(path); err != nil {
				log.Printf("Error processing %s: %v", path, err)
			} else {
				count++
			}
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nProcessed %d files\n", count)
}
