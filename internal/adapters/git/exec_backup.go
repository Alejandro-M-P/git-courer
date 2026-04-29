package git

import (
	"fmt"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

func (a *ExecAdapter) CreateBackup(operation string, stashUntracked bool) (domain.Backup, error) {
	timestamp := time.Now().Format("20060102150405")
	ref := fmt.Sprintf("refs/git-courer/backup/%s_%s", timestamp, operation)

	_, err := a.runGit("update-ref", "-m", fmt.Sprintf("git-courer backup: %s", operation), ref, "HEAD")
	if err != nil {
		return domain.Backup{}, fmt.Errorf("failed to create backup ref: %w", err)
	}

	hasStash := false
	unstaged, _ := a.hasUnstaged()
	untracked, _ := a.hasUntracked()

	if unstaged || (stashUntracked && untracked) {
		hasStash = true
		label := fmt.Sprintf("git-courer:backup:%s:%s", operation, timestamp)
		args := []string{"stash", "push", "-m", label}
		if stashUntracked {
			args = append(args, "--include-untracked")
		}
		_, err = a.runGit(args...)
		if err != nil {
			return domain.Backup{}, fmt.Errorf("failed to create stash: %w", err)
		}
	}

	return domain.Backup{
		Ref:       ref,
		HasStash:  hasStash,
		Operation: operation,
		CreatedAt: time.Now(),
	}, nil
}

func (a *ExecAdapter) RestoreBackup(backup domain.Backup) error {
	_, err := a.runGit("update-ref", "HEAD", backup.Ref)
	if err != nil {
		return fmt.Errorf("failed to restore HEAD: %w (ref %s still exists for manual restore)", err, backup.Ref)
	}

	if backup.HasStash {
		stashIdx, sErr := a.findStashByLabel("git-courer:backup:" + backup.Operation)
		if sErr == nil {
			_, err = a.runGit("stash", "pop", fmt.Sprintf("stash@{%d}", stashIdx))
			if err != nil {
				return fmt.Errorf("failed to pop stash: %w", err)
			}
		}
	}
	return nil
}

func (a *ExecAdapter) DeleteBackup(backup domain.Backup) error {
	_, err := a.runGit("update-ref", "-d", backup.Ref)
	if err != nil {
		return fmt.Errorf("failed to delete backup ref: %w", err)
	}

	if backup.HasStash {
		stashIdx, sErr := a.findStashByLabel("git-courer:backup:" + backup.Operation)
		if sErr == nil {
			_, err = a.runGit("stash", "drop", fmt.Sprintf("stash@{%d}", stashIdx))
			if err != nil {
				return fmt.Errorf("failed to drop stash: %w", err)
			}
		}
	}
	return nil
}

func (a *ExecAdapter) hasUnstaged() (bool, error) {
	out, err := a.runGit("status", "--porcelain")
	if err != nil {
		return false, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		if len(line) < 2 {
			continue
		}
		if line[1] != ' ' && line[1] != '?' {
			return true, nil
		}
		if line[0] != ' ' && line[0] != '?' {
			return true, nil
		}
	}
	return false, nil
}

func (a *ExecAdapter) hasUntracked() (bool, error) {
	out, err := a.runGit("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (a *ExecAdapter) hasUnstagedOrUntracked() (bool, error) {
	out, err := a.runGit("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (a *ExecAdapter) findStashByLabel(label string) (int, error) {
	out, err := a.runGit("stash", "list")
	if err != nil {
		return -1, err
	}
	for i, line := range strings.Split(out, "\n") {
		if strings.Contains(line, label) {
			return i, nil
		}
	}
	return -1, fmt.Errorf("stash with label %q not found", label)
}