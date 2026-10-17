package common

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ztrue/tracerr"
)

func commit(repoConfig RepoConfig) error {
	repoPath := repoConfig.RepoPath
	status, err := GitCommand(repoConfig, []string{"status", "--porcelain=v1", "-z", "--untracked-files=all"})
	if err != nil {
		return tracerr.Wrap(err)
	}
	commitMsg := []string{}
	entries := strings.Split(status.String(), "\x00")
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if entry == "" {
			continue
		}
		if len(entry) < 4 {
			return tracerr.Errorf("malformed git status entry: %q", entry)
		}
		code, filePath := entry[:2], entry[3:]
		paths := []string{filePath}
		if strings.ContainsAny(code, "RC") {
			i++
			if i >= len(entries) || entries[i] == "" {
				return tracerr.Errorf("missing rename source for %q", filePath)
			}
			if code[1] == 'R' {
				paths = append(paths, entries[i])
			}
		}
		// An emptied tracked file is a real change; skipping it would leave the
		// tree dirty and make the rebase refuse to start.
		ignore, err := isIgnoredPath(repoPath, filePath)
		if err == nil && !ignore && code == "??" {
			ignore, err = isEmptyFile(filepath.Join(repoPath, filePath))
		}
		if err != nil {
			return tracerr.Wrap(err)
		}
		if ignore {
			continue
		}
		_, err = GitCommand(repoConfig, append([]string{"add", "-A", "--"}, paths...))
		if err != nil {
			return tracerr.Wrap(err)
		}
		prefix := " " + string(code[1]) + " "
		if code == "??" {
			prefix = "?? "
		}
		commitMsg = append(commitMsg, prefix+filePath)
	}
	sort.Strings(commitMsg)
	msg := strings.Join(commitMsg, "\n")

	if len(commitMsg) == 0 {
		return nil
	}

	_, err = GitCommand(repoConfig, []string{"commit", "-m", msg})
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func GitCommand(repoConfig RepoConfig, args []string) (bytes.Buffer, error) {
	repoPath := repoConfig.RepoPath

	var outb, errb bytes.Buffer

	cmd := "git"
	if repoConfig.GitExec != "" {
		cmd = repoConfig.GitExec
	}

	statusCmd := exec.Command(cmd, args...)
	statusCmd.Dir = repoPath
	statusCmd.Stdout = &outb
	statusCmd.Stderr = &errb
	err := statusCmd.Run()

	if err != nil {
		fullCmd := "git " + strings.Join(args, " ")
		err := tracerr.Errorf("%w: Command: %s\nStdOut: %s\nStdErr: %s", err, fullCmd, outb.String(), errb.String())
		return outb, err
	}
	return outb, nil
}
