package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/abcdlsj/otter/internal/config"
)

const (
	defaultReadLimit = 2000
	maxLineLength    = 2000
	maxBytes         = 50 * 1024
)

type File struct{}

func (File) Name() string { return "file" }
func (File) Desc() string { return "Read, write, list, or search files (respects security settings)" }
func (File) Args() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type": "string",
				"enum": []string{"read", "write", "list", "search"},
			},
			"path": map[string]any{
				"type": "string",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "content for write",
			},
			"pattern": map[string]any{
				"type":        "string",
				"description": "search pattern (regex) for search action",
			},
			"offset": map[string]any{
				"type":        "number",
				"description": "line number to start reading from (0-based)",
			},
			"limit": map[string]any{
				"type":        "number",
				"description": fmt.Sprintf("number of lines to read (defaults to %d)", defaultReadLimit),
			},
		},
		"required": []string{"action", "path"},
	}
}

func (f File) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Action  string `json:"action"`
		Path    string `json:"path"`
		Content string `json:"content"`
		Pattern string `json:"pattern"`
		Offset  int    `json:"offset"`
		Limit   int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}

	// Clean the path
	args.Path = filepath.Clean(args.Path)

	// Check permissions based on action
	cfg := &config.C
	switch args.Action {
	case "read", "list", "search":
		if !cfg.CheckReadPermission(args.Path) {
			return "", fmt.Errorf("permission denied: cannot read %s", args.Path)
		}
	case "write":
		if !cfg.CheckWritePermission(args.Path) {
			if cfg.Security.Readonly {
				return "", fmt.Errorf("permission denied: readonly mode is enabled")
			}
			return "", fmt.Errorf("permission denied: cannot write to %s", args.Path)
		}
	}

	switch args.Action {
	case "read":
		return readFile(args.Path, args.Offset, args.Limit)

	case "write":
		// Additional safety: check if file exists (overwrite protection)
		if cfg.Security.ConfirmDestructive {
			if _, err := os.Stat(args.Path); err == nil {
				return "", fmt.Errorf("confirmation required: file %s already exists. Set confirm_destructive=false to allow overwrites", args.Path)
			}
		}
		if err := os.MkdirAll(filepath.Dir(args.Path), 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(args.Path, []byte(args.Content), 0644); err != nil {
			return "", err
		}
		return "ok", nil

	case "list":
		entries, err := os.ReadDir(args.Path)
		if err != nil {
			return "", err
		}
		var sb strings.Builder
		for _, e := range entries {
			if e.IsDir() {
				sb.WriteString("d ")
			} else {
				sb.WriteString("  ")
			}
			sb.WriteString(e.Name())
			sb.WriteString("\n")
		}
		return sb.String(), nil

	case "search":
		if args.Pattern == "" {
			return "", fmt.Errorf("pattern is required for search")
		}
		name, grepArgs := "rg", []string{"-n", "--no-heading", args.Pattern, args.Path}
		if _, err := exec.LookPath("rg"); err != nil {
			name, grepArgs = "grep", []string{"-rn", args.Pattern, args.Path}
		}
		out, err := exec.CommandContext(ctx, name, grepArgs...).CombinedOutput()
		if err != nil {
			if len(out) == 0 {
				return "no matches found", nil
			}
			return string(out), nil
		}
		return string(out), nil
	}

	return "", fmt.Errorf("unknown action: %s", args.Action)
}

func readFile(path string, offset, limit int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if limit <= 0 {
		limit = defaultReadLimit
	}

	scanner := bufio.NewScanner(f)
	lineNum := 0
	lines := []string{}
	bytes := 0

	for scanner.Scan() {
		if lineNum < offset {
			lineNum++
			continue
		}
		if len(lines) >= limit {
			break
		}

		line := scanner.Text()
		if len([]rune(line)) > maxLineLength {
			line = string([]rune(line)[:maxLineLength]) + "..."
		}

		lineBytes := len(line) + 1
		if bytes+lineBytes > maxBytes {
			lines = append(lines, "...")
			break
		}
		bytes += lineBytes

		lines = append(lines, fmt.Sprintf("%d| %s", lineNum+1, line))
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	output := strings.Join(lines, "\n")
	if len(lines) >= limit || bytes >= maxBytes {
		output += fmt.Sprintf("\n\n(File has more lines. Use 'offset' parameter to read beyond line %d)", lineNum)
	}

	return output, nil
}
