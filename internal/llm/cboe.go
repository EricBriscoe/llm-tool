package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/EricBriscoe/llm-tool/internal/config"
)

// CBOEClient implements the Client interface for the CBOE API
type CBOEClient struct {
	email      string
	token      string
	endpoint   string
	model      string
	datasource string
}

// Content represents a single content item in a CBOE message
type cboeContent struct {
	Text string `json:"text"`
}

// CBOEMessage represents a message in the CBOE API format
type cboeMessage struct {
	Role    string        `json:"role"`
	Content []cboeContent `json:"content"`
}

// CBOEDataSource represents a datasource in the CBOE API
type cboeDataSource struct {
	Name       string                 `json:"name"`
	Custom     bool                   `json:"custom,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// CBOECompletionRequest represents a request to the CBOE chat API
type cboeCompletionRequest struct {
	Messages    []cboeMessage   `json:"messages"`
	Email       string          `json:"email"`
	Token       string          `json:"token"`
	Datasources []cboeDataSource `json:"datasources,omitempty"`
}

// CBOECompletionResponse represents a response from the CBOE chat API
type cboeCompletionResponse struct {
	Answer  string `json:"answer"`
	Sources []struct {
		Name      string `json:"name"`
		URL       string `json:"url,omitempty"`
		Text      string `json:"text,omitempty"`
		Timestamp string `json:"timestamp,omitempty"`
	} `json:"sources,omitempty"`
	Error string `json:"error,omitempty"`
}

// NewCBOEClient creates a new CBOE client
func NewCBOEClient(cfg *config.Config) (*CBOEClient, error) {
	if cfg.CBOE.Email == "" || cfg.CBOE.Token == "" {
		return nil, fmt.Errorf("CBOE email and token required in config")
	}

	endpoint := cfg.CBOE.Endpoint
	if endpoint == "" {
		endpoint = "http://ai.api.us.cboe.net:5005"
	}
	
	return &CBOEClient{
		email:      cfg.CBOE.Email,
		token:      cfg.CBOE.Token,
		endpoint:   endpoint,
		model:      cfg.CBOE.Model,
		datasource: cfg.CBOE.Datasource,
	}, nil
}

// StreamResponse streams a response from the CBOE API
func (c *CBOEClient) StreamResponse(ctx context.Context, prompt string, model string) error {
	// Create request body
	reqBody := cboeCompletionRequest{
		Messages: []cboeMessage{
			{
				Role: "user",
				Content: []cboeContent{
					{Text: prompt},
				},
			},
		},
		Email: c.email,
		Token: c.token,
	}
	
	// Add datasource if configured
	if c.datasource != "" {
		reqBody.Datasources = []cboeDataSource{
			{
				Name:   c.datasource,
				Custom: true,
			},
		}
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.endpoint+"/chat_stream", // Use streaming endpoint
		strings.NewReader(string(jsonData)),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, body)
	}

	// Handle the streaming response
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Check if the line is a data line (in SSE format)
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			
			// Try to parse as JSON to handle structured responses
			var respData map[string]interface{}
			if err := json.Unmarshal([]byte(data), &respData); err == nil {
				if answer, ok := respData["answer"].(string); ok {
					fmt.Print(answer)
				} else {
					fmt.Print(data)
				}
			} else {
				// If not valid JSON, print the data as is
				fmt.Print(data)
			}
		}
	}
	
	fmt.Println() // End with newline
	return scanner.Err()
}

// ReviewCodeDiff reviews a git diff using the CBOE API
func (c *CBOEClient) ReviewCodeDiff(ctx context.Context, diff string, model string) error {
	prompt := fmt.Sprintf(`Review this git diff and provide actionable feedback:

%s

Please analyze:
1. Code quality issues
2. Potential bugs
3. Security concerns
4. Performance considerations
5. Suggested improvements
`, diff)

	// Create a system message and user message
	reqBody := cboeCompletionRequest{
		Messages: []cboeMessage{
			{
				Role: "system",
				Content: []cboeContent{
					{Text: "You are a helpful code reviewer. Provide clear, concise, and constructive feedback on git diffs."},
				},
			},
			{
				Role: "user",
				Content: []cboeContent{
					{Text: prompt},
				},
			},
		},
		Email: c.email,
		Token: c.token,
	}

	// Add datasource if configured
	if c.datasource != "" {
		reqBody.Datasources = []cboeDataSource{
			{
				Name:   c.datasource,
				Custom: true,
			},
		}
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// For code review, using non-streaming endpoint might be more appropriate
	// as we want the complete analysis
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.endpoint+"/chat",
		strings.NewReader(string(jsonData)),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, body)
	}

	// Parse the response
	var response cboeCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Error != "" {
		return fmt.Errorf("API returned error: %s", response.Error)
	}

	fmt.Print("\n=== Code Review ===\n")
	fmt.Print(response.Answer)
	
	// Print sources if available
	if len(response.Sources) > 0 {
		fmt.Print("\n\n=== Sources ===\n")
		for i, source := range response.Sources {
			fmt.Printf("%d. %s\n", i+1, source.Name)
			if source.URL != "" {
				fmt.Printf("   URL: %s\n", source.URL)
			}
		}
	}
	
	fmt.Println("\n=== End of Review ===")
	return nil
}

// RefactorFile refactors a file based on user instructions using the CBOE API
func (c *CBOEClient) RefactorFile(ctx context.Context, filename string, content string, instructions string, model string) (string, error) {
	prompt := fmt.Sprintf(`Refactor the following file based on these instructions:

Instructions:
%s

Filename: %s

Content:
%s

Please provide the complete refactored file content, maintaining the original functionality unless the instructions 
specifically require changes. Keep all imports and package declarations.`, 
		instructions, filename, content)

	// Create system message and user message
	reqBody := cboeCompletionRequest{
		Messages: []cboeMessage{
			{
				Role: "system",
				Content: []cboeContent{
					{Text: "You are an expert software engineer tasked with refactoring code files. Provide only the refactored code without explanations unless explicitly asked."},
				},
			},
			{
				Role: "user",
				Content: []cboeContent{
					{Text: prompt},
				},
			},
		},
		Email: c.email,
		Token: c.token,
	}

	// Add datasource if configured
	if c.datasource != "" {
		reqBody.Datasources = []cboeDataSource{
			{
				Name:   c.datasource,
				Custom: true,
			},
		}
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.endpoint+"/chat",
		strings.NewReader(string(jsonData)),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, body)
	}

	// Parse the response
	var response cboeCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Error != "" {
		return "", fmt.Errorf("API returned error: %s", response.Error)
	}

	return response.Answer, nil
}

// SetupToken performs the initial token setup for a CBOE account
func SetupToken(email, token, endpoint string) error {
	if endpoint == "" {
		endpoint = "http://ai.api.us.cboe.net:5005"
	}
	
	type setupRequest struct {
		Email string `json:"email"`
		Token string `json:"token"`
	}
	
	reqBody := setupRequest{
		Email: email,
		Token: token,
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	
	req, err := http.NewRequest(
		"POST",
		endpoint+"/setup_token",
		strings.NewReader(string(jsonData)),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, body)
	}
	
	fmt.Printf("Token setup successful: %s\n", body)
	return nil
}
