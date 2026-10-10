package common

import (
	"errors"
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
