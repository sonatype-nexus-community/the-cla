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
// +build go1.16

package github

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v64/github"
	"github.com/sonatype-nexus-community/the-cla/db"
	"github.com/sonatype-nexus-community/the-cla/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/stretchr/testify/assert"
	webhook "gopkg.in/go-playground/webhooks.v5/github"
)

func TestCreateLabelIfNotExists_GetLabelError(t *testing.T) {
	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	forcedError := fmt.Errorf("forced GetLabel error")
	GHImpl = &GHInterfaceMock{
		IssuesMock: IssuesMock{
			mockGetLabelError: forcedError,
			MockGetLabelResponse: &github.Response{
				Response: &http.Response{},
			},
		},
	}

	client := GHImpl.NewClient(nil)

	label, err := _createRepoLabelIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", "", "", "")
	assert.EqualError(t, err, forcedError.Error())
	assert.Nil(t, label)
}

func TestCreateLabelIfNotExists_LabelExists(t *testing.T) {
	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	labelName := "we already got one"
	existingLabel := &github.Label{Name: &labelName}
	GHImpl = &GHInterfaceMock{
		IssuesMock: IssuesMock{
			mockGetLabel: existingLabel,
			MockGetLabelResponse: &github.Response{
				Response: &http.Response{},
			},
		},
	}

	client := GHImpl.NewClient(nil)

	label, err := _createRepoLabelIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", "", "", "")
	assert.NoError(t, err)
	assert.Equal(t, label, existingLabel)
}

func TestCreateLabelIfNotExists_CreateError(t *testing.T) {
	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	forcedError := fmt.Errorf("forced CreateLabel error")
	GHImpl = &GHInterfaceMock{IssuesMock: IssuesMock{
		MockGetLabelResponse: &github.Response{
			Response: &http.Response{StatusCode: http.StatusNotFound},
		},
		mockCreateLabelError: forcedError},
	}
	client := GHImpl.NewClient(nil)

	label, err := _createRepoLabelIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", "", "", "")
	assert.EqualError(t, err, forcedError.Error())
	assert.Nil(t, label)
}

func TestCreateLabelIfNotExists(t *testing.T) {
	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	labelName := labelNameCLANotSigned
	labelColor := "fa3a3a"
	labelDescription := "The CLA is not signed"
	labelToCreate := &github.Label{Name: &labelName, Color: &labelColor, Description: &labelDescription}
	GHImpl = &GHInterfaceMock{IssuesMock: IssuesMock{
		MockGetLabelResponse: &github.Response{
			Response: &http.Response{StatusCode: http.StatusNotFound},
		},
		mockCreateLabel: labelToCreate},
	}

	client := GHImpl.NewClient(nil)

	label, err := _createRepoLabelIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", "", "", "")
	assert.NoError(t, err)
	assert.Equal(t, label, labelToCreate)
}

func TestAddLabelToIssueIfNotExists_ListLabelsByIssueError(t *testing.T) {
	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	forcedError := fmt.Errorf("forced ListLabelsByIssue error")
	GHImpl = &GHInterfaceMock{IssuesMock: IssuesMock{mockListLabelsByIssueError: forcedError}}

	client := GHImpl.NewClient(nil)

	label, err := _addLabelToIssueIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", 0, "")
	assert.EqualError(t, err, forcedError.Error())
	assert.Nil(t, label)
}

func TestAddLabelToIssueIfNotExists_LabelAlreadyExists(t *testing.T) {
	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	labelName := labelNameCLANotSigned
	existingLabel := &github.Label{Name: &labelName}
	existingLabelList := []*github.Label{existingLabel}
	GHImpl = &GHInterfaceMock{
		IssuesMock: IssuesMock{mockListLabelsByIssue: existingLabelList},
	}

	client := GHImpl.NewClient(nil)

	label, err := _addLabelToIssueIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", 0, labelName)
	assert.NoError(t, err)
	assert.Equal(t, existingLabel, label)
}

func Test_AddLabelToIssueIfNotExists_AddLabelError(t *testing.T) {
	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	forcedError := fmt.Errorf("forced AddLabels error")
	GHImpl = &GHInterfaceMock{
		IssuesMock: IssuesMock{mockAddLabelsError: forcedError},
	}

	client := GHImpl.NewClient(nil)

	label, err := _addLabelToIssueIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", 0, "")
	assert.EqualError(t, err, forcedError.Error())
	assert.Nil(t, label)
}

func Test_AddLabelToIssueIfNotExists(t *testing.T) {
	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	labelName := labelNameCLANotSigned
	labelColor := "fa3a3a"
	labelDescription := "The CLA is not signed"
	labelToCreate := &github.Label{Name: &labelName, Color: &labelColor, Description: &labelDescription}
	GHImpl = &GHInterfaceMock{IssuesMock: IssuesMock{mockAddLabels: []*github.Label{labelToCreate}}}

	client := GHImpl.NewClient(nil)

	label, err := _addLabelToIssueIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", 0, labelNameCLANotSigned)
	assert.NoError(t, err)
	// real gitHub API returns different result, but does not matter to us now
	assert.Nil(t, label)
}

type mockCLADb struct {
	t                               *testing.T
	assertParameters                bool
	insertSignatureUserSignature    *types.UserSignature
	insertSignatureError            error
	hasAuthorSignedLogin            string
	hasAuthorSignedCLAVersion       string
	hasAuthorSignedResult           bool
	hasAuthorSignedSignature        *types.UserSignature
	hasAuthorSignedError            error
	migrateDBSourceURL              string
	migrateDBSourceError            error
	storeUsersNeedingToSignEvalInfo *types.EvaluationInfo
	// storeUsersNeedingToSignTime    time.Time
	storeUsersNeedingToSignCLAErr error
	getPRsForUserUser             *types.UserSignature
	getPRsForUserEvalInfo         []types.EvaluationInfo
	getPRsForUserError            error
	removePRsUsersSigned          []types.UserSignature
	removePRsEvalInfo             *types.EvaluationInfo
	removePRsError                error
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

func (m mockCLADb) HasAuthorSignedTheCla(login, claVersion string) (bool, *types.UserSignature, error) {
	if m.assertParameters {
		assert.Equal(m.t, m.hasAuthorSignedLogin, login)
		assert.Equal(m.t, m.hasAuthorSignedCLAVersion, claVersion)
	}
	return m.hasAuthorSignedResult, m.hasAuthorSignedSignature, m.hasAuthorSignedError
}

func (m mockCLADb) MigrateDB(migrateSourceURL string) error {
	if m.assertParameters {
		assert.Equal(m.t, m.migrateDBSourceURL, migrateSourceURL)
	}
	return m.migrateDBSourceError
}

func (m mockCLADb) StorePRAuthorsMissingSignature(evalInfo *types.EvaluationInfo, checkedAt time.Time) error {
	if m.assertParameters {
		assert.Equal(m.t, m.storeUsersNeedingToSignEvalInfo, evalInfo)
		assert.NotNil(m.t, checkedAt) // not going nuts over time check here
	}
	return m.storeUsersNeedingToSignCLAErr
}

func (m mockCLADb) GetPRsForUser(user *types.UserSignature) ([]types.EvaluationInfo, error) {
	if m.assertParameters {
		assert.Equal(m.t, m.getPRsForUserUser, user)
	}
	return m.getPRsForUserEvalInfo, m.getPRsForUserError
}

func (m mockCLADb) RemovePRsForUsers(usersSigned []types.UserSignature, evalInfo *types.EvaluationInfo) error {
	if m.assertParameters {
		assert.Equal(m.t, m.removePRsUsersSigned, usersSigned)
		assert.Equal(m.t, m.removePRsEvalInfo, evalInfo)
	}
	return m.removePRsError
}

func TestHandlePullRequestIsCollaboratorError(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(EnvGhAppId)
	defer func() {
		resetEnvVariable(t, EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(EnvGhAppId, "-1"))

	resetPemFileImpl := SetupTestPemFile(t)
	defer resetPemFileImpl()

	resetGHJWTImpl := SetupMockGHJWT()
	defer resetGHJWTImpl()

	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	mockAuthorLogin := "myAuthorLogin"
	mockRepositoryCommits := []*github.RepositoryCommit{{Author: &github.User{Login: &mockAuthorLogin}}}
	forcedError := fmt.Errorf("forced IsCollaborator error")
	GHImpl = &GHInterfaceMock{
		PullRequestsMock: PullRequestsMock{mockRepositoryCommits: mockRepositoryCommits},
		RepositoriesMock: RepositoriesMock{
			isCollaboratorErr: forcedError,
		},
	}

	prEvent := webhook.PullRequestPayload{}

	mockDB, logger := setupMockDB(t, true)
	mockDB.hasAuthorSignedLogin = mockAuthorLogin

	err := HandlePullRequest(logger, mockDB, prEvent, 0, "")
	assert.EqualError(t, err, forcedError.Error())
}

func TestHandlePullRequestIsCollaboratorTrueCollaborator(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(EnvGhAppId)
	defer func() {
		resetEnvVariable(t, EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(EnvGhAppId, "-1"))

	resetPemFileImpl := SetupTestPemFile(t)
	defer resetPemFileImpl()

	resetGHJWTImpl := SetupMockGHJWT()
	defer resetGHJWTImpl()

	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	mockAuthorLogin := "myAuthorLogin"
	mockRepositoryCommits := []*github.RepositoryCommit{{Author: &github.User{Login: &mockAuthorLogin}}}
	GHImpl = &GHInterfaceMock{
		PullRequestsMock: PullRequestsMock{mockRepositoryCommits: mockRepositoryCommits},
		RepositoriesMock: RepositoriesMock{
			isCollaboratorResult: true,
		},
		IssuesMock: IssuesMock{
			//mockGetLabel: &github.Label{},
			MockGetLabelResponse: &github.Response{
				Response: &http.Response{},
			},
			MockRemoveLabelResponse: &github.Response{
				Response: &http.Response{},
			},
		},
	}

	prEvent := webhook.PullRequestPayload{}

	mockDB, logger := setupMockDB(t, true)
	mockDB.hasAuthorSignedLogin = mockAuthorLogin
	mockDB.removePRsEvalInfo = &types.EvaluationInfo{}

	err := HandlePullRequest(logger, mockDB, prEvent, 0, "")
	assert.NoError(t, err)
}

func TestHandlePullRequestCreateLabelError(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(EnvGhAppId)
	defer func() {
		resetEnvVariable(t, EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(EnvGhAppId, "-1"))

	resetPemFileImpl := SetupTestPemFile(t)
	defer resetPemFileImpl()

	resetGHJWTImpl := SetupMockGHJWT()
	defer resetGHJWTImpl()

	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	mockAuthorLogin := "myAuthorLogin"
	mockRepositoryCommits := []*github.RepositoryCommit{{Author: &github.User{Login: &mockAuthorLogin}}}
	forcedError := fmt.Errorf("forced CreateLabel error")
	GHImpl = &GHInterfaceMock{
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

func TestHandlePullRequestAddLabelsToIssueError(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(EnvGhAppId)
	defer func() {
		resetEnvVariable(t, EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(EnvGhAppId, "-1"))

	resetPemFileImpl := SetupTestPemFile(t)
	defer resetPemFileImpl()

	resetGHJWTImpl := SetupMockGHJWT()
	defer resetGHJWTImpl()

	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	mockAuthorLogin := "myAuthorLogin"
	mockRepositoryCommits := []*github.RepositoryCommit{{Author: &github.User{Login: &mockAuthorLogin}}}
	forcedError := fmt.Errorf("forced AddLabelsToIssue error")
	GHImpl = &GHInterfaceMock{
		PullRequestsMock: PullRequestsMock{mockRepositoryCommits: mockRepositoryCommits},
		IssuesMock: IssuesMock{
			mockGetLabel:       &github.Label{},
			mockAddLabelsError: forcedError,
			MockGetLabelResponse: &github.Response{
				Response: &http.Response{},
			},
		},
	}

	prEvent := webhook.PullRequestPayload{}

	mockDB, logger := setupMockDB(t, true)
	mockDB.hasAuthorSignedLogin = mockAuthorLogin

	err := HandlePullRequest(logger, mockDB, prEvent, 0, "")
	assert.EqualError(t, err, forcedError.Error())
}

func TestHandlePullRequestGetAppError(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(EnvGhAppId)
	defer func() {
		resetEnvVariable(t, EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(EnvGhAppId, "-1"))

	resetPemFileImpl := SetupTestPemFile(t)
	defer resetPemFileImpl()

	resetGHJWTImpl := SetupMockGHJWT()
	defer resetGHJWTImpl()
	//forcedError := fmt.Errorf("forced Get App error")
	GHJWTImpl = &GHJWTMock{
		AppsMock: AppsMock{
			mockInstallation: &github.Installation{
				AppSlug: &appSlug,
			},
			//mockAppErr: forcedError,
			mockAppResp: &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}},
		},
	}

	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	mockAuthorLogin := "myAuthorLogin"
	mockRepositoryCommits := []*github.RepositoryCommit{{Author: &github.User{Login: &mockAuthorLogin}}}
	GHImpl = &GHInterfaceMock{
		PullRequestsMock: PullRequestsMock{mockRepositoryCommits: mockRepositoryCommits},
		IssuesMock: IssuesMock{
			mockGetLabel: &github.Label{},
			MockGetLabelResponse: &github.Response{
				Response: &http.Response{},
			},
			MockRemoveLabelResponse: &github.Response{
				Response: &http.Response{StatusCode: http.StatusNotFound},
			},
		},
	}

	prEvent := webhook.PullRequestPayload{}

	mockDB, logger := setupMockDB(t, true)
	mockDB.hasAuthorSignedLogin = mockAuthorLogin
	mockDB.storeUsersNeedingToSignEvalInfo = &types.EvaluationInfo{
		UserSignatures: []types.UserSignature{
			{
				User: types.User{
					Login: mockAuthorLogin,
				},
			},
		},
	}

	err := HandlePullRequest(logger, mockDB, prEvent, 0, "")
	//assert.EqualError(t, err, forcedError.Error())
	assert.True(t, strings.HasPrefix(err.Error(), "it done broke: "))
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

func TestHandlePullRequestListCommitsError(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(EnvGhAppId)
	defer func() {
		resetEnvVariable(t, EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(EnvGhAppId, "-1"))

	resetPemFileImpl := SetupTestPemFile(t)
	defer resetPemFileImpl()

	resetGHJWTImpl := SetupMockGHJWT()
	defer resetGHJWTImpl()

	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	forcedError := fmt.Errorf("forced ListCommits error")
	GHImpl = &GHInterfaceMock{
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

func TestHandlePullRequestListCommits(t *testing.T) {
	origGHAppIDEnvVar := os.Getenv(EnvGhAppId)
	defer func() {
		resetEnvVariable(t, EnvGhAppId, origGHAppIDEnvVar)
	}()
	assert.NoError(t, os.Setenv(EnvGhAppId, "-1"))

	resetPemFileImpl := SetupTestPemFile(t)
	defer resetPemFileImpl()

	resetGHJWTImpl := SetupMockGHJWT()
	defer resetGHJWTImpl()
	mockExternalUrl := "fakeExternalURL"
	GHJWTImpl = &GHJWTMock{
		AppsMock: AppsMock{
			mockInstallation: &github.Installation{
				AppSlug: &appSlug,
			},
			//mockAppErr: forcedError,
			mockAppResp: &github.Response{Response: &http.Response{StatusCode: http.StatusOK}},
			mockApp:     &github.App{ExternalURL: &mockExternalUrl},
		},
	}

	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
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
	GHImpl = &GHInterfaceMock{
		PullRequestsMock: PullRequestsMock{
			mockRepositoryCommits: mockRepositoryCommits,
		},
		IssuesMock: IssuesMock{
			mockGetLabel: &github.Label{},
			MockGetLabelResponse: &github.Response{
				Response: &http.Response{},
			},
			MockRemoveLabelResponse: &github.Response{
				Response: &http.Response{},
			},
		},
	}

	prEvent := webhook.PullRequestPayload{}

	mockDB, logger := setupMockDB(t, false)
	err := HandlePullRequest(logger, mockDB, prEvent, 0, "")
	assert.NoError(t, err)
}

func Test_removeLabelFromIssueIfExists_Removed(t *testing.T) {
	issuesMock := &IssuesMock{
		MockRemoveLabelResponse: &github.Response{
			Response: &http.Response{},
		},
	}
	assert.NoError(t, _removeLabelFromIssueIfApplied(zaptest.NewLogger(t), issuesMock, "", "", 0, ""))
}

func Test_removeLabelFromIssueIfExists_NotExistsIsIgnored(t *testing.T) {
	issuesMock := &IssuesMock{
		MockRemoveLabelResponse: &github.Response{
			Response: &http.Response{StatusCode: http.StatusNotFound},
		},
	}
	assert.NoError(t, _removeLabelFromIssueIfApplied(zaptest.NewLogger(t), issuesMock, "", "", 0, ""))
}

func Test_removeLabelFromIssueIfExists_Error(t *testing.T) {
	forcedError := fmt.Errorf("forced label remove error")
	issuesMock := &IssuesMock{
		MockRemoveLabelResponse: &github.Response{
			Response: &http.Response{},
		},
		mockRemoveLabelError: forcedError,
	}
	assert.EqualError(t, _removeLabelFromIssueIfApplied(zaptest.NewLogger(t), issuesMock, "", "", 0, ""),
		forcedError.Error())
}

func TestReviewPriorPRsGetPRsDBError(t *testing.T) {
	mockDB, logger := setupMockDB(t, true)

	login := "myUserLogin"
	claVersion := "myCLAVersion"
	now := time.Now()
	user := types.UserSignature{
		User: types.User{
			Login: login,
		},
		CLAVersion: claVersion,
		TimeSigned: now,
	}

	mockDB.getPRsForUserUser = &user
	forcedError := fmt.Errorf("forced db error")
	mockDB.getPRsForUserError = forcedError

	assert.EqualError(t, ReviewPriorPRs(logger, mockDB, &user), forcedError.Error())
}

func TestReviewPriorPRsEvaluatePRError(t *testing.T) {
	mockDB, logger := setupMockDB(t, true)

	login := "myUserLogin"
	claVersion := "myCLAVersion"
	now := time.Now()
	user := types.UserSignature{
		User: types.User{
			Login: login,
		},
		CLAVersion: claVersion,
		TimeSigned: now,
	}

	mockDB.getPRsForUserUser = &user
	mockDB.getPRsForUserEvalInfo = []types.EvaluationInfo{
		{
			UserSignatures: []types.UserSignature{user},
		},
	}
	mockDB.hasAuthorSignedLogin = login
	mockDB.hasAuthorSignedCLAVersion = claVersion

	resetPemFileImpl := SetupTestPemFile(t)
	defer resetPemFileImpl()

	resetGHJWTImpl := SetupMockGHJWT()
	defer resetGHJWTImpl()

	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	forcedError := fmt.Errorf("forced create status error")
	GHImpl = &GHInterfaceMock{
		RepositoriesMock: RepositoriesMock{
			t:                 t,
			createStatusError: forcedError,
		},
	}

	assert.EqualError(t, ReviewPriorPRs(logger, mockDB, &user), forcedError.Error())
}

func TestReviewPriorPRsEvalSuccess(t *testing.T) {
	mockDB, logger := setupMockDB(t, true)

	login := "myUserLogin"
	claVersion := "myCLAVersion"
	now := time.Now()
	user := types.UserSignature{
		User: types.User{
			Login: login,
		},
		CLAVersion: claVersion,
		TimeSigned: now,
	}

	mockDB.getPRsForUserUser = &user
	mockDB.getPRsForUserEvalInfo = []types.EvaluationInfo{
		{
			UserSignatures: []types.UserSignature{user},
		},
	}
	mockDB.hasAuthorSignedLogin = login
	mockDB.hasAuthorSignedCLAVersion = claVersion
	mockDB.hasAuthorSignedResult = true
	mockDB.hasAuthorSignedSignature = &user

	mockDB.removePRsEvalInfo = &mockDB.getPRsForUserEvalInfo[0]

	resetPemFileImpl := SetupTestPemFile(t)
	defer resetPemFileImpl()

	resetGHJWTImpl := SetupMockGHJWT()
	defer resetGHJWTImpl()

	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	//forcedError := fmt.Errorf("forced create status error")
	GHImpl = &GHInterfaceMock{
		IssuesMock: IssuesMock{
			MockGetLabelResponse: &github.Response{
				Response: &http.Response{},
			},
			MockRemoveLabelResponse: &github.Response{
				Response: &http.Response{},
			},
		},
	}

	assert.NoError(t, ReviewPriorPRs(logger, mockDB, &user))
}

func TestReviewPriorPRs(t *testing.T) {
	mockDB, logger := setupMockDB(t, true)

	login := "myUserLogin"
	claVersion := "myCLAVersion"
	now := time.Now()
	user := types.UserSignature{
		User: types.User{
			Login: login,
		},
		CLAVersion: claVersion,
		TimeSigned: now,
	}

	mockDB.getPRsForUserUser = &user

	resetPemFileImpl := SetupTestPemFile(t)
	defer resetPemFileImpl()

	assert.NoError(t, ReviewPriorPRs(logger, mockDB, &user))
}
