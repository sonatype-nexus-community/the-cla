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

package oauth

import (
	"context"
	"net/http"

	"github.com/google/go-github/v42/github"
	"github.com/labstack/echo/v4"
	ourGithub "github.com/sonatype-nexus-community/the-cla/github"
	"golang.org/x/oauth2"
	githuboauth "golang.org/x/oauth2/github"
)

var githubImpl ourGithub.GitHubInterface = &ourGithub.GitHubCreator{}

type OAuthInterface interface {
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	Client(ctx context.Context, t *oauth2.Token) *http.Client
	GetOAuthUser(logger echo.Logger, code string) (user *github.User, err error)
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

func (oa *OAuthImpl) GetOAuthUser(logger echo.Logger, code string) (user *github.User, err error) {
	token, err := oa.Exchange(context.Background(), code)
	if err != nil {
		logger.Error(err)
		return
	}

	oauthClient := oa.Client(context.Background(), token)

	client := githubImpl.NewClient(oauthClient)

	user, _, err = client.Users.Get(context.Background(), "")
	if err != nil {
		logger.Error(err)
		return
	}

	return
}

const envReactAppGithubClientId = "REACT_APP_GITHUB_CLIENT_ID"
const envGithubClientSecret = "GITHUB_CLIENT_SECRET"

func CreateOAuth(clientID, clientSecret string) OAuthInterface {
	oauthConf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"user:email"},
		Endpoint:     githuboauth.Endpoint,
	}
	oAuthImpl := OAuthImpl{
		oauthConf: oauthConf,
	}
	return &oAuthImpl
}

var oauthImpl OAuthInterface
