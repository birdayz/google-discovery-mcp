// Package discovery parses Google API Discovery Documents and generates MCP tool code.
package discovery

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Document represents a Google API Discovery Document.
// See: https://developers.google.com/discovery/v1/reference/apis
type Document struct {
	ID                string                `json:"id"`
	Name              string                `json:"name"`
	Version           string                `json:"version"`
	Title             string                `json:"title"`
	Description       string                `json:"description"`
	RootURL           string                `json:"rootUrl"`
	ServicePath       string                `json:"servicePath"`
	DocumentationLink string                `json:"documentationLink"`
	Schemas           map[string]*Schema    `json:"schemas"`
	Resources         map[string]*Resource  `json:"resources"`
	Methods           map[string]*Method    `json:"methods"`    // Top-level methods (rare)
	Parameters        map[string]*Parameter `json:"parameters"` // Common parameters
}

// Resource represents an API resource (e.g., "videos", "playlists").
type Resource struct {
	Methods   map[string]*Method   `json:"methods"`
	Resources map[string]*Resource `json:"resources"` // Nested resources
}

// Method represents an API method (e.g., "videos.list", "videos.insert").
type Method struct {
	ID                    string                `json:"id"`
	Path                  string                `json:"path"`
	HTTPMethod            string                `json:"httpMethod"`
	Description           string                `json:"description"`
	Parameters            map[string]*Parameter `json:"parameters"`
	ParameterOrder        []string              `json:"parameterOrder"`
	Request               *SchemaRef            `json:"request"`
	Response              *SchemaRef            `json:"response"`
	Scopes                []string              `json:"scopes"`
	MediaUpload           *MediaUpload          `json:"mediaUpload"`
	SupportsMediaDownload bool                  `json:"supportsMediaDownload"`
}

// Parameter represents a method parameter.
type Parameter struct {
	Type             string   `json:"type"`
	Description      string   `json:"description"`
	Required         bool     `json:"required"`
	Location         string   `json:"location"` // "path" or "query"
	Repeated         bool     `json:"repeated"`
	Default          string   `json:"default"`
	Enum             []string `json:"enum"`
	EnumDescriptions []string `json:"enumDescriptions"`
	Minimum          string   `json:"minimum"`
	Maximum          string   `json:"maximum"`
	Format           string   `json:"format"` // e.g., "int64", "uint64"
	Pattern          string   `json:"pattern"`
}

// Schema represents a JSON Schema in the Discovery Document.
type Schema struct {
	ID                   string             `json:"id"`
	Type                 string             `json:"type"`
	Format               string             `json:"format"`
	Description          string             `json:"description"`
	Properties           map[string]*Schema `json:"properties"`
	Items                *Schema            `json:"items"`                // For arrays
	AdditionalProperties *Schema            `json:"additionalProperties"` // For maps
	Ref                  string             `json:"$ref"`
	Default              string             `json:"default"`
	Enum                 []string           `json:"enum"`
	EnumDescriptions     []string           `json:"enumDescriptions"`
	Required             bool               `json:"required"` // When used as property
	ReadOnly             bool               `json:"readOnly"`
	Annotations          *Annotations       `json:"annotations"`
}

// Annotations contains metadata about schema fields.
type Annotations struct {
	Required []string `json:"required"`
}

// SchemaRef is a reference to a schema.
type SchemaRef struct {
	Ref string `json:"$ref"`
}

// MediaUpload describes media upload capabilities.
type MediaUpload struct {
	Accept    []string            `json:"accept"`
	MaxSize   string              `json:"maxSize"`
	Protocols map[string]Protocol `json:"protocols"`
}

// Protocol describes an upload protocol.
type Protocol struct {
	Multipart bool   `json:"multipart"`
	Path      string `json:"path"`
}

// Parse parses a Discovery Document from JSON bytes.
func Parse(data []byte) (*Document, error) {
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse discovery document: %w", err)
	}
	return &doc, nil
}

// AllMethods returns all methods from the document, flattened with full names.
// e.g., "videos.list", "playlists.insert", "channels.list"
func (d *Document) AllMethods() map[string]*Method {
	methods := make(map[string]*Method)

	// Top-level methods (rare)
	for name, m := range d.Methods {
		methods[name] = m
	}

	// Resource methods
	for resourceName, resource := range d.Resources {
		collectMethods(resourceName, resource, methods)
	}

	return methods
}

func collectMethods(prefix string, r *Resource, methods map[string]*Method) {
	for methodName, m := range r.Methods {
		fullName := prefix + "." + methodName
		methods[fullName] = m
	}
	for subName, subResource := range r.Resources {
		collectMethods(prefix+"."+subName, subResource, methods)
	}
}

// SortedMethodNames returns method names in sorted order.
func (d *Document) SortedMethodNames() []string {
	methods := d.AllMethods()
	names := make([]string, 0, len(methods))
	for name := range methods {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
