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
	"github.com/google/go-github/v33/github"
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

	oauth := createOAuth()

	assert.Equal(t, forcedClientId, oauth.getConf().ClientID)
	assert.Equal(t, forcedGHClientSecret, oauth.getConf().ClientSecret)
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

/*// RepositoriesMock mocks RepositoriesService
type RepositoriesMock struct {
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
*/

type OAuthMock struct {
	exchangeToken *oauth2.Token
	exchangeError error
}

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

// UsersMock mocks UsersService
type UsersMock struct {
	mockUser     *github.User
	mockResponse *github.Response
	mockGetError error
}

// Get returns a user.
func (u *UsersMock) Get(context.Context, string) (*github.User, *github.Response, error) {
	return u.mockUser, u.mockResponse, u.mockGetError
}

type PullRequestsMock struct {
	mockRepositoryCommits []*github.RepositoryCommit
	mockResponse          *github.Response
	mockListCommitsError  error
}

//goland:noinspection GoUnusedParameter
func (p *PullRequestsMock) ListCommits(ctx context.Context, owner string, repo string, number int, opts *github.ListOptions) ([]*github.RepositoryCommit, *github.Response, error) {
	return p.mockRepositoryCommits, p.mockResponse, p.mockListCommitsError
}

type IssuesMock struct {
	mockCreateLabel         *github.Label
	mockCreateLabelResponse *github.Response
	mockCreateLabelError    error
	mockAddLabels           []*github.Label
	mockAddLabelsResponse   *github.Response
	mockAddLabelsError      error
}

//goland:noinspection GoUnusedParameter
func (i *IssuesMock) CreateLabel(ctx context.Context, owner string, repo string, label *github.Label) (*github.Label, *github.Response, error) {
	return i.mockCreateLabel, i.mockCreateLabelResponse, i.mockCreateLabelError
}

//goland:noinspection GoUnusedParameter
func (i *IssuesMock) AddLabelsToIssue(ctx context.Context, owner string, repo string, number int, labels []string) ([]*github.Label, *github.Response, error) {
	return i.mockAddLabels, i.mockAddLabelsResponse, i.mockAddLabelsError
}

// GitHubMock implements GitHubInterface.
type GitHubMock struct {
	usersMock        UsersMock
	pullRequestsMock PullRequestsMock
	issuesMock       IssuesMock
}

// NewClient something
//goland:noinspection GoUnusedParameter
func (g *GitHubMock) NewClient(httpClient *http.Client) GitHubClient {
	return GitHubClient{
		//Repositories: &RepositoriesMock{},
		Users: &UsersMock{
			mockGetError: g.usersMock.mockGetError,
			mockUser:     g.usersMock.mockUser,
			mockResponse: g.usersMock.mockResponse,
		},
		PullRequests: &PullRequestsMock{
			mockListCommitsError:  g.pullRequestsMock.mockListCommitsError,
			mockRepositoryCommits: g.pullRequestsMock.mockRepositoryCommits,
			mockResponse:          g.pullRequestsMock.mockResponse,
		},
		Issues: &IssuesMock{
			mockCreateLabel:         g.issuesMock.mockCreateLabel,
			mockCreateLabelResponse: g.issuesMock.mockCreateLabelResponse,
			mockCreateLabelError:    g.issuesMock.mockCreateLabelError,
			mockAddLabels:           g.issuesMock.mockAddLabels,
			mockAddLabelsResponse:   g.issuesMock.mockAddLabelsResponse,
			mockAddLabelsError:      g.issuesMock.mockAddLabelsError,
		},
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
	res, err := handlePullRequest(nil, prEvent)
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
	res, err := handlePullRequest(nil, prEvent)
	assert.EqualError(t, err, "could not read private key: open the-cla.pem: no such file or directory")
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
	res, err := handlePullRequest(nil, prEvent)
	assert.EqualError(t, err, forcedError.Error())
	assert.Equal(t, "", res)
}

// convertSqlToDbMockExpect takes a "real" sql string and adds escape characters as needed to produce a
// regex matching string for use with database mock expect calls.
func convertSqlToDbMockExpect(realSql string) string {
	reDollarSign := regexp.MustCompile(`(\$)`)
	sqlMatch := reDollarSign.ReplaceAll([]byte(realSql), []byte(`\$`))

	reLeftParen := regexp.MustCompile(`(\()`)
	sqlMatch = reLeftParen.ReplaceAll(sqlMatch, []byte(`\(`))

	reRightParen := regexp.MustCompile(`(\))`)
	sqlMatch = reRightParen.ReplaceAll(sqlMatch, []byte(`\)`))
	return string(sqlMatch)
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
			Committer: &github.User{
				Login: github.String(login),
				Email: github.String("j@gmail.com"),
			},
			SHA: github.String("johnSHA"),
		},
		{
			Committer: &github.User{
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

func TestHandlePullRequestPullRequestsCreateLabelError(t *testing.T) {
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
	mockRepositoryCommits := []*github.RepositoryCommit{{Committer: &github.User{}}}
	forcedError := fmt.Errorf("forced CreateLabel error")
	githubImpl = &GitHubMock{
		pullRequestsMock: PullRequestsMock{mockRepositoryCommits: mockRepositoryCommits},
		issuesMock:       IssuesMock{mockCreateLabelError: forcedError},
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

	mock.ExpectQuery(convertSqlToDbMockExpect(sqlSelectUserSignature)).
		WithArgs("", getCurrentCLAVersion()).
		WillReturnRows(sqlmock.NewRows([]string{"LoginName,Email,GivenName,SignedAt,ClaVersion"}))

	res, err := handlePullRequest(nil, prEvent)
	// #TODO change assertion below to verify forcedError is returned when CreateLabel fails.
	//assert.EqualError(t, err, forcedError.Error())
	assert.NoError(t, err)
	assert.Equal(t, "Author:  Email:  Commit SHA: ", res)
}

func TestHandlePullRequestPullRequestsAddLabelsToIssueError(t *testing.T) {
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
	mockRepositoryCommits := []*github.RepositoryCommit{{Committer: &github.User{}}}
	forcedError := fmt.Errorf("forced AddLabelsToIssue error")
	githubImpl = &GitHubMock{
		pullRequestsMock: PullRequestsMock{mockRepositoryCommits: mockRepositoryCommits},
		issuesMock:       IssuesMock{mockAddLabelsError: forcedError},
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

	mock.ExpectQuery(convertSqlToDbMockExpect(sqlSelectUserSignature)).
		WithArgs("", getCurrentCLAVersion()).
		WillReturnRows(sqlmock.NewRows([]string{"LoginName,Email,GivenName,SignedAt,ClaVersion"}))

	res, err := handlePullRequest(nil, prEvent)
	assert.EqualError(t, err, forcedError.Error())
	assert.Equal(t, "Author:  Email:  Commit SHA: ", res)
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

func newMockDb(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		assert.NoError(t, err)
	}

	return db, mock
}

type AnyTime struct{}

// Match satisfies sqlmock.Argument interface
func (a AnyTime) Match(v driver.Value) bool {
	_, ok := v.(time.Time)
	return ok
}

func TestProcessSignClaDBInsertError(t *testing.T) {
	user := UserSignature{}
	c, rec := setupMockContextSignCla(t, map[string]string{"Content-Type": "application/json"}, user)

	dbMock, mock := newMockDb(t)
	defer func() {
		_ = dbMock.Close()
	}()
	origDb := db
	defer func() {
		db = origDb
	}()
	db = dbMock

	forcedError := fmt.Errorf("forced SQL insert error")
	mock.ExpectExec(convertSqlToDbMockExpect(sqlInsertSignature)).
		WithArgs(user.User.Login, user.User.Email, user.User.GivenName, AnyTime{}, user.CLAVersion).
		WillReturnError(forcedError).
		WillReturnResult(sqlmock.NewErrorResult(forcedError))

	assert.NoError(t, processSignCla(c), "some db error")
	assert.Equal(t, http.StatusBadRequest, c.Response().Status)
	assert.Equal(t, forcedError.Error(), rec.Body.String())
}

func TestProcessSignClaSigned(t *testing.T) {
	user := UserSignature{}
	c, rec := setupMockContextSignCla(t, map[string]string{"Content-Type": "application/json"}, user)

	dbMock, mock := newMockDb(t)
	defer func() {
		_ = dbMock.Close()
	}()
	origDb := db
	defer func() {
		db = origDb
	}()
	db = dbMock

	forcedError := fmt.Errorf("forced SQL insert error")
	mock.ExpectExec(convertSqlToDbMockExpect(sqlInsertSignature)).
		WithArgs(user.User.Login, user.User.Email, user.User.GivenName, AnyTime{}, user.CLAVersion).
		WillReturnResult(sqlmock.NewErrorResult(forcedError))

	assert.NoError(t, processSignCla(c), "some db error")
	assert.Equal(t, http.StatusCreated, c.Response().Status)

	// ignore/truncate the TimeSigned suffix
	jsonUserBytes, err := json.Marshal(user)
	assert.NoError(t, err)
	jsonUser := string(jsonUserBytes)
	reg := regexp.MustCompile(`(.*)"TimeSigned.*`)
	jsonUserPrefix := reg.ReplaceAllString(jsonUser, "${1}")
	assert.True(t, strings.HasPrefix(rec.Body.String(), jsonUserPrefix), "body:\n%s,\nprefix:\n%s", rec.Body.String(), jsonUserPrefix)
}

func TestMigrateDBErrorPostgresWithInstance(t *testing.T) {
	dbMock, _ := newMockDb(t)
	defer func() {
		_ = dbMock.Close()
	}()

	assert.EqualError(t, migrateDB(dbMock), "all expectations were already fulfilled, call to Query 'SELECT CURRENT_DATABASE()' with args [] was not expected in line 0: SELECT CURRENT_DATABASE()")
}

func setupMockPostgresWithInstance(mock sqlmock.Sqlmock) (args []driver.Value) {
	// mocks for 'postgres.WithInstance()'
	mock.ExpectQuery(`SELECT CURRENT_DATABASE()`).
		WillReturnRows(sqlmock.NewRows([]string{"col1"}).FromCSVString("theDatabaseName"))
	mock.ExpectQuery(`SELECT CURRENT_SCHEMA()`).
		WillReturnRows(sqlmock.NewRows([]string{"col1"}).FromCSVString("theDatabaseSchema"))

	args = []driver.Value{"1014225327"}
	mock.ExpectExec(`SELECT pg_advisory_lock\(\$1\)`).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS "schema_migrations" \(version bigint not null primary key, dirty boolean not null\)`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec(`SELECT pg_advisory_unlock\(\$1\)`).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(0, 0))
	return
}

func TestMigrateDBErrorMigrateUp(t *testing.T) {
	dbMock, mock := newMockDb(t)
	defer func() {
		_ = dbMock.Close()
	}()

	setupMockPostgresWithInstance(mock)

	assert.EqualError(t, migrateDB(dbMock), "try lock failed in line 0: SELECT pg_advisory_lock($1) (details: all expectations were already fulfilled, call to ExecQuery 'SELECT pg_advisory_lock($1)' with args [{Name: Ordinal:1 Value:1014225327}] was not expected)")
}

func TestMigrateDB(t *testing.T) {
	dbMock, mock := newMockDb(t)
	defer func() {
		_ = dbMock.Close()
	}()

	args := setupMockPostgresWithInstance(mock)

	// mocks for the migrate.Up()
	mock.ExpectExec(`SELECT pg_advisory_lock\(\$1\)`).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery(`SELECT version, dirty FROM "schema_migrations" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"version", "dirty"}).FromCSVString("-1,false"))

	mock.ExpectBegin()
	mock.ExpectExec(`TRUNCATE "schema_migrations"`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`INSERT INTO "schema_migrations" \(version, dirty\) VALUES \(\$1, \$2\)`).
		WithArgs(1, true).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	mock.ExpectExec(`BEGIN; CREATE EXTENSION pgcrypto; CREATE TABLE signatures*`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectExec(`TRUNCATE "schema_migrations"`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`INSERT INTO "schema_migrations" \(version, dirty\) VALUES \(\$1, \$2\)`).
		WithArgs(1, false).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	mock.ExpectExec(`SELECT pg_advisory_unlock\(\$1\)`).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(0, 0))

	assert.NoError(t, migrateDB(dbMock))
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

func TestHasCommitterSignedTheClaQueryError(t *testing.T) {
	user := UserSignature{}
	c, rec := setupMockContextProcessWebhook(t, user)

	dbMock, mock := newMockDb(t)
	defer func() {
		_ = dbMock.Close()
	}()
	origDb := db
	defer func() {
		db = origDb
	}()
	db = dbMock

	forcedError := fmt.Errorf("forced SQL query error")
	mock.ExpectQuery(convertSqlToDbMockExpect(sqlSelectUserSignature)).
		WillReturnError(forcedError)

	committer := github.User{}
	hasSigned, err := hasCommitterSignedTheCla(c.Logger(), committer)
	assert.EqualError(t, err, forcedError.Error())
	assert.False(t, hasSigned)
	assert.Equal(t, "", rec.Body.String())
}

func TestHasCommitterSignedTheClaReadRowError(t *testing.T) {
	user := UserSignature{}
	c, rec := setupMockContextProcessWebhook(t, user)

	dbMock, mock := newMockDb(t)
	defer func() {
		_ = dbMock.Close()
	}()
	origDb := db
	defer func() {
		db = origDb
	}()
	db = dbMock

	loginName := "myLoginName"
	mock.ExpectQuery(convertSqlToDbMockExpect(sqlSelectUserSignature)).
		WithArgs(loginName, getCurrentCLAVersion()).
		WillReturnRows(sqlmock.NewRows([]string{"LoginName", "Email", "GivenName", "SignedAt", "ClaVersion"}).
			FromCSVString(`myLoginName,myEmail,myGivenName,INVALID_TIME_VALUE_TO_CAUSE_ROW_READ_ERROR,myClaVersion`))

	committer := github.User{}
	committer.Login = &loginName
	hasSigned, err := hasCommitterSignedTheCla(c.Logger(), committer)
	assert.EqualError(t, err, "sql: Scan error on column index 3, name \"SignedAt\": unsupported Scan, storing driver.Value type []uint8 into type *time.Time")
	assert.True(t, hasSigned)
	assert.Equal(t, "", rec.Body.String())
}

func TestHasCommitterSignedTheClaTrue(t *testing.T) {
	user := UserSignature{}
	c, rec := setupMockContextProcessWebhook(t, user)

	dbMock, mock := newMockDb(t)
	defer func() {
		_ = dbMock.Close()
	}()
	origDb := db
	defer func() {
		db = origDb
	}()
	db = dbMock

	loginName := "myLoginName"
	rs := sqlmock.NewRows([]string{"LoginName", "Email", "GivenName", "SignedAt", "ClaVersion"})
	now := time.Now()
	rs.AddRow(loginName, "myEmail", "myGivenName", now, "myClaVersion")
	mock.ExpectQuery(convertSqlToDbMockExpect(sqlSelectUserSignature)).
		WithArgs(loginName, getCurrentCLAVersion()).
		WillReturnRows(rs)

	committer := github.User{}
	committer.Login = &loginName
	hasSigned, err := hasCommitterSignedTheCla(c.Logger(), committer)
	assert.NoError(t, err)
	assert.True(t, hasSigned)
	assert.Equal(t, "", rec.Body.String())
}
