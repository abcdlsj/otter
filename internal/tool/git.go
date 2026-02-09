package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type Git struct{}

func (Git) Name() string { return "git" }
func (Git) Desc() string {
	return "Execute git commands (status, diff, log, show). Provides safe, read-only git operations by default."
}
func (Git) Args() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "Git subcommand to execute: 'status', 'diff', 'log', 'show', 'branch'",
				"enum":        []string{"status", "diff", "log", "show", "branch"},
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Path to git repository (default: current directory)",
			},
			"args": map[string]any{
				"type":        "array",
				"description": "Additional arguments for the git command",
				"items": map[string]any{
					"type": "string",
				},
			},
			"max_lines": map[string]any{
				"type":        "number",
				"description": "Maximum number of lines to return (default: 200, max: 500)",
			},
		},
		"required": []string{"command"},
	}
}

func (g Git) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Command  string   `json:"command"`
		Path     string   `json:"path"`
		Args     []string `json:"args"`
		MaxLines int      `json:"max_lines"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}

	if args.Path == "" {
		args.Path = "."
	}
	if args.MaxLines <= 0 {
		args.MaxLines = 200
	}
	if args.MaxLines > 500 {
		args.MaxLines = 500
	}

	// Build git command
	cmdArgs := []string{"-C", args.Path}
	
	switch args.Command {
	case "status":
		cmdArgs = append(cmdArgs, "status", "--porcelain", "-b")
		cmdArgs = append(cmdArgs, args.Args...)
	case "diff":
		cmdArgs = append(cmdArgs, "diff", "--no-color")
		cmdArgs = append(cmdArgs, args.Args...)
	case "log":
		cmdArgs = append(cmdArgs, "log", "--oneline", "--no-decorate", "-n", "20")
		cmdArgs = append(cmdArgs, args.Args...)
	case "show":
		cmdArgs = append(cmdArgs, "show", "--no-color", "--stat")
		if len(args.Args) > 0 {
			cmdArgs = append(cmdArgs, args.Args...)
		} else {
			cmdArgs = append(cmdArgs, "HEAD")
		}
	case "branch":
		cmdArgs = append(cmdArgs, "branch", "-a")
		cmdArgs = append(cmdArgs, args.Args...)
	default:
		return "", fmt.Errorf("unsupported git command: %s", args.Command)
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	
	output := string(out)
	
	// Handle common errors gracefully
	if err != nil {
		// Check if it's a "not a git repository" error
		if strings.Contains(output, "not a git repository") {
			return "Error: not a git repository (or any parent up to mount point)", nil
		}
		// For other errors, return the output anyway as it often contains useful info
		if output == "" {
			return "", fmt.Errorf("git command failed: %w", err)
		}
	}

	// Truncate if too long
	lines := strings.Split(output, "\n")
	if len(lines) > args.MaxLines {
		lines = lines[:args.MaxLines]
		output = strings.Join(lines, "\n") + fmt.Sprintf("\n\n... (truncated, showing %d/%d lines)", args.MaxLines, len(lines))
	}

	// Format status output for better readability
	if args.Command == "status" {
		return g.formatStatus(output), nil
	}

	return output, nil
}

func (g Git) formatStatus(output string) string {
	if strings.HasPrefix(output, "Error:") {
		return output
	}

	lines := strings.Split(output, "\n")
	var b strings.Builder
	var staged, unstaged, untracked []string

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if strings.HasPrefix(line, "##") {
			b.WriteString("Branch: " + strings.TrimPrefix(line, "## ") + "\n")
			continue
		}
		if len(line) < 2 {
			continue
		}
		x, y, file := line[0], line[1], line[3:]
		switch {
		case x != ' ' && x != '?':
			staged = append(staged, file)
		case y != ' ':
			unstaged = append(unstaged, file)
		case x == '?':
			untracked = append(untracked, file)
		}
	}

	for _, f := range staged {
		fmt.Fprintf(&b, "+ %s\n", f)
	}
	for _, f := range unstaged {
		fmt.Fprintf(&b, "~ %s\n", f)
	}
	for _, f := range untracked {
		fmt.Fprintf(&b, "? %s\n", f)
	}

	if len(staged)+len(unstaged)+len(untracked) == 0 {
		b.WriteString("(clean)")
	}
	return b.String()
}
