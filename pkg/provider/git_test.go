package provider

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-semantic-release/semantic-release/v2/pkg/provider"
	"github.com/stretchr/testify/require"
)

var testGitPath string

func TestGit(t *testing.T) {
	var err error
	testGitPath, err = setupRepo()
	require.NoError(t, err)
	t.Run("NewRepository", newRepository)
	t.Run("GetInfo", getInfo)
	t.Run("GetReleases", getReleases)
	t.Run("GetCommits", getCommits)
	t.Run("CreateRelease", createRelease)
}

func newRepository(t *testing.T) {
	require := require.New(t)
	repo := &Repository{}
	err := repo.Init(map[string]string{})
	require.EqualError(err, "repository does not exist")

	repo = &Repository{}
	err = repo.Init(map[string]string{
		"git_path":       testGitPath,
		"default_branch": "development",
		"tagger_name":    "test",
		"tagger_email":   "test@test.com",
		"auth":           "basic",
		"auth_username":  "test",
		"auth_password":  "test",
	})

	require.NoError(err)
	require.Equal("development", repo.defaultBranch)
	require.Equal("test", repo.taggerName)
	require.Equal("test@test.com", repo.taggerEmail)
	require.NotNil(repo.auth)
}

func setupRepo() (string, error) {
	dir, err := ioutil.TempDir("", "provider-git")
	if err != nil {
		return "", err
	}
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		return "", err
	}

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"http://localhost:3000/test/test.git"},
	})
	if err != nil {
		return "", err
	}
	w, err := repo.Worktree()
	if err != nil {
		return "", err
	}

	author := &object.Signature{
		Name:  "test",
		Email: "test@test.com",
		When:  time.Now(),
	}
	versionCount := 0
	betaCount := 1
	for i := 0; i < 100; i++ {
		commit, err := w.Commit(fmt.Sprintf("feat: commit %d", i), &git.CommitOptions{Author: author})
		if err != nil {
			return "", err
		}
		if i%10 == 0 {
			if _, err := repo.CreateTag(fmt.Sprintf("v1.%d.0", versionCount), commit, nil); err != nil {
				return "", err
			}
			versionCount++
		}
		if i%5 == 0 {
			if _, err := repo.CreateTag(fmt.Sprintf("v2.0.0-beta.%d", betaCount), commit, nil); err != nil {
				return "", err
			}
			betaCount++
		}
	}

	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("new-fix"),
		Create: true,
	})
	if err != nil {
		return "", err
	}

	if _, err = w.Commit("fix: error", &git.CommitOptions{Author: author}); err != nil {
		return "", err
	}
	if err = w.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName("master")}); err != nil {
		return "", err
	}

	err = repo.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			"refs/heads/*:refs/heads/*",
			"refs/tags/*:refs/tags/*",
		},
		Auth: &http.BasicAuth{
			Username: "test",
			Password: "test",
		},
		Force: true,
	})
	if err != nil {
		return "", err
	}
	return dir, nil
}

func createRepo() (*Repository, error) {
	repo := &Repository{}
	err := repo.Init(map[string]string{
		"git_path":      testGitPath,
		"auth":          "basic",
		"auth_username": "test",
		"auth_password": "test",
	})
	if err != nil {
		return nil, err
	}

	return repo, nil
}

func getInfo(t *testing.T) {
	require := require.New(t)
	repo, err := createRepo()
	require.NoError(err)
	repoInfo, err := repo.GetInfo()
	require.NoError(err)
	require.Equal("master", repoInfo.DefaultBranch)
}

func getCommits(t *testing.T) {
	require := require.New(t)
	repo, err := createRepo()
	require.NoError(err)
	commits, err := repo.GetCommits("", "master")
	require.NoError(err)
	require.Len(commits, 100)

	for _, c := range commits {
		require.True(strings.HasPrefix(c.RawMessage, "feat: commit"))
	}
}

func createRelease(t *testing.T) {
	require := require.New(t)
	repo, err := createRepo()
	require.NoError(err)

	gRepo, err := git.PlainOpen(testGitPath)
	require.NoError(err)
	head, err := gRepo.Head()
	require.NoError(err)

	err = repo.CreateRelease(&provider.CreateReleaseConfig{
		NewVersion: "2.0.0",
		SHA:        head.Hash().String(),
		Changelog:  "new feature",
	})
	require.NoError(err)

	tagRef, err := gRepo.Tag("v2.0.0")
	require.NoError(err)

	tagObj, err := gRepo.TagObject(tagRef.Hash())
	require.NoError(err)

	require.Equal("new feature\n", tagObj.Message)
}

func getReleases(t *testing.T) {
	require := require.New(t)
	repo, err := createRepo()
	require.NoError(err)

	releases, err := repo.GetReleases("")
	require.NoError(err)
	require.Len(releases, 30)

	releases, err = repo.GetReleases("^v2")
	require.NoError(err)
	require.Len(releases, 20)
}
