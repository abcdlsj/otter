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

type View struct{}

func (View) Name() string { return "view" }
func (View) Desc() string {
	return "View file contents or directory structure. For files: displays content with line numbers. For directories: shows tree-like structure with files and subdirectories."
}
func (View) Args() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to file or directory to view",
			},
			"view_range": map[string]any{
				"type":        "array",
				"description": "For files: [start_line, end_line] to view specific range (1-based, inclusive). Omit to view entire file.",
				"items": map[string]any{
					"type": "number",
				},
			},
			"depth": map[string]any{
				"type":        "number",
				"description": "For directories: max depth to traverse (default: 3, max: 5)",
			},
		},
		"required": []string{"path"},
	}
}

func (v View) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Path      string `json:"path"`
		ViewRange []int  `json:"view_range"`
		Depth     int    `json:"depth"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}

	if args.Path == "" {
		args.Path = "."
	}

	// Clean the path
	args.Path = filepath.Clean(args.Path)

	// Check read permission
	cfg := &config.C
	if !cfg.CheckReadPermission(args.Path) {
		return "", fmt.Errorf("permission denied: cannot read %s", args.Path)
	}

	// Check if path exists
	info, err := os.Stat(args.Path)
	if err != nil {
		return "", fmt.Errorf("cannot access path: %w", err)
	}

	// Handle directory
	if info.IsDir() {
		if args.Depth <= 0 {
			args.Depth = 3
		}
		if args.Depth > 5 {
			args.Depth = 5
		}
		return v.viewDirectory(ctx, args.Path, args.Depth, cfg)
	}

	// Handle file
	return v.viewFile(args.Path, args.ViewRange)
}

func (v View) viewFile(path string, viewRange []int) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	totalLines := len(lines)

	// Determine range to display
	startLine := 1
	endLine := totalLines

	if len(viewRange) >= 1 {
		startLine = viewRange[0]
		if startLine < 1 {
			startLine = 1
		}
	}
	if len(viewRange) >= 2 {
		endLine = viewRange[1]
		if endLine > totalLines {
			endLine = totalLines
		}
	}

	if startLine > endLine {
		return "", fmt.Errorf("invalid range: start line %d is after end line %d", startLine, endLine)
	}

	// Build output with line numbers
	var result strings.Builder
	result.WriteString(fmt.Sprintf("// %s (%d lines total)\n", path, totalLines))

	maxLineNumWidth := len(fmt.Sprintf("%d", endLine))
	lineNumFormat := fmt.Sprintf("%%%d%%d| %%s\n", maxLineNumWidth)

	for i := startLine - 1; i < endLine && i < len(lines); i++ {
		line := lines[i]
		// Truncate very long lines
		if len(line) > 200 {
			line = line[:200] + "..."
		}
		result.WriteString(fmt.Sprintf(lineNumFormat, i+1, line))
	}

	// Add truncation notice if applicable
	if startLine > 1 || endLine < totalLines {
		result.WriteString(fmt.Sprintf("\n[Showing lines %d-%d of %d]", startLine, endLine, totalLines))
	}

	return result.String(), nil
}

func (v View) viewDirectory(ctx context.Context, dir string, maxDepth int, cfg *config.Config) (string, error) {
	var result strings.Builder
	result.WriteString(fmt.Sprintf("// Directory: %s\n\n", dir))

	entries, err := v.collectDirectoryEntries(ctx, dir, 0, maxDepth, cfg)
	if err != nil {
		return "", err
	}

	if len(entries) == 0 {
		result.WriteString("(empty directory)")
		return result.String(), nil
	}

	result.WriteString(strings.Join(entries, "\n"))
	return result.String(), nil
}

func (v View) collectDirectoryEntries(ctx context.Context, dir string, currentDepth, maxDepth int, cfg *config.Config) ([]string, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var result []string
	indent := strings.Repeat("  ", currentDepth)

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files and common ignore patterns
		if strings.HasPrefix(name, ".") {
			continue
		}

		// Skip common build/output directories
		if entry.IsDir() {
			if name == "node_modules" || name == "vendor" || name == "dist" || name == "build" || name == "target" || name == "__pycache__" || name == ".git" {
				result = append(result, fmt.Sprintf("%s📁 %s/ (skipped)", indent, name))
				continue
			}
		}

		fullPath := filepath.Join(dir, name)

		// Check read permission
		if !cfg.CheckReadPermission(fullPath) {
			continue
		}

		if entry.IsDir() {
			result = append(result, fmt.Sprintf("%s📁 %s/", indent, name))
			if currentDepth < maxDepth {
				subEntries, err := v.collectDirectoryEntries(ctx, fullPath, currentDepth+1, maxDepth, cfg)
				if err != nil {
					continue // Skip directories we can't read
				}
				result = append(result, subEntries...)
			}
		} else {
			// Show file icon based on extension
			icon := v.getFileIcon(name)
			result = append(result, fmt.Sprintf("%s%s %s", indent, icon, name))
		}
	}

	return result, nil
}

func (v View) getFileIcon(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".go":
		return "🐹"
	case ".py":
		return "🐍"
	case ".js", ".ts", ".jsx", ".tsx":
		return "📜"
	case ".md":
		return "📝"
	case ".json", ".yaml", ".yml", ".toml":
		return "⚙️"
	case ".sh", ".bash", ".zsh":
		return "🐚"
	case ".dockerfile", ".dockerignore":
		return "🐳"
	case ".mod", ".sum":
		return "📦"
	case ".html", ".css":
		return "🌐"
	case ".test", "_test.go":
		return "🧪"
	default:
		return "📄"
	}
}
