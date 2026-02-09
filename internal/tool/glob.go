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

type Glob struct{}

func (Glob) Name() string { return "glob" }
func (Glob) Desc() string {
	return "Find files matching glob patterns (e.g., '*.go', '**/*.md')"
}

func (Glob) Args() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Glob pattern to match (e.g., '*.go', '**/*.md'). Supports ** for recursive matching",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Root directory to search from (default: current directory)",
			},
			"max_results": map[string]any{
				"type":        "number",
				"description": "Maximum number of results (default: 100, max: 200)",
			},
		},
		"required": []string{"pattern"},
	}
}

func (g Glob) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Pattern    string `json:"pattern"`
		Path       string `json:"path"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}

	if args.Path == "" {
		args.Path = "."
	}
	if args.MaxResults <= 0 || args.MaxResults > 200 {
		args.MaxResults = 100
	}

	args.Path = filepath.Clean(args.Path)
	cfg := &config.C

	if !cfg.CheckReadPermission(args.Path) {
		return "", fmt.Errorf("permission denied: cannot read %s", args.Path)
	}

	var matches []string
	seen := make(map[string]bool)

	// Handle recursive ** pattern
	if strings.Contains(args.Pattern, "**") {
		err := g.globRecursive(ctx, args.Path, args.Pattern, &matches, seen, cfg)
		if err != nil {
			return "", err
		}
	} else {
		err := g.globSimple(ctx, args.Path, args.Pattern, &matches, seen, cfg)
		if err != nil {
			return "", err
		}
	}

	if len(matches) == 0 {
		return "No files found", nil
	}

	if len(matches) > args.MaxResults {
		matches = matches[:args.MaxResults]
	}

	return strings.Join(matches, "\n"), nil
}

func (g Glob) globSimple(ctx context.Context, root, pattern string, matches *[]string, seen map[string]bool, cfg *config.Config) error {
	fullPattern := filepath.Join(root, pattern)
	files, err := filepath.Glob(fullPattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}

	for _, f := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		info, err := os.Stat(f)
		if err != nil || info.IsDir() {
			continue
		}
		if !cfg.CheckReadPermission(f) {
			continue
		}
		if seen[f] {
			continue
		}
		seen[f] = true

		rel, _ := filepath.Rel(root, f)
		*matches = append(*matches, rel)
	}
	return nil
}

func (g Glob) globRecursive(ctx context.Context, root, pattern string, matches *[]string, seen map[string]bool, cfg *config.Config) error {
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		return g.globSimple(ctx, root, pattern, matches, seen, cfg)
	}

	base := strings.TrimSuffix(parts[0], "/")
	filePattern := strings.TrimPrefix(parts[1], "/")

	searchRoot := root
	if base != "" {
		searchRoot = filepath.Join(root, base)
	}

	return filepath.WalkDir(searchRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			if name == "node_modules" || name == "vendor" || name == "dist" || name == "build" || name == "target" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		matched, _ := filepath.Match(filePattern, d.Name())
		if !matched {
			return nil
		}
		if !cfg.CheckReadPermission(path) || seen[path] {
			return nil
		}
		seen[path] = true

		rel, _ := filepath.Rel(root, path)
		*matches = append(*matches, rel)
		return nil
	})
}
