package models

type ProviderType string

const (
	ProviderOpenAI    ProviderType = "openai"
	ProviderAnthropic ProviderType = "anthropic"
	ProviderGoogle    ProviderType = "google"
	ProviderDeepSeek  ProviderType = "deepseek"
	ProviderOllama    ProviderType = "ollama"
	ProviderCustom    ProviderType = "custom"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Argument string `json:"argument"`
}

type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Output     string `json:"output"`
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Tools       []ToolDef `json:"tools,omitempty"`
}

type ChatResponse struct {
	ID      string  `json:"id"`
	Model   string  `json:"model"`
	Message Message `json:"message"`
	Usage   Usage   `json:"usage,omitempty"`
}

type StreamChunk struct {
	Content   string `json:"content"`
	Reasoning string `json:"reasoning,omitempty"`
	Done      bool   `json:"done"`
	Model     string `json:"model,omitempty"`
	Usage     *Usage `json:"usage,omitempty"`
	Error     error  `json:"-"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Schema      any    `json:"schema"`
}

type Model struct {
	ID       string       `json:"id"`
	Provider ProviderType `json:"provider"`
	Name     string       `json:"name"`
}

type ProviderConfig struct {
	Name       string       `json:"name"`
	Type       ProviderType `json:"type"`
	BaseURL    string       `json:"base_url"`
	APIKey     string       `json:"api_key"`
	Models     []string     `json:"models"`
	RateLimit  int          `json:"rate_limit"`
	TimeoutSec int          `json:"timeout_sec,omitempty"` // per-request timeout; 0 = default
}

type Config struct {
	DefaultProvider string           `json:"default_provider"`
	DefaultModel    string           `json:"default_model"`
	Providers       []ProviderConfig `json:"providers"`
	Memory          MemoryConfig     `json:"memory"`
}

type MemoryConfig struct {
	Enabled     bool   `json:"enabled"`
	VectorDB    string `json:"vector_db"`
	MaxSessions int    `json:"max_sessions"`
}
