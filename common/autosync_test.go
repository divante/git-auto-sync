package common

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

// A failed conflict notification must not replace the rebase error
func Test_AutoSyncConflictNotifyFailureKeepsRebaseError(t *testing.T) {
	repoConfig := PrepareMultiFixtures(t, "rebase_both_commits_conflict", []string{"rebase_parent"})

	errNotify := errors.New("no notification service")
	origNotify := notifyConflict
	notifyConflict = func(title, message, icon string) error { return errNotify }
	t.Cleanup(func() { notifyConflict = origNotify })

	err := AutoSync(repoConfig)
	assert.Assert(t, errors.Is(err, errRebaseFailed), err)
	assert.Assert(t, !errors.Is(err, errNotify), err)
}

// A tracked change the commit step leaves behind makes git refuse the rebase.
// The cycle is skipped instead of failing, and the change stays on disk for
// the next cycle.
func Test_AutoSyncSkipsCycleWhenRebaseRefusesDirtyTree(t *testing.T) {
	repoConfig := PrepareMultiFixtures(t, "rebase_both_commits", []string{"rebase_parent"})
	repoPath := repoConfig.RepoPath

	// Swap files are tracked here but always skipped by the commit step
	swapPath := filepath.Join(repoPath, "note.swp")
	assert.NilError(t, os.WriteFile(swapPath, []byte("v1\n"), 0644))
	_, err := GitCommand(repoConfig, []string{"add", "-f", "note.swp"})
	assert.NilError(t, err)
	_, err = GitCommand(repoConfig, []string{"commit", "-m", "track swap"})
	assert.NilError(t, err)
	assert.NilError(t, os.WriteFile(swapPath, []byte("v2\n"), 0644))

	err = AutoSync(repoConfig)
	assert.NilError(t, err)

	status, err := GitCommand(repoConfig, []string{"status", "--porcelain", "--untracked-files=no"})
	assert.NilError(t, err)
	assert.Equal(t, status.String(), " M note.swp\n")
}
