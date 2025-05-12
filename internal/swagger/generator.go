package swagger

import (
	"api-gateway/internal/config"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// OpenAPISpec represents the OpenAPI 3.0 specification
type OpenAPISpec struct {
	OpenAPI    string               `yaml:"openapi" json:"openapi"`
	Info       Info                 `yaml:"info" json:"info"`
	Servers    []Server             `yaml:"servers" json:"servers"`
	Paths      map[string]*PathItem `yaml:"paths" json:"paths"`
	Components *Components          `yaml:"components" json:"components"`
}

type Info struct {
	Title       string `yaml:"title" json:"title"`
	Description string `yaml:"description" json:"description"`
	Version     string `yaml:"version" json:"version"`
}

type Server struct {
	URL string `yaml:"url" json:"url"`
}

type PathItem struct {
	Get     *Operation `yaml:"get,omitempty" json:"get,omitempty"`
	Post    *Operation `yaml:"post,omitempty" json:"post,omitempty"`
	Put     *Operation `yaml:"put,omitempty" json:"put,omitempty"`
	Delete  *Operation `yaml:"delete,omitempty" json:"delete,omitempty"`
	Options *Operation `yaml:"options,omitempty" json:"options,omitempty"`
}

type Operation struct {
	Summary     string                `yaml:"summary" json:"summary"`
	Description string                `yaml:"description,omitempty" json:"description,omitempty"`
	Security    []map[string][]string `yaml:"security,omitempty" json:"security,omitempty"`
	Responses   map[string]*Response  `yaml:"responses" json:"responses"`
}

type Response struct {
	Description string                `yaml:"description" json:"description"`
	Content     map[string]*MediaType `yaml:"content,omitempty" json:"content,omitempty"`
}

type MediaType struct {
	Schema *Schema `yaml:"schema" json:"schema"`
}

type Schema struct {
	Type       string             `yaml:"type" json:"type"`
	Properties map[string]*Schema `yaml:"properties,omitempty" json:"properties,omitempty"`
}

type Components struct {
	SecuritySchemes map[string]*SecurityScheme `yaml:"securitySchemes" json:"securitySchemes"`
}

type SecurityScheme struct {
	Type         string `yaml:"type" json:"type"`
	In           string `yaml:"in,omitempty" json:"in,omitempty"`
	Name         string `yaml:"name,omitempty" json:"name,omitempty"`
	Scheme       string `yaml:"scheme,omitempty" json:"scheme,omitempty"`
	BearerFormat string `yaml:"bearerFormat,omitempty" json:"bearerFormat,omitempty"`
	Description  string `yaml:"description,omitempty" json:"description,omitempty"`
}

// GenerateSwagger generates OpenAPI specification from routes configuration
func GenerateSwagger(routes *config.RouteConfig) (*OpenAPISpec, error) {
	spec := &OpenAPISpec{
		OpenAPI: "3.0.3",
		Info: Info{
			Title:       "Oortfy API Gateway",
			Description: "API Gateway for Oortfy microservices",
			Version:     "1.0.0",
		},
		Servers: []Server{
			{URL: "/"},
		},
		Paths: make(map[string]*PathItem),
		Components: &Components{
			SecuritySchemes: map[string]*SecurityScheme{
				"BearerAuth": {
					Type:         "http",
					Scheme:       "bearer",
					BearerFormat: "JWT",
				},
				"ApiKeyAuth": {
					Type: "apiKey",
					In:   "header",
					Name: "x-api-key",
				},
				"QueryTokenAuth": {
					Type:        "apiKey",
					In:          "query",
					Name:        "token",
					Description: "JWT token in query parameter (fallback authentication)",
				},
				"QueryApiKeyAuth": {
					Type:        "apiKey",
					In:          "query",
					Name:        "api_key",
					Description: "API key in query parameter (fallback authentication)",
				},
			},
		},
	}

	// Convert routes to OpenAPI paths
	for _, route := range routes.Routes {
		pathItem := &PathItem{}

		// Handle wildcard paths
		path := route.Path
		if strings.HasSuffix(path, "/*") {
			path = strings.TrimSuffix(path, "/*")
			path += "/{path}"
		}

		// Create operations for each method
		for _, method := range route.Methods {
			operation := &Operation{
				Summary: fmt.Sprintf("Proxy to %s", strings.TrimPrefix(route.Upstream, "http://")),
				Responses: map[string]*Response{
					"200": {
						Description: "Success",
					},
				},
			}

			// Add security requirements if authentication is required
			if route.Middlewares.RequireAuth {
				operation.Security = []map[string][]string{
					{"BearerAuth": {}},
					{"ApiKeyAuth": {}},
					{"QueryTokenAuth": {}},
					{"QueryApiKeyAuth": {}},
				}
			}

			// Add operation to path item based on method
			switch strings.ToUpper(method) {
			case "GET":
				pathItem.Get = operation
			case "POST":
				pathItem.Post = operation
			case "PUT":
				pathItem.Put = operation
			case "DELETE":
				pathItem.Delete = operation
			case "OPTIONS":
				pathItem.Options = operation
			}
		}

		spec.Paths[path] = pathItem
	}

	return spec, nil
}

// WriteSwaggerFile generates and writes the Swagger specification to a file
func WriteSwaggerFile(routes *config.RouteConfig, outputPath string) error {
	spec, err := GenerateSwagger(routes)
	if err != nil {
		return fmt.Errorf("failed to generate swagger spec: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("failed to marshal swagger spec: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write swagger file: %w", err)
	}

	return nil
}
