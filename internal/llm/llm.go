package llm

import (
	"context"
	"fmt"

	"github.com/abcdlsj/otter/internal/config"
	"github.com/tmc/langchaingo/llms"
)

type Message struct {
	Role             string
	Content          string
	ReasoningContent string
	ToolCalls        []ToolCall
	ToolResults      []ToolResult
}

type ToolCall struct {
	ID   string
	Name string
	Args string
}

type ToolResult struct {
	ToolCallID string
	Content    string
}

type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
}

type Response struct {
	Content          string
	ReasoningContent string
	ToolCalls        []ToolCall
	StopReason       string
}

type StreamChunk struct {
	Content string
	Error   error
}

type Provider interface {
	Chat(ctx context.Context, lg Logger, messages []Message, tools []Tool, toolResults []ToolResult) (*Response, error)
	ChatStream(ctx context.Context, lg Logger, messages []Message, tools []Tool, toolResults []ToolResult) (<-chan StreamChunk, <-chan *Response)
}

type Logger interface {
	WriteJSON(filename string, data []byte) error
	Debug(msg string, keyvals ...any)
	Info(msg string, keyvals ...any)
	Warn(msg string, keyvals ...any)
	Error(msg string, keyvals ...any)
}

type LLM struct {
	provider Provider
}

func New() (*LLM, error) {
	provider, err := CreateProviderFromConfig()
	if err != nil {
		return nil, err
	}
	return &LLM{provider: provider}, nil
}

func CreateProviderFromConfig() (Provider, error) {
	p := config.C.CurrentProvider()
	if p == nil {
		return nil, fmt.Errorf("no provider configured")
	}

	m := config.C.CurrentModel()
	if m == nil {
		return nil, fmt.Errorf("no model configured")
	}

	switch p.Name {
	case "anthropic", "claude":
		return NewAnthropicProvider(p.APIKey, m.Name, p.BaseURL)
	case "openai":
		return NewOpenAIProvider(p.APIKey, m.Name, p.BaseURL, p.Headers)
	default:
		return nil, fmt.Errorf("unknown provider: %s", p.Name)
	}
}

func (l *LLM) Chat(ctx context.Context, lg Logger, messages []Message, tools []Tool, toolResults []ToolResult) (*Response, error) {
	return l.provider.Chat(ctx, lg, messages, tools, toolResults)
}

func (l *LLM) ChatStream(ctx context.Context, lg Logger, messages []Message, tools []Tool, toolResults []ToolResult) (<-chan StreamChunk, <-chan *Response) {
	return l.provider.ChatStream(ctx, lg, messages, tools, toolResults)
}

func FromLangchainMessages(msgs []llms.MessageContent) []Message {
	var messages []Message
	for _, m := range msgs {
		msg := Message{
			Role: convertRole(m.Role),
		}

		for _, part := range m.Parts {
			switch p := part.(type) {
			case llms.TextContent:
				msg.Content += p.Text
			case llms.ToolCall:
				msg.ToolCalls = append(msg.ToolCalls, ToolCall{
					ID:   p.ID,
					Name: p.FunctionCall.Name,
					Args: p.FunctionCall.Arguments,
				})
			}
		}

		messages = append(messages, msg)
	}
	return messages
}

func FromLangchainTools(tools []llms.Tool) []Tool {
	var result []Tool
	for _, t := range tools {
		schema, _ := t.Function.Parameters.(map[string]any)
		result = append(result, Tool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: schema,
		})
	}
	return result
}

func convertRole(role llms.ChatMessageType) string {
	switch role {
	case llms.ChatMessageTypeHuman:
		return "user"
	case llms.ChatMessageTypeAI:
		return "assistant"
	case llms.ChatMessageTypeSystem:
		return "system"
	case llms.ChatMessageTypeTool:
		return "tool"
	default:
		return "user"
	}
}
