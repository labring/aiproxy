package convert

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
)

// Parser represents an OpenAPI parser
type Parser struct {
	doc *openapi3.T
}

// NewParser creates a new OpenAPI parser
func NewParser() *Parser {
	return &Parser{}
}

func getFronHTTP(u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// ParseFile parses an OpenAPI document from a file
func (p *Parser) ParseFile(filePath string) error {
	if strings.HasPrefix(filePath, "http://") || strings.HasPrefix(filePath, "https://") {
		data, err := getFronHTTP(filePath)
		if err != nil {
			return err
		}
		return p.Parse(data)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI file: %w", err)
	}

	return p.Parse(data)
}

func (p *Parser) ParseFileV2(filePath string) error {
	if strings.HasPrefix(filePath, "http://") || strings.HasPrefix(filePath, "https://") {
		data, err := getFronHTTP(filePath)
		if err != nil {
			return err
		}
		return p.Parse(data)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI file: %w", err)
	}

	return p.ParseV2(data)
}

// Parse parses an OpenAPI document from bytes
func (p *Parser) Parse(data []byte) error {
	loader := openapi3.NewLoader()

	// Parse the document (loader can handle both JSON and YAML)
	doc, err := loader.LoadFromData(data)
	if err != nil {
		return fmt.Errorf("failed to parse OpenAPI document: %w", err)
	}

	p.doc = doc
	return nil
}

func (p *Parser) ParseV2(data []byte) error {
	var doc2 openapi2.T
	err := doc2.UnmarshalJSON(data)
	if err != nil {
		return fmt.Errorf("failed to parse OpenAPI document: %w", err)
	}

	doc3, err := openapi2conv.ToV3(&doc2)
	if err != nil {
		return fmt.Errorf("failed to convert OpenAPI document: %w", err)
	}

	p.doc = doc3
	return nil
}

// GetDocument returns the parsed OpenAPI document
func (p *Parser) GetDocument() *openapi3.T {
	return p.doc
}

// GetPaths returns all paths in the OpenAPI document
func (p *Parser) GetPaths() *openapi3.Paths {
	if p.doc == nil {
		return nil
	}
	return p.doc.Paths
}

// GetServers returns all servers in the OpenAPI document
func (p *Parser) GetServers() []*openapi3.Server {
	if p.doc == nil {
		return nil
	}
	return p.doc.Servers
}

// GetInfo returns the info section of the OpenAPI document
func (p *Parser) GetInfo() *openapi3.Info {
	if p.doc == nil {
		return nil
	}
	return p.doc.Info
}

// GetOperationID generates an operation ID if one is not provided
func (p *Parser) GetOperationID(path, method string, operation *openapi3.Operation) string {
	if operation.OperationID != "" {
		return operation.OperationID
	}

	// Generate an operation ID based on the path and method
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	var pathName string
	if len(pathParts) > 0 {
		pathName = strings.Join(pathParts, "_")
		pathName = strings.ReplaceAll(pathName, "{", "")
		pathName = strings.ReplaceAll(pathName, "}", "")
	} else {
		pathName = "root"
	}

	return fmt.Sprintf("%s_%s", strings.ToLower(method), pathName)
}
