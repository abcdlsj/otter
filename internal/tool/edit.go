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

// Edit 工具用于精确编辑文件内容，通过查找并替换特定文本
type Edit struct{}

func (Edit) Name() string { return "edit" }
func (Edit) Desc() string {
	return "Edit a file by replacing exact text. The oldText must match exactly (including whitespace). Use this for precise, surgical edits."
}
func (Edit) Args() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to edit",
			},
			"oldText": map[string]any{
				"type":        "string",
				"description": "Exact text to find and replace (must match exactly including whitespace)",
			},
			"newText": map[string]any{
				"type":        "string",
				"description": "New text to replace the old text with",
			},
		},
		"required": []string{"path", "oldText", "newText"},
	}
}

func (e Edit) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Path    string `json:"path"`
		OldText string `json:"oldText"`
		NewText string `json:"newText"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if args.Path == "" {
		return "", fmt.Errorf("path is required")
	}
	if args.OldText == "" {
		return "", fmt.Errorf("oldText is required")
	}

	// Clean the path
	args.Path = filepath.Clean(args.Path)

	// Check write permission
	cfg := &config.C
	if !cfg.CheckWritePermission(args.Path) {
		if cfg.Security.Readonly {
			return "", fmt.Errorf("permission denied: readonly mode is enabled")
		}
		return "", fmt.Errorf("permission denied: cannot write to %s", args.Path)
	}

	// Check if file exists
	info, err := os.Stat(args.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", args.Path)
		}
		return "", fmt.Errorf("cannot access file: %w", err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", args.Path)
	}

	// Read file content
	content, err := os.ReadFile(args.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	oldContent := string(content)

	// Check if oldText exists in the file
	if !strings.Contains(oldContent, args.OldText) {
		// Provide helpful context about what was found
		const contextLen = 100
		preview := oldContent
		if len(preview) > contextLen {
			preview = preview[:contextLen] + "..."
		}
		preview = strings.ReplaceAll(preview, "\n", "\\n")
		return "", fmt.Errorf("oldText not found in file. The text must match exactly (including whitespace and indentation). File starts with: %s", preview)
	}

	// Count occurrences
	occurrences := strings.Count(oldContent, args.OldText)
	if occurrences > 1 {
		return "", fmt.Errorf("oldText appears %d times in the file. Please provide more context to make it unique", occurrences)
	}

	// Perform the replacement
	newContent := strings.Replace(oldContent, args.OldText, args.NewText, 1)

	// Check for destructive changes
	if cfg.Security.ConfirmDestructive {
		// In confirm_destructive mode, we could add additional checks here
		// For now, we proceed with the edit
	}

	// Write the modified content back
	if err := os.WriteFile(args.Path, []byte(newContent), info.Mode()); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Calculate line numbers for the report
	linesBefore := strings.Count(oldContent[:strings.Index(oldContent, args.OldText)], "\n") + 1
	oldLines := strings.Count(args.OldText, "\n")
	newLines := strings.Count(args.NewText, "\n")

	result := fmt.Sprintf("✓ Edited %s\n", args.Path)
	result += fmt.Sprintf("  Replaced %d line(s) at line %d with %d line(s)", oldLines+1, linesBefore, newLines+1)

	return result, nil
}
