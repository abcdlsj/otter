package llm

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/sashabaranov/go-openai"
)

type OpenAIProvider struct {
	client *openai.Client
	model  string
}

type headerRoundTripper struct {
	headers map[string]string
	base    http.RoundTripper
}

func (t *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	return t.base.RoundTrip(req)
}

func NewOpenAIProvider(apiKey, model, baseURL string, headers map[string]string) (*OpenAIProvider, error) {
	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	if len(headers) > 0 {
		config.HTTPClient = &http.Client{
			Transport: &headerRoundTripper{
				headers: headers,
				base:    http.DefaultTransport,
			},
		}
	}

	client := openai.NewClientWithConfig(config)
	return &OpenAIProvider{
		client: client,
		model:  model,
	}, nil
}

func (p *OpenAIProvider) Chat(ctx context.Context, lg Logger, messages []Message, tools []Tool, toolResults []ToolResult) (*Response, error) {
	req := p.buildChatRequest(messages, tools)

	if lg != nil {
		lg.Debug("openai request", "messages", len(messages), "tools", len(tools))
	}

	resp, err := p.client.CreateChatCompletion(ctx, req)
	if err != nil {
		if lg != nil {
			lg.Error("openai request failed", "error", err)
		}
		return nil, err
	}

	if lg != nil {
		lg.Info("openai response received")
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices")
	}

	choice := resp.Choices[0]
	response := &Response{
		Content:          choice.Message.Content,
		ReasoningContent: choice.Message.ReasoningContent,
	}

	for _, tc := range choice.Message.ToolCalls {
		response.ToolCalls = append(response.ToolCalls, ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: tc.Function.Arguments,
		})
	}

	return response, nil
}

func (p *OpenAIProvider) ChatStream(ctx context.Context, lg Logger, messages []Message, tools []Tool, toolResults []ToolResult) (<-chan StreamChunk, <-chan *Response) {
	chunkCh := make(chan StreamChunk, 16)
	respCh := make(chan *Response, 1)

	go func() {
		defer close(chunkCh)
		defer close(respCh)

		req := p.buildChatRequest(messages, tools)
		stream, err := p.client.CreateChatCompletionStream(ctx, req)
		if err != nil {
			chunkCh <- StreamChunk{Error: err}
			return
		}
		defer stream.Close()

		var fullContent strings.Builder
		var fullReasoning strings.Builder
		toolCallsMap := make(map[int]*ToolCall)

		for {
			chunk, err := stream.Recv()
			if err != nil {
				if err.Error() != "EOF" {
					chunkCh <- StreamChunk{Error: err}
				}
				break
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			delta := chunk.Choices[0].Delta

			if delta.Content != "" {
				fullContent.WriteString(delta.Content)
				chunkCh <- StreamChunk{Content: delta.Content}
			}

			if delta.ReasoningContent != "" {
				fullReasoning.WriteString(delta.ReasoningContent)
			}

			for _, tc := range delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}

				if _, exists := toolCallsMap[idx]; !exists {
					toolCallsMap[idx] = &ToolCall{}
				}

				if tc.ID != "" {
					toolCallsMap[idx].ID = tc.ID
				}
				if tc.Function.Name != "" {
					toolCallsMap[idx].Name = tc.Function.Name
				}
				toolCallsMap[idx].Args += tc.Function.Arguments
			}
		}

		var toolCalls []ToolCall
		for i := 0; i < len(toolCallsMap); i++ {
			if tc, ok := toolCallsMap[i]; ok {
				toolCalls = append(toolCalls, *tc)
			}
		}

		respCh <- &Response{
			Content:          fullContent.String(),
			ReasoningContent: fullReasoning.String(),
			ToolCalls:        toolCalls,
		}
	}()

	return chunkCh, respCh
}

func (p *OpenAIProvider) buildChatRequest(messages []Message, tools []Tool) openai.ChatCompletionRequest {
	var msgs []openai.ChatCompletionMessage
	for _, msg := range messages {
		if msg.Role == "tool" && len(msg.ToolResults) > 0 {
			for _, result := range msg.ToolResults {
				msgs = append(msgs, openai.ChatCompletionMessage{
					Role:       "tool",
					Content:    result.Content,
					ToolCallID: result.ToolCallID,
				})
			}
		} else {
			msgs = append(msgs, p.toOpenAIMessage(msg))
		}
	}

	var openaiTools []openai.Tool
	for _, t := range tools {
		openaiTools = append(openaiTools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	return openai.ChatCompletionRequest{
		Model:    p.model,
		Messages: msgs,
		Tools:    openaiTools,
	}
}

func (p *OpenAIProvider) toOpenAIMessage(msg Message) openai.ChatCompletionMessage {
	if msg.Role == "tool" && len(msg.ToolResults) > 0 {
		return openai.ChatCompletionMessage{
			Role:       "tool",
			Content:    msg.ToolResults[0].Content,
			ToolCallID: msg.ToolResults[0].ToolCallID,
		}
	}

	var toolCalls []openai.ToolCall
	for _, tc := range msg.ToolCalls {
		toolCalls = append(toolCalls, openai.ToolCall{
			ID:   tc.ID,
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      tc.Name,
				Arguments: tc.Args,
			},
		})
	}

	return openai.ChatCompletionMessage{
		Role:             msg.Role,
		Content:          msg.Content,
		ReasoningContent: msg.ReasoningContent,
		ToolCalls:        toolCalls,
	}
}
