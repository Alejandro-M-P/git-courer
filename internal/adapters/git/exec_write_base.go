package git

import (
	"log"
)

func (a *ExecAdapter) Add(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	log.Printf("[DEBUG] gitAdapter.Add: paths=%v", paths)

	for _, p := range paths {
		if _, err := a.runGit("add", p); err != nil {
			log.Printf("[DEBUG] gitAdapter.Add: error=%v", err)
			return err
		}
	}

	return nil
}

func (a *ExecAdapter) Remove(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	_, err := a.runGit(append([]string{"rm"}, paths...)...)
	return err
}

func (a *ExecAdapter) Reset(mode string, commit string) (string, error) {
	return a.runGit("reset", mode, commit)
}

func (a *ExecAdapter) ResetSoft(target string) error {
	_, err := a.runGit("reset", "--soft", target)
	return err
}

func (a *ExecAdapter) Restore(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	_, err := a.runGit(append([]string{"restore"}, paths...)...)
	return err
}

func (a *ExecAdapter) Clean() error {
	_, err := a.runGit("clean", "-fd")
	return err
}

func (a *ExecAdapter) Merge(branch string) (string, error) {
	return a.runGit("merge", branch)
}

func (a *ExecAdapter) MergeAbort() (string, error) {
	return a.runGit("merge", "--abort")
}

func (a *ExecAdapter) MergeContinue() (string, error) {
	return a.runGit("merge", "--continue")
}

func (a *ExecAdapter) MergeSkip() (string, error) {
	return a.runGit("merge", "--skip")
}

func (a *ExecAdapter) ConfigSet(key, value string) (string, error) {
	return a.runGit("config", key, value)
}
