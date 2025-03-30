package llm

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/EricBriscoe/llm-tool/internal/config"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiClient implements the Client interface for Google's Gemini API
type GeminiClient struct {
	client *genai.Client
	model  string
}

// NewGeminiClient creates a new Gemini client
func NewGeminiClient(cfg *config.Config) (*GeminiClient, error) {
	if cfg.Gemini.APIKey == "" {
		return nil, fmt.Errorf("gemini API key not set in config")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(cfg.Gemini.APIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	model := cfg.Gemini.Model
	if model == "" {
		model = "gemini-2.0-flash-lite"
	}
	
	return &GeminiClient{
		client: client,
		model:  model,
	}, nil
}

// StreamResponse streams a response from the Gemini API
func (c *GeminiClient) StreamResponse(ctx context.Context, prompt string, model string) error {
	if model == "" {
		model = c.model
	}

	genModel := c.client.GenerativeModel(model)
	genModel.SetTemperature(0.2)
	
	stream := genModel.GenerateContentStream(ctx, genai.Text(prompt))
	if stream == nil {
		return fmt.Errorf("failed to generate content: stream is nil")
	}

	for {
		resp, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Silently handle the "no more items in iterator" error
			if strings.Contains(err.Error(), "no more items in iterator") {
				break
			}
			return fmt.Errorf("error receiving response: %w", err)
		}

		for _, part := range resp.Candidates[0].Content.Parts {
			if text, ok := part.(genai.Text); ok {
				fmt.Print(string(text))
			}
		}
	}
	
	fmt.Println() // End with newline
	return nil
}

// ReviewCodeDiff reviews a git diff using the Gemini API
func (c *GeminiClient) ReviewCodeDiff(ctx context.Context, diff string, model string) error {
	if model == "" {
		model = c.model
	}

	prompt := fmt.Sprintf(`Review this git diff and provide actionable feedback:

%s

Please analyze:
1. Code quality issues
2. Potential bugs
3. Security concerns
4. Performance considerations
5. Suggested improvements
`, diff)

	genModel := c.client.GenerativeModel(model)
	genModel.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text("You are a helpful code reviewer. Provide clear, concise, and constructive feedback on git diffs.")},
	}

	resp, err := genModel.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return fmt.Errorf("failed to generate content: %w", err)
	}

	fmt.Print("\n=== Code Review ===\n")
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			fmt.Print(string(text))
		}
	}
	fmt.Println("\n=== End of Review ===")
	return nil
}

// RefactorFile refactors a file based on user instructions using the Gemini API
func (c *GeminiClient) RefactorFile(ctx context.Context, filename string, content string, instructions string, model string) (string, error) {
	if model == "" {
		model = c.model
	}

	prompt := fmt.Sprintf(`Refactor the following file based on these instructions:

Instructions:
%s

Filename: %s

Content:
%s

Please provide the complete refactored file content, maintaining the original functionality unless the instructions 
specifically require changes. Keep all imports and package declarations.`, 
		instructions, filename, content)

	genModel := c.client.GenerativeModel(model)
	genModel.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text("You are an expert software engineer tasked with refactoring code files. Provide only the refactored code without explanations unless explicitly asked.")},
	}

	resp, err := genModel.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	var result strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			result.WriteString(string(text))
		}
	}

	return result.String(), nil
}
