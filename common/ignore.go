package common

import (
	"context"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ztrue/tracerr"
)

func ShouldIgnoreFile(repoPath string, filePath string) (bool, error) {
	if !filepath.IsAbs(filePath) {
		filePath = path.Join(repoPath, filePath)
	}

	ignored, err := isIgnoredPath(repoPath, filePath)
	if err != nil || ignored {
		return ignored, err
	}
	return isEmptyFile(filePath)
}

// isIgnoredPath applies every ignore rule except the empty-file placeholder
// rule, which only suits files Git does not track yet.
func isIgnoredPath(repoPath string, filePath string) (bool, error) {
	if !filepath.IsAbs(filePath) {
		filePath = path.Join(repoPath, filePath)
	}

	fileName := filepath.Base(filePath)
	var isTempFile = strings.HasSuffix(fileName, ".swp") || // vim
		strings.HasPrefix(fileName, "~") || // emacs
		strings.HasSuffix(fileName, "~") // kate

	if isTempFile {
		return true, nil
	}

	relativePath, err := filepath.Rel(repoPath, filePath)
	if err != nil {
		return false, err
	}
	if relativePath == ".git" || strings.HasPrefix(relativePath, ".git"+string(os.PathSeparator)) {
		return true, nil
	}

	// Native Git prunes ignored directories instead of recursively reading
	// their contents, which may disappear while a runtime database writes.
	return isFileIgnoredByGit(repoPath, filePath)
}

func isFileIgnoredByGit(repoPath string, filePath string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "check-ignore", "--quiet", "--", filePath)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, tracerr.Errorf("git check-ignore: %w: %s", err, output)
}

func isEmptyFile(filePath string) (bool, error) {
	stat, err := os.Stat(filePath)

	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return stat.Size() == 0, nil
}
