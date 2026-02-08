package event

import "github.com/abcdlsj/otter/internal/types"

type Type string

const (
	TextDelta    Type = "text_delta"
	ToolStart    Type = "tool_start"
	ToolEnd      Type = "tool_end"
	CompactStart Type = "compact_start"
	CompactEnd   Type = "compact_end"
	Done         Type = "done"
	Error        Type = "error"
)

type Event struct {
	Type Type
	Data any
}

type TextDeltaData struct {
	Text string
}

type ToolStartData struct {
	ID   string
	Name string
	Args string
}

type ToolEndData struct {
	ID     string
	Name   string
	Result string
	Error  string
}

type DoneData struct {
	FullText     string
	InputTokens  int64
	OutputTokens int64
	Messages     []Message
}

type Message struct {
	Role        string
	Content     string
	ToolCalls   []types.ToolCall
	ToolResults []types.ToolResult
}

type CompactStartData struct {
	Tokens    int64
	Threshold int64
}

type CompactEndData struct {
	Before int64
	After  int64
}

type ErrorData struct {
	Message string
}
