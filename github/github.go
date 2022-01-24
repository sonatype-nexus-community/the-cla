//
// Copyright 2021-present Sonatype Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//go:build go1.16
// +build go1.16

package github

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"net/http"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v42/github"
	"github.com/sonatype-nexus-community/the-cla/db"
	"github.com/sonatype-nexus-community/the-cla/types"
	webhook "gopkg.in/go-playground/webhooks.v5/github"
)

const FilenameTheClaPem string = "the-cla.pem"
const EnvGhAppId = "GH_APP_ID"

// RepositoriesService handles communication with the repository related methods
// of the GitHub API.
// https://godoc.org/github.com/google/go-github/github#RepositoriesService
type RepositoriesService interface {
	Get(context.Context, string, string) (*github.Repository, *github.Response, error)
	ListStatuses(ctx context.Context, owner, repo, ref string, opts *github.ListOptions) ([]*github.RepoStatus, *github.Response, error)
	CreateStatus(ctx context.Context, owner, repo, ref string, status *github.RepoStatus) (*github.RepoStatus, *github.Response, error)
}

// UsersService handles communication with the user related methods
// of the GitHub API.
// https://godoc.org/github.com/google/go-github/github#UsersService
type UsersService interface {
	Get(context.Context, string) (*github.User, *github.Response, error)
}

// PullRequestsService handles communication with the pull request related
// methods of the GitHub API.
//
// GitHub API docs: https://docs.github.com/en/free-pro-team@latest/rest/reference/pulls/
type PullRequestsService interface {
	ListCommits(ctx context.Context, owner string, repo string, number int, opts *github.ListOptions) ([]*github.RepositoryCommit, *github.Response, error)
}

// IssuesService handles communication with the issue related
// methods of the GitHub API.
//
// GitHub API docs: https://docs.github.com/en/free-pro-team@latest/rest/reference/issues/
type IssuesService interface {
	GetLabel(ctx context.Context, owner string, repo string, name string) (*github.Label, *github.Response, error)
	ListLabelsByIssue(ctx context.Context, owner string, repo string, issueNumber int, opts *github.ListOptions) ([]*github.Label, *github.Response, error)
	CreateLabel(ctx context.Context, owner string, repo string, label *github.Label) (*github.Label, *github.Response, error)
	AddLabelsToIssue(ctx context.Context, owner string, repo string, number int, labels []string) ([]*github.Label, *github.Response, error)
	CreateComment(ctx context.Context, owner string, repo string, number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error)
	ListComments(ctx context.Context, owner string, repo string, number int, opts *github.IssueListCommentsOptions) ([]*github.IssueComment, *github.Response, error)
}

// GitHubClient manages communication with the GitHub API.
// https://github.com/google/go-github/issues/113
type GitHubClient struct {
	Repositories RepositoriesService
	Users        UsersService
	PullRequests PullRequestsService
	Issues       IssuesService
}

// GitHubInterface defines all necessary methods.
// https://godoc.org/github.com/google/go-github/github#NewClient
type GitHubInterface interface {
	NewClient(httpClient *http.Client) GitHubClient
}

// GitHubCreator implements GitHubInterface.
type GitHubCreator struct{}

// NewClient returns a new GitHubInterface instance.
func (g *GitHubCreator) NewClient(httpClient *http.Client) GitHubClient {
	client := github.NewClient(httpClient)
	return GitHubClient{
		Repositories: client.Repositories,
		Users:        client.Users,
		PullRequests: client.PullRequests,
		Issues:       client.Issues,
	}
}

var githubImpl GitHubInterface = &GitHubCreator{}

func HandlePullRequest(logger *zap.Logger,
	postgres db.IClaDB,
	payload webhook.PullRequestPayload,
	appId int,
	claVersion string) (string, error) {
	logger.Debug("Attempting to start authenticating with GitHub")

	owner := payload.Repository.Owner.Login
	repo := payload.Repository.Name
	sha := payload.PullRequest.Head.Sha
	pullRequestID := int(payload.Number)

	logger.Debug(fmt.Sprintf("Transport setup, using appID: %d and installation ID: %d", appId, payload.Installation.ID))
	itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, int64(appId), payload.Installation.ID, FilenameTheClaPem)
	if err != nil {
		return "", err
	}

	client := githubImpl.NewClient(&http.Client{Transport: itr})

	err = createRepoStatus(logger, client.Repositories, owner, repo, sha, "pending", "Paul Botsco, the CLA verifier is running")
	if err != nil {
		return "", err
	}

	opts := &github.ListOptions{}

	commits, _, err := client.PullRequests.ListCommits(
		context.Background(),
		owner,
		repo,
		pullRequestID, opts)
	if err != nil {
		return "", err
	}

	// TODO: Once we have stuff in a DB, we can iterate over the list of commits,
	// find the authors, and check if they have signed the CLA (and the version that is most current)
	// The following loop will change a loop as a result
	var usersNeedingToSignCLA []types.UserSignature

	for _, v := range commits {
		// It is important to use GetAuthor() instead of v.Commit.GetCommitter() because the committer can be the GH webflow user, where as the author is
		// the canonical author of the commit
		author := *v.GetAuthor()

		hasAuthorSigned, err := postgres.HasAuthorSignedTheCla(*author.Login, claVersion)
		if err != nil {
			return "", err
		}
		if !hasAuthorSigned {
			usersNeedingToSignCLA = append(usersNeedingToSignCLA,
				types.UserSignature{
					User: types.User{
						Login:     author.GetLogin(),
						Email:     author.GetEmail(),
						GivenName: author.GetName(),
					},
					CLAVersion: claVersion,
					//TimeSigned: time.Time{},
				})
		}
	}

	if len(usersNeedingToSignCLA) > 0 {
		err := createRepoLabel(logger, client.Issues, owner, repo, labelNameCLANotSigned, "ff3333", "The CLA needs to be signed", pullRequestID)
		if err != nil {
			return "", err
		}

		var users []string
		for _, v := range usersNeedingToSignCLA {
			users = append(users, " @"+v.User.Login)
		}

		message := "Thanks for the contribution. Before we can merge this, we need %s to sign the Contributor License Agreement"
		userMsg := strings.Join(users, ",")

		_, err = addCommentToIssueIfNotExists(logger, client.Issues, owner, repo, pullRequestID, fmt.Sprintf(message, userMsg))
		if err != nil {
			return "", err
		}

		err = createRepoStatus(logger, client.Repositories, owner, repo, sha, "failure", "One or more contributors need to sign the CLA")
		if err != nil {
			return "", err
		}
	} else {
		logger.Debug("Attempting to create label for signed CLA")
		err = createRepoLabel(logger, client.Issues, owner, repo, labelNameCLASigned, "66CC00", "The CLA is signed", pullRequestID)
		if err != nil {
			return "", err
		}

		err = createRepoStatus(logger, client.Repositories, owner, repo, sha, "success", "All contributors have signed the CLA")
		if err != nil {
			return "", err
		}
	}

	return "things", nil
}

func createRepoStatus(logger *zap.Logger,
	repositoryService RepositoriesService,
	owner, repo, sha, state, description string) error {
	_, _, err := repositoryService.CreateStatus(context.Background(), owner, repo, sha, &github.RepoStatus{State: &state, Description: &description})
	if err != nil {
		return err
	}
	return nil
}

const labelNameCLANotSigned string = ":monocle_face: cla not signed"
const labelNameCLASigned string = ":heart_eyes: cla signed"

func createRepoLabel(logger *zap.Logger,
	issuesService IssuesService,
	owner, repo, name, color, description string,
	pullRequestID int) error {
	logger.Debug(fmt.Sprintf("Attempting to add or create label for: %s", name))

	lbl, err := _createRepoLabelIfNotExists(logger, issuesService, owner, repo, name, color, description)
	if err != nil {
		return err
	}

	_, err = _addLabelToIssueIfNotExists(logger, issuesService, owner, repo, pullRequestID, lbl.GetName())
	if err != nil {
		return err
	}

	return nil
}

func _createRepoLabelIfNotExists(logger *zap.Logger,
	issuesService IssuesService,
	owner, repo, name, color, description string) (desiredLabel *github.Label, err error) {
	logger.Debug(fmt.Sprintf("Attempting to create label: %s", name))

	desiredLabel, res, err := issuesService.GetLabel(context.Background(), owner, repo, name)
	if err != nil {
		return
	}
	if desiredLabel != nil {
		logger.Debug(fmt.Sprintf("Found existing label, returning it %+v", desiredLabel))
		return
	}
	if res.StatusCode == 404 {
		logger.Debug(fmt.Sprintf("Looks like the label doesn't exist, so create it, name: %s, color: %s, description: %s", name, color, description))

		strName := name
		strColor := color
		strDescription := description
		newLabel := &github.Label{Name: &strName, Color: &strColor, Description: &strDescription}
		desiredLabel, _, err = issuesService.CreateLabel(context.Background(), owner, repo, newLabel)

		return
	}

	return
}

func addCommentToIssueIfNotExists(logger *zap.Logger, issuesService IssuesService, owner, repo string, issueNumber int, message string) (*github.IssueComment, error) {
	opts := &github.IssueListCommentsOptions{}
	comments, _, err := issuesService.ListComments(context.Background(), owner, repo, issueNumber, opts)
	if err != nil {
		return nil, err
	}
	alreadyCommented := false
	for _, v := range comments {
		if *v.Body == message {
			alreadyCommented = true
		}
	}

	if !alreadyCommented {
		prComment := &github.IssueComment{}
		prComment.Body = &message

		comment, _, err := issuesService.CreateComment(context.Background(), owner, repo, issueNumber, prComment)
		if err != nil {
			return nil, err
		}
		return comment, err
	}

	return nil, nil
}

func _addLabelToIssueIfNotExists(logger *zap.Logger, issuesService IssuesService, owner, repo string, issueNumber int, labelName string) (desiredLabel *github.Label, err error) {
	// check if label is already added to issue
	opts := github.ListOptions{}
	issueLabels, _, err := issuesService.ListLabelsByIssue(context.Background(), owner, repo, issueNumber, &opts)
	if err != nil {
		return
	}
	for _, existingLabel := range issueLabels {
		if *existingLabel.Name == labelName {
			logger.Debug("Found label on issue, getting out of here")
			// label already exists on this issue
			desiredLabel = existingLabel
			return
		}
	}

	// didn't find the label on this issue, so add the label to this issue
	// @TODO Verify this does not remove existing labels (any label not in our "add" array)
	logger.Debug("Attempting to add label to issue")
	logger.Debug(fmt.Sprintf("Label: %s", labelName))
	_, _, err = issuesService.AddLabelsToIssue(
		context.Background(),
		owner,
		repo,
		issueNumber,
		[]string{labelName},
	)
	return
}
