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
	"encoding/json"
	"github.com/google/go-github/v42/github"
	"github.com/labstack/echo/v4"
	ourGithub "github.com/sonatype-nexus-community/the-cla/github"
	"github.com/sonatype-nexus-community/the-cla/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	webhook "gopkg.in/go-playground/webhooks.v5/github"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func resetEnvVariable(t *testing.T, variableName, originalValue string) {
	if originalValue == "" {
		assert.NoError(t, os.Unsetenv(variableName))
	} else {
		assert.NoError(t, os.Setenv(variableName, originalValue))
	}
}

func resetEnvVarPGHost(t *testing.T, origEnvPGHost string) {
	resetEnvVariable(t, envPGHost, origEnvPGHost)
}

func TestMainDBOpenPanic(t *testing.T) {
	errRecovered = nil
	origEnvPGHost := os.Getenv(envPGHost)
	defer func() {
		resetEnvVarPGHost(t, origEnvPGHost)
	}()
	assert.NoError(t, os.Setenv(envPGHost, "bogus-db-hostname"))

	defer func() {
		errRecovered = nil
	}()

	main()

	assert.True(t, strings.HasPrefix(errRecovered.Error(), "failed to ping database. host: bogus-db-hostname, port: "))
}

const mockClaText = `mock Cla text.`

func setupMockContextCLA(t *testing.T) echo.Context {
	logger = zaptest.NewLogger(t)

	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, pathClaText, strings.NewReader(mockClaText))
	req.Header.Set(echo.HeaderContentType, echo.MIMETextPlainCharsetUTF8)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c
}

func TestHandleRetrieveCLAText_MissingClaURL(t *testing.T) {
	origClaUrl := os.Getenv(envClsUrl)
	defer func() {
		resetEnvVariable(t, envClsUrl, origClaUrl)
	}()
	resetEnvVariable(t, envClsUrl, "")

	assert.EqualError(t, handleRetrieveCLAText(setupMockContextCLA(t)), msgMissingClaUrl)
}

func TestHandleRetrieveCLAText_BadResponseCode(t *testing.T) {
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
	assert.EqualError(t, handleRetrieveCLAText(setupMockContextCLA(t)), "unexpected cla text response code: 403")
}

func TestHandleRetrieveCLAText(t *testing.T) {
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
	assert.NoError(t, handleRetrieveCLAText(setupMockContextCLA(t)))
	assert.Equal(t, callCount, 1)

	// Ensure that subsequent calls use the cache

	assert.NoError(t, handleRetrieveCLAText(setupMockContextCLA(t)))
	assert.Equal(t, callCount, 1)
}

func TestHandleRetrieveCLATextWithBadURL(t *testing.T) {
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
	assert.Error(t, handleRetrieveCLAText(setupMockContextCLA(t)), `unsupported protocol scheme "badurlprotocolhttp"`)
	assert.Equal(t, callCount, 0)
}

func setupMockContextOAuth(t *testing.T, queryParams map[string]string) (c echo.Context, rec *httptest.ResponseRecorder) {
	logger = zaptest.NewLogger(t)

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

func TestHandleProcessGitHubOAuthMissingQueryParamState(t *testing.T) {
	c, rec := setupMockContextOAuth(t, map[string]string{})
	assert.NoError(t, handleProcessGitHubOAuth(c))
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

func TestHandleProcessWebhookMissingHeaderGitHubEvent(t *testing.T) {
	c, rec := setupMockContextWebhook(t, map[string]string{}, github.PullRequestEvent{})

	assert.NoError(t, handleProcessWebhook(c))
	assert.Equal(t, http.StatusBadRequest, c.Response().Status)
	assert.Equal(t, "missing X-GitHub-Event Header", rec.Body.String())
}

func TestHandleProcessWebhookUnhandledGitHubEvent(t *testing.T) {
	c, rec := setupMockContextWebhook(t,
		map[string]string{
			"X-GitHub-Event": "unknownGitHubEventHeaderValue",
		}, github.PullRequestEvent{})

	assert.NoError(t, handleProcessWebhook(c))
	assert.Equal(t, http.StatusBadRequest, c.Response().Status)
	assert.Equal(t, msgUnhandledGitHubEventType, rec.Body.String())
}

func TestHandleProcessWebhookGitHubEventPullRequestPayloadActionIgnored(t *testing.T) {
	actionText := "someIgnoredAction"
	c, rec := setupMockContextWebhook(t,
		map[string]string{
			"X-GitHub-Event": string(webhook.PullRequestEvent),
		}, github.PullRequestEvent{Action: &actionText})

	origGHAppIDEnvVar := os.Getenv(ourGithub.EnvGhAppId)
	defer func() {
		resetEnvVariable(t, ourGithub.EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(ourGithub.EnvGhAppId, "-1"))

	assert.NoError(t, handleProcessWebhook(c))
	assert.Equal(t, http.StatusAccepted, c.Response().Status)
	assert.Equal(t, "No action taken for: someIgnoredAction", rec.Body.String())
}

func TestHandleProcessWebhookGitHubEventPullRequestOpenedBadGH_APP_ID(t *testing.T) {
	actionText := "opened"
	c, rec := setupMockContextWebhook(t,
		map[string]string{
			"X-GitHub-Event": string(webhook.PullRequestEvent),
		}, github.PullRequestEvent{Action: &actionText})

	origGHAppIDEnvVar := os.Getenv(ourGithub.EnvGhAppId)
	defer func() {
		resetEnvVariable(t, ourGithub.EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(ourGithub.EnvGhAppId, "nonNumericGHAppID"))

	assert.NoError(t, handleProcessWebhook(c))
	assert.Equal(t, http.StatusBadRequest, c.Response().Status)
	assert.Equal(t, `strconv.Atoi: parsing "nonNumericGHAppID": invalid syntax`, rec.Body.String())
}

func TestHandleProcessWebhookGitHubEventPullRequestOpenedMissingPemFile(t *testing.T) {
	actionText := "opened"
	c, rec := setupMockContextWebhook(t,
		map[string]string{
			"X-GitHub-Event": string(webhook.PullRequestEvent),
		}, github.PullRequestEvent{Action: &actionText})

	origGHAppIDEnvVar := os.Getenv(ourGithub.EnvGhAppId)
	defer func() {
		resetEnvVariable(t, ourGithub.EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(ourGithub.EnvGhAppId, "-1"))

	// move pem file if it exists
	pemBackupFile := ourGithub.FilenameTheClaPem + "_orig"
	errRename := os.Rename(ourGithub.FilenameTheClaPem, pemBackupFile)
	defer func() {
		if errRename == nil {
			assert.NoError(t, os.Rename(pemBackupFile, ourGithub.FilenameTheClaPem), "error renaming pem file in test")
		}
	}()

	assert.NoError(t, handleProcessWebhook(c))
	assert.Equal(t, http.StatusBadRequest, c.Response().Status)
	assert.Equal(t, "could not read private key: open the-cla.pem: no such file or directory", rec.Body.String())
}

func TestHandleProcessWebhookGitHubEventPullRequestPayloadActionHandled(t *testing.T) {
	verifyActionHandled(t, "opened")
	verifyActionHandled(t, "reopened")
	verifyActionHandled(t, "synchronize")
}

func verifyActionHandled(t *testing.T, actionText string) {
	c, rec := setupMockContextWebhook(t,
		map[string]string{
			"X-GitHub-Event": string(webhook.PullRequestEvent),
		}, github.PullRequestEvent{Action: &actionText})

	origGHAppIDEnvVar := os.Getenv(ourGithub.EnvGhAppId)
	defer func() {
		resetEnvVariable(t, ourGithub.EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(ourGithub.EnvGhAppId, "-1"))

	// move pem file if it exists
	pemBackupFile := ourGithub.FilenameTheClaPem + "_orig"
	errRename := os.Rename(ourGithub.FilenameTheClaPem, pemBackupFile)
	defer func() {
		assert.NoError(t, os.Remove(ourGithub.FilenameTheClaPem))
		if errRename == nil {
			assert.NoError(t, os.Rename(pemBackupFile, ourGithub.FilenameTheClaPem), "error renaming pem file in test")
		}
	}()
	//ourGithub.SetupTestPemFile(t)
	//
	//origGithubImpl := githubImpl
	//defer func() {
	//	githubImpl = origGithubImpl
	//}()
	//githubImpl = &GitHubMock{}

	assert.NoError(t, handleProcessWebhook(c))
	assert.Equal(t, http.StatusAccepted, c.Response().Status)
	assert.Equal(t, "", rec.Body.String())
}

func setupMockContextSignCla(t *testing.T, headers map[string]string, user types.UserSignature) (c echo.Context, rec *httptest.ResponseRecorder) {
	logger = zaptest.NewLogger(t)

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

func TestHandleProcessSignClaBindError(t *testing.T) {
	c, rec := setupMockContextSignCla(t, map[string]string{}, types.UserSignature{})
	assert.EqualError(t, handleProcessSignCla(c), "code=415, message=Unsupported Media Type")
	assert.Equal(t, 0, c.Response().Status)
	assert.Equal(t, "", rec.Body.String())
}

func setupMockContextProcessWebhook(t *testing.T, user types.UserSignature) (c echo.Context, rec *httptest.ResponseRecorder) {
	// Setup
	e := echo.New()

	reqBody, err := json.Marshal(user)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, pathWebhook, strings.NewReader(string(reqBody)))

	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	return
}
