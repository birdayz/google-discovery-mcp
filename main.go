// google-discovery-mcp generates MCP tool structs from Google API Discovery Documents.
//
// Usage:
//
//	google-discovery-mcp -api youtube -version v3                    # Fetch from Google
//	google-discovery-mcp -file youtube-v3.json                       # Use local file
//	google-discovery-mcp -api youtube -version v3 -methods videos.list,videos.insert
//	google-discovery-mcp -api youtube -version v3 -schema            # Include schema types
//	google-discovery-mcp -list                                       # List all Google APIs
//
// The tool generates Go structs with jsonschema tags suitable for MCP servers.
// Use -schema to also generate types for request/response body schemas.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/birdayz/google-discovery-mcp/discovery"
)

func main() {
	var (
		apiName        = flag.String("api", "", "API name (e.g., youtube, drive, gmail)")
		version        = flag.String("version", "", "API version (e.g., v3, v1)")
		file           = flag.String("file", "", "Path to local Discovery Document JSON file")
		methods        = flag.String("methods", "", "Comma-separated list of methods to generate (default: all)")
		pkg            = flag.String("package", "tools", "Go package name for generated code")
		prefix         = flag.String("prefix", "", "Tool name prefix (default: {api}_)")
		structPrefix   = flag.String("struct-prefix", "API", "Struct name prefix (default: API)")
		output         = flag.String("output", "", "Output file (default: stdout)")
		listAPIs       = flag.Bool("list", false, "List all available Google APIs")
		listMethods    = flag.Bool("list-methods", false, "List all methods in the API")
		generateSchema = flag.Bool("schema", false, "Generate schema types (request/response bodies)")
	)
	flag.Parse()

	if *listAPIs {
		if err := doListAPIs(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Load document
	var doc *discovery.Document
	var err error

	switch {
	case *file != "":
		doc, err = discovery.LoadFile(*file)
	case *apiName != "" && *version != "":
		fmt.Fprintf(os.Stderr, "Fetching %s %s from googleapis.com...\n", *apiName, *version)
		doc, err = discovery.Fetch(*apiName, *version)
	default:
		fmt.Fprintf(os.Stderr, "Usage: google-discovery-mcp -api NAME -version VERSION\n")
		fmt.Fprintf(os.Stderr, "       google-discovery-mcp -file PATH\n")
		fmt.Fprintf(os.Stderr, "       google-discovery-mcp -list\n\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading document: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Loaded: %s (%s)\n", doc.Title, doc.ID)

	// List methods mode
	if *listMethods {
		fmt.Printf("Methods in %s:\n\n", doc.Name)
		for _, name := range doc.SortedMethodNames() {
			m := doc.AllMethods()[name]
			desc := m.Description
			if len(desc) > 80 {
				desc = desc[:77] + "..."
			}
			desc = strings.ReplaceAll(desc, "\n", " ")
			fmt.Printf("  %-40s %s\n", name, desc)
		}
		fmt.Printf("\nTotal: %d methods\n", len(doc.AllMethods()))
		return
	}

	// Generate code
	opts := discovery.GenerateOptions{
		PackageName:    *pkg,
		Prefix:         *prefix,
		StructPrefix:   *structPrefix,
		GenerateSchema: *generateSchema,
	}
	if *methods != "" {
		opts.Methods = strings.Split(*methods, ",")
	}

	code, err := discovery.GenerateMCPTools(doc, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating code: %v\n", err)
		// Print the code anyway for debugging
		if code != "" {
			fmt.Println(code)
		}
		os.Exit(1)
	}

	// Output
	if *output != "" {
		if err := os.WriteFile(*output, []byte(code), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Generated %s\n", *output)
	} else {
		fmt.Println(code)
	}
}

func doListAPIs() error {
	fmt.Fprintf(os.Stderr, "Fetching API list from googleapis.com...\n")
	apis, err := discovery.ListAPIs()
	if err != nil {
		return err
	}

	fmt.Printf("Available Google APIs:\n\n")
	for _, api := range apis {
		pref := " "
		if api.Preferred {
			pref = "*"
		}
		fmt.Printf("%s %-30s %-10s %s\n", pref, api.Name, api.Version, api.Title)
	}
	fmt.Printf("\n* = preferred version\n")
	fmt.Printf("Total: %d APIs\n", len(apis))
	return nil
}
