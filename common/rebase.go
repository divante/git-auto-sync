package common

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/ztrue/tracerr"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

var errRebaseFailed = errors.New("git rebase failed")

// errDirtyTree means git refused to start the rebase because tracked files
// changed after the commit step, usually a write racing the sync cycle.
var errDirtyTree = errors.New("git rebase refused: tracked changes in working tree")

func rebase(repoConfig RepoConfig) error {
	repoPath := repoConfig.RepoPath
	bi, err := fetchBranchInfo(repoPath)
	if err != nil {
		return tracerr.Wrap(err)
	}

	_, rebaseErr := GitCommand(repoConfig, []string{"rebase", bi.UpstreamRemote + "/" + bi.UpstreamBranch})
	if rebaseErr != nil {
		rebaseInProgress, err := isRebasing(repoPath)
		if err != nil {
			return tracerr.Wrap(err)
		}

		var exerr *exec.ExitError
		if errors.As(rebaseErr, &exerr) && exerr.ExitCode() == 1 && rebaseInProgress {
			_, err := GitCommand(repoConfig, []string{"rebase", "--abort"})
			if err != nil {
				return tracerr.Wrap(err)
			}
			return fmt.Errorf("%w: %v", errRebaseFailed, rebaseErr)
		}

		status, err := GitCommand(repoConfig, []string{"status", "--porcelain", "--untracked-files=no"})
		if err != nil {
			return tracerr.Wrap(err)
		}
		if !rebaseInProgress && status.Len() > 0 {
			return fmt.Errorf("%w: %v", errDirtyTree, rebaseErr)
		}
		return tracerr.Wrap(rebaseErr)
	}

	return nil
}

func exists(name string) (bool, error) {
	_, err := os.Stat(name)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

type branchInfo struct {
	CurrentBranch  string
	UpstreamRemote string
	UpstreamBranch string
}

func fetchBranchInfo(repoPath string) (branchInfo, error) {
	repo, err := git.PlainOpenWithOptions(repoPath, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return branchInfo{}, tracerr.Wrap(err)
	}

	config, err := repo.Config()
	if err != nil {
		return branchInfo{}, tracerr.Wrap(err)
	}

	ref, err := repo.Reference(plumbing.HEAD, false)
	if err != nil {
		return branchInfo{}, tracerr.Wrap(err)
	}

	currentBranchName := ref.Target().Short()
	branchConfig := config.Branches[currentBranchName]
	if branchConfig == nil {
		// No tracking branch, nothing to do
		return branchInfo{CurrentBranch: currentBranchName}, nil
	}

	return branchInfo{
		CurrentBranch:  currentBranchName,
		UpstreamRemote: branchConfig.Remote,
		UpstreamBranch: branchConfig.Merge.Short(),
	}, nil
}

func isRebasing(repoPath string) (bool, error) {
	ra, err := exists(path.Join(repoPath, ".git", "rebase-apply"))
	if err != nil {
		return false, tracerr.Wrap(err)
	}

	rm, err := exists(path.Join(repoPath, ".git", "rebase-merge"))
	if err != nil {
		return false, tracerr.Wrap(err)
	}

	return ra || rm, nil
}
