package github

import (
	"context"
	"fmt"
	"go.uber.org/zap/zaptest"
	"net/http"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/go-github/v42/github"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
	webhook "gopkg.in/go-playground/webhooks.v5/github"
)

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
	mockGetLabel                  *github.Label
	mockGetLabelResponse          *github.Response
	mockGetLabelError             error
	mockListLabelsByIssue         []*github.Label
	mockListLabelsByIssueResponse *github.Response
	mockListLabelsByIssueError    error
	mockCreateLabel               *github.Label
	mockCreateLabelResponse       *github.Response
	mockCreateLabelError          error
	mockAddLabels                 []*github.Label
	mockAddLabelsResponse         *github.Response
	mockAddLabelsError            error
	mockComment                   *github.IssueComment
	mockCreateCommentResponse     *github.Response
	mockCreateCommentError        error
	mockListComments              []*github.IssueComment
	mockListCommentsResponse      *github.Response
	mockListCommentsError         error
}

//goland:noinspection GoUnusedParameter
func (i *IssuesMock) GetLabel(ctx context.Context, owner string, repo string, labelName string) (*github.Label, *github.Response, error) {
	return i.mockGetLabel, i.mockGetLabelResponse, i.mockGetLabelError
}

//goland:noinspection GoUnusedParameter
func (i *IssuesMock) CreateLabel(ctx context.Context, owner string, repo string, label *github.Label) (*github.Label, *github.Response, error) {
	return i.mockCreateLabel, i.mockCreateLabelResponse, i.mockCreateLabelError
}

//goland:noinspection GoUnusedParameter
func (i *IssuesMock) ListLabelsByIssue(ctx context.Context, owner string, repo string, issueNumber int, opts *github.ListOptions) ([]*github.Label, *github.Response, error) {
	return i.mockListLabelsByIssue, i.mockListLabelsByIssueResponse, i.mockListLabelsByIssueError
}

//goland:noinspection GoUnusedParameter
func (i *IssuesMock) AddLabelsToIssue(ctx context.Context, owner string, repo string, number int, labels []string) ([]*github.Label, *github.Response, error) {
	return i.mockAddLabels, i.mockAddLabelsResponse, i.mockAddLabelsError
}

//goland:noinspection GoUnusedParameter
func (i *IssuesMock) CreateComment(ctx context.Context, owner string, repo string, number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error) {
	return i.mockComment, i.mockCreateCommentResponse, i.mockCreateCommentError
}

//goland:noinspection GoUnusedParameter
func (i *IssuesMock) ListComments(ctx context.Context, owner string, repo string, number int, opts *github.IssueListCommentsOptions) ([]*github.IssueComment, *github.Response, error) {
	return i.mockListComments, i.mockListCommentsResponse, i.mockListCommentsError
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
			mockGetLabel:                  g.issuesMock.mockGetLabel,
			mockGetLabelResponse:          g.issuesMock.mockGetLabelResponse,
			mockGetLabelError:             g.issuesMock.mockGetLabelError,
			mockListLabelsByIssue:         g.issuesMock.mockListLabelsByIssue,
			mockListLabelsByIssueResponse: g.issuesMock.mockListLabelsByIssueResponse,
			mockListLabelsByIssueError:    g.issuesMock.mockListLabelsByIssueError,
			mockCreateLabel:               g.issuesMock.mockCreateLabel,
			mockCreateLabelResponse:       g.issuesMock.mockCreateLabelResponse,
			mockCreateLabelError:          g.issuesMock.mockCreateLabelError,
			mockAddLabels:                 g.issuesMock.mockAddLabels,
			mockAddLabelsResponse:         g.issuesMock.mockAddLabelsResponse,
			mockAddLabelsError:            g.issuesMock.mockAddLabelsError,
		},
	}
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

func SetupTestPemFile(t *testing.T) {
	assert.NoError(t, os.WriteFile(FilenameTheClaPem, []byte(testPrivatePem), 0644))
}

func resetEnvVariable(t *testing.T, variableName, originalValue string) {
	if originalValue == "" {
		assert.NoError(t, os.Unsetenv(variableName))
	} else {
		assert.NoError(t, os.Setenv(variableName, originalValue))
	}
}

func TestCreateLabelIfNotExists_GetLabelError(t *testing.T) {
	origGithubImpl := githubImpl
	defer func() {
		githubImpl = origGithubImpl
	}()
	forcedError := fmt.Errorf("forced GetLabel error")
	githubImpl = &GitHubMock{
		issuesMock: IssuesMock{mockGetLabelError: forcedError},
	}

	client := githubImpl.NewClient(nil)

	label, err := _createRepoLabelIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", "", "", "")
	assert.EqualError(t, err, forcedError.Error())
	assert.Nil(t, label)
}

func TestCreateLabelIfNotExists_LabelExists(t *testing.T) {
	origGithubImpl := githubImpl
	defer func() {
		githubImpl = origGithubImpl
	}()
	labelName := "we already got one"
	existingLabel := &github.Label{Name: &labelName}
	githubImpl = &GitHubMock{
		issuesMock: IssuesMock{mockGetLabel: existingLabel},
	}

	client := githubImpl.NewClient(nil)

	label, err := _createRepoLabelIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", "", "", "")
	assert.NoError(t, err)
	assert.Equal(t, label, existingLabel)
}

func TestCreateLabelIfNotExists_CreateError(t *testing.T) {
	origGithubImpl := githubImpl
	defer func() {
		githubImpl = origGithubImpl
	}()
	forcedError := fmt.Errorf("forced CreateLabel error")
	githubImpl = &GitHubMock{issuesMock: IssuesMock{mockCreateLabelError: forcedError}}
	client := githubImpl.NewClient(nil)

	label, err := _createRepoLabelIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", "", "", "")
	assert.EqualError(t, err, forcedError.Error())
	assert.Nil(t, label)
}

func TestCreateLabelIfNotExists(t *testing.T) {
	origGithubImpl := githubImpl
	defer func() {
		githubImpl = origGithubImpl
	}()
	labelName := labelNameCLANotSigned
	labelColor := "fa3a3a"
	labelDescription := "The CLA is not signed"
	labelToCreate := &github.Label{Name: &labelName, Color: &labelColor, Description: &labelDescription}
	githubImpl = &GitHubMock{issuesMock: IssuesMock{mockCreateLabel: labelToCreate}}

	client := githubImpl.NewClient(nil)

	label, err := _createRepoLabelIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", "", "", "")
	assert.NoError(t, err)
	assert.Equal(t, label, labelToCreate)
}

func TestAddLabelToIssueIfNotExists_ListLabelsByIssueError(t *testing.T) {
	origGithubImpl := githubImpl
	defer func() {
		githubImpl = origGithubImpl
	}()
	forcedError := fmt.Errorf("forced ListLabelsByIssue error")
	githubImpl = &GitHubMock{issuesMock: IssuesMock{mockListLabelsByIssueError: forcedError}}

	client := githubImpl.NewClient(nil)

	label, err := _addLabelToIssueIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", 0, "")
	assert.EqualError(t, err, forcedError.Error())
	assert.Nil(t, label)
}

func TestAddLabelToIssueIfNotExists_LabelAlreadyExists(t *testing.T) {
	origGithubImpl := githubImpl
	defer func() {
		githubImpl = origGithubImpl
	}()
	labelName := labelNameCLANotSigned
	existingLabel := &github.Label{Name: &labelName}
	existingLabelList := []*github.Label{existingLabel}
	githubImpl = &GitHubMock{
		issuesMock: IssuesMock{mockListLabelsByIssue: existingLabelList},
	}

	client := githubImpl.NewClient(nil)

	label, err := _addLabelToIssueIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", 0, "")
	assert.NoError(t, err)
	assert.Equal(t, existingLabel, label)
}

func Test_AddLabelToIssueIfNotExists_AddLabelError(t *testing.T) {
	origGithubImpl := githubImpl
	defer func() {
		githubImpl = origGithubImpl
	}()
	forcedError := fmt.Errorf("forced AddLabels error")
	githubImpl = &GitHubMock{
		issuesMock: IssuesMock{mockAddLabelsError: forcedError},
	}

	client := githubImpl.NewClient(nil)

	label, err := _addLabelToIssueIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", 0, "")
	assert.EqualError(t, err, forcedError.Error())
	assert.Nil(t, label)
}

func Test_AddLabelToIssueIfNotExists(t *testing.T) {
	origGithubImpl := githubImpl
	defer func() {
		githubImpl = origGithubImpl
	}()
	labelName := labelNameCLANotSigned
	labelColor := "fa3a3a"
	labelDescription := "The CLA is not signed"
	labelToCreate := &github.Label{Name: &labelName, Color: &labelColor, Description: &labelDescription}
	githubImpl = &GitHubMock{issuesMock: IssuesMock{mockAddLabels: []*github.Label{labelToCreate}}}

	client := githubImpl.NewClient(nil)

	label, err := _addLabelToIssueIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", 0, "")
	assert.NoError(t, err)
	// real gitHub API returns different result, but does not matter to us now
	assert.Nil(t, label)
}

func TestHandlePullRequestPullRequestsCreateLabelError(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(EnvGhAppId)
	defer func() {
		resetEnvVariable(t, EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(EnvGhAppId, "-1"))

	// move pem file if it exists
	pemBackupFile := FilenameTheClaPem + "_orig"
	errRename := os.Rename(FilenameTheClaPem, pemBackupFile)
	defer func() {
		assert.NoError(t, os.Remove(FilenameTheClaPem))
		if errRename == nil {
			assert.NoError(t, os.Rename(pemBackupFile, FilenameTheClaPem), "error renaming pem file in test")
		}
	}()
	SetupTestPemFile(t)

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
	assert.EqualError(t, err, forcedError.Error())
	assert.Equal(t, "Author:  Email:  Commit SHA: ", res)
}

func TestHandlePullRequestPullRequestsAddLabelsToIssueError(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(EnvGhAppId)
	defer func() {
		resetEnvVariable(t, EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(EnvGhAppId, "-1"))

	// move pem file if it exists
	pemBackupFile := FilenameTheClaPem + "_orig"
	errRename := os.Rename(FilenameTheClaPem, pemBackupFile)
	defer func() {
		assert.NoError(t, os.Remove(FilenameTheClaPem))
		if errRename == nil {
			assert.NoError(t, os.Rename(pemBackupFile, FilenameTheClaPem), "error renaming pem file in test")
		}
	}()
	SetupTestPemFile(t)

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

func TestHandlePullRequestBadGH_APP_ID(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(EnvGhAppId)
	defer func() {
		resetEnvVariable(t, EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(EnvGhAppId, "nonNumericGHAppID"))

	prEvent := webhook.PullRequestPayload{}
	res, err := handlePullRequest(setupMockContextLogger(), prEvent)
	assert.EqualError(t, err, `strconv.Atoi: parsing "nonNumericGHAppID": invalid syntax`)
	assert.Equal(t, "", res)
}

func TestHandlePullRequestMissingPemFile(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(EnvGhAppId)
	defer func() {
		resetEnvVariable(t, EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(EnvGhAppId, "-1"))

	// move pem file if it exists
	pemBackupFile := FilenameTheClaPem + "_orig"
	errRename := os.Rename(FilenameTheClaPem, pemBackupFile)
	defer func() {
		if errRename == nil {
			assert.NoError(t, os.Rename(pemBackupFile, FilenameTheClaPem), "error renaming pem file in test")
		}
	}()

	prEvent := webhook.PullRequestPayload{}
	res, err := handlePullRequest(setupMockContextLogger(), prEvent)
	assert.EqualError(t, err, "could not read private key: open the-cla.pem: no such file or directory")
	assert.Equal(t, "", res)
}

func TestHandlePullRequestPullRequestsListCommitsError(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(EnvGhAppId)
	defer func() {
		resetEnvVariable(t, EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(EnvGhAppId, "-1"))

	// move pem file if it exists
	pemBackupFile := FilenameTheClaPem + "_orig"
	errRename := os.Rename(FilenameTheClaPem, pemBackupFile)
	defer func() {
		assert.NoError(t, os.Remove(FilenameTheClaPem))
		if errRename == nil {
			assert.NoError(t, os.Rename(pemBackupFile, FilenameTheClaPem), "error renaming pem file in test")
		}
	}()
	SetupTestPemFile(t)

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
	origGHAppIDEnvVar := os.Getenv(EnvGhAppId)
	defer func() {
		resetEnvVariable(t, EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(EnvGhAppId, "-1"))

	// move pem file if it exists
	pemBackupFile := FilenameTheClaPem + "_orig"
	errRename := os.Rename(FilenameTheClaPem, pemBackupFile)
	defer func() {
		assert.NoError(t, os.Remove(FilenameTheClaPem))
		if errRename == nil {
			assert.NoError(t, os.Rename(pemBackupFile, FilenameTheClaPem), "error renaming pem file in test")
		}
	}()
	SetupTestPemFile(t)

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
