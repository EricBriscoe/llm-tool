package fileutil

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/fatih/color"
)

// StagedFile represents a file that has been processed and is staged for update
type StagedFile struct {
	OriginalPath string
	StagedPath   string
	Content      string
	IsNew        bool
}

// StagingArea manages files that have been processed and are ready for review
type StagingArea struct {
	Files      []StagedFile
	StagingDir string
}

// NewStagingArea creates a new staging area for processing files
func NewStagingArea() (*StagingArea, error) {
	// Create temporary directory for staging
	stagingDir, err := os.MkdirTemp("", "llm-tool-staging-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create staging directory: %w", err)
	}

	return &StagingArea{
		Files:      make([]StagedFile, 0),
		StagingDir: stagingDir,
	}, nil
}

// StageFile adds a file to the staging area
func (sa *StagingArea) StageFile(originalPath string, content string, isNew bool) (*StagedFile, error) {
	// Create a matching path structure in the staging directory
	var stagedPath string
	if isNew {
		// For new files, use just the basename to avoid path issues
		stagedPath = filepath.Join(sa.StagingDir, filepath.Base(originalPath))
	} else {
		// For existing files, try to maintain relative structure
		relPath := filepath.Base(originalPath)
		stagedPath = filepath.Join(sa.StagingDir, relPath)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(stagedPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create staging subdirectory: %w", err)
	}

	// Write content to staged file
	if err := os.WriteFile(stagedPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write staged file: %w", err)
	}

	stagedFile := StagedFile{
		OriginalPath: originalPath,
		StagedPath:   stagedPath,
		Content:      content,
		IsNew:        isNew,
	}

	sa.Files = append(sa.Files, stagedFile)
	return &stagedFile, nil
}

// Cleanup removes the staging directory
func (sa *StagingArea) Cleanup() error {
	return os.RemoveAll(sa.StagingDir)
}

// ShowDiff displays the diff between original and staged files
func (sa *StagingArea) ShowDiff() error {
	for _, file := range sa.Files {
		if file.IsNew {
			fmt.Printf("New file: %s\n", file.OriginalPath)
			highlightContent(file.Content)
			continue
		}

		fmt.Printf("\nDiff for %s:\n", file.OriginalPath)
		if err := displayDiff(file.OriginalPath, file.StagedPath); err != nil {
			return fmt.Errorf("failed to display diff: %w", err)
		}
	}
	return nil
}

// displayDiff shows the difference between two files using diff command
func displayDiff(originalPath, stagedPath string) error {
	// Check if diff command exists
	diffCmd, err := exec.LookPath("diff")
	if err == nil {
		// Try to use diff -u with color if supported
		cmd := exec.Command(diffCmd, "--color=always", "-u", originalPath, stagedPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		// Ignore exit code 1, which just means files differ
		if err != nil && cmd.ProcessState.ExitCode() != 1 {
			// Try without color if that failed
			cmd = exec.Command(diffCmd, "-u", originalPath, stagedPath)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			// Still ignore exit code 1
			if err != nil && cmd.ProcessState.ExitCode() != 1 {
				return fmt.Errorf("diff command failed: %w", err)
			}
		}
		return nil
	}

	// Fall back to manual diff display if diff command isn't available
	return manualDiff(originalPath, stagedPath)
}

// manualDiff is a fallback method to show diffs when the diff command is not available
func manualDiff(originalPath, stagedPath string) error {
	original, err := os.ReadFile(originalPath)
	if err != nil {
		return fmt.Errorf("failed to read original file: %w", err)
	}

	staged, err := os.ReadFile(stagedPath)
	if err != nil {
		return fmt.Errorf("failed to read staged file: %w", err)
	}

	fmt.Printf("--- %s\n", originalPath)
	fmt.Printf("+++ %s\n", stagedPath)
	// Here we could implement a more sophisticated diff algorithm,
	// but for simplicity, we'll just show the full content with + and - indicators
	fmt.Println("Original:")
	fmt.Print(string(original))
	fmt.Println("\nModified:")
	fmt.Print(string(staged))

	return nil
}

// ApplyChanges writes all staged changes to their original locations
func (sa *StagingArea) ApplyChanges() error {
	for _, file := range sa.Files {
		// Create directory if it doesn't exist (especially for new files)
		dir := filepath.Dir(file.OriginalPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		// Write file content to original location
		if err := os.WriteFile(file.OriginalPath, []byte(file.Content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", file.OriginalPath, err)
		}

		fmt.Printf("Applied changes to: %s\n", file.OriginalPath)
	}
	return nil
}

// ReadFileContent reads the content of a file, or from stdin if filename is "-"
func ReadFileContent(filename string) (string, error) {
	if filename == "-" {
		// Read from stdin
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("failed to read from stdin: %w", err)
		}
		return string(bytes), nil
	}

	// Read from file
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filename, err)
	}
	return string(bytes), nil
}

// highlightContent prints the content with syntax highlighting
func highlightContent(content string) {
	green := color.New(color.FgGreen)
	green.Println(content)
}
