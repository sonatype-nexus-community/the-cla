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
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/google/go-github/v33/github"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/oauth2"
	githuboauth "golang.org/x/oauth2/github"
)

type User struct {
	Login string
	Email string
}

const pathClaText string = "/cla-text"
const pathOAuthCallback string = "/oath-callback"
const buildLocation string = "build"

func main() {
	e := echo.New()
	addr := ":4200"

	err := godotenv.Load(".env")
	if err != nil {
		e.Logger.Error(err)
	}

	e.Use(middleware.CORS())

	e.GET(pathClaText, retrieveCLAText)

	e.GET(pathOAuthCallback, processGitHubOAuth)

	e.Static("/", buildLocation)

	e.Debug = true

	e.Logger.Fatal(e.Start(addr))
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
