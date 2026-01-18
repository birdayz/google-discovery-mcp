package discovery

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const discoveryBaseURL = "https://www.googleapis.com/discovery/v1/apis"

// Fetch downloads a Discovery Document from Google's API.
// api is the API name (e.g., "youtube")
// version is the API version (e.g., "v3")
func Fetch(api, version string) (*Document, error) {
	url := fmt.Sprintf("%s/%s/%s/rest", discoveryBaseURL, api, version)
	return FetchURL(url)
}

// FetchURL downloads a Discovery Document from a URL.
func FetchURL(url string) (*Document, error) {
	resp, err := http.Get(url) //nolint:gosec // URL is constructed from user input, but this is a CLI tool
	if err != nil {
		return nil, fmt.Errorf("failed to fetch discovery document: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch discovery document: %s\n%s", resp.Status, body)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read discovery document: %w", err)
	}

	return Parse(data)
}

// LoadFile loads a Discovery Document from a local file.
func LoadFile(path string) (*Document, error) {
	data, err := os.ReadFile(path) //nolint:gosec // Path is from user input, but this is a CLI tool
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return Parse(data)
}

// ListAPIs returns a list of all available Google APIs.
func ListAPIs() ([]APIInfo, error) {
	resp, err := http.Get(discoveryBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to list APIs: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Items []APIInfo `json:"items"`
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read API list: %w", err)
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse API list: %w", err)
	}
	return result.Items, nil
}

// APIInfo contains basic information about an available API.
type APIInfo struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	DiscoveryRestURL  string `json:"discoveryRestUrl"`
	DocumentationLink string `json:"documentationLink"`
	Preferred         bool   `json:"preferred"`
}
