package github

import (
	"fmt"
	"github.com/sonatype-nexus-community/the-cla/db"
	"github.com/sonatype-nexus-community/the-cla/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"net/http"
	"os"
	"testing"

	"github.com/google/go-github/v42/github"
	"github.com/stretchr/testify/assert"
	webhook "gopkg.in/go-playground/webhooks.v5/github"
)

func TestCreateLabelIfNotExists_GetLabelError(t *testing.T) {
	origGithubImpl := GithubImpl
	defer func() {
		GithubImpl = origGithubImpl
	}()
	forcedError := fmt.Errorf("forced GetLabel error")
	GithubImpl = &GHInterfaceMock{
		IssuesMock: IssuesMock{mockGetLabelError: forcedError},
	}

	client := GithubImpl.NewClient(nil)

	label, err := _createRepoLabelIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", "", "", "")
	assert.EqualError(t, err, forcedError.Error())
	assert.Nil(t, label)
}

func TestCreateLabelIfNotExists_LabelExists(t *testing.T) {
	origGithubImpl := GithubImpl
	defer func() {
		GithubImpl = origGithubImpl
	}()
	labelName := "we already got one"
	existingLabel := &github.Label{Name: &labelName}
	GithubImpl = &GHInterfaceMock{
		IssuesMock: IssuesMock{mockGetLabel: existingLabel},
	}

	client := GithubImpl.NewClient(nil)

	label, err := _createRepoLabelIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", "", "", "")
	assert.NoError(t, err)
	assert.Equal(t, label, existingLabel)
}

func TestCreateLabelIfNotExists_CreateError(t *testing.T) {
	origGithubImpl := GithubImpl
	defer func() {
		GithubImpl = origGithubImpl
	}()
	forcedError := fmt.Errorf("forced CreateLabel error")
	GithubImpl = &GHInterfaceMock{IssuesMock: IssuesMock{
		MockGetLabelResponse: &github.Response{
			Response: &http.Response{StatusCode: http.StatusNotFound},
		},
		mockCreateLabelError: forcedError},
	}
	client := GithubImpl.NewClient(nil)

	label, err := _createRepoLabelIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", "", "", "")
	assert.EqualError(t, err, forcedError.Error())
	assert.Nil(t, label)
}

func TestCreateLabelIfNotExists(t *testing.T) {
	origGithubImpl := GithubImpl
	defer func() {
		GithubImpl = origGithubImpl
	}()
	labelName := labelNameCLANotSigned
	labelColor := "fa3a3a"
	labelDescription := "The CLA is not signed"
	labelToCreate := &github.Label{Name: &labelName, Color: &labelColor, Description: &labelDescription}
	GithubImpl = &GHInterfaceMock{IssuesMock: IssuesMock{
		MockGetLabelResponse: &github.Response{
			Response: &http.Response{StatusCode: http.StatusNotFound},
		},
		mockCreateLabel: labelToCreate},
	}

	client := GithubImpl.NewClient(nil)

	label, err := _createRepoLabelIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", "", "", "")
	assert.NoError(t, err)
	assert.Equal(t, label, labelToCreate)
}

func TestAddLabelToIssueIfNotExists_ListLabelsByIssueError(t *testing.T) {
	origGithubImpl := GithubImpl
	defer func() {
		GithubImpl = origGithubImpl
	}()
	forcedError := fmt.Errorf("forced ListLabelsByIssue error")
	GithubImpl = &GHInterfaceMock{IssuesMock: IssuesMock{mockListLabelsByIssueError: forcedError}}

	client := GithubImpl.NewClient(nil)

	label, err := _addLabelToIssueIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", 0, "")
	assert.EqualError(t, err, forcedError.Error())
	assert.Nil(t, label)
}

func TestAddLabelToIssueIfNotExists_LabelAlreadyExists(t *testing.T) {
	origGithubImpl := GithubImpl
	defer func() {
		GithubImpl = origGithubImpl
	}()
	labelName := labelNameCLANotSigned
	existingLabel := &github.Label{Name: &labelName}
	existingLabelList := []*github.Label{existingLabel}
	GithubImpl = &GHInterfaceMock{
		IssuesMock: IssuesMock{mockListLabelsByIssue: existingLabelList},
	}

	client := GithubImpl.NewClient(nil)

	label, err := _addLabelToIssueIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", 0, labelName)
	assert.NoError(t, err)
	assert.Equal(t, existingLabel, label)
}

func Test_AddLabelToIssueIfNotExists_AddLabelError(t *testing.T) {
	origGithubImpl := GithubImpl
	defer func() {
		GithubImpl = origGithubImpl
	}()
	forcedError := fmt.Errorf("forced AddLabels error")
	GithubImpl = &GHInterfaceMock{
		IssuesMock: IssuesMock{mockAddLabelsError: forcedError},
	}

	client := GithubImpl.NewClient(nil)

	label, err := _addLabelToIssueIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", 0, "")
	assert.EqualError(t, err, forcedError.Error())
	assert.Nil(t, label)
}

func Test_AddLabelToIssueIfNotExists(t *testing.T) {
	origGithubImpl := GithubImpl
	defer func() {
		GithubImpl = origGithubImpl
	}()
	labelName := labelNameCLANotSigned
	labelColor := "fa3a3a"
	labelDescription := "The CLA is not signed"
	labelToCreate := &github.Label{Name: &labelName, Color: &labelColor, Description: &labelDescription}
	GithubImpl = &GHInterfaceMock{IssuesMock: IssuesMock{mockAddLabels: []*github.Label{labelToCreate}}}

	client := GithubImpl.NewClient(nil)

	label, err := _addLabelToIssueIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", 0, labelNameCLANotSigned)
	assert.NoError(t, err)
	// real gitHub API returns different result, but does not matter to us now
	assert.Nil(t, label)
}

type mockCLADb struct {
	t                            *testing.T
	assertParameters             bool
	insertSignatureUserSignature *types.UserSignature
	insertSignatureError         error
	hasAuthorSignedLogin         string
	hasAuthorSignedCLAVersion    string
	hasAuthorSignedResult        bool
	hasAuthorSignedError         error
	migrateDBSourceURL           string
	migrateDBSourceError         error
}

var _ db.IClaDB = (*mockCLADb)(nil)

func setupMockDB(t *testing.T, assertParameters bool) (mock *mockCLADb, logger *zap.Logger) {
	mock = &mockCLADb{
		t:                t,
		assertParameters: assertParameters,
	}
	return mock, zaptest.NewLogger(t)
}
func (m mockCLADb) InsertSignature(u *types.UserSignature) error {
	if m.assertParameters {
		assert.Equal(m.t, m.insertSignatureUserSignature, u)
	}
	return m.insertSignatureError
}

func (m mockCLADb) HasAuthorSignedTheCla(login, claVersion string) (bool, error) {
	if m.assertParameters {
		assert.Equal(m.t, m.hasAuthorSignedLogin, login)
		assert.Equal(m.t, m.hasAuthorSignedCLAVersion, claVersion)
	}
	return m.hasAuthorSignedResult, m.hasAuthorSignedError
}

func (m mockCLADb) MigrateDB(migrateSourceURL string) error {
	if m.assertParameters {
		assert.Equal(m.t, m.migrateDBSourceURL, migrateSourceURL)
	}
	return m.migrateDBSourceError
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

	origGithubImpl := GithubImpl
	defer func() {
		GithubImpl = origGithubImpl
	}()
	mockAuthorLogin := "myAuthorLogin"
	mockRepositoryCommits := []*github.RepositoryCommit{{Author: &github.User{Login: &mockAuthorLogin}}}
	forcedError := fmt.Errorf("forced CreateLabel error")
	GithubImpl = &GHInterfaceMock{
		PullRequestsMock: PullRequestsMock{mockRepositoryCommits: mockRepositoryCommits},
		IssuesMock: IssuesMock{
			MockGetLabelResponse: &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}},
			mockCreateLabelError: forcedError,
		},
	}

	prEvent := webhook.PullRequestPayload{}

	mockDB, logger := setupMockDB(t, true)
	mockDB.hasAuthorSignedLogin = mockAuthorLogin

	err := HandlePullRequest(logger, mockDB, prEvent, 0, "")
	assert.EqualError(t, err, forcedError.Error())
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

	origGithubImpl := GithubImpl
	defer func() {
		GithubImpl = origGithubImpl
	}()
	mockAuthorLogin := "myAuthorLogin"
	mockRepositoryCommits := []*github.RepositoryCommit{{Author: &github.User{Login: &mockAuthorLogin}}}
	forcedError := fmt.Errorf("forced AddLabelsToIssue error")
	GithubImpl = &GHInterfaceMock{
		PullRequestsMock: PullRequestsMock{mockRepositoryCommits: mockRepositoryCommits},
		IssuesMock: IssuesMock{
			mockGetLabel:       &github.Label{},
			mockAddLabelsError: forcedError,
		},
	}

	prEvent := webhook.PullRequestPayload{}

	mockDB, logger := setupMockDB(t, true)
	mockDB.hasAuthorSignedLogin = mockAuthorLogin

	err := HandlePullRequest(logger, mockDB, prEvent, 0, "")
	assert.EqualError(t, err, forcedError.Error())
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
	mockDB, logger := setupMockDB(t, true)
	err := HandlePullRequest(logger, mockDB, prEvent, 0, "")
	assert.EqualError(t, err, "could not read private key: open the-cla.pem: no such file or directory")
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

	origGithubImpl := GithubImpl
	defer func() {
		GithubImpl = origGithubImpl
	}()
	forcedError := fmt.Errorf("forced ListCommits error")
	GithubImpl = &GHInterfaceMock{
		RepositoriesMock: *setupMockRepositoriesService(t, false),
		PullRequestsMock: PullRequestsMock{
			mockListCommitsError: forcedError,
		},
	}

	prEvent := webhook.PullRequestPayload{}
	mockDB, logger := setupMockDB(t, true)
	err := HandlePullRequest(logger, mockDB, prEvent, 0, "")
	assert.EqualError(t, err, forcedError.Error())
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

	origGithubImpl := GithubImpl
	defer func() {
		GithubImpl = origGithubImpl
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
	GithubImpl = &GHInterfaceMock{
		PullRequestsMock: PullRequestsMock{
			mockRepositoryCommits: mockRepositoryCommits,
		},
		IssuesMock: IssuesMock{
			mockGetLabel: &github.Label{},
		},
	}

	prEvent := webhook.PullRequestPayload{}

	mockDB, logger := setupMockDB(t, false)
	err := HandlePullRequest(logger, mockDB, prEvent, 0, "")
	assert.NoError(t, err)
}
