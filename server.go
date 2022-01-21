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
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/sonatype-nexus-community/the-cla/db"
	ourGithub "github.com/sonatype-nexus-community/the-cla/github"
	"github.com/sonatype-nexus-community/the-cla/oauth"
	"github.com/sonatype-nexus-community/the-cla/types"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	webhook "gopkg.in/go-playground/webhooks.v5/github"
)

const pathClaText string = "/cla-text"
const pathOAuthCallback string = "/oauth-callback"
const pathSignCla string = "/sign-cla"
const pathWebhook string = "/webhook-integration"

const buildLocation string = "build"

const envGhAppId string = "GH_APP_ID"
const envReactAppClaVersion string = "REACT_APP_CLA_VERSION"
const envGhWebhookSecret string = "GH_WEBHOOK_SECRET"

const envReactAppGithubClientId string = "REACT_APP_GITHUB_CLIENT_ID"
const envGithubClientSecret string = "GITHUB_CLIENT_SECRET"

const filenameTheClaPem string = "the-cla.pem"

const msgUnhandledGitHubEventType = "I do not handle this type of event, sorry!"

var postgresDB db.IClaDB

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
	pg, err := sql.Open("postgres", psqlInfo)

	if err != nil {
		e.Logger.Error(err)
	}
	defer func() {
		if err := pg.Close(); err != nil {
			e.Logger.Error(err)
		}
	}()

	err = pg.Ping()
	if err != nil {
		e.Logger.Error(err)
	}

	postgresDB = db.New(pg, e.Logger)

	err = postgresDB.MigrateDB()
	if err != nil {
		e.Logger.Error(err)
	} else {
		e.Logger.Info("DB migration has occurred")
	}

	e.Use(middleware.CORS())

	e.GET(pathClaText, handleRetrieveCLAText)

	e.GET(pathOAuthCallback, handleProcessGitHubOAuth)

	e.POST(pathWebhook, handleProcessWebhook)

	e.PUT(pathSignCla, handleProcessSignCla)

	e.Static("/", buildLocation)

	e.Debug = true

	e.Logger.Fatal(e.Start(addr))
}

func handleProcessWebhook(c echo.Context) (err error) {
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

	appId, err := strconv.Atoi(os.Getenv(envGhAppId))
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	switch payload := payload.(type) {
	case webhook.PullRequestPayload:
		switch payload.Action {
		case "opened", "reopened", "synchronize":
			// Getting a JWT Apps Transport to ask GitHub about stuff that needs a JWT for asking, such as installInfo
			atr, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, int64(appId), filenameTheClaPem)
			if err != nil {
				c.Logger().Error(err)
				return c.String(http.StatusBadRequest, err.Error())
			}

			ghJWTClient := ourGithub.NewJWTClient(&http.Client{Transport: atr}, c.Logger(), int(payload.Installation.ID))

			installInfo, err := ghJWTClient.GetInstallInfo()
			if err != nil {
				c.Logger().Error(err)
				return c.String(http.StatusBadRequest, err.Error())
			}

			// See if we can move this to a longer lived thing, maybe? It's going to recreate the transport and http Client each time we get a payload
			c.Logger().Debugf("Transport setup, using appID: %d and installation ID: %d", appId, payload.Installation.ID)

			itr, err := ghinstallation.NewKeyFromFile(
				http.DefaultTransport,
				int64(appId),
				payload.Installation.ID,
				filenameTheClaPem,
			)
			if err != nil {
				c.Logger().Error(err)
				return c.String(http.StatusBadRequest, err.Error())
			}

			ghClient := ourGithub.NewClient(
				&http.Client{Transport: itr},
				c.Logger(),
				postgresDB,
				ourGithub.GitHubClientOptions{
					AppID:      appId,
					CLAVersion: getCurrentCLAVersion(),
					BotName:    *installInfo.AppSlug,
				},
			)

			res, err := ghClient.HandlePullRequest(payload)

			if err != nil {
				c.Logger().Error(err)
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

func getCurrentCLAVersion() (requiredClaVersion string) {
	// TODO should we read this from env var?
	return os.Getenv(envReactAppClaVersion)
}

func handleProcessSignCla(c echo.Context) (err error) {
	c.Logger().Debug("Attempting to sign the CLA")
	user := new(types.UserSignature)

	if err := c.Bind(user); err != nil {
		return err
	}

	user.TimeSigned = time.Now()

	err = postgresDB.InsertSignature(user)
	if err != nil {
		c.Logger().Error(err)
		return c.String(http.StatusBadRequest, err.Error())
	}

	c.Logger().Debug("CLA signed successfully")
	return c.JSON(http.StatusCreated, user)
}

func handleProcessGitHubOAuth(c echo.Context) (err error) {
	c.Logger().Debug("Attempting to fetch GitHub crud")

	code := c.QueryParam("code")

	state := c.QueryParam("state")
	if state == "" {
		return
	}

	oauthImpl := oauth.CreateOAuth(os.Getenv(envReactAppGithubClientId), os.Getenv(envGithubClientSecret))

	user, err := oauthImpl.GetOAuthUser(c.Logger(), code)

	return c.JSON(http.StatusOK, user)
}

const envClsUrl = "CLA_URL"
const msgMissingClaUrl = "missing " + envClsUrl + " environment variable"

func handleRetrieveCLAText(c echo.Context) (err error) {
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
