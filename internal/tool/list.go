package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abcdlsj/otter/internal/config"
)

type List struct{}

func (List) Name() string { return "list" }
func (List) Desc() string { return "List files and directories with optional filtering and recursion" }
func (List) Args() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Directory path to list (default: current directory)",
			},
			"recursive": map[string]any{
				"type":        "boolean",
				"description": "List recursively (default: false)",
			},
			"glob": map[string]any{
				"type":        "string",
				"description": "Glob pattern to filter files (e.g., '*.go', '*.ts', '*.md')",
			},
			"max_results": map[string]any{
				"type":        "number",
				"description": "Maximum number of entries to return (default: 100)",
			},
		},
		"required": []string{},
	}
}

func (l List) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Path       string `json:"path"`
		Recursive  bool   `json:"recursive"`
		Glob       string `json:"glob"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}

	// Default values
	if args.Path == "" {
		args.Path = "."
	}
	if args.MaxResults <= 0 {
		args.MaxResults = 100
	}

	// Clean the path
	args.Path = filepath.Clean(args.Path)

	// Check read permission
	cfg := &config.C
	if !cfg.CheckReadPermission(args.Path) {
		return "", fmt.Errorf("permission denied: cannot read %s", args.Path)
	}

	var entries []string
	count := 0

	if args.Recursive {
		err := filepath.WalkDir(args.Path, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // Skip files we can't access
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Skip hidden directories and common ignore patterns
			if d.IsDir() {
				name := d.Name()
				if name != "." && strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				if name == "node_modules" || name == "vendor" || name == "dist" || name == "build" || name == ".git" {
					return filepath.SkipDir
				}
				return nil
			}

			// Check read permission
			if !cfg.CheckReadPermission(path) {
				return nil
			}

			// Check glob pattern
			if args.Glob != "" {
				matched, err := filepath.Match(args.Glob, filepath.Base(path))
				if err != nil || !matched {
					return nil
				}
			}

			if count < args.MaxResults {
				// Get relative path from the starting directory
				relPath, _ := filepath.Rel(args.Path, path)
				if relPath == "" {
					relPath = path
				}
				entries = append(entries, relPath)
			}
			count++

			if count >= args.MaxResults {
				return filepath.SkipAll
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	} else {
		// Non-recursive listing
		dirEntries, err := os.ReadDir(args.Path)
		if err != nil {
			return "", err
		}

		for _, e := range dirEntries {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			default:
			}

			// Skip hidden files
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}

			fullPath := filepath.Join(args.Path, name)

			// Check read permission
			if !cfg.CheckReadPermission(fullPath) {
				continue
			}

			// Check glob pattern (only for files)
			if args.Glob != "" && !e.IsDir() {
				matched, err := filepath.Match(args.Glob, name)
				if err != nil || !matched {
					continue
				}
			}

			if count >= args.MaxResults {
				break
			}

			prefix := "  "
			if e.IsDir() {
				prefix = "d "
			}
			entries = append(entries, prefix+name)
			count++
		}
	}

	if len(entries) == 0 {
		return "no files found", nil
	}

	result := strings.Join(entries, "\n")
	if count >= args.MaxResults {
		result += fmt.Sprintf("\n\n... (showing %d entries, use max_results or glob to filter)", args.MaxResults)
	}
	return result, nil
}
