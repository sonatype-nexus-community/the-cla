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
	logger = zaptest.NewLogger(t)

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
	assert.NoError(t, os.WriteFile(ourGithub.FilenameTheClaPem, []byte(testPrivatePem), 0644))
}

type mockGitHub struct {
	t                *testing.T
	assertParameters bool
	newClientHttp    *http.Client
	newGithubClient  ourGithub.GitHubClient
}

func (m mockGitHub) NewClient(httpClient *http.Client) ourGithub.GitHubClient {
	if m.assertParameters {
		assert.Equal(m.t, m.newClientHttp, httpClient)
	}
	return m.newGithubClient
}

var _ ourGithub.GitHubInterface = (*mockGitHub)(nil)

type mockRepositories struct {
}

var _ ourGithub.RepositoriesService = (*mockRepositories)(nil)

//goland:noinspection GoUnusedParameter
func (m mockRepositories) Get(ctx context.Context, s string, s2 string) (repository *github.Repository, resp *github.Response, err error) {
	return
}

//goland:noinspection GoUnusedParameter
func (m mockRepositories) ListStatuses(ctx context.Context, owner, repo, ref string, opts *github.ListOptions) (repoStatus []*github.RepoStatus, resp *github.Response, err error) {
	return
}

//goland:noinspection GoUnusedParameter
func (m mockRepositories) CreateStatus(ctx context.Context, owner, repo, ref string, status *github.RepoStatus) (repoStatus *github.RepoStatus, resp *github.Response, err error) {
	return
}

type mockPullRequests struct {
}

var _ ourGithub.PullRequestsService = (*mockPullRequests)(nil)

//goland:noinspection GoUnusedParameter
func (m mockPullRequests) ListCommits(ctx context.Context, owner string, repo string, number int, opts *github.ListOptions) (repoCommits []*github.RepositoryCommit, resp *github.Response, err error) {
	return
}

type mockIssues struct {
	getLabelResp *github.Response
}

var _ ourGithub.IssuesService = (*mockIssues)(nil)

//goland:noinspection GoUnusedParameter
func (m mockIssues) GetLabel(ctx context.Context, owner string, repo string, name string) (label *github.Label, resp *github.Response, err error) {
	resp = m.getLabelResp
	return
}

//goland:noinspection GoUnusedParameter
func (m mockIssues) ListLabelsByIssue(ctx context.Context, owner string, repo string, issueNumber int, opts *github.ListOptions) (labels []*github.Label, resp *github.Response, err error) {
	return
}

//goland:noinspection GoUnusedParameter
func (m mockIssues) CreateLabel(ctx context.Context, owner string, repo string, label *github.Label) (resultLabel *github.Label, resp *github.Response, err error) {
	return
}

//goland:noinspection GoUnusedParameter
func (m mockIssues) AddLabelsToIssue(ctx context.Context, owner string, repo string, number int, labels []string) (resultLabels []*github.Label, resp *github.Response, err error) {
	return
}

//goland:noinspection GoUnusedParameter
func (m mockIssues) CreateComment(ctx context.Context, owner string, repo string, number int, comment *github.IssueComment) (resultComment *github.IssueComment, resp *github.Response, err error) {
	return
}

//goland:noinspection GoUnusedParameter
func (m mockIssues) ListComments(ctx context.Context, owner string, repo string, number int, opts *github.IssueListCommentsOptions) (comments []*github.IssueComment, resp *github.Response, err error) {
	return
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
	setupTestPemFile(t)

	origGithubImpl := ourGithub.GithubImpl
	defer func() {
		ourGithub.GithubImpl = origGithubImpl
	}()
	ourGithub.GithubImpl = mockGitHub{
		t:                t,
		assertParameters: false,
		newGithubClient: ourGithub.GitHubClient{
			Repositories: mockRepositories{},
			Users:        nil,
			PullRequests: mockPullRequests{},
			Issues: mockIssues{
				getLabelResp: &github.Response{
					Response: &http.Response{},
				},
			},
		},
	}

	assert.NoError(t, handleProcessWebhook(c))
	assert.Equal(t, http.StatusAccepted, c.Response().Status)
	assert.Equal(t, "accepted pull request for processing", rec.Body.String())
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
