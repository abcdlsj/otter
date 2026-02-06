package llm

import (
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestToOpenAIMessage(t *testing.T) {
	p := &OpenAIProvider{}

	tests := []struct {
		name string
		msg  Message
		want openai.ChatCompletionMessage
	}{
		{
			name: "user message",
			msg:  Message{Role: "user", Content: "hello"},
			want: openai.ChatCompletionMessage{Role: "user", Content: "hello"},
		},
		{
			name: "assistant with reasoning",
			msg: Message{
				Role:             "assistant",
				Content:          "result",
				ReasoningContent: "let me think...",
			},
			want: openai.ChatCompletionMessage{
				Role:             "assistant",
				Content:          "result",
				ReasoningContent: "let me think...",
			},
		},
		{
			name: "tool message",
			msg: Message{
				Role:        "tool",
				ToolResults: []ToolResult{{ToolCallID: "call_123", Content: "file content"}},
			},
			want: openai.ChatCompletionMessage{
				Role:       "tool",
				Content:    "file content",
				ToolCallID: "call_123",
			},
		},
		{
			name: "assistant with tool calls",
			msg: Message{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ToolCall{
					{ID: "call_1", Name: "file", Args: `{"action":"list"}`},
				},
			},
			want: openai.ChatCompletionMessage{
				Role: "assistant",
				ToolCalls: []openai.ToolCall{
					{ID: "call_1", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{
						Name:      "file",
						Arguments: `{"action":"list"}`,
					}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.toOpenAIMessage(tt.msg)
			if got.Role != tt.want.Role {
				t.Errorf("Role = %q, want %q", got.Role, tt.want.Role)
			}
			if got.Content != tt.want.Content {
				t.Errorf("Content = %q, want %q", got.Content, tt.want.Content)
			}
			if got.ReasoningContent != tt.want.ReasoningContent {
				t.Errorf("ReasoningContent = %q, want %q", got.ReasoningContent, tt.want.ReasoningContent)
			}
			if got.ToolCallID != tt.want.ToolCallID {
				t.Errorf("ToolCallID = %q, want %q", got.ToolCallID, tt.want.ToolCallID)
			}
			if len(got.ToolCalls) != len(tt.want.ToolCalls) {
				t.Errorf("ToolCalls len = %d, want %d", len(got.ToolCalls), len(tt.want.ToolCalls))
			}
		})
	}
}

func TestBuildChatRequest(t *testing.T) {
	p := &OpenAIProvider{model: "gpt-4"}

	msgs := []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi", ReasoningContent: "thinking"},
	}
	tools := []Tool{
		{Name: "file", Description: "file tool", InputSchema: map[string]any{"type": "object"}},
	}

	req := p.buildChatRequest(msgs, tools)

	if req.Model != "gpt-4" {
		t.Errorf("Model = %q, want gpt-4", req.Model)
	}
	if len(req.Messages) != 2 {
		t.Errorf("Messages len = %d, want 2", len(req.Messages))
	}
	if len(req.Tools) != 1 {
		t.Errorf("Tools len = %d, want 1", len(req.Tools))
	}

	// Check reasoning_content is preserved
	if req.Messages[1].ReasoningContent != "thinking" {
		t.Errorf("ReasoningContent = %q, want thinking", req.Messages[1].ReasoningContent)
	}
}
