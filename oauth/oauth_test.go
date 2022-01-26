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

package oauth

import (
	"context"
	"github.com/google/go-github/v42/github"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/oauth2"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func resetEnvVariable(t *testing.T, variableName, originalValue string) {
	if originalValue == "" {
		assert.NoError(t, os.Unsetenv(variableName))
	} else {
		assert.NoError(t, os.Setenv(variableName, originalValue))
	}
}

func TestCreateOAuth(t *testing.T) {
	origGHClientId := os.Getenv(envReactAppGithubClientId)
	defer func() {
		resetEnvVariable(t, envReactAppGithubClientId, origGHClientId)
	}()
	forcedClientId := "myGHClientId"
	assert.NoError(t, os.Setenv(envReactAppGithubClientId, forcedClientId))

	origGHClientSecret := os.Getenv(envGithubClientSecret)
	defer func() {
		resetEnvVariable(t, envGithubClientSecret, origGHClientSecret)
	}()
	forcedGHClientSecret := "myGHClientSecret"
	assert.NoError(t, os.Setenv(envGithubClientSecret, forcedGHClientSecret))

	oauth := CreateOAuth(forcedClientId, forcedGHClientSecret)

	assert.Equal(t, forcedClientId, oauth.getConf().ClientID)
	assert.Equal(t, forcedGHClientSecret, oauth.getConf().ClientSecret)
}

type OAuthMock struct {
	t                *testing.T
	assertParameters bool
	exchangeToken    *oauth2.Token
	exchangeError    error
	getUserLogger    *zap.Logger
	getUserCode      string
	getUserUser      *github.User
	getUserErr       error
}

var _ OAuthInterface = (*OAuthMock)(nil)

// Exchange takes the code and returns a real token.
//goland:noinspection GoUnusedParameter
func (o *OAuthMock) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return o.exchangeToken, o.exchangeError
}

// Client returns a new http.Client.
//goland:noinspection GoUnusedParameter
func (o *OAuthMock) Client(ctx context.Context, t *oauth2.Token) *http.Client {
	return &http.Client{}
}

func (o *OAuthMock) getConf() *oauth2.Config {
	return nil
}

func (o *OAuthMock) GetOAuthUser(logger *zap.Logger, code string) (user *github.User, err error) {
	if o.assertParameters {
		assert.Equal(o.t, o.getUserLogger, logger)
		assert.Equal(o.t, o.getUserCode, code)
	}
	return o.getUserUser, o.getUserErr
}

func setupMockOAuth(t *testing.T, assertParameters bool) (mockOAuth OAuthMock, logger *zap.Logger) {
	logger = zaptest.NewLogger(t)
	mockOAuth = OAuthMock{
		t:                t,
		assertParameters: assertParameters,
	}
	return
}

// TODO We can likely delete the OAuth mock and this test
func TestGetOAuthUserMock(t *testing.T) {
	oauth, logger := setupMockOAuth(t, true)
	oauth.getUserLogger = logger

	user, err := oauth.GetOAuthUser(logger, "")
	assert.Equal(t, (*github.User)(nil), user)
	assert.Equal(t, nil, err)
}

func TestGetOAuthUserFail(t *testing.T) {
	logger := zaptest.NewLogger(t)
	oauth := CreateOAuth("myClientId", "myClientSecret")

	user, err := oauth.GetOAuthUser(logger, "myOAuthCode")
	assert.Nil(t, user)
	assert.True(t, err != nil)
}
