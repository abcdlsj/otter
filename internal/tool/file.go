package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type File struct{}

func (File) Name() string { return "file" }
func (File) Desc() string { return "Read, write, list, or search files" }
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
		},
		"required": []string{"action", "path"},
	}
}

func (File) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Action  string `json:"action"`
		Path    string `json:"path"`
		Content string `json:"content"`
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}

	switch args.Action {
	case "read":
		data, err := os.ReadFile(args.Path)
		if err != nil {
			return "", err
		}
		return string(data), nil

	case "write":
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
