package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// GetDiff returns the git diff between the current branch and the specified branch
func GetDiff(branchName string, workingDir string) (string, error) {
	cmd := exec.Command("git", "diff", branchName)
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git diff error: %w: %s", err, stderr.String())
	}

	// If no diff, try to get diff between current branch and the given branch
	if strings.TrimSpace(out.String()) == "" {
		currentBranch, err := getCurrentBranch(workingDir)
		if err != nil {
			return "", err
		}

		cmd = exec.Command("git", "diff", fmt.Sprintf("%s...%s", branchName, currentBranch))
		if workingDir != "" {
			cmd.Dir = workingDir
		}

		out.Reset()
		stderr.Reset()
		cmd.Stdout = &out
		cmd.Stderr = &stderr

		err = cmd.Run()
		if err != nil {
			return "", fmt.Errorf("git diff error: %w: %s", err, stderr.String())
		}
	}

	return out.String(), nil
}

// getCurrentBranch returns the name of the current branch
func getCurrentBranch(workingDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error getting current branch: %w", err)
	}

	return strings.TrimSpace(out.String()), nil
}
