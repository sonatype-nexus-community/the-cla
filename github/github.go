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
	"time"

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
	RemoveLabelForIssue(ctx context.Context, owner string, repo string, number int, label string) (*github.Response, error)
	CreateComment(ctx context.Context, owner string, repo string, number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error)
	ListComments(ctx context.Context, owner string, repo string, number int, opts *github.IssueListCommentsOptions) ([]*github.IssueComment, *github.Response, error)
}

// AppsService provides access to the installation related functions
// in the GitHub API.
//
// GitHub API docs: https://docs.github.com/en/free-pro-team@latest/rest/reference/apps/
type AppsService interface {
	// Get a single GitHub App. Passing the empty string will get
	// the authenticated GitHub App.
	Get(ctx context.Context, appSlug string) (*github.App, *github.Response, error)
}

// GHClient manages communication with the GitHub API.
// https://github.com/google/go-github/issues/113
type GHClient struct {
	Repositories RepositoriesService
	Users        UsersService
	PullRequests PullRequestsService
	Issues       IssuesService
	Apps         AppsService
}

// GHInterface defines all necessary methods.
// https://godoc.org/github.com/google/go-github/github#NewClient
type GHInterface interface {
	NewClient(httpClient *http.Client) GHClient
}

// GHCreator implements GHInterface.
type GHCreator struct{}

// NewClient returns a new GHInterface instance.
func (g *GHCreator) NewClient(httpClient *http.Client) GHClient {
	client := github.NewClient(httpClient)
	return GHClient{
		Repositories: client.Repositories,
		Users:        client.Users,
		PullRequests: client.PullRequests,
		Issues:       client.Issues,
		Apps:         client.Apps,
	}
}

var GHImpl GHInterface = &GHCreator{}

func HandlePullRequest(logger *zap.Logger, postgres db.IClaDB, payload webhook.PullRequestPayload, appId int64, claVersion string) error {

	evalInfo := types.EvaluationInfo{
		RepoOwner: payload.Repository.Owner.Login,
		RepoName:  payload.Repository.Name,
		Sha:       payload.PullRequest.Head.Sha,
		PRNumber:  payload.Number,
		AppId:     appId,
		InstallId: payload.Installation.ID,
		// UserSignatures/Authors will be populated later
	}

	return EvaluatePullRequest(logger, postgres, &evalInfo, claVersion)
}

func EvaluatePullRequest(logger *zap.Logger, postgres db.IClaDB, evalInfo *types.EvaluationInfo, claVersion string) error {
	logger.Debug("start authenticating with GitHub",
		zap.Any("eval", evalInfo),
	)
	itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, evalInfo.AppId, evalInfo.InstallId, FilenameTheClaPem)
	if err != nil {
		return err
	}

	client := GHImpl.NewClient(&http.Client{Transport: itr})

	err = createRepoStatus(client.Repositories, evalInfo.RepoOwner, evalInfo.RepoName, evalInfo.Sha, "pending", "Paul Botsco, the CLA verifier is running")
	if err != nil {
		return err
	}

	opts := &github.ListOptions{}

	commits, _, err := client.PullRequests.ListCommits(
		context.Background(),
		evalInfo.RepoOwner,
		evalInfo.RepoName,
		int(evalInfo.PRNumber), opts)
	if err != nil {
		return err
	}

	// TODO: Once we have stuff in a DB, we can iterate over the list of commits,
	// find the authors, and check if they have signed the CLA (and the version that is most current)
	// The following loop will change a loop as a result
	var usersNeedingToSignCLA []types.UserSignature
	var usersSigned []types.UserSignature

	for _, v := range commits {
		// It is important to use GetAuthor() instead of v.Commit.GetCommitter() because the committer can be the GH webflow user, whereas the author is
		// the canonical author of the commit
		author := *v.GetAuthor()

		var foundUserSigned *types.UserSignature
		hasAuthorSigned, foundUserSigned, err := postgres.HasAuthorSignedTheCla(*author.Login, claVersion)
		if err != nil {
			return err
		}
		if !hasAuthorSigned {
			userMissingSignature := types.UserSignature{
				User: types.User{
					Login:     author.GetLogin(),
					Email:     author.GetEmail(),
					GivenName: author.GetName(),
				},
				CLAVersion: claVersion,
				// do not populate TimeSigned
			}
			logger.Debug("missing author signature",
				zap.Any("UserSignature", userMissingSignature))
			usersNeedingToSignCLA = append(usersNeedingToSignCLA, userMissingSignature)
		} else {
			usersSigned = append(usersSigned, *foundUserSigned)
		}
	}

	if len(usersNeedingToSignCLA) > 0 {
		err := createRepoLabel(logger, client.Issues, evalInfo.RepoOwner, evalInfo.RepoName, labelNameCLANotSigned, "ff3333", "The CLA needs to be signed", evalInfo.PRNumber)
		if err != nil {
			return err
		}
		// handle case where PR was previously open and all authors had signed cla - meaning the old "all signed" label is applied
		err = _removeLabelFromIssueIfApplied(logger, client.Issues, evalInfo.RepoOwner, evalInfo.RepoName, evalInfo.PRNumber, labelNameCLASigned)
		if err != nil {
			return err
		}

		var users []string
		for _, v := range usersNeedingToSignCLA {
			users = append(users, " @"+v.User.Login)
		}

		// store failed users in the db, so we can reevaluate their PR's after they sign the CLA
		evalInfo.UserSignatures = usersNeedingToSignCLA
		err = postgres.StorePRAuthorsMissingSignature(evalInfo, time.Now())
		if err != nil {
			return err
		}

		// get info needed to show link to sign the cla
		app, err := getApp(logger, evalInfo.AppId)
		if err != nil {
			return err
		}
		// Maybe use app.Name in the remaining hard coded Paul Botsco repo status message above...or not.
		//appName := app.Name
		appExternalUrl := *app.ExternalURL

		message := "Thanks for the contribution. Before we can merge this, we need %s to [sign the Contributor License Agreement](%s)"
		userMsg := strings.Join(users, ",")

		_, err = addCommentToIssueIfNotExists(client.Issues, evalInfo.RepoOwner, evalInfo.RepoName, int(evalInfo.PRNumber), fmt.Sprintf(message, userMsg, appExternalUrl))
		if err != nil {
			return err
		}

		err = createRepoStatus(client.Repositories, evalInfo.RepoOwner, evalInfo.RepoName, evalInfo.Sha, "failure", "One or more contributors need to sign the CLA")
		if err != nil {
			return err
		}
	} else {
		logger.Debug("create label for signed CLA")
		err = createRepoLabel(logger, client.Issues, evalInfo.RepoOwner, evalInfo.RepoName, labelNameCLASigned, "66CC00", "The CLA is signed", evalInfo.PRNumber)
		if err != nil {
			return err
		}
		// handle case where PR was previously open and some authors had NOT signed cla - meaning the old "not signed" label is applied
		err = _removeLabelFromIssueIfApplied(logger, client.Issues, evalInfo.RepoOwner, evalInfo.RepoName, evalInfo.PRNumber, labelNameCLANotSigned)
		if err != nil {
			return err
		}

		err = createRepoStatus(client.Repositories, evalInfo.RepoOwner, evalInfo.RepoName, evalInfo.Sha, "success", "All contributors have signed the CLA")
		if err != nil {
			return err
		}

		// delete prior failed user info from the db for this PR
		if err = postgres.RemovePRsForUsers(usersSigned, evalInfo); err != nil {
			return err
		}
	}

	return nil
}

func getApp(logger *zap.Logger, appId int64) (app *github.App, err error) {
	// Getting a JWT Apps Transport to ask GitHub about stuff that needs a JWT for asking, such as App
	var atr *ghinstallation.AppsTransport
	atr, err = ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, appId, FilenameTheClaPem)
	if err != nil {
		logger.Error("failed to get JWT key",
			zap.Int64("appId", appId),
			zap.Error(err),
		)
		return
	}
	jwtClient := GHImpl.NewClient(&http.Client{Transport: atr})
	app, _, err = jwtClient.Apps.Get(context.Background(),
		// Passing the empty string will get the authenticated GitHub App.
		"")
	if err != nil {
		logger.Error("failed to get app",
			zap.Int64("appId", appId),
			zap.Error(err),
		)
		return
	}
	return
}

func createRepoStatus(repositoryService RepositoriesService, owner, repo, sha, state, description string) error {
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
	pullRequestID int64) error {
	logger.Debug("add or create label", zap.String("name", name))

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
	logger.Debug("maybe create label", zap.String("name", name))

	desiredLabel, res, err := issuesService.GetLabel(context.Background(), owner, repo, name)
	if res.StatusCode == 404 {
		strName := name
		strColor := color
		strDescription := description
		newLabel := &github.Label{Name: &strName, Color: &strColor, Description: &strDescription}
		logger.Debug("label doesn't exist, so create it", zap.Any("newLabel", newLabel))
		desiredLabel, _, err = issuesService.CreateLabel(context.Background(), owner, repo, newLabel)

		return
	}
	if err != nil {
		return
	}
	if desiredLabel != nil {
		logger.Debug("found existing label", zap.Any("desiredLabel", desiredLabel))
		return
	}

	return
}

func addCommentToIssueIfNotExists(issuesService IssuesService, owner, repo string, issueNumber int, message string) (*github.IssueComment, error) {
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

func _addLabelToIssueIfNotExists(logger *zap.Logger, issuesService IssuesService, owner, repo string, issueNumber int64, labelName string) (desiredLabel *github.Label, err error) {
	// check if label is already added to issue
	opts := github.ListOptions{}
	issueLabels, _, err := issuesService.ListLabelsByIssue(context.Background(), owner, repo, int(issueNumber), &opts)
	if err != nil {
		return
	}
	for _, existingLabel := range issueLabels {
		if *existingLabel.Name == labelName {
			logger.Debug("found label on issue, getting out of here", zap.String("labelName", labelName))
			// label already exists on this issue
			desiredLabel = existingLabel
			return
		}
	}

	// didn't find the label on this issue, so add the label to this issue
	// note: this does not remove existing labels (any label not in our "add" array)
	logger.Debug("add label to issue",
		zap.String("owner", owner),
		zap.String("repo", repo),
		zap.Int64("issueNumber", issueNumber),
		zap.String("labelName", labelName),
	)
	_, _, err = issuesService.AddLabelsToIssue(
		context.Background(),
		owner,
		repo,
		int(issueNumber),
		[]string{labelName},
	)
	return
}

func _removeLabelFromIssueIfApplied(logger *zap.Logger, issuesService IssuesService, owner string, repo string, pullRequestID int64, labelToRemove string) (err error) {
	var resp *github.Response
	resp, err = issuesService.RemoveLabelForIssue(context.Background(), owner, repo, int(pullRequestID), labelToRemove)
	if resp.StatusCode == http.StatusNotFound {
		// the label was not applied, so move along as if no error occurred
		err = nil
	} else {
		logger.Debug("removed old label",
			zap.String("owner", owner),
			zap.String("repo", repo),
			zap.Int64("pullRequestID", pullRequestID),
			zap.String("labelToRemove", labelToRemove),
		)
	}
	return
}

func ReviewPriorPRs(logger *zap.Logger, postgres db.IClaDB, user *types.UserSignature) (err error) {
	var evals []types.EvaluationInfo
	if evals, err = postgres.GetPRsForUser(user); err != nil {
		return
	}

	logger.Debug("review evaluations", zap.Any("evals", evals))

	var eval types.EvaluationInfo
	for _, eval = range evals {
		// get PR webhook parameter equivalents
		if err = EvaluatePullRequest(logger, postgres, &eval, user.CLAVersion); err != nil {
			return
		}
	}
	return
}
