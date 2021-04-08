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
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
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
	Login    string `json:"login"`
	Email    string `json:"email"`
	FullName string `json:"fullName"`
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
	defer db.Close()

	err = db.Ping()
	if err != nil {
		e.Logger.Error(err)
	}

	err = migrateDB(db)
	if err != nil {
		e.Logger.Error(err)
	}

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

const GH_WEBHOOK_SECRET string = "GH_WEBHOOK_SECRET"

func processWebhook(c echo.Context) (err error) {
	ghSecret := os.Getenv(GH_WEBHOOK_SECRET)

	hook, _ := webhook.New(webhook.Options.Secret(ghSecret))

	payload, err := hook.Parse(c.Request(), webhook.PullRequestEvent)

	if err != nil {
		if err == webhook.ErrEventNotFound {
			c.Logger().Debug("Unsupported event type encountered", err)

			return c.String(http.StatusBadRequest, "I do not handle this type of event, sorry!")
		}
		return c.String(http.StatusBadRequest, err.Error())
	}

	switch payload := payload.(type) {
	case webhook.PullRequestPayload:
		switch payload.Action {
		case "opened", "reopened", "synchronize":
			res, err := handlePullRequest(payload)

			if err != nil {
				return c.String(http.StatusBadRequest, err.Error())
			}

			return c.String(http.StatusAccepted, res)
		default:
			return c.String(http.StatusAccepted, fmt.Sprintf("No action taken for: %s", payload.Action))
		}
	default:
		c.Logger().Debug("Unhandled payload type encountered")

		return c.String(http.StatusBadRequest, fmt.Sprintf("I do not handle this type of payload, sorry! Type: %T", payload))
	}
}

const THE_CLA_PEM string = "the-cla.pem"
const GH_APP_ID string = "GH_APP_ID"

func handlePullRequest(payload webhook.PullRequestPayload) (response string, err error) {
	appId, err := strconv.Atoi(os.Getenv(GH_APP_ID))
	if err != nil {
		return
	}
	tr := http.DefaultTransport

	itr, err := ghinstallation.NewKeyFromFile(tr, int64(appId), payload.Installation.ID, THE_CLA_PEM)
	if err != nil {
		return
	}

	client := github.NewClient(&http.Client{Transport: itr})

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
	committers := []string{}
	for _, v := range commits {
		committer := *v.GetCommitter()
		committers = append(committers,
			fmt.Sprintf(
				"Author: %s Email: %s Commit SHA: %s",
				committer.GetLogin(),
				committer.GetEmail(),
				v.GetSHA(),
			))
	}

	// This is basically junk just for testing, can be removed
	response = strings.Join(committers, ",")

	// TODO: once we know if someone hasn't signed, and the sha1 for the commit in question, we can
	// mark the commit as having failed a check, and apply a label to the PR of not signed
	// Alternatively if everything is ok, we can remove the label, and say yep! All signed up!

	// TODO: extract to another method for creating a label, so we can see if it exists before we create it
	strName := ":monocle_face: cla not signed"
	strColor := "fa3a3a"
	strDescription := "The CLA is not signed"

	lbl := &github.Label{Name: &strName, Color: &strColor, Description: &strDescription}

	_, _, err = client.Issues.CreateLabel(
		context.Background(),
		payload.Repository.Owner.Login,
		payload.Repository.Name,
		lbl,
	)

	// TODO: Garbage error check
	if err != nil {
		fmt.Print(err)
	}

	client.Issues.AddLabelsToIssue(
		context.Background(),
		payload.Repository.Owner.Login,
		payload.Repository.Name,
		int(payload.Number),
		[]string{*lbl.Name},
	)

	return
}

func processSignCla(c echo.Context) (err error) {
	c.Logger().Debug("Attempting to sign the CLA")
	user := new(UserSignature)

	if err := c.Bind(user); err != nil {
		return err
	}

	user.TimeSigned = time.Now()

	sqlStatement := `INSERT INTO signatures
		(LoginName, Email, GivenName, SignedAt, ClaVersion)
		VALUES ($1, $2, $3, $4, $5)`

	_, err = db.Exec(sqlStatement, user.User.Login, user.User.Email, user.User.FullName, user.TimeSigned, user.CLAVersion)
	if err != nil {
		c.Logger().Error(err)

		return c.String(http.StatusBadRequest, err.Error())
	}

	c.Logger().Debug("CLA signed successfully")
	return c.JSON(http.StatusCreated, user)
}

func processGitHubOAuth(c echo.Context) (err error) {
	c.Logger().Debug("Attempting to fetch GitHub crud")

	code := c.QueryParam("code")

	state := c.QueryParam("state")

	clientID := os.Getenv("REACT_APP_GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")

	oauthConf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"user:email"},
		Endpoint:     githuboauth.Endpoint,
	}

	if state == "" {
		return
	}

	token, err := oauthConf.Exchange(context.Background(), code)
	if err != nil {
		c.Logger().Error(err)
		return
	}

	oauthClient := oauthConf.Client(context.Background(), token)

	client := github.NewClient(oauthClient)

	user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		c.Logger().Error(err)
		return
	}

	return c.JSON(http.StatusOK, user)
}

const envClsUrl string = "CLA_URL"
const msgMissingClaUrl string = "missing " + envClsUrl + " environment variable"

func retrieveCLAText(c echo.Context) (err error) {
	c.Logger().Debug("Attempting to fetch CLA text")
	claURL := os.Getenv(envClsUrl)

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

	return c.String(http.StatusOK, string(content))
}
