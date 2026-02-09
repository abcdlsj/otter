package tool

import (
	"context"
	"encoding/json"
)

type Compact struct{}

func (Compact) Name() string { return "compact" }
func (Compact) Desc() string {
	return "Request the agent to compact the conversation history to reduce token usage. Use this when the conversation is getting long and you want to free up context space."
}
func (Compact) Args() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (Compact) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	// The actual compact logic is handled by the agent
	// This tool serves as a signal to trigger compacting
	return "Compaction requested. The agent will summarize older messages to reduce token usage.", nil
}
