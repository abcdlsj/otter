package llm

import (
	"github.com/abcdlsj/otter/internal/types"
	"github.com/pkoukk/tiktoken-go"
)

// estimateTokens uses tiktoken to estimate token count for text
func estimateTokens(text string, model string) int64 {
	encoding := "cl100k_base" // default for gpt-4, gpt-3.5, text-embedding-ada-002
	
	// Map common models to their encodings
	switch model {
	case "gpt-4o", "gpt-4o-mini":
		encoding = "o200k_base"
	}
	
	tkm, err := tiktoken.GetEncoding(encoding)
	if err != nil {
		// Fallback: rough estimate ~4 chars per token
		return int64(len(text) / 4)
	}
	
	tokens := tkm.Encode(text, nil, nil)
	return int64(len(tokens))
}

// EstimateMessagesTokens estimates tokens for a slice of messages
func EstimateMessagesTokens(messages []Message, model string) int64 {
	var total int64
	for _, msg := range messages {
		total += estimateTokens(msg.Content, model)
		total += 4
	}
	return total
}

// EstimateOutputTokens estimates output tokens from content and tool calls
func EstimateOutputTokens(content string, toolCalls []types.ToolCall, model string) int64 {
	total := estimateTokens(content, model)
	for _, tc := range toolCalls {
		total += estimateTokens(tc.Name, model)
		total += estimateTokens(tc.Args, model)
		total += 4
	}
	return total
}
