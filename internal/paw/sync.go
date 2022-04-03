package paw

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gogitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"golang.org/x/crypto/ssh"
)

const (
	syncKey = "synckey.age"
	remote  = "origin"
)

func SyncGitRepo(ctx context.Context, s Storage, vault *Vault, signer ssh.Signer) error {
	path := vaultRootPath(s, vault.Name)
	repo, err := git.PlainOpen(path)
	if err != nil {
		return err
	}
	c, err := repo.Config()
	if err != nil {
		return err
	}

	parts := strings.Split(c.Remotes[remote].URLs[0], "@")
	user := parts[0]
	auth := &gogitssh.PublicKeys{
		User:   user,
		Signer: signer,
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	log.Println(1)
	err = wt.PullContext(ctx, &git.PullOptions{
		Auth:     auth,
		Progress: os.Stdout,
	})
	if err != nil {
		switch {
		case errors.Is(err, transport.ErrEmptyRemoteRepository):
			// empty repo, continue to push first data
		case errors.Is(err, git.NoErrAlreadyUpToDate):
			// no remote changes
		default:
			return err
		}
	}

	log.Println(2)
	err = wt.AddWithOptions(&git.AddOptions{
		All: true,
	})
	if err != nil {
		return err
	}

	msg := "paw-sync-" + time.Now().Format(time.RFC3339)
	_, err = wt.Commit(msg, &git.CommitOptions{
		All: true,
	})
	if err != nil {
		return err
	}

	return repo.PushContext(ctx, &git.PushOptions{
		RemoteName: remote,
		Auth:       auth,
	})
}

func SyncInitGitRepo(s Storage, vault *Vault, remoteURL string, branch string) error {

	path := vaultRootPath(s, vault.Name)

	repo, err := git.PlainInit(path, false)
	if err != nil {
		return err
	}

	err = repo.CreateBranch(&config.Branch{
		Name:   branch,
		Remote: remote,
		Merge:  plumbing.NewBranchReferenceName(branch),
	})
	if err != nil {
		return err
	}

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: remote,
		URLs: []string{remoteURL},
	})
	return err
}
