package git

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// maxBackups is the auto-prune threshold: when the existing backup count
// reaches this value, CreateBackup fires PruneBackups(30d) before creating a
// new ref. Prune failure is logged but never blocks backup creation.
const maxBackups = 20

func (a *ExecAdapter) CreateBackup(operation string, mode domain.StashMode) (domain.Backup, error) {
	timestamp := time.Now().Format("20060102150405")
	ref := fmt.Sprintf("refs/git-courer/backup/%s_%s", timestamp, operation)

	// Auto-prune gate: prune before creating a new ref when the backup count
	// has reached the threshold. Prune failure MUST NOT block the backup.
	if backups, lErr := a.ListBackups(); lErr != nil {
		log.Printf("[WARN] backup: failed to list backups before auto-prune: %v", lErr)
	} else if len(backups) >= maxBackups {
		if pErr := a.PruneBackups(30 * 24 * time.Hour); pErr != nil {
			// Prune failure is logged (preserving the wrapped cause in the
			// message) but MUST NOT block the backup — the ref still gets created.
			log.Printf("[WARN] backup: auto-prune before %s failed: %v", operation, pErr)
		}
	}

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
		// ListBackups uses %(contents) which fails when a ref points at a
		// missing object. Recover by listing refnames only — the commit is
		// resolved per-ref below, and corrupt refs are deleted there. This
		// keeps ListBackups' behavior unchanged while letting PruneBackups
		// honor its corrupt-ref recovery contract.
		log.Printf("[WARN] backup: ListBackups failed during prune, falling back to refname-only listing: %v", err)
		backups, err = a.listBackupRefs()
		if err != nil {
			return fmt.Errorf("listing backup refs for prune: %w", err)
		}
	}

	// Collect tips: HEAD + every local branch tip from refs/heads/.
	tips, err := a.collectReachabilityTips()
	if err != nil {
		return fmt.Errorf("collecting reachability tips: %w", err)
	}

	now := time.Now()
	for _, b := range backups {
		// Resolve the backup's target commit. A ref that cannot be resolved is
		// corrupt/dangling — recover by deleting it and continuing the loop.
		commit, err := a.resolveBackupCommit(b.Ref)
		if err != nil {
			log.Printf("[WARN] backup: cannot resolve commit for %s, deleting: %v", b.Ref, err)
			if dErr := a.DeleteBackup(b); dErr != nil {
				log.Printf("[WARN] backup: failed to delete corrupt ref %s: %v", b.Ref, dErr)
			}
			continue
		}

		// Reachability: a backup is reachable if its commit is an ancestor of
		// HEAD or any local branch tip. Short-circuit on the first match.
		reachable := a.isReachable(commit, tips)

		switch {
		case !reachable:
			// Unreachable from all tips — delete regardless of age.
			if dErr := a.DeleteBackup(b); dErr != nil {
				log.Printf("[WARN] backup: failed to delete unreachable ref %s: %v", b.Ref, dErr)
			}
		case now.Sub(b.CreatedAt) > olderThan:
			// Reachable AND older than the window — delete.
			if dErr := a.DeleteBackup(b); dErr != nil {
				log.Printf("[WARN] backup: failed to delete stale ref %s: %v", b.Ref, dErr)
			}
		default:
			// Reachable and recent — keep.
		}
	}
	return nil
}

// listBackupRefs lists backup refs by refname only (without resolving the
// target object), used as a fallback when ListBackups fails due to a corrupt
// ref. The returned Backup entries carry Ref/Operation/CreatedAt; the target
// commit is resolved later per-ref by resolveBackupCommit.
func (a *ExecAdapter) listBackupRefs() ([]domain.Backup, error) {
	out, err := a.runGit("for-each-ref", "refs/git-courer/backup/", "--format=%(refname)")
	if err != nil {
		return nil, err
	}
	var backups []domain.Backup
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		ref := strings.TrimSpace(line)
		if ref == "" {
			continue
		}
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

// collectReachabilityTips returns the commit OIDs for HEAD and every local
// branch tip under refs/heads/. HEAD is always first.
func (a *ExecAdapter) collectReachabilityTips() ([]string, error) {
	var tips []string

	head, err := a.runGit("rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("resolving HEAD: %w", err)
	}
	if h := strings.TrimSpace(head); h != "" {
		tips = append(tips, h)
	}

	// Local branch tips: one OID per line.
	out, err := a.runGit("for-each-ref", "refs/heads/", "--format=%(objectname)")
	if err != nil {
		return nil, fmt.Errorf("listing local branch tips: %w", err)
	}
	for _, line := range strings.Split(out, "\n") {
		if oid := strings.TrimSpace(line); oid != "" {
			tips = append(tips, oid)
		}
	}
	return tips, nil
}

// resolveBackupCommit resolves a backup ref to its target commit OID via
// `git rev-parse {ref}^{commit}`. Returns an error if the ref is corrupt or
// points at an object that cannot be peeled to a commit.
func (a *ExecAdapter) resolveBackupCommit(ref string) (string, error) {
	out, err := a.runGit("rev-parse", fmt.Sprintf("%s^{commit}", ref))
	if err != nil {
		return "", fmt.Errorf("rev-parse %s^{commit}: %w", ref, err)
	}
	return strings.TrimSpace(out), nil
}

// isReachable reports whether commit is an ancestor of ANY of the given tips.
// It short-circuits on the first reachable tip. A tip identical to commit is
// also considered reachable (merge-base --is-ancestor is reflexive for equal
// commits).
func (a *ExecAdapter) isReachable(commit string, tips []string) bool {
	for _, tip := range tips {
		if tip == "" {
			continue
		}
		// git merge-base --is-ancestor exits 0 when commit is an ancestor of
		// tip (inclusive), non-zero otherwise. runGit returns an error on
		// non-zero exit, so a non-nil error here means "not an ancestor of
		// this tip" — try the next one.
		if _, err := a.runGit("merge-base", "--is-ancestor", commit, tip); err == nil {
			return true
		}
	}
	return false
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
