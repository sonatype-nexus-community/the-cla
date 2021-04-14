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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/v33/github"
	"golang.org/x/oauth2"
	webhook "gopkg.in/go-playground/webhooks.v5/github"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

const mockClaText = `mock Cla text.`

func setupMockContextCLA() echo.Context {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, pathClaText, strings.NewReader(mockClaText))
	req.Header.Set(echo.HeaderContentType, echo.MIMETextPlainCharsetUTF8)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c
}

func TestRetrieveCLAText_MissingClaURL(t *testing.T) {
	assert.EqualError(t, retrieveCLAText(setupMockContextCLA()), msgMissingClaUrl)
}

func TestRetrieveCLAText_BadResponseCode(t *testing.T) {
	origClaUrl := os.Getenv(envClsUrl)
	defer func() {
		if origClaUrl == "" {
			assert.NoError(t, os.Unsetenv(envClsUrl))
		} else {
			assert.NoError(t, os.Setenv(envClsUrl, origClaUrl))
		}
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, pathClaText, r.URL.EscapedPath())

		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	assert.NoError(t, os.Setenv(envClsUrl, ts.URL+pathClaText))
	assert.EqualError(t, retrieveCLAText(setupMockContextCLA()), "unexpected cla text response code: 403")
}

func TestRetrieveCLAText(t *testing.T) {
	callCount := 0

	origClaUrl := os.Getenv(envClsUrl)
	defer func() {
		if origClaUrl == "" {
			assert.NoError(t, os.Unsetenv(envClsUrl))
		} else {
			assert.NoError(t, os.Setenv(envClsUrl, origClaUrl))
		}
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, pathClaText, r.URL.EscapedPath())
		callCount += 1

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockClaText))
	}))

	defer ts.Close()

	assert.NoError(t, os.Setenv(envClsUrl, ts.URL+pathClaText))
	assert.NoError(t, retrieveCLAText(setupMockContextCLA()))
	assert.Equal(t, callCount, 1)

	// Ensure that subsequent calls use the cache

	assert.NoError(t, retrieveCLAText(setupMockContextCLA()))
	assert.Equal(t, callCount, 1)
}

func TestRetrieveCLATextWithBadURL(t *testing.T) {
	callCount := 0

	origClaUrl := os.Getenv(envClsUrl)
	defer func() {
		if origClaUrl == "" {
			assert.NoError(t, os.Unsetenv(envClsUrl))
		} else {
			assert.NoError(t, os.Setenv(envClsUrl, origClaUrl))
		}
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, pathClaText, r.URL.EscapedPath())
		callCount += 1

		// nobody home, be we should not even be knocking on this door - call should not occur
		w.WriteHeader(http.StatusNotFound)
	}))

	defer ts.Close()

	assert.NoError(t, os.Setenv(envClsUrl, "badURLProtocol"+ts.URL+pathClaText))
	assert.Error(t, retrieveCLAText(setupMockContextCLA()), "unsupported protocol scheme \"badurlprotocolhttp\"")
	assert.Equal(t, callCount, 0)
}

func setupMockContextOAuth(queryParams map[string]string) (c echo.Context, rec *httptest.ResponseRecorder) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, pathOAuthCallback, strings.NewReader("mock OAuth stuff"))

	q := req.URL.Query()
	for k, v := range queryParams {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	return
}

func TestProcessGitHubOAuthMissingQueryParamState(t *testing.T) {
	c, rec := setupMockContextOAuth(map[string]string{})
	assert.NoError(t, processGitHubOAuth(c))
	assert.Equal(t, 0, c.Response().Status)
	assert.Equal(t, "", rec.Body.String())
}

type OAuthMock struct {
	exchangeForceError error
}

// Exchange takes the code and returns a real token.
func (o *OAuthMock) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	if o.exchangeForceError != nil {
		return nil, o.exchangeForceError
	}
	return &oauth2.Token{
		AccessToken: "testAccessToken",
		Expiry:      time.Now().Add(1 * time.Hour),
	}, nil
}

// Client returns a new http.Client.
func (o *OAuthMock) Client(ctx context.Context, t *oauth2.Token) *http.Client {
	return &http.Client{}
}

// RepositoriesMock mocks RepositoriesService
type RepositoriesMock struct {
	RepositoriesService
}

// Get returns a repository.
func (r *RepositoriesMock) Get(context.Context, string, string) (*github.Repository, *github.Response, error) {
	return &github.Repository{
		ID:              github.Int64(185409993),
		Name:            github.String("wayne"),
		Description:     github.String("some description"),
		Language:        github.String("JavaScript"),
		StargazersCount: github.Int(3141),
		HTMLURL:         github.String("https://www.foo.com"),
		FullName:        github.String("john/wayne"),
	}, nil, nil
}

// UsersMock mocks UsersService
type UsersMock struct {
	usersForceError error
	UsersService
}

// Get returns a user.
func (u *UsersMock) Get(context.Context, string) (*github.User, *github.Response, error) {
	if u.usersForceError != nil {
		return nil, nil, u.usersForceError
	}
	return &github.User{
		Login: github.String("john"),
	}, nil, nil
}

// GitHubMock implements GitHubInterface.
type GitHubMock struct {
	usersMock UsersMock
}

// NewClient something
func (g *GitHubMock) NewClient(httpClient *http.Client) GitHubClient {
	return GitHubClient{
		Repositories: &RepositoriesMock{},
		Users:        &UsersMock{usersForceError: g.usersMock.usersForceError},
	}
}

func TestProcessGitHubOAuthMissingQueryParamCode(t *testing.T) {
	origOAuth := oauthImpl
	defer func() {
		oauthImpl = origOAuth
	}()
	oauthImpl = &OAuthMock{}

	origGithubImpl := githubImpl
	defer func() {
		githubImpl = origGithubImpl
	}()
	githubImpl = &GitHubMock{}

	c, rec := setupMockContextOAuth(map[string]string{
		"state": "testState",
	})
	assert.NoError(t, processGitHubOAuth(c))
	assert.Equal(t, http.StatusOK, c.Response().Status)
	assert.Equal(t, `{"login":"john"}
`, rec.Body.String())
}

func TestProcessGitHubOAuth_ExchangeError(t *testing.T) {
	origOAuth := oauthImpl
	defer func() {
		oauthImpl = origOAuth
	}()
	forcedError := fmt.Errorf("forced Exchange error")
	oauthImpl = &OAuthMock{
		exchangeForceError: forcedError,
	}

	origGithubImpl := githubImpl
	defer func() {
		githubImpl = origGithubImpl
	}()
	githubImpl = &GitHubMock{}

	c, rec := setupMockContextOAuth(map[string]string{
		"state": "testState",
	})
	assert.Error(t, forcedError, processGitHubOAuth(c))
	assert.Equal(t, 0, c.Response().Status)
	assert.Equal(t, "", rec.Body.String())
}

func TestProcessGitHubOAuth_UsersServiceError(t *testing.T) {
	origOAuth := oauthImpl
	defer func() {
		oauthImpl = origOAuth
	}()
	oauthImpl = &OAuthMock{}

	origGithubImpl := githubImpl
	defer func() {
		githubImpl = origGithubImpl
	}()
	forcedError := fmt.Errorf("forced Users error")
	githubImpl = &GitHubMock{
		UsersMock{
			usersForceError: forcedError,
		},
	}

	c, rec := setupMockContextOAuth(map[string]string{
		"state": "testState",
	})
	assert.Error(t, forcedError, processGitHubOAuth(c))
	assert.Equal(t, 0, c.Response().Status)
	assert.Equal(t, "", rec.Body.String())
}

func setupMockContextWebhook(t *testing.T, headers map[string]string, prEvent github.PullRequestEvent) (c echo.Context, rec *httptest.ResponseRecorder) {
	// Setup
	e := echo.New()

	reqBody, err := json.Marshal(prEvent)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, pathWebhook, strings.NewReader(string(reqBody)))

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	return
}

func TestProcessWebhookMissingHeaderGitHubEvent(t *testing.T) {
	c, rec := setupMockContextWebhook(t, map[string]string{}, github.PullRequestEvent{})

	assert.NoError(t, processWebhook(c))
	assert.Equal(t, http.StatusBadRequest, c.Response().Status)
	assert.Equal(t, "missing X-GitHub-Event Header", rec.Body.String())
}

func TestProcessWebhookUnhandledGitHubEvent(t *testing.T) {
	c, rec := setupMockContextWebhook(t,
		map[string]string{
			"X-GitHub-Event": "unknownGitHubEventHeaderValue",
		}, github.PullRequestEvent{})

	assert.NoError(t, processWebhook(c))
	assert.Equal(t, http.StatusBadRequest, c.Response().Status)
	assert.Equal(t, msgUnhandledGitHubEventType, rec.Body.String())
}

func TestProcessWebhookGitHubEventPullRequestPayloadActionIgnored(t *testing.T) {
	actionText := "someIgnoredAction"
	c, rec := setupMockContextWebhook(t,
		map[string]string{
			"X-GitHub-Event": string(webhook.PullRequestEvent),
		}, github.PullRequestEvent{Action: &actionText})

	assert.NoError(t, processWebhook(c))
	assert.Equal(t, http.StatusAccepted, c.Response().Status)
	assert.Equal(t, "No action taken for: someIgnoredAction", rec.Body.String())
}

func xxxTestProcessWebhookGitHubEventPullRequestPayloadActionHandled(t *testing.T) {
	verifyActionHandled(t, "opened")
	verifyActionHandled(t, "reopened")
	verifyActionHandled(t, "synchronize")
}

func verifyActionHandled(t *testing.T, actionText string) {
	c, rec := setupMockContextWebhook(t,
		map[string]string{
			"X-GitHub-Event": string(webhook.PullRequestEvent),
		}, github.PullRequestEvent{Action: &actionText})

	assert.NoError(t, processWebhook(c))
	assert.Equal(t, http.StatusAccepted, c.Response().Status)
	assert.Equal(t, "No action taken for: someIgnoredAction", rec.Body.String())
}
