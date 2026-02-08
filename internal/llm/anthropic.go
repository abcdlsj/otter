package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/abcdlsj/otter/internal/logger"
	"github.com/abcdlsj/otter/internal/types"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type AnthropicProvider struct {
	client *anthropic.Client
	model  string
}

func NewAnthropicProvider(apiKey, model, baseURL string) (*AnthropicProvider, error) {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := anthropic.NewClient(opts...)
	return &AnthropicProvider{
		client: &client,
		model:  model,
	}, nil
}

func (p *AnthropicProvider) Chat(ctx context.Context, lg logger.Logger, messages []Message, tools []Tool, toolResults []types.ToolResult) (*Response, error) {
	// 构建消息
	var apiMessages []anthropic.MessageParam
	var systemContent string

	for _, msg := range messages {
		if msg.Role == "system" {
			systemContent = msg.Content
			continue
		}

		if msg.Role == "user" {
			var blocks []anthropic.ContentBlockParamUnion
			// 添加文本内容
			if msg.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
			}
			// 添加工具结果
			for _, tr := range msg.ToolResults {
				blocks = append(blocks, anthropic.NewToolResultBlock(tr.ToolCallID, tr.Content, false))
			}
			apiMessages = append(apiMessages, anthropic.NewUserMessage(blocks...))
		} else if msg.Role == "assistant" {
			var blocks []anthropic.ContentBlockParamUnion
			if msg.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
			}
			for _, tc := range msg.ToolCalls {
				var args map[string]any
				json.Unmarshal([]byte(tc.Args), &args)
				blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, args, tc.Name))
			}
			apiMessages = append(apiMessages, anthropic.NewAssistantMessage(blocks...))
		} else if msg.Role == "tool" {
			var blocks []anthropic.ContentBlockParamUnion
			for _, tr := range msg.ToolResults {
				blocks = append(blocks, anthropic.NewToolResultBlock(tr.ToolCallID, tr.Content, false))
			}
			if len(blocks) > 0 {
				apiMessages = append(apiMessages, anthropic.NewUserMessage(blocks...))
			}
		}
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 16384,
		Messages:  apiMessages,
	}

	if systemContent != "" {
		params.System = []anthropic.TextBlockParam{{
			Type: "text",
			Text: systemContent,
		}}
	}

	if len(tools) > 0 {
		toolUnions := make([]anthropic.ToolUnionParam, len(tools))
		for i, t := range tools {
			schemaBytes, _ := json.Marshal(t.InputSchema)
			var inputSchema anthropic.ToolInputSchemaParam
			json.Unmarshal(schemaBytes, &inputSchema)

			toolParam := anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(t.Description),
				InputSchema: inputSchema,
			}
			toolUnions[i] = anthropic.ToolUnionParam{OfTool: &toolParam}
		}
		params.Tools = toolUnions
	}

	if debug, _ := json.MarshalIndent(params, "", "  "); debug != nil {
		lg.WriteJSON(fmt.Sprintf("request_%s_anthropic.json", time.Now().Format("150405")), debug)
		lg.Debug("llm request", "model", p.model, "messages", len(messages), "tools", len(tools))
	}

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		lg.Error("llm request failed", "error", err)
		return nil, err
	}

	if resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0 {
		lg.Info("llm response", "stop_reason", resp.StopReason, "usage_input", resp.Usage.InputTokens, "usage_output", resp.Usage.OutputTokens)
	} else {
		lg.Info("llm response", "stop_reason", resp.StopReason)
	}

	return parseAnthropicResponse(resp, messages), nil
}

func (p *AnthropicProvider) ChatStream(ctx context.Context, lg logger.Logger, messages []Message, tools []Tool, toolResults []types.ToolResult) (<-chan StreamChunk, <-chan *Response) {
	chunkCh := make(chan StreamChunk, 16)
	respCh := make(chan *Response, 1)

	go func() {
		defer close(chunkCh)
		defer close(respCh)

		resp, err := p.Chat(ctx, lg, messages, tools, toolResults)
		if err != nil {
			chunkCh <- StreamChunk{Error: err}
			return
		}

		if resp.Content != "" {
			chunkCh <- StreamChunk{Content: resp.Content}
		}

		respCh <- resp
	}()

	return chunkCh, respCh
}

func parseAnthropicResponse(resp *anthropic.Message, messages []Message) *Response {
	var content string
	var toolCalls []types.ToolCall

	for _, block := range resp.Content {
		switch b := block.AsAny().(type) {
		case anthropic.TextBlock:
			content += b.Text
		case anthropic.ToolUseBlock:
			argsJSON, _ := json.Marshal(b.Input)
			toolCalls = append(toolCalls, types.ToolCall{
				ID:   b.ID,
				Name: b.Name,
				Args: string(argsJSON),
			})
		}
	}

	response := &Response{
		Content:    content,
		ToolCalls:  toolCalls,
		StopReason: string(resp.StopReason),
	}
	if resp.Usage.InputTokens > 0 {
		response.InputTokens = int64(resp.Usage.InputTokens)
	} else {
		response.InputTokens = EstimateMessagesTokens(messages, "claude")
	}
	if resp.Usage.OutputTokens > 0 {
		response.OutputTokens = int64(resp.Usage.OutputTokens)
	} else {
		response.OutputTokens = EstimateOutputTokens(content, toolCalls, "claude")
	}
	return response
}
