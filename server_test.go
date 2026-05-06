//
// Copyright (c) 2021-present Sonatype, Inc.
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

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v72/github"
	"github.com/sonatype-nexus-community/the-cla/db"
	ourGithub "github.com/sonatype-nexus-community/the-cla/github"
	"github.com/sonatype-nexus-community/the-cla/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	webhook "gopkg.in/go-playground/webhooks.v5/github"
)

func init() {
	gin.SetMode(gin.TestMode)
}

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

func TestZapLoggerFilterSkipsELB(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "bing ELB-HealthChecker yadda")
	logger := zaptest.NewLogger(t)
	result := ZapLoggerFilterAwsElb(logger)
	c, _ := newTestGinContext(req)
	result(c)
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

// newTestGinContext creates a gin.Context backed by an httptest recorder.
func newTestGinContext(req *http.Request) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	return c, rec
}

func setupMockContextCLA(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	logger = zaptest.NewLogger(t)

	req := httptest.NewRequest(http.MethodPost, pathClaText, strings.NewReader(mockClaText))
	req.Header.Set("Content-Type", "text/plain; charset=UTF-8")
	return newTestGinContext(req)
}

func TestHandleRetrieveCLAText_MissingClaURL(t *testing.T) {
	origClaUrl := os.Getenv(envClaUrl)
	defer func() {
		resetEnvVariable(t, envClaUrl, origClaUrl)
	}()
	resetEnvVariable(t, envClaUrl, "")

	c, rec := setupMockContextCLA(t)
	handleRetrieveCLAText(c)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, msgMissingClaUrl, rec.Body.String())
}

func TestHandleRetrieveCLAText_BadResponseCode(t *testing.T) {
	origClaUrl := os.Getenv(envClaUrl)
	defer func() {
		resetEnvVariable(t, envClaUrl, origClaUrl)
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, pathClaText, r.URL.EscapedPath())

		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	assert.NoError(t, os.Setenv(envClaUrl, ts.URL+pathClaText))
	c, rec := setupMockContextCLA(t)
	handleRetrieveCLAText(c)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "unexpected cla text response code: 403", rec.Body.String())
}

func TestHandleRetrieveCLAText(t *testing.T) {
	callCount := 0

	origClaUrl := os.Getenv(envClaUrl)
	defer func() {
		resetEnvVariable(t, envClaUrl, origClaUrl)
	}()

	// Clear cache for this URL
	delete(claCache, os.Getenv(envClaUrl))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, pathClaText, r.URL.EscapedPath())
		callCount += 1

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockClaText))
	}))
	defer ts.Close()

	assert.NoError(t, os.Setenv(envClaUrl, ts.URL+pathClaText))

	// Clear cache for test URL
	delete(claCache, ts.URL+pathClaText)

	c, rec := setupMockContextCLA(t)
	handleRetrieveCLAText(c)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, callCount, 1)

	// Ensure that subsequent calls use the cache
	c2, rec2 := setupMockContextCLA(t)
	handleRetrieveCLAText(c2)
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, callCount, 1)
}

func TestHandleRetrieveCLATextWithBadURL(t *testing.T) {
	callCount := 0

	origClaUrl := os.Getenv(envClaUrl)
	defer func() {
		resetEnvVariable(t, envClaUrl, origClaUrl)
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, pathClaText, r.URL.EscapedPath())
		callCount += 1

		// nobody home, be we should not even be knocking on this door - call should not occur
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	assert.NoError(t, os.Setenv(envClaUrl, "badURLProtocol"+ts.URL+pathClaText))
	c, rec := setupMockContextCLA(t)
	handleRetrieveCLAText(c)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), `unsupported protocol scheme`)
	assert.Equal(t, callCount, 0)
}

func setupMockContextOAuth(t *testing.T, queryParams map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	logger = zaptest.NewLogger(t)

	req := httptest.NewRequest(http.MethodGet, pathOAuthCallback, strings.NewReader("mock OAuth stuff"))

	q := req.URL.Query()
	for k, v := range queryParams {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	return newTestGinContext(req)
}

func TestHandleProcessGitHubOAuthMissingQueryParamState(t *testing.T) {
	c, rec := setupMockContextOAuth(t, map[string]string{})
	handleProcessGitHubOAuth(c)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "", rec.Body.String())
}

func setupMockContextWebhook(t *testing.T, headers map[string]string, prEvent github.PullRequestEvent) (*gin.Context, *httptest.ResponseRecorder) {
	logger = zaptest.NewLogger(t)

	reqBody, err := json.Marshal(prEvent)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, pathWebhook, strings.NewReader(string(reqBody)))

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return newTestGinContext(req)
}

func TestHandleProcessWebhookMissingHeaderGitHubEvent(t *testing.T) {
	c, rec := setupMockContextWebhook(t, map[string]string{}, github.PullRequestEvent{})

	handleProcessWebhook(c)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "missing X-GitHub-Event Header", rec.Body.String())
}

func TestHandleProcessWebhookUnhandledGitHubEvent(t *testing.T) {
	c, rec := setupMockContextWebhook(t,
		map[string]string{
			"X-GitHub-Event": "unknownGitHubEventHeaderValue",
		}, github.PullRequestEvent{})

	handleProcessWebhook(c)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, msgUnhandledGitHubEventType, rec.Body.String())
}

// Deal with side effect if local machine has a .env file setup, so we clear the webhook secret for sanity's sake,
// env variable value should be restored in defer() call.
func clearEnvGHWebhookSecretMadness(t *testing.T) (origGHWebhookSecret string) {
	origGHWebhookSecret = os.Getenv(envGhWebhookSecret)
	resetEnvVariable(t, envGhWebhookSecret, "") // clear it
	return origGHWebhookSecret
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

	origGHWebhookSecret := clearEnvGHWebhookSecretMadness(t)
	defer func() {
		resetEnvVariable(t, envGhWebhookSecret, origGHWebhookSecret)
	}()

	handleProcessWebhook(c)
	assert.Equal(t, http.StatusAccepted, rec.Code)
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

	origGHWebhookSecret := clearEnvGHWebhookSecretMadness(t)
	defer func() {
		resetEnvVariable(t, envGhWebhookSecret, origGHWebhookSecret)
	}()

	handleProcessWebhook(c)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, `strconv.ParseInt: parsing "nonNumericGHAppID": invalid syntax`, rec.Body.String())
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

	origGHWebhookSecret := clearEnvGHWebhookSecretMadness(t)
	defer func() {
		resetEnvVariable(t, envGhWebhookSecret, origGHWebhookSecret)
	}()

	handleProcessWebhook(c)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
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

	mock, dbIF, closeDbFunc := db.SetupMockDB(t)
	defer closeDbFunc()
	postgresDB = dbIF

	mock.ExpectQuery(db.ConvertSqlToDbMockExpect(db.SqlSelectUnsignedUsersForPR)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	origGHAppIDEnvVar := os.Getenv(ourGithub.EnvGhAppId)
	defer func() {
		resetEnvVariable(t, ourGithub.EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(ourGithub.EnvGhAppId, "-1"))

	resetPemFileImpl := ourGithub.SetupTestPemFile(t)
	defer resetPemFileImpl()

	resetGHJWTImpl := ourGithub.SetupMockGHJWT()
	defer resetGHJWTImpl()

	origGithubImpl := ourGithub.GHImpl
	defer func() {
		ourGithub.GHImpl = origGithubImpl
	}()
	ourGithub.GHImpl = &ourGithub.GHInterfaceMock{
		IssuesMock: ourGithub.IssuesMock{
			MockGetLabelResponse: &github.Response{
				Response: &http.Response{},
			},
			MockRemoveLabelResponse: &github.Response{
				Response: &http.Response{},
			},
		},
	}

	origGHWebhookSecret := clearEnvGHWebhookSecretMadness(t)
	defer func() {
		resetEnvVariable(t, envGhWebhookSecret, origGHWebhookSecret)
	}()

	handleProcessWebhook(c)
	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, "accepted pull request for processing", rec.Body.String())
}


func TestHandleProcessSignClaBindError(t *testing.T) {
	// Invalid JSON body causes ShouldBindJSON to fail
	logger = zaptest.NewLogger(t)
	req := httptest.NewRequest(http.MethodPut, pathSignCla, strings.NewReader("not valid json{"))
	req.Header.Set("Content-Type", "application/json")
	c, rec := newTestGinContext(req)
	handleProcessSignCla(c)
	assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
}

func TestHandleProcessSignClaFallsBackToEnvClaUrl(t *testing.T) {
	// When the client sends an empty claTextUrl (e.g. built without REACT_APP_CLA_URL),
	// the server should fall back to its own REACT_APP_CLA_URL env var to fetch the CLA text.
	logger = zaptest.NewLogger(t)

	const testCLAText = "CLA text from server env"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testCLAText))
	}))
	defer ts.Close()

	origClaUrl := os.Getenv(envClaUrl)
	defer resetEnvVariable(t, envClaUrl, origClaUrl)
	assert.NoError(t, os.Setenv(envClaUrl, ts.URL+pathClaText))
	delete(claCache, ts.URL+pathClaText)

	body := types.UserSignature{
		User:        types.User{Login: "testuser", Email: "test@example.com", GivenName: "Test User"},
		CLAVersion:  "1.0",
		CLATextUrl:  "", // empty — simulates a frontend built without REACT_APP_CLA_URL
	}
	bodyJSON, err := json.Marshal(body)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPut, pathSignCla, strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")
	c, rec := newTestGinContext(req)

	mock, dbIF, closeDbFunc := db.SetupMockDB(t)
	defer closeDbFunc()
	postgresDB = dbIF

	mock.ExpectExec(db.ConvertSqlToDbMockExpect(db.SqlInsertSignature)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(db.ConvertSqlToDbMockExpect(db.SqlSelectUserSignature)).
		WillReturnRows(sqlmock.NewRows([]string{}))

	handleProcessSignCla(c)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var result types.UserSignature
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.Equal(t, testCLAText, result.CLAText)
}

func setupMockContextSignature(t *testing.T, queryParams map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	logger = zaptest.NewLogger(t)

	req := httptest.NewRequest(http.MethodGet, pathSignCla, nil)

	q := req.URL.Query()
	for k, v := range queryParams {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	return newTestGinContext(req)
}

func TestHandleSignatureMissingLogin(t *testing.T) {
	c, rec := setupMockContextSignature(t, map[string]string{})

	handleSignature(c)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, fmt.Sprintf(msgTemplateMissingQueryParam, queryParameterLogin), rec.Body.String())
}

func TestHandleSignatureMissingCLAVersion(t *testing.T) {
	c, rec := setupMockContextSignature(t, map[string]string{queryParameterLogin: "myLogin"})

	handleSignature(c)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, fmt.Sprintf(msgTemplateMissingQueryParam, queryParameterCLAVersion), rec.Body.String())
}

func TestHandleSignatureHasAuthorSignedError(t *testing.T) {
	c, rec := setupMockContextSignature(t, map[string]string{
		queryParameterLogin:      "myLogin",
		queryParameterCLAVersion: "myCLAVersion",
	})

	mock, dbIF, closeDbFunc := db.SetupMockDB(t)
	defer closeDbFunc()
	postgresDB = dbIF

	forcedError := fmt.Errorf("forced SQL query error")
	mock.ExpectQuery(db.ConvertSqlToDbMockExpect(db.SqlSelectUserSignature)).
		WillReturnError(forcedError)

	handleSignature(c)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, forcedError.Error(), rec.Body.String())
}

func TestHandleSignatureHasAuthorSignedFalse(t *testing.T) {
	c, rec := setupMockContextSignature(t, map[string]string{
		queryParameterLogin:      "myLogin",
		queryParameterCLAVersion: "myCLAVersion",
	})

	mock, dbIF, closeDbFunc := db.SetupMockDB(t)
	defer closeDbFunc()
	postgresDB = dbIF

	mock.ExpectQuery(db.ConvertSqlToDbMockExpect(db.SqlSelectUserSignature)).
		WillReturnRows(sqlmock.NewRows([]string{"LoginName", "Email", "GivenName", "SignedAt", "ClaVersion"}))

	handleSignature(c)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "cla version myCLAVersion not signed by myLogin", rec.Body.String())
}

func TestHandleSignatureHasAuthorSignedAndHidesFields(t *testing.T) {
	const testLogin = "myLogin"
	const testCLAVersion = "myCLAVersion"
	const testCLATextUrl = "https://my.url/text"
	const testCLAText = "This is the CLA text"
	c, rec := setupMockContextSignature(t, map[string]string{
		queryParameterLogin:      testLogin,
		queryParameterCLAVersion: testCLAVersion,
	})

	mock, dbIF, closeDbFunc := db.SetupMockDB(t)
	defer closeDbFunc()
	postgresDB = dbIF

	now := time.Now()
	mock.ExpectQuery(db.ConvertSqlToDbMockExpect(db.SqlSelectUserSignature)).
		WillReturnRows(sqlmock.NewRows([]string{"LoginName", "Email", "GivenName", "SignedAt", "ClaVersion", "ClaTextUrl", "ClaText"}).
			AddRow(testLogin, "myEmail", "myGivenName", now, testCLAVersion, testCLATextUrl, testCLAText))

	handleSignature(c)
	assert.Equal(t, http.StatusOK, rec.Code)

	expectedJsonSignature, err := json.Marshal(types.UserSignature{
		User: types.User{
			Login:     testLogin,
			Email:     hiddenFieldValue, // hide email
			GivenName: hiddenFieldValue, // hide given name
		},
		CLAVersion: testCLAVersion,
		TimeSigned: now,
		CLATextUrl: testCLATextUrl,
		CLAText:    testCLAText,
	})
	assert.NoError(t, err)
	assert.Equal(t, string(expectedJsonSignature), strings.TrimRight(rec.Body.String(), "\n"))
}

func saveEnvInfoCredentials(t *testing.T) (resetInfoCreds func()) {
	origInfoUsername := os.Getenv(envInfoUsername)
	origInfoPassword := os.Getenv(envInfoPassword)
	resetInfoCreds = func() {
		resetEnvVariable(t, envInfoUsername, origInfoUsername)
		resetEnvVariable(t, envInfoUsername, origInfoPassword)
	}

	// setup testing logger while we're here
	logger = zaptest.NewLogger(t)

	return
}

func TestInfoBasicValidatorMissingEnv(t *testing.T) {
	resetInfoCreds := saveEnvInfoCredentials(t)
	defer resetInfoCreds()
	assert.NoError(t, os.Unsetenv(envInfoUsername))
	assert.NoError(t, os.Unsetenv(envInfoPassword))

	isValid := infoBasicValidator("yadda", "bing")
	assert.False(t, isValid)
}

func TestInfoBasicValidatorInValid(t *testing.T) {
	resetInfoCreds := saveEnvInfoCredentials(t)
	defer resetInfoCreds()
	assert.NoError(t, os.Setenv(envInfoUsername, "yadda"))
	assert.NoError(t, os.Setenv(envInfoPassword, "Doh!"))

	isValid := infoBasicValidator("yadda", "bing")
	assert.False(t, isValid)
}

func TestInfoBasicValidatorValid(t *testing.T) {
	resetInfoCreds := saveEnvInfoCredentials(t)
	defer resetInfoCreds()
	assert.NoError(t, os.Setenv(envInfoUsername, "yadda"))
	assert.NoError(t, os.Setenv(envInfoPassword, "bing"))

	isValid := infoBasicValidator("yadda", "bing")
	assert.True(t, isValid)
}

func TestNotifySignatureCompleteFails(t *testing.T) {
	setupMockContextCLA(t)

	// Ensure SMTP env vars are unset regardless of what godotenv loaded earlier in the test run
	for _, key := range []string{envSmtpHost, envSmtpPort, envNotificationAddress} {
		orig := os.Getenv(key)
		assert.NoError(t, os.Unsetenv(key))
		defer resetEnvVariable(t, key, orig)
	}

	testSignature := new(types.UserSignature)
	testSignature.User.Login = "LOGIN-ID"
	testSignature.User.Email = "someone@somewhere.tld"
	testSignature.User.GivenName = "A Person"
	testSignature.CLAVersion = "0.0.0"
	testSignature.TimeSigned = time.Now()
	testSignature.CLATextUrl = "https://a.url/cla.txt"
	testSignature.CLAText = "Some text here"

	err := notifySignatureComplete(testSignature)

	assert.EqualError(t, err, "SMTP Host, SMTP Port or Notification Address are empty - cannot send notification")
}
