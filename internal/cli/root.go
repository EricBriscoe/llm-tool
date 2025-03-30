package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/EricBriscoe/llm-tool/internal/config"
	"github.com/EricBriscoe/llm-tool/internal/fileutil"
	"github.com/EricBriscoe/llm-tool/internal/git"
	"github.com/EricBriscoe/llm-tool/internal/llm"
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	var provider string
	var model string
	var repoPath string
	var email string
	var token string
	var endpoint string
	var datasource string
	var applyChanges bool
	var outputDir string

	rootCmd := &cobra.Command{
		Use:   "llm-tool",
		Short: "Query LLM APIs from the command line",
		Long:  `A CLI tool to interact with various LLM providers and stream responses.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == "help" {
				return nil
			}
			return nil
		},
	}

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long:  `View and update configuration settings.`,
	}

	configPathCmd := &cobra.Command{
		Use:   "path",
		Short: "Show config file path",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(config.GetConfigPath())
		},
	}

	// Add a setup token command for CBOE
	setupTokenCmd := &cobra.Command{
		Use:   "setup-cboe",
		Short: "Setup CBOE token for authentication",
		RunE: func(cmd *cobra.Command, args []string) error {
			if email == "" || token == "" {
				return fmt.Errorf("both email and token are required")
			}
			
			// First set up the token with CBOE API
			err := llm.SetupToken(email, token, endpoint)
			if err != nil {
				return fmt.Errorf("failed to set up token: %w", err)
			}
			
			// Then save to config
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			
			cfg.CBOE.Email = email
			cfg.CBOE.Token = token
			if endpoint != "" {
				cfg.CBOE.Endpoint = endpoint
			}
			
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}
			
			fmt.Println("CBOE credentials saved to config file")
			return nil
		},
	}

	askCmd := &cobra.Command{
		Use:   "ask [prompt]",
		Short: "Ask a question to an LLM",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := args[0]
			
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			
			if provider == "" {
				provider = cfg.DefaultProvider
			}
			
			// If datasource is provided, update the config temporarily
			if provider == "cboe" && datasource != "" {
				cfg.CBOE.Datasource = datasource
			}
			
			client, err := llm.NewClient(provider, cfg)
			if err != nil {
				return err
			}
			
			return client.StreamResponse(cmd.Context(), prompt, model)
		},
	}

	reviewCmd := &cobra.Command{
		Use:   "review [branch-name]",
		Short: "Review code diff between current branch and specified branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branchName := args[0]
			
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			
			if provider == "" {
				provider = cfg.DefaultProvider
			}
			
			diff, err := git.GetDiff(branchName, repoPath)
			if err != nil {
				return fmt.Errorf("failed to get diff: %w", err)
			}
			
			if diff == "" {
				return fmt.Errorf("no diff found between current branch and %s", branchName)
			}
			
			client, err := llm.NewClient(provider, cfg)
			if err != nil {
				return err
			}
			
			return client.ReviewCodeDiff(cmd.Context(), diff, model)
		},
	}

	editCmd := &cobra.Command{
		Use:   "edit [flags] [instructions] [files...]",
		Short: "Edit or refactor files using an LLM",
		Long: `Edit or refactor files using an LLM based on instructions.
Files can be provided as arguments or piped through stdin.
Changes are staged for review before being applied.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the refactoring instructions from stdin if no files are specified
			// or from the first arg if there are files specified
			var instructions string
			var files []string

			// Check if we're receiving from a pipe
			info, _ := os.Stdin.Stat()
			isPipe := (info.Mode() & os.ModeCharDevice) == 0

			if len(args) == 0 {
				if !isPipe {
					return fmt.Errorf("no files specified and no input from pipe")
				}
				// Read from stdin, treat as a single file "-"
				files = []string{"-"}
				
				// Prompt for instructions interactively
				reader := bufio.NewReader(os.Stdin)
				fmt.Print("Enter refactoring instructions: ")
				instructionsBytes, err := reader.ReadBytes('\n')
				if err != nil {
					return fmt.Errorf("failed to read instructions: %w", err)
				}
				instructions = strings.TrimSpace(string(instructionsBytes))
				
				// Re-open stdin for file content
				// Note: This is a limitation - we can't easily get both instructions and file content
				// from stdin in the same session. In practice, users would provide instructions as args.
				fmt.Println("Now enter the file content to be refactored (Ctrl+D when finished):")
			} else if isPipe {
				// Pipe exists but we also have args, first arg is instructions
				instructions = args[0]
				files = []string{"-"} // Read file content from stdin
			} else {
				// No pipe, first arg is instructions, rest are files
				if len(args) < 2 {
					return fmt.Errorf("please provide both instructions and at least one file")
				}
				instructions = args[0]
				files = args[1:]
			}
			
			if instructions == "" {
				return fmt.Errorf("refactoring instructions cannot be empty")
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if provider == "" {
				provider = cfg.DefaultProvider
			}

			client, err := llm.NewClient(provider, cfg)
			if err != nil {
				return err
			}

			// Create staging area for processed files
			stagingArea, err := fileutil.NewStagingArea()
			if err != nil {
				return err
			}
			defer stagingArea.Cleanup()

			// Process each file
			fmt.Printf("Processing %d files with the following instructions:\n%s\n\n", len(files), instructions)
			
			for _, filename := range files {
				fmt.Printf("Processing file: %s\n", filename)
				
				// Read file content
				content, err := fileutil.ReadFileContent(filename)
				if err != nil {
					return err
				}
				
				// Process with LLM
				refactoredContent, err := client.RefactorFile(cmd.Context(), filename, content, instructions, model)
				if err != nil {
					return fmt.Errorf("failed to refactor %s: %w", filename, err)
				}
				
				// Stage the result
				isNew := !fileExists(filename) || filename == "-"
				outputFilename := filename
				if outputDir != "" {
					// If output directory is specified, write there instead
					outputFilename = filepath.Join(outputDir, filepath.Base(filename))
				}
				if isNew && outputFilename == "-" {
					outputFilename = "output.txt"
				}
				
				_, err = stagingArea.StageFile(outputFilename, refactoredContent, isNew)
				if err != nil {
					return fmt.Errorf("failed to stage file %s: %w", outputFilename, err)
				}
				
				fmt.Printf("✓ Processed %s\n", filename)
			}
			
			// Show diffs and prompt for confirmation
			fmt.Println("\nReview of changes:")
			if err := stagingArea.ShowDiff(); err != nil {
				return fmt.Errorf("failed to show diffs: %w", err)
			}
			
			if !applyChanges {
				// Ask for confirmation
				fmt.Print("\nApply these changes? [y/N] ")
				var response string
				fmt.Scanln(&response)
				
				if response != "y" && response != "Y" {
					fmt.Println("Changes not applied.")
					return nil
				}
			}
			
			// Apply changes
			if err := stagingArea.ApplyChanges(); err != nil {
				return fmt.Errorf("failed to apply changes: %w", err)
			}
			
			fmt.Println("All changes applied successfully.")
			return nil
		},
	}

	// Add flags to editCmd
	editCmd.Flags().StringVarP(&provider, "provider", "p", "", "LLM provider (openai, cboe, gemini)")
	editCmd.Flags().StringVarP(&model, "model", "m", "", "Model to use (defaults to config)")
	editCmd.Flags().BoolVarP(&applyChanges, "yes", "y", false, "Apply changes without confirmation")
	editCmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory for refactored files")
	editCmd.Flags().StringVarP(&datasource, "datasource", "d", "", "Datasource to use (CBOE only)")

	setupTokenCmd.Flags().StringVarP(&email, "email", "e", "", "Email for CBOE authentication")
	setupTokenCmd.Flags().StringVarP(&token, "token", "t", "", "Token for CBOE authentication")
	setupTokenCmd.Flags().StringVarP(&endpoint, "endpoint", "", "", "CBOE API endpoint (optional)")
	
	askCmd.Flags().StringVarP(&provider, "provider", "p", "", "LLM provider (openai, cboe, gemini)")
	askCmd.Flags().StringVarP(&model, "model", "m", "", "Model to use (defaults to config)")
	askCmd.Flags().StringVarP(&datasource, "datasource", "d", "", "Datasource to use (CBOE only)")
	
	reviewCmd.Flags().StringVarP(&provider, "provider", "p", "", "LLM provider (openai, cboe, gemini)")
	reviewCmd.Flags().StringVarP(&model, "model", "m", "", "Model to use (defaults to config)")
	reviewCmd.Flags().StringVarP(&repoPath, "repo", "r", "", "Path to git repository (defaults to current directory)")

	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(setupTokenCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(askCmd)
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(editCmd)
	return rootCmd
}

// fileExists checks if a file exists and is not a directory
func fileExists(filename string) bool {
	if filename == "-" {
		return false
	}
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
