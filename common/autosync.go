package common

import (
	"errors"
	"log"

	"github.com/gen2brain/beeep"
	"github.com/ztrue/tracerr"
)

// notifyConflict is replaced in tests
var notifyConflict = beeep.Alert

// FIXME: Add logs for when we commit, pull, and push
func AutoSync(repoConfig RepoConfig) error {
	var err error
	err = ensureGitAuthor(repoConfig)
	if err != nil {
		return tracerr.Wrap(err)
	}

	err = commit(repoConfig)
	if err != nil {
		return tracerr.Wrap(err)
	}

	err = fetch(repoConfig)
	if err != nil {
		return tracerr.Wrap(err)
	}

	err = rebase(repoConfig)
	if errors.Is(err, errDirtyTree) {
		// The racing write produces its own event, and the next cycle commits
		// it before rebasing. Exiting here would only restart the daemon.
		log.Printf("%s: skipping sync cycle: %v", repoConfig.RepoPath, err)
		return nil
	}
	if err != nil {
		if errors.Is(err, errRebaseFailed) {
			repoPath := repoConfig.RepoPath
			// The alert is best effort; on a headless host it fails and must not
			// replace the rebase error that explains the stop.
			if notifyErr := notifyConflict("Git Auto Sync - Conflict", "Could not rebase for - "+repoPath, "assets/warning.png"); notifyErr != nil {
				log.Printf("%s conflict notification failed: %v", repoPath, notifyErr)
			}
		}
		// How should we continue?
		// - Keep sending the notification each time?
		// - Or something a bit better?
		return tracerr.Wrap(err)
	}

	err = push(repoConfig)
	if err != nil {
		return tracerr.Wrap(err)
	}

	// -> do a merge
	// -> push the changes

	return nil
}
