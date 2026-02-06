package event

type Type string

const (
	TextDelta Type = "text_delta"
	ToolStart Type = "tool_start"
	ToolEnd   Type = "tool_end"
	Done      Type = "done"
	Error     Type = "error"
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
	FullText string
}

type ErrorData struct {
	Message string
}
