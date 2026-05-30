package git

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

func (a *ExecAdapter) CreateBackup(operation string, mode domain.StashMode) (domain.Backup, error) {
	timestamp := time.Now().Format("20060102150405")
	ref := fmt.Sprintf("refs/git-courer/backup/%s_%s", timestamp, operation)

	_, err := a.runGit("update-ref", "-m", fmt.Sprintf("git-courer backup: %s", operation), ref, "HEAD")
	if err != nil {
		return domain.Backup{}, fmt.Errorf("failed to create backup ref: %w", err)
	}

	hasStash := false
	if mode != domain.StashNone {
		unstaged, _ := a.hasUnstaged()
		untracked, _ := a.hasUntracked()

		if unstaged || (mode == domain.StashAll && untracked) {
			hasStash = true
			label := fmt.Sprintf("git-courer:backup:%s:%s", operation, timestamp)
			args := []string{"stash", "push", "-m", label}
			if mode == domain.StashAll {
				args = append(args, "--include-untracked")
			}
			_, err = a.runGit(args...)
			if err != nil {
				return domain.Backup{}, fmt.Errorf("failed to create stash: %w", err)
			}
		}
	}

	return domain.Backup{
		Ref:       ref,
		HasStash:  hasStash,
		Operation: operation,
		CreatedAt: time.Now(),
		StashMode: mode,
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

func (a *ExecAdapter) ListBackups() ([]domain.Backup, error) {
	out, err := a.runGit("for-each-ref", "refs/git-courer/backup/", "--format=%(refname)|%(contents)")
	if err != nil {
		return nil, err
	}
	var backups []domain.Backup
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}
		// Parsing timestamp and op from refname: refs/git-courer/backup/20260503020256_ADD
		ref := parts[0]
		name := filepath.Base(ref)
		nameParts := strings.SplitN(name, "_", 2)
		if len(nameParts) < 2 {
			continue
		}
		t, _ := time.Parse("20060102150405", nameParts[0])
		backups = append(backups, domain.Backup{
			Ref:       ref,
			Operation: nameParts[1],
			CreatedAt: t,
		})
	}
	return backups, nil
}

func (a *ExecAdapter) PruneBackups(olderThan time.Duration) error {
	backups, err := a.ListBackups()
	if err != nil {
		return err
	}
	now := time.Now()
	for _, b := range backups {
		if now.Sub(b.CreatedAt) > olderThan {
			a.DeleteBackup(b)
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
