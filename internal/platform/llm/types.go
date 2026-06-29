package llm

type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
	Name       string
}

type Schema struct {
	Name   string
	Strict bool
	Schema map[string]any
}

type StructuredContract[T any] interface {
	Schema() Schema
	Decode(raw []byte) (T, error)
}

type ToolSpec struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type ToolCall struct {
	ID            string
	FunctionName  string
	ArgumentsJSON map[string]any
}

type Request struct {
	Messages    []Message
	Schema      *Schema
	Tools       []ToolSpec
	ToolChoice  string
	MaxTokens   int
	Temperature float64
	FreeText    bool
}

type Response struct {
	Content           string
	RawJSON           []byte
	PromptTokens      int
	CompletionTokens  int
	TruncatedByLength bool
	ToolCalls         []ToolCall
}
