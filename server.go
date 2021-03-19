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

func main() {
	e := echo.New()
	addr := ":4200"

	err := godotenv.Load(".env")
	if err != nil {
		e.Logger.Fatal(err)
	}

	e.Use(middleware.CORS())

	e.GET("/cla-text", retrieveCLAText)

	e.GET("/oauth-callback", processGitHubOAuth)

	e.Static("/", "build")

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

	token, err := oauthConf.Exchange(oauth2.NoContext, code)
	if err != nil {
		c.Logger().Error(err)
		return
	}

	oauthClient := oauthConf.Client(oauth2.NoContext, token)

	client := github.NewClient(oauthClient)

	user, _, err := client.Users.Get(oauth2.NoContext, "")
	if err != nil {
		c.Logger().Error(err)
		return
	}

	return c.JSON(http.StatusOK, user)
}

func retrieveCLAText(c echo.Context) (err error) {
	claURL := os.Getenv("CLA_URL")

	c.Logger().Debug(claURL)

	client := http.Client{}

	resp, err := client.Get(claURL)
	if err != nil {
		c.Logger().Error(err)
		return
	}

	c.Logger().Debug(resp.StatusCode)

	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.Logger().Error(err)
		return
	}

	return c.String(http.StatusOK, string(content))
}
