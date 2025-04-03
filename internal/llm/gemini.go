package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EricBriscoe/llm-tool/internal/config"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiClient implements the Client interface for Google's Gemini API
type GeminiClient struct {
	client *genai.Client
	model  string
	// Add historyPath to store conversation history
	historyPath string
}

// ChatHistoryContent represents a simplified version of genai.Content for serialization
type ChatHistoryContent struct {
	Role  string   `json:"Role"`
	Parts []string `json:"Parts"`
}

// ChatHistory represents the conversation history for the Gemini chat
type ChatHistory struct {
	Model     string              `json:"model"`
	Messages  []ChatHistoryContent `json:"messages"`
	Timestamp time.Time           `json:"timestamp"`
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
	
	// Create history path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}
	
	historyDir := filepath.Join(homeDir, ".config", "llm-tool", "history")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}
	
	historyPath := filepath.Join(historyDir, "gemini_chat_history.json")
	
	return &GeminiClient{
		client:      client,
		model:       model,
		historyPath: historyPath,
	}, nil
}

// loadChatHistory loads the conversation history from file
func (c *GeminiClient) loadChatHistory() (*ChatHistory, error) {
	// Check if history file exists
	if _, err := os.Stat(c.historyPath); os.IsNotExist(err) {
		// Return empty history if file doesn't exist
		return &ChatHistory{
			Model:     c.model,
			Messages:  []ChatHistoryContent{},
			Timestamp: time.Now(),
		}, nil
	}

	// Read file
	data, err := os.ReadFile(c.historyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read chat history: %w", err)
	}

	// Unmarshal JSON
	var history ChatHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("failed to parse chat history: %w", err)
	}

	return &history, nil
}

// saveChatHistory saves the conversation history to file
func (c *GeminiClient) saveChatHistory(history *ChatHistory) error {
	// Update timestamp
	history.Timestamp = time.Now()
	
	// Marshal to JSON
	data, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("failed to marshal chat history: %w", err)
	}

	// Write to file
	if err := os.WriteFile(c.historyPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save chat history: %w", err)
	}

	return nil
}

// convertToGenAIContents converts our serializable history format to genai.Content array
func convertToGenAIContents(history *ChatHistory) []*genai.Content {
	contents := make([]*genai.Content, 0, len(history.Messages))
	
	for _, msg := range history.Messages {
		parts := make([]genai.Part, 0, len(msg.Parts))
		for _, text := range msg.Parts {
			parts = append(parts, genai.Text(text))
		}
		
		content := &genai.Content{
			Parts: parts,
			Role:  msg.Role,
		}
		contents = append(contents, content)
	}
	
	return contents
}

// StreamResponse streams a response from the Gemini API
func (c *GeminiClient) StreamResponse(ctx context.Context, prompt string, model string) error {
	if model == "" {
		model = c.model
	}

	// Load chat history
	history, err := c.loadChatHistory()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not load chat history: %v\n", err)
		// Continue with empty history if there's an error
		history = &ChatHistory{
			Model:     model,
			Messages:  []ChatHistoryContent{},
			Timestamp: time.Now(),
		}
	}

	// Create generative model
	genModel := c.client.GenerativeModel(model)
	genModel.SetTemperature(0.2)
	
	// Create a chat session
	cs := genModel.StartChat()
	
	// Set history if available
	if len(history.Messages) > 0 {
		genaiContents := convertToGenAIContents(history)
		cs.History = genaiContents
	}

	// Create user message for history
	userHistoryContent := ChatHistoryContent{
		Role:  "user",
		Parts: []string{prompt},
	}
	
	// Send message using chat session
	iter := cs.SendMessageStream(ctx, genai.Text(prompt))

	// Prepare to collect response for history
	var responseParts []string

	for {
		resp, err := iter.Next()
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
				responseParts = append(responseParts, string(text))
			}
		}
	}
	
	// Add the user message to history
	history.Messages = append(history.Messages, userHistoryContent)
	
	// Create and add the model's response to history
	modelHistoryContent := ChatHistoryContent{
		Role:  "model",
		Parts: responseParts,
	}
	history.Messages = append(history.Messages, modelHistoryContent)
	
	// Limit history size to last 20 messages (10 exchanges)
	if len(history.Messages) > 20 {
		history.Messages = history.Messages[len(history.Messages)-20:]
	}
	
	// Save updated history
	if err := c.saveChatHistory(history); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not save chat history: %v\n", err)
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

// ClearChatHistory clears the stored conversation history
func (c *GeminiClient) ClearChatHistory() error {
	if _, err := os.Stat(c.historyPath); os.IsNotExist(err) {
		// Nothing to clear
		return nil
	}
	
	if err := os.Remove(c.historyPath); err != nil {
		return fmt.Errorf("failed to clear chat history: %w", err)
	}
	
	return nil
}
