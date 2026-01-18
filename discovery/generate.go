package discovery

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"
	"text/template"
	"unicode"
)

// GenerateOptions configures code generation.
type GenerateOptions struct {
	PackageName    string   // Go package name (default: "tools")
	Methods        []string // Specific methods to generate (empty = all)
	Prefix         string   // Tool name prefix (e.g., "youtube_")
	StructPrefix   string   // Struct name prefix (default: "API")
	GenerateSchema bool     // Generate schema types (request/response bodies)
}

// GenerateMCPTools generates Go code for MCP tools from a Discovery Document.
func GenerateMCPTools(doc *Document, opts GenerateOptions) (string, error) {
	if opts.PackageName == "" {
		opts.PackageName = "tools"
	}
	if opts.Prefix == "" {
		opts.Prefix = doc.Name + "_"
	}
	if opts.StructPrefix == "" {
		opts.StructPrefix = "API"
	}

	allMethods := doc.AllMethods()
	var methodsToGenerate []*MethodInfo

	// Filter methods if specified
	methodNames := opts.Methods
	if len(methodNames) == 0 {
		methodNames = doc.SortedMethodNames()
	}

	for _, name := range methodNames {
		m, ok := allMethods[name]
		if !ok {
			return "", fmt.Errorf("method not found: %s", name)
		}
		methodsToGenerate = append(methodsToGenerate, &MethodInfo{
			FullName:     name,
			Method:       m,
			Prefix:       opts.Prefix,
			StructPrefix: opts.StructPrefix,
		})
	}

	// Collect schemas needed by the methods
	var schemasToGen []*SchemaInfo
	if opts.GenerateSchema {
		schemasToGen = collectSchemas(methodsToGenerate, doc.Schemas)
	}

	data := &TemplateData{
		PackageName:    opts.PackageName,
		APIName:        doc.Name,
		APITitle:       doc.Title,
		APIVersion:     doc.Version,
		Methods:        methodsToGenerate,
		Schemas:        doc.Schemas,
		SchemasToGen:   schemasToGen,
		AllSchemas:     doc.Schemas,
		GenerateSchema: opts.GenerateSchema,
	}

	var buf bytes.Buffer
	if err := codeTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}

	// Format the generated code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Return unformatted code with error info for debugging
		return buf.String(), fmt.Errorf("generated code has syntax errors: %w", err)
	}

	return string(formatted), nil
}

// TemplateData is passed to the code generation template.
type TemplateData struct {
	PackageName    string
	APIName        string
	APITitle       string
	APIVersion     string
	Methods        []*MethodInfo
	Schemas        map[string]*Schema
	SchemasToGen   []*SchemaInfo // Schemas to generate, in dependency order
	AllSchemas     map[string]*Schema
	GenerateSchema bool // Whether to generate schema types
}

// MethodInfo wraps a Method with generation helpers.
type MethodInfo struct {
	FullName     string // e.g., "videos.list"
	Method       *Method
	Prefix       string // e.g., "youtube_"
	StructPrefix string // e.g., "API"
}

// ToolName returns the MCP tool name (e.g., "youtube_videos_list").
func (m *MethodInfo) ToolName() string {
	return m.Prefix + strings.ReplaceAll(m.FullName, ".", "_")
}

// StructName returns the Go struct name for args (e.g., "APIVideosListArgs").
func (m *MethodInfo) StructName() string {
	parts := strings.Split(m.FullName, ".")
	var result string
	for _, p := range parts {
		result += exportedName(p)
	}
	return m.StructPrefix + result + "Args"
}

// Description returns a cleaned description for the tool.
func (m *MethodInfo) Description() string {
	desc := cleanDescription(m.Method.Description)
	if len(desc) > 200 {
		desc = desc[:197] + "..."
	}
	return desc
}

// SortedParams returns parameters sorted by: required first, then alphabetically.
func (m *MethodInfo) SortedParams() []*ParamInfo {
	var params []*ParamInfo
	for name, p := range m.Method.Parameters {
		params = append(params, &ParamInfo{Name: name, Param: p})
	}
	sort.Slice(params, func(i, j int) bool {
		// Required params first
		if params[i].Param.Required != params[j].Param.Required {
			return params[i].Param.Required
		}
		// Then by parameter order if specified
		iOrder := indexOf(m.Method.ParameterOrder, params[i].Name)
		jOrder := indexOf(m.Method.ParameterOrder, params[j].Name)
		if iOrder != jOrder {
			if iOrder == -1 {
				return false
			}
			if jOrder == -1 {
				return true
			}
			return iOrder < jOrder
		}
		// Then alphabetically
		return params[i].Name < params[j].Name
	})
	return params
}

// ParamInfo wraps a Parameter with generation helpers.
type ParamInfo struct {
	Name  string
	Param *Parameter
}

// FieldName returns the Go field name (exported).
func (p *ParamInfo) FieldName() string {
	return exportedName(p.Name)
}

// JSONTag returns the json struct tag.
func (p *ParamInfo) JSONTag() string {
	if p.Param.Required {
		return p.Name
	}
	return p.Name + ",omitempty"
}

// GoType returns the Go type for this parameter.
func (p *ParamInfo) GoType() string {
	return paramGoType(p.Param)
}

// SchemaDescription returns the jsonschema description.
func (p *ParamInfo) SchemaDescription() string {
	desc := cleanDescription(p.Param.Description)

	// Add enum values to description if present
	if len(p.Param.Enum) > 0 {
		enumStr := strings.Join(p.Param.Enum, ", ")
		if desc != "" {
			desc += " "
		}
		desc += "Values: " + enumStr
	}

	// Add default if present
	if p.Param.Default != "" {
		desc += " (default: " + p.Param.Default + ")"
	}

	return desc
}

// SchemaInfo wraps a Schema with generation helpers.
type SchemaInfo struct {
	Name        string             // Schema name (e.g., "Video", "VideoStatus")
	Schema      *Schema            // The schema definition
	AllSchemas  map[string]*Schema // Reference to all schemas for resolving $ref
	RequiredSet map[string]bool    // Set of required property names
}

// NewSchemaInfo creates a SchemaInfo from a schema.
func NewSchemaInfo(name string, schema *Schema, allSchemas map[string]*Schema) *SchemaInfo {
	requiredSet := make(map[string]bool)
	if schema.Annotations != nil {
		for _, req := range schema.Annotations.Required {
			requiredSet[req] = true
		}
	}
	return &SchemaInfo{
		Name:        name,
		Schema:      schema,
		AllSchemas:  allSchemas,
		RequiredSet: requiredSet,
	}
}

// StructName returns the Go struct name for this schema.
func (s *SchemaInfo) StructName() string {
	return exportedName(s.Name)
}

// Description returns the schema description.
func (s *SchemaInfo) Description() string {
	return cleanDescription(s.Schema.Description)
}

// SortedProperties returns schema properties sorted by: required first, then alphabetically.
func (s *SchemaInfo) SortedProperties() []*PropertyInfo {
	var props []*PropertyInfo
	for name, prop := range s.Schema.Properties {
		required := s.RequiredSet[name] || prop.Required
		props = append(props, &PropertyInfo{
			Name:       name,
			Property:   prop,
			Required:   required,
			AllSchemas: s.AllSchemas,
		})
	}
	sort.Slice(props, func(i, j int) bool {
		if props[i].Required != props[j].Required {
			return props[i].Required
		}
		return props[i].Name < props[j].Name
	})
	return props
}

// PropertyInfo wraps a schema property with generation helpers.
type PropertyInfo struct {
	Name       string
	Property   *Schema
	Required   bool
	AllSchemas map[string]*Schema
}

// FieldName returns the Go field name (exported).
func (p *PropertyInfo) FieldName() string {
	return exportedName(p.Name)
}

// JSONTag returns the json struct tag.
func (p *PropertyInfo) JSONTag() string {
	if p.Required {
		return p.Name
	}
	return p.Name + ",omitempty"
}

// GoType returns the Go type for this property.
func (p *PropertyInfo) GoType() string {
	return p.resolveType(p.Property, !p.Required)
}

// resolveType resolves the Go type for a schema, handling refs, arrays, objects, etc.
func (p *PropertyInfo) resolveType(schema *Schema, optional bool) string {
	// Handle $ref
	if schema.Ref != "" {
		// Reference to another schema - use its exported name
		refType := exportedName(schema.Ref)
		// Check if the referenced schema is a simple type (wrapper)
		if refSchema, ok := p.AllSchemas[schema.Ref]; ok {
			if refSchema.Type != "" && refSchema.Type != "object" && refSchema.Type != "array" {
				return scalarGoType(refSchema.Type, refSchema.Format, optional)
			}
		}
		return "*" + refType
	}

	switch schema.Type {
	case "array":
		if schema.Items != nil {
			elemType := p.resolveType(schema.Items, false) // array elements aren't individually optional
			return "[]" + elemType
		}
		return "[]any"
	case "object":
		if schema.AdditionalProperties != nil {
			valueType := p.resolveType(schema.AdditionalProperties, false)
			return "map[string]" + valueType
		}
		// Inline object - use any since we can't generate anonymous structs well
		return "map[string]any"
	default:
		return scalarGoType(schema.Type, schema.Format, optional)
	}
}

// SchemaDescription returns the jsonschema description for this property.
func (p *PropertyInfo) SchemaDescription() string {
	desc := cleanDescription(p.Property.Description)

	// Add enum values to description if present
	if len(p.Property.Enum) > 0 {
		enumStr := strings.Join(p.Property.Enum, ", ")
		if desc != "" {
			desc += " "
		}
		desc += "Values: " + enumStr
	}

	// Add default if present
	if p.Property.Default != "" {
		desc += " (default: " + p.Property.Default + ")"
	}

	// Add read-only indicator
	if p.Property.ReadOnly {
		desc += " (read-only)"
	}

	return desc
}

// cleanDescription sanitizes a description for use in Go struct tags.
func cleanDescription(desc string) string {
	desc = strings.ReplaceAll(desc, "\n", " ")
	desc = strings.ReplaceAll(desc, `"`, "'") // Replace double quotes
	desc = strings.ReplaceAll(desc, "`", "'") // Replace backticks
	desc = strings.TrimSpace(desc)
	// Collapse multiple spaces
	for strings.Contains(desc, "  ") {
		desc = strings.ReplaceAll(desc, "  ", " ")
	}
	return desc
}

// Helper functions

func exportedName(s string) string {
	if s == "" {
		return ""
	}
	// Handle camelCase and snake_case
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")

	words := strings.Fields(s)
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	result := strings.Join(words, "")

	// Handle acronyms
	result = strings.ReplaceAll(result, "Id", "ID")
	result = strings.ReplaceAll(result, "Url", "URL")
	result = strings.ReplaceAll(result, "Http", "HTTP")
	result = strings.ReplaceAll(result, "Api", "API")

	// Ensure first char is uppercase
	if result != "" {
		runes := []rune(result)
		runes[0] = unicode.ToUpper(runes[0])
		result = string(runes)
	}

	return result
}

func paramGoType(p *Parameter) string {
	optional := !p.Required
	if p.Repeated {
		return "[]" + scalarGoType(p.Type, p.Format, false) // array elements aren't optional
	}
	return scalarGoType(p.Type, p.Format, optional)
}

// scalarGoType returns the Go type for a scalar Discovery Document type.
// If optional is true and it's a boolean, returns *bool to distinguish absent from false.
func scalarGoType(typ, typeFormat string, optional bool) string {
	switch typ {
	case "string":
		return "string"
	case "integer":
		switch typeFormat {
		case "int32":
			return "int32"
		case "uint32":
			return "uint32"
		case "int64":
			return "int64"
		case "uint64":
			return "uint64"
		default:
			return "int64"
		}
	case "number":
		switch typeFormat {
		case "float":
			return "float32"
		case "double":
			return "float64"
		default:
			return "float64"
		}
	case "boolean":
		if optional {
			return "*bool"
		}
		return "bool"
	case "any":
		return "any"
	default:
		return "any"
	}
}

func indexOf(slice []string, s string) int {
	for i, v := range slice {
		if v == s {
			return i
		}
	}
	return -1
}

// collectSchemas collects all schemas needed by the given methods, including dependencies.
// Returns schemas in dependency order (dependencies first).
func collectSchemas(methods []*MethodInfo, allSchemas map[string]*Schema) []*SchemaInfo {
	needed := make(map[string]bool)

	// Find all directly referenced schemas
	for _, m := range methods {
		if m.Method.Request != nil && m.Method.Request.Ref != "" {
			collectSchemaRefs(m.Method.Request.Ref, allSchemas, needed)
		}
		if m.Method.Response != nil && m.Method.Response.Ref != "" {
			collectSchemaRefs(m.Method.Response.Ref, allSchemas, needed)
		}
	}

	// Convert to SchemaInfo list, sorted by name for deterministic output
	var names []string
	for name := range needed {
		names = append(names, name)
	}
	sort.Strings(names)

	var result []*SchemaInfo
	for _, name := range names {
		if schema, ok := allSchemas[name]; ok {
			result = append(result, NewSchemaInfo(name, schema, allSchemas))
		}
	}

	return result
}

// collectSchemaRefs recursively collects a schema and all its dependencies.
func collectSchemaRefs(schemaName string, allSchemas map[string]*Schema, needed map[string]bool) {
	if needed[schemaName] {
		return // Already collected
	}

	schema, ok := allSchemas[schemaName]
	if !ok {
		return // Schema not found
	}

	needed[schemaName] = true

	// Collect property references
	for _, prop := range schema.Properties {
		collectSchemaRefsFromSchema(prop, allSchemas, needed)
	}

	// Collect items references (for arrays)
	if schema.Items != nil {
		collectSchemaRefsFromSchema(schema.Items, allSchemas, needed)
	}

	// Collect additionalProperties references (for maps)
	if schema.AdditionalProperties != nil {
		collectSchemaRefsFromSchema(schema.AdditionalProperties, allSchemas, needed)
	}
}

// collectSchemaRefsFromSchema collects schema references from a schema definition.
func collectSchemaRefsFromSchema(schema *Schema, allSchemas map[string]*Schema, needed map[string]bool) {
	if schema.Ref != "" {
		collectSchemaRefs(schema.Ref, allSchemas, needed)
	}
	for _, prop := range schema.Properties {
		collectSchemaRefsFromSchema(prop, allSchemas, needed)
	}
	if schema.Items != nil {
		collectSchemaRefsFromSchema(schema.Items, allSchemas, needed)
	}
	if schema.AdditionalProperties != nil {
		collectSchemaRefsFromSchema(schema.AdditionalProperties, allSchemas, needed)
	}
}

var codeTemplate = template.Must(template.New("mcp").Parse(`// Code generated by google-discovery-mcp. DO NOT EDIT.
// Source: {{.APIName}} {{.APIVersion}}
// API: {{.APITitle}}

package {{.PackageName}}
{{if .GenerateSchema}}
// =============================================================================
// Schema Types (Request/Response Bodies)
// =============================================================================
{{range .SchemasToGen}}
// {{.StructName}} - {{.Description}}
type {{.StructName}} struct {
{{- range .SortedProperties}}
	{{.FieldName}} {{.GoType}} ` + "`" + `json:"{{.JSONTag}}" jsonschema:"{{.SchemaDescription}}"` + "`" + `
{{- end}}
}
{{end}}{{end}}
// =============================================================================
// Tool Argument Types (URL Parameters)
// =============================================================================
{{range .Methods}}
// {{.StructName}} are the arguments for {{.ToolName}}.
// {{.Description}}
type {{.StructName}} struct {
{{- range .SortedParams}}
	{{.FieldName}} {{.GoType}} ` + "`" + `json:"{{.JSONTag}}" jsonschema:"{{.SchemaDescription}}"` + "`" + `
{{- end}}
}
{{end}}

// GeneratedToolDefinitions returns MCP tool definitions for the generated tools.
// Use this to register tools with your MCP server.
var GeneratedToolDefinitions = map[string]string{
{{- range .Methods}}
	"{{.ToolName}}": ` + "`" + `{{.Description}}` + "`" + `,
{{- end}}
}
`))
