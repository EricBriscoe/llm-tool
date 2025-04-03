package llm

import (
	"context"
	"fmt"

	"github.com/EricBriscoe/llm-tool/internal/config"
)

// Client defines the interface for LLM API clients
type Client interface {
	StreamResponse(ctx context.Context, prompt string, model string) error
	ReviewCodeDiff(ctx context.Context, diff string, model string) error
	RefactorFile(ctx context.Context, filename string, content string, instructions string, model string) (string, error)
	ClearChatHistory() error
}

// NewClient creates a new LLM client based on the provider
func NewClient(provider string, cfg *config.Config) (Client, error) {
	switch provider {
	case "openai":
		return NewOpenAIClient(cfg)
	case "cboe":
		return NewCBOEClient(cfg)
	case "gemini":
		return NewGeminiClient(cfg)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}
