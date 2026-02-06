package agent

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/abcdlsj/otter/internal/config"
	"github.com/abcdlsj/otter/internal/event"
	"github.com/abcdlsj/otter/internal/llm"
	"github.com/abcdlsj/otter/internal/msg"
	"github.com/abcdlsj/otter/internal/tool"
)

type Agent struct {
	llm      *llm.LLM
	tools    *tool.Set
	maxSteps int
}

func New(l *llm.LLM, t *tool.Set) *Agent {
	return &Agent{
		llm:      l,
		tools:    t,
		maxSteps: config.C.MaxSteps,
	}
}

func (a *Agent) Run(ctx context.Context, sessionID string, history []msg.Msg, input string) <-chan event.Event {
	ch := make(chan event.Event, 64)

	go func() {
		defer close(ch)

		messages := a.buildMessages(history, input)
		tools := llm.FromLangchainTools(a.tools.ToLangchain())
		var fullText strings.Builder

		for step := 0; step < a.maxSteps; step++ {
			select {
			case <-ctx.Done():
				ch <- event.Event{Type: event.Error, Data: event.ErrorData{Message: "cancelled"}}
				return
			default:
			}

			resp := a.chat(ctx, messages, tools, ch)
			if resp == nil {
				return
			}

			if resp.Content != "" {
				fullText.WriteString(resp.Content)
			}

			if len(resp.ToolCalls) == 0 {
				ch <- event.Event{Type: event.Done, Data: event.DoneData{FullText: fullText.String()}}
				return
			}

			messages = append(messages, llm.Message{
				Role:             "assistant",
				Content:          resp.Content,
				ReasoningContent: resp.ReasoningContent,
				ToolCalls:        resp.ToolCalls,
			})

			results := a.runTools(ctx, resp.ToolCalls, ch)
			if len(results) > 0 {
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

func (a *Agent) buildMessages(history []msg.Msg, input string) []llm.Message {
	var messages []llm.Message

	messages = append(messages, llm.Message{
		Role:    "system",
		Content: systemPrompt,
	})

	for _, m := range history {
		messages = append(messages, llm.Message{
			Role:    m.Role,
			Content: m.Text,
		})
	}

	messages = append(messages, llm.Message{
		Role:    "user",
		Content: input,
	})

	return messages
}

func (a *Agent) chat(ctx context.Context, messages []llm.Message, tools []llm.Tool, ch chan event.Event) *llm.Response {
	if config.C.Stream {
		chunkCh, respCh := a.llm.ChatStream(ctx, nil, messages, tools, nil)
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

	resp, err := a.llm.Chat(ctx, nil, messages, tools, nil)
	if err != nil {
		ch <- event.Event{Type: event.Error, Data: event.ErrorData{Message: err.Error()}}
		return nil
	}
	if resp != nil && resp.Content != "" {
		ch <- event.Event{Type: event.TextDelta, Data: event.TextDeltaData{Text: resp.Content}}
	}
	return resp
}

func (a *Agent) runTools(ctx context.Context, calls []llm.ToolCall, ch chan event.Event) []llm.ToolResult {
	var results []llm.ToolResult
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
			ch <- event.Event{
				Type: event.ToolEnd,
				Data: event.ToolEndData{
					ID:    tc.ID,
					Name:  tc.Name,
					Error: "unknown tool",
				},
			}
			results = append(results, llm.ToolResult{
				ToolCallID: tc.ID,
				Content:    "error: unknown tool",
			})
			continue
		}

		result, err := t.Run(ctx, json.RawMessage(tc.Args))
		if err != nil {
			ch <- event.Event{
				Type: event.ToolEnd,
				Data: event.ToolEndData{
					ID:    tc.ID,
					Name:  tc.Name,
					Error: err.Error(),
				},
			}
			results = append(results, llm.ToolResult{
				ToolCallID: tc.ID,
				Content:    "error: " + err.Error(),
			})
			continue
		}

		if len(result) > 4000 {
			result = result[:4000] + "\n... (truncated)"
		}

		ch <- event.Event{
			Type: event.ToolEnd,
			Data: event.ToolEndData{
				ID:     tc.ID,
				Name:   tc.Name,
				Result: result,
			},
		}
		results = append(results, llm.ToolResult{
			ToolCallID: tc.ID,
			Content:    result,
		})
	}
	return results
}

const systemPrompt = `You are a helpful AI coding assistant. You have access to tools that can help you complete tasks.

When given a task:
1. Analyze what needs to be done
2. Use the available tools to gather information or make changes
3. Provide clear, concise responses

Available tools:
- shell: Execute shell commands
- file: Read, write, list, or search files

Always be helpful and efficient. If you need to make changes, use the file tool. If you need to run commands, use the shell tool.`
