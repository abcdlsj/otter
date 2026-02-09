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
	return "View file contents or directory structure. For files: displays content with line numbers. For directories: shows tree-like structure."
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
	args.Path = filepath.Clean(args.Path)

	cfg := &config.C
	if !cfg.CheckReadPermission(args.Path) {
		return "", fmt.Errorf("permission denied: cannot read %s", args.Path)
	}

	info, err := os.Stat(args.Path)
	if err != nil {
		return "", fmt.Errorf("cannot access path: %w", err)
	}

	if info.IsDir() {
		if args.Depth <= 0 {
			args.Depth = 3
		}
		if args.Depth > 5 {
			args.Depth = 5
		}
		return v.viewDir(ctx, args.Path, args.Depth, cfg)
	}
	return v.viewFile(args.Path, args.ViewRange)
}

func (v View) viewFile(path string, viewRange []int) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	totalLines := len(lines)

	startLine, endLine := 1, totalLines
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

	var b strings.Builder
	fmt.Fprintf(&b, "// %s (%d lines total)\n", path, totalLines)

	maxWidth := len(fmt.Sprintf("%d", endLine))
	format := fmt.Sprintf("%%%dd| %%s\n", maxWidth)

	for i := startLine - 1; i < endLine && i < len(lines); i++ {
		line := lines[i]
		if len(line) > 200 {
			line = line[:200] + "..."
		}
		fmt.Fprintf(&b, format, i+1, line)
	}

	if startLine > 1 || endLine < totalLines {
		fmt.Fprintf(&b, "\n[Showing lines %d-%d of %d]", startLine, endLine, totalLines)
	}
	return b.String(), nil
}

func (v View) viewDir(ctx context.Context, dir string, maxDepth int, cfg *config.Config) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "// Directory: %s\n\n", dir)

	entries, err := v.collectEntries(ctx, dir, 0, maxDepth, cfg)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		b.WriteString("(empty directory)")
		return b.String(), nil
	}

	for _, e := range entries {
		b.WriteString(e)
		b.WriteByte('\n')
	}
	return b.String(), nil
}

type dirEntry struct {
	name     string
	path     string
	isDir    bool
	depth    int
	skipped  bool
}

func (v View) collectEntries(ctx context.Context, dir string, depth, maxDepth int, cfg *config.Config) ([]string, error) {
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
	indent := strings.Repeat("  ", depth)

	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		fullPath := filepath.Join(dir, name)
		if !cfg.CheckReadPermission(fullPath) {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		if e.IsDir() {
			// Skip common directories
			switch name {
			case "node_modules", "vendor", "dist", "build", "target", "__pycache__", ".git":
				result = append(result, fmt.Sprintf("%s%s/ (skipped)", indent, name))
				continue
			}
			result = append(result, fmt.Sprintf("%s%s/", indent, name))
			if depth < maxDepth {
				sub, err := v.collectEntries(ctx, fullPath, depth+1, maxDepth, cfg)
				if err == nil {
					result = append(result, sub...)
				}
			}
		} else {
			size := formatSize(info.Size())
			result = append(result, fmt.Sprintf("%s%s (%s)", indent, name, size))
		}
	}
	return result, nil
}

func formatSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.2f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
