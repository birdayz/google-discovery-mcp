package discovery

import (
	"strings"
	"testing"
)

// TestYouTubeAPIGeneration tests generation against the real YouTube API Discovery Document.
// This is an integration test that requires network access.
func TestYouTubeAPIGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	doc, err := Fetch("youtube", "v3")
	if err != nil {
		t.Fatalf("failed to fetch YouTube API discovery document: %v", err)
	}

	opts := GenerateOptions{
		PackageName:    "youtube",
		GenerateSchema: true,
		Methods:        []string{"videos.list", "videos.insert", "videos.update"},
	}

	code, err := GenerateMCPTools(doc, opts)
	if err != nil {
		t.Fatalf("failed to generate code: %v", err)
	}

	// Test 1: VideoStatus should have *bool for optional boolean fields
	t.Run("VideoStatus_optional_booleans", func(t *testing.T) {
		if !containsFieldType(code, "MadeForKids", "*bool") {
			t.Error("VideoStatus.MadeForKids should be *bool")
		}
		if !containsFieldType(code, "Embeddable", "*bool") {
			t.Error("VideoStatus.Embeddable should be *bool")
		}
		if !containsFieldType(code, "SelfDeclaredMadeForKids", "*bool") {
			t.Error("VideoStatus.SelfDeclaredMadeForKids should be *bool")
		}
		if !containsFieldType(code, "PublicStatsViewable", "*bool") {
			t.Error("VideoStatus.PublicStatsViewable should be *bool")
		}
	})

	// Test 2: VideoContentDetails should have *bool for optional booleans
	t.Run("VideoContentDetails_optional_booleans", func(t *testing.T) {
		if !containsFieldType(code, "LicensedContent", "*bool") {
			t.Error("VideoContentDetails.LicensedContent should be *bool")
		}
		if !containsFieldType(code, "HasCustomThumbnail", "*bool") {
			t.Error("VideoContentDetails.HasCustomThumbnail should be *bool")
		}
	})

	// Test 3: Verify Video struct is generated with proper references
	t.Run("Video_struct_references", func(t *testing.T) {
		if !strings.Contains(code, "type Video struct") {
			t.Error("Video struct should be generated")
		}
		if !containsFieldType(code, "Status", "*VideoStatus") {
			t.Error("Video.Status should reference *VideoStatus")
		}
		if !containsFieldType(code, "Snippet", "*VideoSnippet") {
			t.Error("Video.Snippet should reference *VideoSnippet")
		}
		if !containsFieldType(code, "ContentDetails", "*VideoContentDetails") {
			t.Error("Video.ContentDetails should reference *VideoContentDetails")
		}
	})

	// Test 4: Verify arrays are generated correctly
	t.Run("array_types", func(t *testing.T) {
		// VideoSnippet.Tags should be []string
		if !containsFieldType(code, "Tags", "[]string") {
			t.Error("VideoSnippet.Tags should be []string")
		}
	})

	// Test 5: Verify VideoListResponse has Items as array of Video pointers
	t.Run("VideoListResponse_items", func(t *testing.T) {
		if !strings.Contains(code, "type VideoListResponse struct") {
			t.Error("VideoListResponse struct should be generated")
		}
		if !containsFieldType(code, "Items", "[]*Video") {
			t.Error("VideoListResponse.Items should be []*Video")
		}
	})

	// Test 6: Verify string fields remain string (not affected by *bool change)
	t.Run("string_fields_unchanged", func(t *testing.T) {
		if !containsFieldType(code, "PrivacyStatus", "string") {
			t.Error("VideoStatus.PrivacyStatus should be string")
		}
		if !containsFieldType(code, "UploadStatus", "string") {
			t.Error("VideoStatus.UploadStatus should be string")
		}
	})

	// Test 7: Verify tool argument types are also generated
	t.Run("tool_argument_types", func(t *testing.T) {
		if !strings.Contains(code, "type APIVideosListArgs struct") {
			t.Error("APIVideosListArgs should be generated")
		}
		if !strings.Contains(code, "type APIVideosInsertArgs struct") {
			t.Error("APIVideosInsertArgs should be generated")
		}
		if !strings.Contains(code, "type APIVideosUpdateArgs struct") {
			t.Error("APIVideosUpdateArgs should be generated")
		}
	})

	// Test 8: Verify json tags include omitempty for optional fields
	t.Run("json_tags_omitempty", func(t *testing.T) {
		if !strings.Contains(code, `json:"madeForKids,omitempty"`) {
			t.Error("madeForKids should have omitempty in json tag")
		}
		if !strings.Contains(code, `json:"embeddable,omitempty"`) {
			t.Error("embeddable should have omitempty in json tag")
		}
	})

	// Test 9: Verify AccessPolicy.Allowed is *bool (used in VideoContentDetails.CountryRestriction)
	t.Run("AccessPolicy_allowed", func(t *testing.T) {
		if !strings.Contains(code, "type AccessPolicy struct") {
			t.Error("AccessPolicy struct should be generated")
		}
		if !containsFieldType(code, "Allowed", "*bool") {
			t.Error("AccessPolicy.Allowed should be *bool")
		}
	})

	// Test 10: Verify maps are generated correctly
	t.Run("map_types", func(t *testing.T) {
		// Video.Localizations should be a map
		if !strings.Contains(code, "map[string]*VideoLocalization") {
			t.Error("Video.Localizations should be map[string]*VideoLocalization")
		}
	})
}

// TestYouTubeAPISchemaCount verifies we're generating a reasonable number of schemas.
func TestYouTubeAPISchemaCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	doc, err := Fetch("youtube", "v3")
	if err != nil {
		t.Fatalf("failed to fetch YouTube API discovery document: %v", err)
	}

	opts := GenerateOptions{
		PackageName:    "youtube",
		GenerateSchema: true,
		Methods:        []string{"videos.list", "videos.insert", "videos.update"},
	}

	code, err := GenerateMCPTools(doc, opts)
	if err != nil {
		t.Fatalf("failed to generate code: %v", err)
	}

	// Count struct definitions
	structCount := strings.Count(code, "type ")
	t.Logf("Generated %d type definitions", structCount)

	// Should have at least 20 types for videos.list/insert/update
	// (Video, VideoStatus, VideoSnippet, VideoContentDetails, etc.)
	if structCount < 20 {
		t.Errorf("expected at least 20 type definitions, got %d", structCount)
	}
}
