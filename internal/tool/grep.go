package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/abcdlsj/otter/internal/config"
)

type Grep struct{}

func (Grep) Name() string { return "grep" }
func (Grep) Desc() string { return "Search file contents using regex pattern" }
func (Grep) Args() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Regex pattern to search for",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Directory or file to search in",
			},
			"glob": map[string]any{
				"type":        "string",
				"description": "File glob pattern to filter files (e.g., '*.go', '*.ts')",
			},
			"max_results": map[string]any{
				"type":        "number",
				"description": "Maximum number of results to return (default: 50)",
			},
		},
		"required": []string{"pattern", "path"},
	}
}

func (Grep) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Pattern    string `json:"pattern"`
		Path       string `json:"path"`
		Glob       string `json:"glob"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}

	if args.MaxResults <= 0 {
		args.MaxResults = 50
	}

	// Compile regex
	re, err := regexp.Compile(args.Pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %w", err)
	}

	// Check if path is a file or directory
	info, err := os.Stat(args.Path)
	if err != nil {
		return "", fmt.Errorf("cannot access path: %w", err)
	}

	var results []string
	resultCount := 0

	cfg := &config.C
	if info.IsDir() {
		err = grepDir(ctx, cfg, args.Path, args.Glob, re, &results, &resultCount, args.MaxResults)
	} else {
		err = grepFile(cfg, args.Path, re, &results, &resultCount, args.MaxResults)
	}

	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "no matches found", nil
	}

	output := strings.Join(results, "\n")
	if resultCount >= args.MaxResults {
		output += fmt.Sprintf("\n\n... (truncated, showing %d of %d+ matches)", args.MaxResults, resultCount)
	}
	return output, nil
}

func grepDir(ctx context.Context, cfg *config.Config, dir string, glob string, re *regexp.Regexp, results *[]string, count *int, max int) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
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
			if name == "node_modules" || name == "vendor" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		if !cfg.CheckReadPermission(path) {
			return nil
		}
		if glob != "" {
			matched, _ := filepath.Match(glob, filepath.Base(path))
			if !matched {
				return nil
			}
		}
		if info, err := d.Info(); err == nil && info.Size() > 1024*1024 {
			return nil
		}
		return grepFile(cfg, path, re, results, count, max)
	})
}

func grepFile(cfg *config.Config, path string, re *regexp.Regexp, results *[]string, count *int, max int) error {
	if !cfg.CheckReadPermission(path) {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if !re.MatchString(line) {
			continue
		}
		*count++
		if len(*results) < max {
			if len(line) > 200 {
				line = line[:200] + "..."
			}
			*results = append(*results, fmt.Sprintf("%s:%d: %s", path, lineNum, line))
		}
		if *count >= max {
			return nil
		}
	}
	return scanner.Err()
}
