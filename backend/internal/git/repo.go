package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func EnsureRepo(repoURL, localPath string) (string, error) {
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		if err := os.MkdirAll(localPath, 0o755); err != nil {
			return "", err
		}
		if err := runGit("clone", repoURL, localPath); err != nil {
			return "", err
		}
		return HeadCommit(localPath)
	}
	if _, err := os.Stat(filepath.Join(localPath, ".git")); os.IsNotExist(err) {
		if err := os.RemoveAll(localPath); err != nil {
			return "", err
		}
		if err := os.MkdirAll(localPath, 0o755); err != nil {
			return "", err
		}
		if err := runGit("clone", repoURL, localPath); err != nil {
			return "", err
		}
		return HeadCommit(localPath)
	}
	return HeadCommit(localPath)
}

func Pull(localPath string) (string, string, error) {
	oldCommit, err := HeadCommit(localPath)
	if err != nil {
		return "", "", err
	}
	if err := runGitC(localPath, "pull", "--ff-only", "origin", "HEAD"); err != nil {
		if isAlreadyUpToDate(err) {
			return oldCommit, oldCommit, nil
		}
		return "", "", err
	}
	newCommit, err := HeadCommit(localPath)
	if err != nil {
		return "", "", err
	}
	return oldCommit, newCommit, nil
}

func Diff(localPath, oldCommit, newCommit string) ([]string, error) {
	out, err := runGitOutput("-C", localPath, "diff", "--name-only", oldCommit, newCommit)
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

func HeadCommit(localPath string) (string, error) {
	out, err := runGitOutput("-C", localPath, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func FileExistsAtCommit(localPath, commit, file string) (bool, error) {
	out, err := runGitOutput("-C", localPath, "ls-tree", "-r", "--name-only", commit, "--", file)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == file, nil
}

func runGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runGitC(localPath string, args ...string) error {
	fullArgs := append([]string{"-C", localPath}, args...)
	return runGit(fullArgs...)
}

func runGitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}

func splitLines(s string) []string {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(s), "\r\n", "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

func isAlreadyUpToDate(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "already up to date")
}
