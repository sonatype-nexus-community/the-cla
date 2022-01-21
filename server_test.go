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
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/go-github/v39/github"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
	webhook "gopkg.in/go-playground/webhooks.v5/github"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func resetEnvVariable(t *testing.T, variableName, originalValue string) {
	if originalValue == "" {
		assert.NoError(t, os.Unsetenv(variableName))
	} else {
		assert.NoError(t, os.Setenv(variableName, originalValue))
	}
}

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
		resetEnvVariable(t, envClsUrl, origClaUrl)
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
		resetEnvVariable(t, envClsUrl, origClaUrl)
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
		resetEnvVariable(t, envClsUrl, origClaUrl)
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
	assert.Error(t, retrieveCLAText(setupMockContextCLA()), `unsupported protocol scheme "badurlprotocolhttp"`)
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
	assert.Equal(t, `null
`, rec.Body.String())
}

func TestProcessGitHubOAuth_ExchangeError(t *testing.T) {
	origOAuth := oauthImpl
	defer func() {
		oauthImpl = origOAuth
	}()
	forcedError := fmt.Errorf("forced Exchange error")
	oauthImpl = &OAuthMock{
		exchangeError: forcedError,
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
		usersMock: UsersMock{
			mockGetError: forcedError,
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
		resetEnvVariable(t, envGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(envGhAppId, "nonNumericGHAppID"))

	prEvent := webhook.PullRequestPayload{}
	res, err := handlePullRequest(setupMockContextLogger(), prEvent)
	assert.EqualError(t, err, `strconv.Atoi: parsing "nonNumericGHAppID": invalid syntax`)
	assert.Equal(t, "", res)
}

func TestHandlePullRequestMissingPemFile(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(envGhAppId)
	defer func() {
		resetEnvVariable(t, envGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(envGhAppId, "-1"))

	// move pem file if it exists
	pemBackupFile := filenameTheClaPem + "_orig"
	errRename := os.Rename(filenameTheClaPem, pemBackupFile)
	defer func() {
		if errRename == nil {
			assert.NoError(t, os.Rename(pemBackupFile, filenameTheClaPem), "error renaming pem file in test")
		}
	}()

	prEvent := webhook.PullRequestPayload{}
	res, err := handlePullRequest(setupMockContextLogger(), prEvent)
	assert.EqualError(t, err, "could not read private key: open the-cla.pem: no such file or directory")
	assert.Equal(t, "", res)
}

func TestHandlePullRequestPullRequestsListCommitsError(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(envGhAppId)
	defer func() {
		resetEnvVariable(t, envGhAppId, origGHAppIDEnvVar)
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
	forcedError := fmt.Errorf("forced ListCommits error")
	githubImpl = &GitHubMock{
		pullRequestsMock: PullRequestsMock{
			mockListCommitsError: forcedError,
		},
	}

	prEvent := webhook.PullRequestPayload{}
	res, err := handlePullRequest(setupMockContextLogger(), prEvent)
	assert.EqualError(t, err, forcedError.Error())
	assert.Equal(t, "", res)
}

func TestHandlePullRequestPullRequestsListCommits(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(envGhAppId)
	defer func() {
		resetEnvVariable(t, envGhAppId, origGHAppIDEnvVar)
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
	login := "john"
	login2 := "doe"
	mockRepositoryCommits := []*github.RepositoryCommit{
		{
			Author: &github.User{
				Login: github.String(login),
				Email: github.String("j@gmail.com"),
			},
			SHA: github.String("johnSHA"),
		},
		{
			Author: &github.User{
				Login: github.String(login2),
				Email: github.String("d@gmail.com"),
			},
			SHA: github.String("doeSHA"),
		},
	}
	githubImpl = &GitHubMock{
		pullRequestsMock: PullRequestsMock{
			mockRepositoryCommits: mockRepositoryCommits,
		},
	}

	prEvent := webhook.PullRequestPayload{}

	dbMock, mock := newMockDb(t)
	defer func() {
		_ = dbMock.Close()
	}()
	origDb := db
	defer func() {
		db = origDb
	}()
	db = dbMock

	requiredClaVersion := getCurrentCLAVersion()
	mock.ExpectQuery(convertSqlToDbMockExpect(sqlSelectUserSignature)).
		WithArgs(login, requiredClaVersion).
		WillReturnRows(sqlmock.NewRows([]string{"LoginName,Email,GivenName,SignedAt,ClaVersion"}))
	mock.ExpectQuery(convertSqlToDbMockExpect(sqlSelectUserSignature)).
		WithArgs(login2, requiredClaVersion).
		WillReturnRows(sqlmock.NewRows([]string{"LoginName,Email,GivenName,SignedAt,ClaVersion"}))

	logger := echo.New().Logger

	res, err := handlePullRequest(logger, prEvent)
	assert.NoError(t, err)
	assert.Equal(t, `Author: `+login+` Email: j@gmail.com Commit SHA: johnSHA,Author: `+login2+` Email: d@gmail.com Commit SHA: doeSHA`, res)
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
		resetEnvVariable(t, envGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(envGhAppId, "nonNumericGHAppID"))

	assert.NoError(t, processWebhook(c))
	assert.Equal(t, http.StatusBadRequest, c.Response().Status)
	assert.Equal(t, `strconv.Atoi: parsing "nonNumericGHAppID": invalid syntax`, rec.Body.String())
}

func TestProcessWebhookGitHubEventPullRequestOpenedMissingPemFile(t *testing.T) {
	actionText := "opened"
	c, rec := setupMockContextWebhook(t,
		map[string]string{
			"X-GitHub-Event": string(webhook.PullRequestEvent),
		}, github.PullRequestEvent{Action: &actionText})

	origGHAppIDEnvVar := os.Getenv(envGhAppId)
	defer func() {
		resetEnvVariable(t, envGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(envGhAppId, "-1"))

	// move pem file if it exists
	pemBackupFile := filenameTheClaPem + "_orig"
	errRename := os.Rename(filenameTheClaPem, pemBackupFile)
	defer func() {
		if errRename == nil {
			assert.NoError(t, os.Rename(pemBackupFile, filenameTheClaPem), "error renaming pem file in test")
		}
	}()

	assert.NoError(t, processWebhook(c))
	assert.Equal(t, http.StatusBadRequest, c.Response().Status)
	assert.Equal(t, "could not read private key: open the-cla.pem: no such file or directory", rec.Body.String())
}

func TestProcessWebhookGitHubEventPullRequestPayloadActionHandled(t *testing.T) {
	verifyActionHandled(t, "opened")
	verifyActionHandled(t, "reopened")
	verifyActionHandled(t, "synchronize")
}

func verifyActionHandled(t *testing.T, actionText string) {
	c, rec := setupMockContextWebhook(t,
		map[string]string{
			"X-GitHub-Event": string(webhook.PullRequestEvent),
		}, github.PullRequestEvent{Action: &actionText})

	origGHAppIDEnvVar := os.Getenv(envGhAppId)
	defer func() {
		resetEnvVariable(t, envGhAppId, origGHAppIDEnvVar)
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

	assert.NoError(t, processWebhook(c))
	assert.Equal(t, http.StatusAccepted, c.Response().Status)
	assert.Equal(t, "", rec.Body.String())
}

func setupMockContextSignCla(t *testing.T, headers map[string]string, user UserSignature) (c echo.Context, rec *httptest.ResponseRecorder) {
	// Setup
	e := echo.New()

	reqBody, err := json.Marshal(user)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPut, pathSignCla, strings.NewReader(string(reqBody)))

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	return
}

func TestProcessSignClaBindError(t *testing.T) {
	c, rec := setupMockContextSignCla(t, map[string]string{}, UserSignature{})
	assert.EqualError(t, processSignCla(c), "code=415, message=Unsupported Media Type")
	assert.Equal(t, 0, c.Response().Status)
	assert.Equal(t, "", rec.Body.String())
}

func setupMockContextProcessWebhook(t *testing.T, user UserSignature) (c echo.Context, rec *httptest.ResponseRecorder) {
	// Setup
	e := echo.New()

	reqBody, err := json.Marshal(user)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, pathWebhook, strings.NewReader(string(reqBody)))

	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	return
}
