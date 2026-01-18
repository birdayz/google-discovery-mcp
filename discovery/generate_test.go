package discovery

import (
	"regexp"
	"strings"
	"testing"
)

// containsFieldType checks if the code contains a field with the given name and type.
// Handles go fmt alignment (multiple spaces/tabs between name and type).
func containsFieldType(code, fieldName, fieldType string) bool {
	// Pattern: fieldName followed by whitespace followed by fieldType
	pattern := regexp.MustCompile(fieldName + `\s+` + regexp.QuoteMeta(fieldType))
	return pattern.MatchString(code)
}

func TestScalarGoType(t *testing.T) {
	tests := []struct {
		name     string
		typ      string
		format   string
		optional bool
		want     string
	}{
		// String types
		{"string required", "string", "", false, "string"},
		{"string optional", "string", "", true, "string"},

		// Integer types
		{"integer default", "integer", "", false, "int64"},
		{"integer int32", "integer", "int32", false, "int32"},
		{"integer uint32", "integer", "uint32", false, "uint32"},
		{"integer int64", "integer", "int64", false, "int64"},
		{"integer uint64", "integer", "uint64", false, "uint64"},

		// Number types
		{"number default", "number", "", false, "float64"},
		{"number float", "number", "float", false, "float32"},
		{"number double", "number", "double", false, "float64"},

		// Boolean types - the critical case
		{"boolean required", "boolean", "", false, "bool"},
		{"boolean optional", "boolean", "", true, "*bool"},

		// Any types
		{"any type", "any", "", false, "any"},
		{"unknown type", "unknown", "", false, "any"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scalarGoType(tt.typ, tt.format, tt.optional)
			if got != tt.want {
				t.Errorf("scalarGoType(%q, %q, %v) = %q, want %q",
					tt.typ, tt.format, tt.optional, got, tt.want)
			}
		})
	}
}

func TestParamGoType(t *testing.T) {
	tests := []struct {
		name  string
		param *Parameter
		want  string
	}{
		{
			name:  "required boolean",
			param: &Parameter{Type: "boolean", Required: true},
			want:  "bool",
		},
		{
			name:  "optional boolean",
			param: &Parameter{Type: "boolean", Required: false},
			want:  "*bool",
		},
		{
			name:  "required string",
			param: &Parameter{Type: "string", Required: true},
			want:  "string",
		},
		{
			name:  "optional string",
			param: &Parameter{Type: "string", Required: false},
			want:  "string",
		},
		{
			name:  "repeated string",
			param: &Parameter{Type: "string", Repeated: true},
			want:  "[]string",
		},
		{
			name:  "repeated boolean",
			param: &Parameter{Type: "boolean", Repeated: true},
			want:  "[]bool", // array elements aren't optional
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := paramGoType(tt.param)
			if got != tt.want {
				t.Errorf("paramGoType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPropertyInfoGoType(t *testing.T) {
	allSchemas := map[string]*Schema{
		"Video": {
			ID:   "Video",
			Type: "object",
			Properties: map[string]*Schema{
				"id": {Type: "string"},
			},
		},
		"StringWrapper": {
			ID:   "StringWrapper",
			Type: "string",
		},
	}

	tests := []struct {
		name     string
		property *Schema
		required bool
		want     string
	}{
		{
			name:     "optional boolean property",
			property: &Schema{Type: "boolean"},
			required: false,
			want:     "*bool",
		},
		{
			name:     "required boolean property",
			property: &Schema{Type: "boolean"},
			required: true,
			want:     "bool",
		},
		{
			name:     "string property",
			property: &Schema{Type: "string"},
			required: false,
			want:     "string",
		},
		{
			name:     "integer property",
			property: &Schema{Type: "integer", Format: "int64"},
			required: false,
			want:     "int64",
		},
		{
			name:     "array of strings",
			property: &Schema{Type: "array", Items: &Schema{Type: "string"}},
			required: false,
			want:     "[]string",
		},
		{
			name:     "array of objects via ref",
			property: &Schema{Type: "array", Items: &Schema{Ref: "Video"}},
			required: false,
			want:     "[]*Video",
		},
		{
			name:     "ref to object schema",
			property: &Schema{Ref: "Video"},
			required: false,
			want:     "*Video",
		},
		{
			name:     "ref to scalar wrapper schema",
			property: &Schema{Ref: "StringWrapper"},
			required: false,
			want:     "string",
		},
		{
			name:     "map with string values",
			property: &Schema{Type: "object", AdditionalProperties: &Schema{Type: "string"}},
			required: false,
			want:     "map[string]string",
		},
		{
			name:     "map with object values via ref",
			property: &Schema{Type: "object", AdditionalProperties: &Schema{Ref: "Video"}},
			required: false,
			want:     "map[string]*Video",
		},
		{
			name:     "inline object without additionalProperties",
			property: &Schema{Type: "object"},
			required: false,
			want:     "map[string]any",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PropertyInfo{
				Name:       "testField",
				Property:   tt.property,
				Required:   tt.required,
				AllSchemas: allSchemas,
			}
			got := p.GoType()
			if got != tt.want {
				t.Errorf("PropertyInfo.GoType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCollectSchemas(t *testing.T) {
	allSchemas := map[string]*Schema{
		"Video": {
			ID:   "Video",
			Type: "object",
			Properties: map[string]*Schema{
				"status":  {Ref: "VideoStatus"},
				"snippet": {Ref: "VideoSnippet"},
			},
		},
		"VideoStatus": {
			ID:   "VideoStatus",
			Type: "object",
			Properties: map[string]*Schema{
				"madeForKids":   {Type: "boolean"},
				"privacyStatus": {Type: "string"},
			},
		},
		"VideoSnippet": {
			ID:   "VideoSnippet",
			Type: "object",
			Properties: map[string]*Schema{
				"title":      {Type: "string"},
				"thumbnails": {Ref: "ThumbnailDetails"},
			},
		},
		"ThumbnailDetails": {
			ID:   "ThumbnailDetails",
			Type: "object",
			Properties: map[string]*Schema{
				"default": {Ref: "Thumbnail"},
			},
		},
		"Thumbnail": {
			ID:   "Thumbnail",
			Type: "object",
			Properties: map[string]*Schema{
				"url":    {Type: "string"},
				"width":  {Type: "integer"},
				"height": {Type: "integer"},
			},
		},
		"Playlist": {
			ID:   "Playlist",
			Type: "object",
			Properties: map[string]*Schema{
				"id": {Type: "string"},
			},
		},
	}

	methods := []*MethodInfo{
		{
			Method: &Method{
				Response: &SchemaRef{Ref: "Video"},
			},
		},
	}

	schemas := collectSchemas(methods, allSchemas)

	// Should collect Video and all its dependencies
	schemaNames := make(map[string]bool)
	for _, s := range schemas {
		schemaNames[s.Name] = true
	}

	// Expected schemas: Video, VideoStatus, VideoSnippet, ThumbnailDetails, Thumbnail
	expected := []string{"Video", "VideoStatus", "VideoSnippet", "ThumbnailDetails", "Thumbnail"}
	for _, name := range expected {
		if !schemaNames[name] {
			t.Errorf("expected schema %q to be collected, but it wasn't", name)
		}
	}

	// Playlist should NOT be collected (not referenced)
	if schemaNames["Playlist"] {
		t.Error("Playlist should not be collected (not referenced by any method)")
	}
}

func TestCollectSchemasWithArrays(t *testing.T) {
	allSchemas := map[string]*Schema{
		"VideoListResponse": {
			ID:   "VideoListResponse",
			Type: "object",
			Properties: map[string]*Schema{
				"items": {Type: "array", Items: &Schema{Ref: "Video"}},
			},
		},
		"Video": {
			ID:   "Video",
			Type: "object",
			Properties: map[string]*Schema{
				"id": {Type: "string"},
			},
		},
	}

	methods := []*MethodInfo{
		{
			Method: &Method{
				Response: &SchemaRef{Ref: "VideoListResponse"},
			},
		},
	}

	schemas := collectSchemas(methods, allSchemas)

	schemaNames := make(map[string]bool)
	for _, s := range schemas {
		schemaNames[s.Name] = true
	}

	if !schemaNames["VideoListResponse"] {
		t.Error("VideoListResponse should be collected")
	}
	if !schemaNames["Video"] {
		t.Error("Video should be collected (referenced in array items)")
	}
}

func TestSchemaInfoSortedProperties(t *testing.T) {
	schema := &Schema{
		ID:   "TestSchema",
		Type: "object",
		Properties: map[string]*Schema{
			"zebra":    {Type: "string"},
			"alpha":    {Type: "string"},
			"required": {Type: "string", Required: true},
			"beta":     {Type: "string"},
		},
		Annotations: &Annotations{
			Required: []string{"required"},
		},
	}

	info := NewSchemaInfo("TestSchema", schema, nil)
	props := info.SortedProperties()

	// Required fields should come first
	if props[0].Name != "required" {
		t.Errorf("first property should be 'required' (it's required), got %q", props[0].Name)
	}

	// Remaining should be alphabetical
	expectedOrder := []string{"required", "alpha", "beta", "zebra"}
	for i, expected := range expectedOrder {
		if props[i].Name != expected {
			t.Errorf("property %d should be %q, got %q", i, expected, props[i].Name)
		}
	}
}

func TestExportedName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"id", "ID"},
		{"videoId", "VideoID"},
		{"url", "URL"},
		{"thumbnailUrl", "ThumbnailURL"},
		{"http_method", "HTTPMethod"},
		{"api_key", "APIKey"},
		{"madeForKids", "MadeForKids"},
		{"snake_case", "SnakeCase"},
		{"kebab-case", "KebabCase"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := exportedName(tt.input)
			if got != tt.want {
				t.Errorf("exportedName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateMCPToolsWithSchema(t *testing.T) {
	doc := &Document{
		Name:    "test",
		Version: "v1",
		Title:   "Test API",
		Schemas: map[string]*Schema{
			"Video": {
				ID:          "Video",
				Type:        "object",
				Description: "A video resource",
				Properties: map[string]*Schema{
					"id":          {Type: "string", Description: "Video ID"},
					"madeForKids": {Type: "boolean", Description: "Is this video made for kids"},
					"embeddable":  {Type: "boolean", Description: "Can be embedded"},
				},
			},
		},
		Resources: map[string]*Resource{
			"videos": {
				Methods: map[string]*Method{
					"list": {
						ID:          "videos.list",
						Description: "List videos",
						Parameters: map[string]*Parameter{
							"part": {Type: "string", Required: true, Description: "Parts to include"},
						},
						Response: &SchemaRef{Ref: "Video"},
					},
				},
			},
		},
	}

	opts := GenerateOptions{
		PackageName:    "testpkg",
		GenerateSchema: true,
	}

	code, err := GenerateMCPTools(doc, opts)
	if err != nil {
		t.Fatalf("GenerateMCPTools failed: %v", err)
	}

	// Verify schema types are generated
	if !strings.Contains(code, "type Video struct") {
		t.Error("generated code should contain Video struct")
	}

	// Verify *bool is used for optional boolean fields
	// Note: go format may add alignment spaces, so we check for the pattern
	if !strings.Contains(code, "MadeForKids") || !strings.Contains(code, "*bool") {
		t.Errorf("MadeForKids should be *bool (optional boolean)\nGenerated code:\n%s", code)
	}
	// Check that MadeForKids is followed by *bool (with possible whitespace)
	if !containsFieldType(code, "MadeForKids", "*bool") {
		t.Errorf("MadeForKids should have type *bool\nGenerated code:\n%s", code)
	}
	if !containsFieldType(code, "Embeddable", "*bool") {
		t.Errorf("Embeddable should have type *bool\nGenerated code:\n%s", code)
	}

	// Verify json tags
	if !strings.Contains(code, `json:"madeForKids,omitempty"`) {
		t.Error("json tag should include omitempty for optional fields")
	}

	// Verify tool argument types are generated
	if !strings.Contains(code, "type APIVideosListArgs struct") {
		t.Error("generated code should contain APIVideosListArgs struct")
	}
}

func TestGenerateMCPToolsWithoutSchema(t *testing.T) {
	doc := &Document{
		Name:    "test",
		Version: "v1",
		Title:   "Test API",
		Schemas: map[string]*Schema{
			"Video": {
				ID:   "Video",
				Type: "object",
			},
		},
		Resources: map[string]*Resource{
			"videos": {
				Methods: map[string]*Method{
					"list": {
						ID:          "videos.list",
						Description: "List videos",
						Parameters:  map[string]*Parameter{},
					},
				},
			},
		},
	}

	opts := GenerateOptions{
		PackageName:    "testpkg",
		GenerateSchema: false, // Schema generation disabled
	}

	code, err := GenerateMCPTools(doc, opts)
	if err != nil {
		t.Fatalf("GenerateMCPTools failed: %v", err)
	}

	// Schema types should NOT be generated
	if strings.Contains(code, "type Video struct") {
		t.Error("Video struct should not be generated when GenerateSchema is false")
	}

	// Tool argument types should still be generated
	if !strings.Contains(code, "type APIVideosListArgs struct") {
		t.Error("tool argument types should still be generated")
	}
}

func TestPropertyInfoJSONTag(t *testing.T) {
	tests := []struct {
		name     string
		propName string
		required bool
		want     string
	}{
		{"required field", "videoId", true, "videoId"},
		{"optional field", "videoId", false, "videoId,omitempty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PropertyInfo{
				Name:     tt.propName,
				Required: tt.required,
			}
			got := p.JSONTag()
			if got != tt.want {
				t.Errorf("JSONTag() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParamInfoJSONTag(t *testing.T) {
	tests := []struct {
		name  string
		param *Parameter
		pName string
		want  string
	}{
		{"required param", &Parameter{Required: true}, "part", "part"},
		{"optional param", &Parameter{Required: false}, "maxResults", "maxResults,omitempty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ParamInfo{
				Name:  tt.pName,
				Param: tt.param,
			}
			got := p.JSONTag()
			if got != tt.want {
				t.Errorf("JSONTag() = %q, want %q", got, tt.want)
			}
		})
	}
}
