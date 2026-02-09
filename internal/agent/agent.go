package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/abcdlsj/otter/internal/config"
	"github.com/abcdlsj/otter/internal/event"
	"github.com/abcdlsj/otter/internal/llm"
	"github.com/abcdlsj/otter/internal/logger"
	"github.com/abcdlsj/otter/internal/prompt"
	"github.com/abcdlsj/otter/internal/tool"
	"github.com/abcdlsj/otter/internal/types"
)

type Agent struct {
	llm      *llm.LLM
	tools    *tool.Set
	maxSteps int
	mode     string
}

func New(l *llm.LLM, t *tool.Set) *Agent {
	return &Agent{
		llm:      l,
		tools:    t,
		maxSteps: config.C.MaxSteps,
		mode:     "default",
	}
}

// NewWithMode creates an agent with a specific mode (e.g., "plan", "explore")
func NewWithMode(l *llm.LLM, t *tool.Set, mode string) *Agent {
	return &Agent{
		llm:      l,
		tools:    t,
		maxSteps: config.C.MaxSteps,
		mode:     mode,
	}
}

func (a *Agent) Run(ctx context.Context, lg logger.Logger, history []llm.Message, input string) <-chan event.Event {
	ch := make(chan event.Event, 64)

	go func() {
		defer close(ch)

		messages := a.buildMessages(ctx, lg, ch, history, input)
		tools := llm.FromLangchainTools(a.tools.ToLangchain())
		var fullText strings.Builder
		var newMsgs []event.Message

		for step := 0; step < a.maxSteps; step++ {
			select {
			case <-ctx.Done():
				ch <- event.Event{Type: event.Error, Data: event.ErrorData{Message: "cancelled"}}
				return
			default:
			}

			resp := a.chat(ctx, lg, messages, tools, ch)
			if resp == nil {
				return
			}

			if resp.Content != "" {
				fullText.WriteString(resp.Content)
			}

			newMsgs = append(newMsgs, event.Message{
				Role:      "assistant",
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			})

			messages = append(messages, llm.Message{
				Role:             "assistant",
				Content:          resp.Content,
				ReasoningContent: resp.ReasoningContent,
				ToolCalls:        resp.ToolCalls,
			})

			if len(resp.ToolCalls) == 0 {
				ch <- event.Event{Type: event.Done, Data: event.DoneData{
					FullText:     fullText.String(),
					InputTokens:  resp.InputTokens,
					OutputTokens: resp.OutputTokens,
					Messages:     newMsgs,
				}}
				return
			}

			results := a.runTools(ctx, resp.ToolCalls, ch)
			if len(results) > 0 {
				newMsgs = append(newMsgs, event.Message{
					Role:        "tool",
					ToolResults: results,
				})
				messages = append(messages, llm.Message{
					Role:        "tool",
					ToolResults: results,
				})
			}
		}

		ch <- event.Event{Type: event.Error, Data: event.ErrorData{Message: "max steps reached"}}
	}()

	return ch
}

func (a *Agent) buildMessages(ctx context.Context, lg logger.Logger, ch chan event.Event, history []llm.Message, input string) []llm.Message {
	messages := make([]llm.Message, 0, len(history)+2)
	messages = append(messages, llm.Message{
		Role:    "system",
		Content: a.systemPrompt(),
	})
	messages = append(messages, history...)
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: input,
	})

	messages = a.maybeCompact(ctx, lg, ch, messages)
	return messages
}

func (a *Agent) chat(ctx context.Context, lg logger.Logger, messages []llm.Message, tools []llm.Tool, ch chan event.Event) *llm.Response {
	if config.C.Stream {
		chunkCh, respCh := a.llm.ChatStream(ctx, lg, messages, tools, nil)
		for chunk := range chunkCh {
			if chunk.Error != nil {
				ch <- event.Event{Type: event.Error, Data: event.ErrorData{Message: chunk.Error.Error()}}
				return nil
			}
			if chunk.Content != "" {
				ch <- event.Event{Type: event.TextDelta, Data: event.TextDeltaData{Text: chunk.Content}}
			}
		}
		return <-respCh
	}

	resp, err := a.llm.Chat(ctx, lg, messages, tools, nil)
	if err != nil {
		ch <- event.Event{Type: event.Error, Data: event.ErrorData{Message: err.Error()}}
		return nil
	}
	if resp != nil && resp.Content != "" {
		ch <- event.Event{Type: event.TextDelta, Data: event.TextDeltaData{Text: resp.Content}}
	}
	return resp
}

func (a *Agent) runTools(ctx context.Context, calls []types.ToolCall, ch chan event.Event) []types.ToolResult {
	var results []types.ToolResult
	for _, tc := range calls {
		ch <- event.Event{
			Type: event.ToolStart,
			Data: event.ToolStartData{
				ID:   tc.ID,
				Name: tc.Name,
				Args: tc.Args,
			},
		}

		t := a.tools.Get(tc.Name)
		if t == nil {
			a.sendToolEnd(ch, tc.ID, tc.Name, "", "unknown tool")
			results = append(results, types.ToolResult{
				ToolCallID: tc.ID,
				Content:    "error: unknown tool",
			})
			continue
		}

		result, err := t.Run(ctx, json.RawMessage(tc.Args))
		if err != nil {
			a.sendToolEnd(ch, tc.ID, tc.Name, "", err.Error())
			results = append(results, types.ToolResult{
				ToolCallID: tc.ID,
				Content:    "error: " + err.Error(),
			})
			continue
		}

		if len(result) > maxToolResultLen {
			result = result[:maxToolResultLen] + "\n... (truncated)"
		}

		a.sendToolEnd(ch, tc.ID, tc.Name, result, "")
		results = append(results, types.ToolResult{
			ToolCallID: tc.ID,
			Content:    result,
		})
	}
	return results
}

func (a *Agent) sendToolEnd(ch chan event.Event, id, name, result, err string) {
	ch <- event.Event{
		Type: event.ToolEnd,
		Data: event.ToolEndData{
			ID:     id,
			Name:   name,
			Result: result,
			Error:  err,
		},
	}
}

func (a *Agent) GenerateTitle(ctx context.Context, lg logger.Logger, text string) (string, error) {
	messages := []llm.Message{
		{Role: "system", Content: "Generate a very short title (max 15 chars) for this conversation in English. Reply with ONLY the title, no quotes, no explanation."},
		{Role: "user", Content: text},
	}
	resp, err := a.llm.Chat(ctx, lg, messages, nil, nil)
	if err != nil {
		return "", err
	}
	title := types.TruncateRunes(strings.TrimSpace(resp.Content), maxTitleLen)
	return title, nil
}

const (
	compactThreshold   = 60000
	compactKeepRecent  = 6
	maxToolResultLen   = 4000
	maxTitleLen        = 20
	maxSummaryContent  = 500
)

func (a *Agent) systemPrompt() string {
	return prompt.Load(a.tools, a.maxSteps, a.mode)
}

// SetMode changes the agent's mode dynamically
func (a *Agent) SetMode(mode string) {
	a.mode = mode
}

// Mode returns the current mode
func (a *Agent) Mode() string {
	return a.mode
}

func (a *Agent) maybeCompact(ctx context.Context, lg logger.Logger, ch chan event.Event, messages []llm.Message) []llm.Message {
	tokens := llm.EstimateMessagesTokens(messages, config.C.CurrentModelName())
	if tokens < compactThreshold {
		return messages
	}

	lg.Info("auto-compact triggered", "tokens", tokens, "threshold", compactThreshold)
	ch <- event.Event{Type: event.CompactStart, Data: event.CompactStartData{
		Tokens:    tokens,
		Threshold: compactThreshold,
	}}

	if len(messages) <= compactKeepRecent+1 {
		return messages
	}

	sys := messages[0]
	recent := messages[len(messages)-compactKeepRecent:]
	toCompact := messages[1 : len(messages)-compactKeepRecent]

	summary, err := a.summarize(ctx, lg, toCompact)
	if err != nil {
		lg.Warn("compact failed, using full history", "err", err)
		return messages
	}

	result := make([]llm.Message, 0, compactKeepRecent+2)
	result = append(result, sys)
	result = append(result, llm.Message{
		Role:    "user",
		Content: "[Previous conversation summary]\n" + summary,
	})
	result = append(result, recent...)

	newTokens := llm.EstimateMessagesTokens(result, config.C.CurrentModelName())
	lg.Info("compact done", "before", tokens, "after", newTokens, "summarized_msgs", len(toCompact))
	ch <- event.Event{Type: event.CompactEnd, Data: event.CompactEndData{
		Before: tokens,
		After:  newTokens,
	}}
	return result
}

func (a *Agent) summarize(ctx context.Context, lg logger.Logger, messages []llm.Message) (string, error) {
	var sb strings.Builder
	for _, m := range messages {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, m.Content))
		for _, tc := range m.ToolCalls {
			sb.WriteString(fmt.Sprintf("[tool_call %s]: %s(%s)\n", tc.ID, tc.Name, tc.Args))
		}
		for _, tr := range m.ToolResults {
			content := types.TruncateRunes(tr.Content, maxSummaryContent)
			if len(content) < len(tr.Content) {
				content += "..."
			}
			sb.WriteString(fmt.Sprintf("[tool_result %s]: %s\n", tr.ToolCallID, content))
		}
	}

	prompt := []llm.Message{
		{Role: "system", Content: "Summarize the following conversation concisely. Preserve: key decisions, important file paths and code changes, current task context. Be brief but complete. Output only the summary."},
		{Role: "user", Content: sb.String()},
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := a.llm.Chat(ctx, lg, prompt, nil, nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}
