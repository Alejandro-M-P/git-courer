package git

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

func (a *ExecAdapter) Commit(message string) (string, error) {
	log.Printf("[DEBUG] gitAdapter.Commit: message=%q", message)
	out, err := a.runGit("commit", "-m", message)
	if err != nil {
		log.Printf("[DEBUG] gitAdapter.Commit: error=%v, out=%q", err, out)
	}
	return out, err
}

func (a *ExecAdapter) IsGHAuthenticated() (bool, error) {
	_, err := a.runGH("auth", "status")
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (a *ExecAdapter) CreateRelease(tagName, changelog string) (string, error) {
	if tagName == "" {
		return "", fmt.Errorf("release tag name is required")
	}
	if changelog == "" {
		changelog = "No changelog provided"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "release", "create", tagName, "-F", "-")
	cmd.Dir = a.workDir
	cmd.Stdin = strings.NewReader(changelog)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("gh release timed out after 60s")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if stderr == "" {
				return "", fmt.Errorf("gh release error (empty stderr). Command: gh release create %s", tagName)
			}
			return "", fmt.Errorf("gh release error: %s", stderr)
		}
		return "", fmt.Errorf("gh release error: %w", err)
	}
	return string(out), nil
}
