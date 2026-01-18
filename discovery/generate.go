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
	PackageName  string   // Go package name (default: "tools")
	Methods      []string // Specific methods to generate (empty = all)
	Prefix       string   // Tool name prefix (e.g., "youtube_")
	StructPrefix string   // Struct name prefix (default: "API")
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

	data := &TemplateData{
		PackageName: opts.PackageName,
		APIName:     doc.Name,
		APITitle:    doc.Title,
		APIVersion:  doc.Version,
		Methods:     methodsToGenerate,
		Schemas:     doc.Schemas,
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
	PackageName string
	APIName     string
	APITitle    string
	APIVersion  string
	Methods     []*MethodInfo
	Schemas     map[string]*Schema
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
		if len(desc) > 0 {
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
		if len(w) > 0 {
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
	if len(result) > 0 {
		runes := []rune(result)
		runes[0] = unicode.ToUpper(runes[0])
		result = string(runes)
	}

	return result
}

func paramGoType(p *Parameter) string {
	if p.Repeated {
		return "[]" + scalarGoType(p.Type, p.Format)
	}
	return scalarGoType(p.Type, p.Format)
}

func scalarGoType(typ, format string) string {
	switch typ {
	case "string":
		return "string"
	case "integer":
		switch format {
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
		switch format {
		case "float":
			return "float32"
		case "double":
			return "float64"
		default:
			return "float64"
		}
	case "boolean":
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

var codeTemplate = template.Must(template.New("mcp").Parse(`// Code generated by discovery-to-mcp. DO NOT EDIT.
// Source: {{.APIName}} {{.APIVersion}}
// API: {{.APITitle}}

package {{.PackageName}}

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
