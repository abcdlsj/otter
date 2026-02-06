package tool

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"
)

type Shell struct{}

func (Shell) Name() string { return "shell" }
func (Shell) Desc() string { return "Execute shell command" }
func (Shell) Args() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"cmd": map[string]any{
				"type":        "string",
				"description": "command to run",
			},
		},
		"required": []string{"cmd"},
	}
}

func (Shell) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Cmd     string `json:"cmd"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}
	if args.Timeout == 0 {
		args.Timeout = 30
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(args.Timeout)*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "sh", "-c", args.Cmd).CombinedOutput()
	if err != nil {
		return string(out) + "\nerror: " + err.Error(), nil
	}
	return string(out), nil
}
