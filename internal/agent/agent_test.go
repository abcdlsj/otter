package agent

import (
	"context"
	"testing"

	"github.com/abcdlsj/otter/internal/event"
	"github.com/abcdlsj/otter/internal/llm"
	"github.com/abcdlsj/otter/internal/tool"
)

type mockProvider struct {
	responses []*llm.Response
	callIdx   int
}

func (m *mockProvider) Chat(ctx context.Context, lg llm.Logger, messages []llm.Message, tools []llm.Tool, toolResults []llm.ToolResult) (*llm.Response, error) {
	if m.callIdx >= len(m.responses) {
		return &llm.Response{Content: "done"}, nil
	}
	resp := m.responses[m.callIdx]
	m.callIdx++
	return resp, nil
}

func (m *mockProvider) ChatStream(ctx context.Context, lg llm.Logger, messages []llm.Message, tools []llm.Tool, toolResults []llm.ToolResult) (<-chan llm.StreamChunk, <-chan *llm.Response) {
	chunkCh := make(chan llm.StreamChunk, 1)
	respCh := make(chan *llm.Response, 1)

	go func() {
		defer close(chunkCh)
		defer close(respCh)

		if m.callIdx < len(m.responses) {
			resp := m.responses[m.callIdx]
			m.callIdx++
			if resp.Content != "" {
				chunkCh <- llm.StreamChunk{Content: resp.Content}
			}
			respCh <- resp
		} else {
			respCh <- &llm.Response{Content: "done"}
		}
	}()

	return chunkCh, respCh
}

func TestAgentToolCalling(t *testing.T) {
	toolSet := tool.NewSet()

	llmClient := &llm.LLM{}
	agent := New(llmClient, toolSet)

	t.Run("tool result message format", func(t *testing.T) {
		messages := []llm.Message{
			{Role: "user", Content: "run ls"},
		}

		resp := &llm.Response{
			Content: "",
			ToolCalls: []llm.ToolCall{
				{ID: "call_123", Name: "shell", Args: `{"cmd":"ls -la"}`},
			},
		}

		messages = append(messages, llm.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		results := agent.runTools(context.Background(), resp.ToolCalls, make(chan event.Event, 10))

		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}

		if results[0].ToolCallID != "call_123" {
			t.Errorf("ToolCallID = %q, want call_123", results[0].ToolCallID)
		}

		messages = append(messages, llm.Message{
			Role:        "tool",
			ToolResults: results,
		})

		if len(messages) != 3 {
			t.Fatalf("expected 3 messages, got %d", len(messages))
		}

		if messages[2].Role != "tool" {
			t.Errorf("message[2].Role = %q, want tool", messages[2].Role)
		}
	})
}

func TestToolArgsParsing(t *testing.T) {
	shell := tool.Shell{}

	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{
			name:    "valid args",
			args:    `{"cmd":"ls -la"}`,
			wantErr: false,
		},
		{
			name:    "empty args",
			args:    "",
			wantErr: true,
		},
		{
			name:    "invalid json",
			args:    "not json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := shell.Run(context.Background(), []byte(tt.args))
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
