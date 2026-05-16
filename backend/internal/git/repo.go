package git

import (
	"errors"
	"os"
	"path/filepath"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func EnsureRepo(repoURL, localPath string) (*gogit.Repository, error) {
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
			return nil, err
		}
		return gogit.PlainClone(localPath, false, &gogit.CloneOptions{URL: repoURL, Progress: nil})
	}
	if _, err := gogit.PlainOpen(localPath); err != nil {
		if _, statErr := os.Stat(filepath.Join(localPath, ".git")); os.IsNotExist(statErr) {
			if err := os.RemoveAll(localPath); err != nil {
				return nil, err
			}
			return gogit.PlainClone(localPath, false, &gogit.CloneOptions{URL: repoURL, Progress: nil})
		}
		return nil, err
	}
	return gogit.PlainOpen(localPath)
}

func Pull(localPath string) (string, string, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return "", "", err
	}
	head, err := repo.Head()
	if err != nil {
		return "", "", err
	}
	oldCommit := head.Hash().String()
	wt, err := repo.Worktree()
	if err != nil {
		return "", "", err
	}
	if err := wt.Pull(&gogit.PullOptions{RemoteName: "origin", Auth: &http.BasicAuth{}}); err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return "", "", err
	}
	head, err = repo.Head()
	if err != nil {
		return "", "", err
	}
	return oldCommit, head.Hash().String(), nil
}

func Diff(localPath, oldCommit, newCommit string) ([]string, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return nil, err
	}
	oldObj, err := repo.CommitObject(plumbing.NewHash(oldCommit))
	if err != nil {
		return nil, err
	}
	newObj, err := repo.CommitObject(plumbing.NewHash(newCommit))
	if err != nil {
		return nil, err
	}
	oldTree, err := oldObj.Tree()
	if err != nil {
		return nil, err
	}
	newTree, err := newObj.Tree()
	if err != nil {
		return nil, err
	}
	changes, err := object.DiffTree(oldTree, newTree)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(changes))
	for _, c := range changes {
		if c.From.Name != "" {
			files = append(files, c.From.Name)
		} else {
			files = append(files, c.To.Name)
		}
	}
	return files, nil
}

func HeadCommit(localPath string) (string, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return "", err
	}
	head, err := repo.Head()
	if err != nil {
		return "", err
	}
	return head.Hash().String(), nil
}

func FileExistsAtCommit(localPath, commit, file string) (bool, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return false, err
	}
	obj, err := repo.CommitObject(plumbing.NewHash(commit))
	if err != nil {
		return false, err
	}
	tree, err := obj.Tree()
	if err != nil {
		return false, err
	}
	_, err = tree.File(file)
	if err != nil {
		return false, err
	}
	return true, nil
}
