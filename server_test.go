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
//goland:noinspection GoUnusedParameter
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
//goland:noinspection GoUnusedParameter
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

type PullRequestsMock struct {
}

//goland:noinspection GoUnusedParameter
func (p *PullRequestsMock) ListCommits(ctx context.Context, owner string, repo string, number int, opts *github.ListOptions) ([]*github.RepositoryCommit, *github.Response, error) {
	return nil, nil, nil
}

type IssuesMock struct {
}

//goland:noinspection GoUnusedParameter
func (i *IssuesMock) CreateLabel(ctx context.Context, owner string, repo string, label *github.Label) (*github.Label, *github.Response, error) {
	return nil, nil, nil
}

//goland:noinspection GoUnusedParameter
func (i *IssuesMock) AddLabelsToIssue(ctx context.Context, owner string, repo string, number int, labels []string) ([]*github.Label, *github.Response, error) {
	return nil, nil, nil
}

// NewClient something
//goland:noinspection GoUnusedParameter
func (g *GitHubMock) NewClient(httpClient *http.Client) GitHubClient {
	return GitHubClient{
		Repositories: &RepositoriesMock{},
		Users:        &UsersMock{usersForceError: g.usersMock.usersForceError},
		PullRequests: &PullRequestsMock{},
		Issues:       &IssuesMock{},
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

func TestHandlePullRequestBadGH_APP_ID(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(envGhAppId)
	defer func() {
		if origGHAppIDEnvVar == "" {
			assert.NoError(t, os.Unsetenv(envGhAppId))
		} else {
			assert.NoError(t, os.Setenv(envGhAppId, origGHAppIDEnvVar))
		}
	}()
	assert.NoError(t, os.Setenv(envGhAppId, "nonNumericGHAppID"))

	prEvent := webhook.PullRequestPayload{}
	res, err := handlePullRequest(prEvent)
	assert.EqualError(t, err, "strconv.Atoi: parsing \"nonNumericGHAppID\": invalid syntax")
	assert.Equal(t, "", res)
}

func TestHandlePullRequestMissingPemFile(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(envGhAppId)
	defer func() {
		if origGHAppIDEnvVar == "" {
			assert.NoError(t, os.Unsetenv(envGhAppId))
		} else {
			assert.NoError(t, os.Setenv(envGhAppId, origGHAppIDEnvVar))
		}
	}()
	assert.NoError(t, os.Setenv(envGhAppId, "-1"))

	prEvent := webhook.PullRequestPayload{}
	res, err := handlePullRequest(prEvent)
	assert.EqualError(t, err, "could not read private key: open the-cla.pem: no such file or directory")
	assert.Equal(t, "", res)
}

func TestHandlePullRequest(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(envGhAppId)
	defer func() {
		if origGHAppIDEnvVar == "" {
			assert.NoError(t, os.Unsetenv(envGhAppId))
		} else {
			assert.NoError(t, os.Setenv(envGhAppId, origGHAppIDEnvVar))
		}
	}()
	assert.NoError(t, os.Setenv(envGhAppId, "-1"))

	// move pem file if it exists
	pemBackupFile := filenameTheClaPem + "_orig"
	errRename := os.Rename(filenameTheClaPem, pemBackupFile)
	defer func() {
		assert.NoError(t, os.Remove(filenameTheClaPem))
		if errRename == nil {
			assert.NoError(t, os.Rename(pemBackupFile, filenameTheClaPem), "error renaming pem file in test")
		}
	}()
	setupTestPemFile(t)

	origGithubImpl := githubImpl
	defer func() {
		githubImpl = origGithubImpl
	}()
	githubImpl = &GitHubMock{}

	prEvent := webhook.PullRequestPayload{}
	res, err := handlePullRequest(prEvent)
	// TODO add assertions here
	assert.NoError(t, err)
	assert.Equal(t, "", res)
}

// generated via: openssl genpkey -algorithm RSA  -outform PEM -out private_key.pem -pkeyopt rsa_keygen_bits:2048
const testPrivatePem = `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDtQtWKdnW9OKJk
XuSx45oixrJqWqpaly23iXvAAcTqg+pFD7Yw1bL9viAYoc7ATcd6Uonz7/d6RugO
JuozsC4X1xYotEWYlB7tKrp+InQ2H0fRC6afGiCdDUgLINfmqShPWgGft4cA7mwH
JSHB6XAGwVsZsxqYIi4wXVPYYJaI3OX5nA/BiRvZMrsaF2PT8dt/5rptMIXxXlwK
tuQVvICxh5CXn5/FaeQcnkXoDESoZcG9nhqSmRdeUJxoiGZ7epVljj7Ef5XKJYoz
uv8vJVTVXwxb7MbcjQ6Zna4iJj4FscwkQyaoFQOzBf+1H5ypZ8CFn/E236tLpwh0
7Xspu5CrAgMBAAECggEBAOd51CKBjj8s+OpZ1l9jgea52il/CULWyciNvolGcJqo
VrBIMuUUKMv8aQ3/F1pwx9QkoOi4TsciVJYyCz6gfWfO9ZSCxH+my0Fx9X7IGH8R
J5zg9A+3iugOpCIPSfSFRomcc4cio/kZo5WY+YVZPW2pyTqajbCtcEjJVNr+6P7e
PAWKI6RXbwGa4Fp8dLHMRq+/i2zuznEzdrTJPBSoW5HUMDvPixhjd+WeYT9pNfZP
P8V2HhSt1qvuVM/epZ8llnmyPaw7ojwAOurG19fDGUvEfjAORYJopOvxeJ1mCY++
HVxcumbx4N2D8IQ/dwbtarMBLpw89GQztxCxokJ7a5ECgYEA/QFTsgQKFQbdlv1z
ooBq3EZPfzebx4mkyCcLmQAliSArJezRewCyelP2A102p5125SMEA1vcsSkZOes8
h4z4HaptHZob1OxG2EBNdOzY41TaG1nzbOAJEkF71ksT30dpaLRCECUfcEWc0waB
cwia1v1xUvfcvwhPJIdzye5V7hkCgYEA8BHMYRfvIMtRgHNPoFNoRxr6BU/gjfV/
FRJLNdMSk3KYve459XGPFvLSAh0eucOVjmkZY8y0BJJdeFVdTjPa2nvk70i9yhGk
MhjVHs1Y7VIRYB6SSoA7hPK3zMELTbMudZS1/Dxe8fCc1/oDhamLAcT1474hXIR2
AYe8T97qBWMCgYA77yWJhSVyR7cUfqP2+d7WoZ1RcLXpdfTgKUe5DezWaBVwnYIe
VlLxYZRkxZ8d49J3g2z+8rL8ENVWACDNp5pbRLUmjwxKy1IZBlqS+UyDxeUJF6zv
vL7JYVPZtt1VRlB1KkaAFps0+HinEOJ3grFTfqRq2Cal5m0BJUlLq7cVeQKBgHLB
Hz/+L9kuNxw+gn5xwDPVClRFtWJGSmPpJbhp18RRj/+iA2R2zt46XfaSsuA7RJ8Z
UACrlhVlXXaq33oFQYUUmf9jdw1DV4h25FDf+bUfeJzIoEcqesj3OLKQSHXww7GC
z2bt+LiPunlm0g4vV/oVizA87zeJPdtHZdWMCbNfAoGBALEVP1RXKsI9M7R01ML5
cocpE9qF81DkPzYsQxDRnheFNE9GOK2snADOiXa/ObvzQ5g57FJ7sJVkm2YECI9N
pNEMHXmW70G0upWmOnjZL6WxXcJjbpZ94SOFiD7GFFLgWs9bI4BdxMDX/EyXQafy
Scy7y5rzNperE0E7Xy1N10NX
-----END PRIVATE KEY-----`

func setupTestPemFile(t *testing.T) {
	assert.NoError(t, os.WriteFile(filenameTheClaPem, []byte(testPrivatePem), 0644))
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

func TestProcessWebhookGitHubEventPullRequestOpenedBadGH_APP_ID(t *testing.T) {
	actionText := "opened"
	c, rec := setupMockContextWebhook(t,
		map[string]string{
			"X-GitHub-Event": string(webhook.PullRequestEvent),
		}, github.PullRequestEvent{Action: &actionText})

	origGHAppIDEnvVar := os.Getenv(envGhAppId)
	defer func() {
		if origGHAppIDEnvVar == "" {
			assert.NoError(t, os.Unsetenv(envGhAppId))
		} else {
			assert.NoError(t, os.Setenv(envGhAppId, origGHAppIDEnvVar))
		}
	}()
	assert.NoError(t, os.Setenv(envGhAppId, "nonNumericGHAppID"))

	assert.NoError(t, processWebhook(c))
	assert.Equal(t, http.StatusBadRequest, c.Response().Status)
	assert.Equal(t, "strconv.Atoi: parsing \"nonNumericGHAppID\": invalid syntax", rec.Body.String())
}

func TestProcessWebhookGitHubEventPullRequestOpenedMissingPemFile(t *testing.T) {
	actionText := "opened"
	c, rec := setupMockContextWebhook(t,
		map[string]string{
			"X-GitHub-Event": string(webhook.PullRequestEvent),
		}, github.PullRequestEvent{Action: &actionText})

	origGHAppIDEnvVar := os.Getenv(envGhAppId)
	defer func() {
		if origGHAppIDEnvVar == "" {
			assert.NoError(t, os.Unsetenv(envGhAppId))
		} else {
			assert.NoError(t, os.Setenv(envGhAppId, origGHAppIDEnvVar))
		}
	}()
	assert.NoError(t, os.Setenv(envGhAppId, "-1"))

	assert.NoError(t, processWebhook(c))
	assert.Equal(t, http.StatusBadRequest, c.Response().Status)
	assert.Equal(t, "could not read private key: open the-cla.pem: no such file or directory", rec.Body.String())
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
