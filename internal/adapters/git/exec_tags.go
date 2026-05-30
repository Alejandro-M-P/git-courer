package git

import (
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

func (a *ExecAdapter) Tag(name, message string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("tag name is required")
	}
	if !domain.IsValidTagName(name) {
		return "", fmt.Errorf("invalid tag name: %s (use semver like v1.0.0 or 1.0.0)", name)
	}
	if message != "" {
		return a.runGit("tag", "-a", name, "-m", message)
	}
	return a.runGit("tag", name)
}

func (a *ExecAdapter) DeleteTag(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("tag name is required")
	}
	exists, err := a.TagExists(name)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("tag %s not found", name)
	}
	return a.runGit("tag", "-d", name)
}

func (a *ExecAdapter) PushTag(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("tag name is required")
	}
	out, err := a.runGit("push", "origin", name)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "already exists") || strings.Contains(errStr, "rejected") {
			return "", fmt.Errorf("tag %s already exists in remote. Use TAG_DELETE_REMOTE first.", name)
		}
		return "", err
	}
	return out, nil
}

func (a *ExecAdapter) DeleteTagRemote(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("tag name is required")
	}
	return a.runGit("push", "origin", ":refs/tags/"+name)
}

func (a *ExecAdapter) LatestTag() (string, error) {
	out, err := a.runGit("describe", "--tags", "--abbrev=0")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (a *ExecAdapter) CommitsFromTag(tag string) (string, error) {
	if tag == "" {
		return "", fmt.Errorf("tag name is required")
	}
	out, err := a.runGit("log", tag+"..HEAD", "--oneline")
	if err != nil {
		return "", err
	}
	return out, nil
}

func (a *ExecAdapter) TagExists(name string) (bool, error) {
	if name == "" {
		return false, fmt.Errorf("tag name is required")
	}
	out, err := a.runGit("tag", "-l", name)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == name, nil
}
