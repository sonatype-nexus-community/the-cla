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
// +build go1.16

package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/go-github/v33/github"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/oauth2"
	githuboauth "golang.org/x/oauth2/github"
	webhook "gopkg.in/go-playground/webhooks.v5/github"
)

type User struct {
	Login     string `json:"login"`
	Email     string `json:"email"`
	GivenName string `json:"name"`
}

type UserSignature struct {
	User       User   `json:"user"`
	CLAVersion string `json:"claVersion"`
	TimeSigned time.Time
}

const pathClaText string = "/cla-text"
const pathOAuthCallback string = "/oauth-callback"
const pathSignCla string = "/sign-cla"
const pathWebhook string = "/webhook-integration"

const buildLocation string = "build"

var db *sql.DB

var claCache = make(map[string]string)

func main() {
	e := echo.New()
	addr := ":4200"

	err := godotenv.Load(".env")
	if err != nil {
		e.Logger.Error(err)
	}

	host := os.Getenv("PG_HOST")
	port, _ := strconv.Atoi(os.Getenv("PG_PORT"))
	user := os.Getenv("PG_USERNAME")
	password := os.Getenv("PG_PASSWORD")
	dbname := os.Getenv("PG_DB_NAME")
	sslMode := os.Getenv("SSL_MODE")

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslMode)
	db, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		e.Logger.Error(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			e.Logger.Error(err)
		}
	}()

	err = db.Ping()
	if err != nil {
		e.Logger.Error(err)
	}

	err = migrateDB(db)
	if err != nil {
		e.Logger.Error(err)
	}

	oauthImpl = createOAuth()

	e.Use(middleware.CORS())

	e.GET(pathClaText, retrieveCLAText)

	e.GET(pathOAuthCallback, processGitHubOAuth)

	e.POST(pathWebhook, processWebhook)

	e.PUT(pathSignCla, processSignCla)

	e.Static("/", buildLocation)

	e.Debug = true

	e.Logger.Fatal(e.Start(addr))
}

func migrateDB(db *sql.DB) (err error) {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://db/migrations",
		"postgres", driver)

	if err != nil {
		return
	}

	if err = m.Up(); err != nil {
		return
	}

	return
}

const envGhWebhookSecret string = "GH_WEBHOOK_SECRET"
const msgUnhandledGitHubEventType = "I do not handle this type of event, sorry!"

func processWebhook(c echo.Context) (err error) {
	ghSecret := os.Getenv(envGhWebhookSecret)

	hook, _ := webhook.New(webhook.Options.Secret(ghSecret))

	payload, err := hook.Parse(c.Request(), webhook.PullRequestEvent)

	if err != nil {
		if err == webhook.ErrEventNotFound {
			c.Logger().Debug("Unsupported event type encountered", err)

			return c.String(http.StatusBadRequest, msgUnhandledGitHubEventType)
		}
		return c.String(http.StatusBadRequest, err.Error())
	}

	switch payload := payload.(type) {
	case webhook.PullRequestPayload:
		switch payload.Action {
		case "opened", "reopened", "synchronize":
			res, err := handlePullRequest(c.Logger(), payload)

			if err != nil {
				return c.String(http.StatusBadRequest, err.Error())
			}

			return c.String(http.StatusAccepted, res)
		default:
			return c.String(http.StatusAccepted, fmt.Sprintf("No action taken for: %s", payload.Action))
		}
	default:
		// theoretically can't get here due to hook.Parse() call above (events param), but better safe than sorry
		c.Logger().Debug("Unhandled payload type encountered")

		return c.String(http.StatusBadRequest, fmt.Sprintf("I do not handle this type of payload, sorry! Type: %T", payload))
	}
}

const filenameTheClaPem string = "the-cla.pem"
const envGhAppId string = "GH_APP_ID"

func handlePullRequest(logger echo.Logger, payload webhook.PullRequestPayload) (response string, err error) {
	appId, err := strconv.Atoi(os.Getenv(envGhAppId))
	if err != nil {
		return
	}
	tr := http.DefaultTransport

	itr, err := ghinstallation.NewKeyFromFile(tr, int64(appId), payload.Installation.ID, filenameTheClaPem)
	if err != nil {
		return
	}

	client := githubImpl.NewClient(&http.Client{Transport: itr})

	opts := &github.ListOptions{}

	commits, _, err := client.PullRequests.ListCommits(
		context.Background(),
		payload.Repository.Owner.Login,
		payload.Repository.Name,
		int(payload.Number), opts)

	if err != nil {
		return
	}

	// TODO: Once we have stuff in a DB, we can iterate over the list of commits,
	// find the authors, and check if they have signed the CLA (and the version that is most current)
	// The following loop will change a loop as a result
	var committers []string
	var usersNeedingToSignCLA []UserSignature
	for _, v := range commits {
		committer := *v.GetCommitter()
		var hasCommitterSigned bool
		hasCommitterSigned, err = hasCommitterSignedTheCla(logger, committer)
		if err != nil {
			return
		}
		if !hasCommitterSigned {
			committers = append(committers,
				fmt.Sprintf(
					"Author: %s Email: %s Commit SHA: %s",
					committer.GetLogin(),
					committer.GetEmail(),
					v.GetSHA(),
				))
			usersNeedingToSignCLA = append(usersNeedingToSignCLA,
				UserSignature{
					User: User{
						Login:     committer.GetLogin(),
						Email:     committer.GetEmail(),
						GivenName: committer.GetName(),
					},
					CLAVersion: getCurrentCLAVersion(),
					//TimeSigned: time.Time{},
				})
		}
	}

	// This is basically junk just for testing, can be removed
	response = strings.Join(committers, ",")

	// TODO: once we know if someone hasn't signed, and the sha1 for the commit in question, we can
	// mark the commit as having failed a check, and apply a label to the PR of not signed
	// Alternatively if everything is ok, we can remove the label, and say yep! All signed up!

	lblCLANotSigned, err := createRepoLabelIfNotExists(client.Issues, payload.Repository.Owner.Login, payload.Repository.Name)
	if err != nil {
		return
	}

	_, err = addLabelToIssueIfNotExists(client.Issues, payload.Repository.Owner.Login, payload.Repository.Name, int(payload.Number), lblCLANotSigned.GetName())
	if err != nil {
		return
	}

	return
}

const labelNameCLANotSigned string = ":monocle_face: cla not signed"

func createRepoLabelIfNotExists(issuesService IssuesService, owner, repo string) (desiredLabel *github.Label, err error) {
	desiredLabel, _, err = issuesService.GetLabel(context.Background(), owner, repo, labelNameCLANotSigned)
	if err != nil {
		return
	}
	if desiredLabel != nil {
		return
	}

	// looks like the label doesn't exist, so create it
	strName := labelNameCLANotSigned
	strColor := "fa3a3a"
	strDescription := "The CLA is not signed"
	newLabel := &github.Label{Name: &strName, Color: &strColor, Description: &strDescription}
	desiredLabel, _, err = issuesService.CreateLabel(context.Background(), owner, repo, newLabel)
	return
}

func addLabelToIssueIfNotExists(issuesService IssuesService, owner, repo string, issueNumber int, labelName string) (desiredLabel *github.Label, err error) {
	// check if label is already added to issue
	opts := github.ListOptions{}
	issueLabels, _, err := issuesService.ListLabelsByIssue(context.Background(), owner, repo, issueNumber, &opts)
	if err != nil {
		return
	}
	for _, existingLabel := range issueLabels {
		if *existingLabel.Name == labelNameCLANotSigned {
			// label already exists on this issue
			desiredLabel = existingLabel
			return
		}
	}

	// didn't find the label on this issue, so add the label to this issue
	// @TODO Verify this does not remove existing labels (any label not in our "add" array)
	_, _, err = issuesService.AddLabelsToIssue(
		context.Background(),
		owner,
		repo,
		issueNumber,
		[]string{labelName},
	)
	return
}

const envReactAppClaVersion = "REACT_APP_CLA_VERSION"

func getCurrentCLAVersion() (requiredClaVersion string) {
	// TODO should we read this from env var?
	return os.Getenv(envReactAppClaVersion)
}

const sqlSelectUserSignature = `SELECT 
		LoginName, Email, GivenName, SignedAt, ClaVersion 
		FROM signatures		
		WHERE LoginName = $1
		AND ClaVersion = $2`

func hasCommitterSignedTheCla(logger echo.Logger, committer github.User) (isSigned bool, err error) {
	rows, err := db.Query(sqlSelectUserSignature, committer.GetLogin(), getCurrentCLAVersion())
	if err != nil {
		return isSigned, err
	}

	var foundUserSignature UserSignature
	for rows.Next() {
		isSigned = true
		foundUserSignature = UserSignature{}
		err = rows.Scan(
			&foundUserSignature.User.Login,
			&foundUserSignature.User.Email,
			&foundUserSignature.User.GivenName,
			&foundUserSignature.TimeSigned,
			&foundUserSignature.CLAVersion,
		)
		if err != nil {
			return isSigned, err
		}
		logger.Debugf("Found user signature for committer: %s, TimeSigned: %s, CLAVersion: %s",
			foundUserSignature.User.Login, foundUserSignature.TimeSigned, foundUserSignature.CLAVersion)
	}

	return isSigned, err
}

const sqlInsertSignature = `INSERT INTO signatures
		(LoginName, Email, GivenName, SignedAt, ClaVersion)
		VALUES ($1, $2, $3, $4, $5)`

func processSignCla(c echo.Context) (err error) {
	c.Logger().Debug("Attempting to sign the CLA")
	user := new(UserSignature)

	if err := c.Bind(user); err != nil {
		return err
	}

	user.TimeSigned = time.Now()

	_, err = db.Exec(sqlInsertSignature, user.User.Login, user.User.Email, user.User.GivenName, user.TimeSigned, user.CLAVersion)
	if err != nil {
		c.Logger().Error(err)

		return c.String(http.StatusBadRequest, err.Error())
	}

	c.Logger().Debug("CLA signed successfully")
	return c.JSON(http.StatusCreated, user)
}

type OAuthInterface interface {
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	Client(ctx context.Context, t *oauth2.Token) *http.Client
	// for testing only
	getConf() *oauth2.Config
}
type OAuthImpl struct {
	oauthConf *oauth2.Config
}

//goland:noinspection GoUnusedParameter
func (oa *OAuthImpl) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return oa.oauthConf.Exchange(ctx, code)
}
func (oa *OAuthImpl) Client(ctx context.Context, t *oauth2.Token) *http.Client {
	return oa.oauthConf.Client(ctx, t)
}
func (oa *OAuthImpl) getConf() *oauth2.Config {
	return oa.oauthConf
}

const envReactAppGithubClientId = "REACT_APP_GITHUB_CLIENT_ID"
const envGithubClientSecret = "GITHUB_CLIENT_SECRET"

func createOAuth() OAuthInterface {
	oauthConf := &oauth2.Config{
		ClientID:     os.Getenv(envReactAppGithubClientId),
		ClientSecret: os.Getenv(envGithubClientSecret),
		Scopes:       []string{"user:email"},
		Endpoint:     githuboauth.Endpoint,
	}
	oAuthImpl := OAuthImpl{
		oauthConf: oauthConf,
	}
	return &oAuthImpl
}

var oauthImpl OAuthInterface

// RepositoriesService handles communication with the repository related methods
// of the GitHub API.
// https://godoc.org/github.com/google/go-github/github#RepositoriesService
type RepositoriesService interface {
	Get(context.Context, string, string) (*github.Repository, *github.Response, error)
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

func processGitHubOAuth(c echo.Context) (err error) {
	c.Logger().Debug("Attempting to fetch GitHub crud")

	code := c.QueryParam("code")

	state := c.QueryParam("state")
	if state == "" {
		return
	}

	token, err := oauthImpl.Exchange(context.Background(), code)
	if err != nil {
		c.Logger().Error(err)
		return
	}

	oauthClient := oauthImpl.Client(context.Background(), token)

	client := githubImpl.NewClient(oauthClient)

	user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		c.Logger().Error(err)
		return
	}

	return c.JSON(http.StatusOK, user)
}

const envClsUrl = "CLA_URL"
const msgMissingClaUrl = "missing " + envClsUrl + " environment variable"

func retrieveCLAText(c echo.Context) (err error) {
	c.Logger().Debug("Attempting to fetch CLA text")
	claURL := os.Getenv(envClsUrl)

	if claCache[claURL] != "" {
		c.Logger().Debug("CLA text was cached, returning")

		return c.String(http.StatusOK, claCache[claURL])
	}

	c.Logger().Debug("CLA text not in cache, moving forward to fetch")
	if claURL == "" {
		return fmt.Errorf(msgMissingClaUrl)
	}

	client := http.Client{}

	resp, err := client.Get(claURL)
	if err != nil {
		c.Logger().Error(err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("unexpected cla text response code: %d", resp.StatusCode)
		c.Logger().Error(err)
		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.Logger().Error(err)
		return
	}

	claCache[claURL] = string(content)

	return c.String(http.StatusOK, claCache[claURL])
}
