package llm

import (
	"context"
	"fmt"
	"io"

	"github.com/EricBriscoe/llm-tool/internal/config"
	"github.com/sashabaranov/go-openai"
)

type OpenAIClient struct {
	client *openai.Client
	model  string
}

func NewOpenAIClient(cfg *config.Config) (*OpenAIClient, error) {
	if cfg.OpenAI.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not set in config")
	}

	client := openai.NewClient(cfg.OpenAI.APIKey)
	model := cfg.OpenAI.Model
	if model == "" {
		model = openai.GPT3Dot5Turbo
	}
	
	return &OpenAIClient{
		client: client,
		model:  model,
	}, nil
}

func (c *OpenAIClient) StreamResponse(ctx context.Context, prompt string, model string) error {
	if model == "" {
		model = c.model
	}

	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Stream: true,
	}

	stream, err := c.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return fmt.Errorf("error creating stream: %w", err)
	}
	defer stream.Close()

	for {
		response, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				fmt.Println() // Add a newline at the end
				return nil
			}
			return fmt.Errorf("stream error: %w", err)
		}

		fmt.Print(response.Choices[0].Delta.Content)
	}
}

func (c *OpenAIClient) ReviewCodeDiff(ctx context.Context, diff string, model string) error {
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

	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a helpful code reviewer. Provide clear, concise, and constructive feedback on git diffs.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Stream: true,
	}

	stream, err := c.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return fmt.Errorf("error creating stream: %w", err)
	}
	defer stream.Close()

	fmt.Print("\n=== Code Review ===\n")
	for {
		response, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				fmt.Println("\n=== End of Review ===")
				return nil
			}
			return fmt.Errorf("stream error: %w", err)
		}

		fmt.Print(response.Choices[0].Delta.Content)
	}
}

// RefactorFile refactors a file based on user instructions using the OpenAI API
func (c *OpenAIClient) RefactorFile(ctx context.Context, filename string, content string, instructions string, model string) (string, error) {
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

	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are an expert software engineer tasked with refactoring code files. Provide only the refactored code without explanations unless explicitly asked.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	}

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI API")
	}

	return resp.Choices[0].Message.Content, nil
}

// ClearChatHistory is a placeholder for OpenAI as we don't currently store chat history
func (c *OpenAIClient) ClearChatHistory() error {
	// OpenAI client doesn't maintain history yet, so this is a no-op
	return nil
}
