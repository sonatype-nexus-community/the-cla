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

package github

import (
	"context"
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

func TestWithJustGHImpl(t *testing.T) {
	// Setup Code before tests
	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()

	t.Run("TestCreateLabelIfNotExists", func(t *testing.T) {
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
	})

	t.Run("TestCreateLabelIfNotExists_CreateError", func(t *testing.T) {
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
	})

	t.Run("TestCreateLabelIfNotExists_GetLabelError", func(t *testing.T) {
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
	})

	t.Run("TestCreateLabelIfNotExists_LabelExists", func(t *testing.T) {
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
	})

	t.Run("TestAddLabelToIssueIfNotExists", func(t *testing.T) {
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
	})

	t.Run("TestAddLabelToIssueIfNotExists_ListLabelsByIssueError", func(t *testing.T) {
		forcedError := fmt.Errorf("forced ListLabelsByIssue error")
		GHImpl = &GHInterfaceMock{IssuesMock: IssuesMock{mockListLabelsByIssueError: forcedError}}

		client := GHImpl.NewClient(nil)

		label, err := _addLabelToIssueIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", 0, "")
		assert.EqualError(t, err, forcedError.Error())
		assert.Nil(t, label)
	})

	t.Run("TestAddLabelToIssueIfNotExists_AddLabelError", func(t *testing.T) {
		forcedError := fmt.Errorf("forced AddLabels error")
		GHImpl = &GHInterfaceMock{
			IssuesMock: IssuesMock{mockAddLabelsError: forcedError},
		}

		client := GHImpl.NewClient(nil)

		label, err := _addLabelToIssueIfNotExists(zaptest.NewLogger(t), client.Issues, "", "", 0, "")
		assert.EqualError(t, err, forcedError.Error())
		assert.Nil(t, label)
	})

	t.Run("TestAddLabelToIssueIfNotExists_LabelAlreadyExists", func(t *testing.T) {
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
	})
}

func TestWithFullEnvironment(t *testing.T) {
	// Setup Code before tests
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

	t.Run("TestHandlePullRequestCreateLabelError", func(t *testing.T) {
		authors := []string{"myAuthorLogin"}
		forcedError := fmt.Errorf("forced CreateLabel error")
		issuesMock := IssuesMock{
			MockGetLabelResponse: &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}},
			mockCreateLabelError: forcedError,
		}
		GHImpl = getGHMock(
			getMockRepositoryCommits(authors, true),
			&issuesMock,
			nil,
		)

		prEvent := webhook.PullRequestPayload{}

		mockDB, logger := setupMockDB(t, true)
		mockDB.hasAuthorSignedLogin = authors[0]

		err := HandlePullRequest(logger, mockDB, prEvent, 0, "")
		assert.EqualError(t, err, forcedError.Error())
	})

	t.Run("TestHandlePullRequestAddLabelsToIssueError", func(t *testing.T) {
		authors := []string{"myAuthorLogin2"}
		forcedError := fmt.Errorf("forced AddLabelsToIssue error")
		issuesMock := IssuesMock{
			mockGetLabel:       &github.Label{},
			mockAddLabelsError: forcedError,
			MockGetLabelResponse: &github.Response{
				Response: &http.Response{},
			},
		}
		GHImpl = getGHMock(
			getMockRepositoryCommits(authors, true),
			&issuesMock,
			nil,
		)
		mockDB, logger := setupMockDB(t, true)
		mockDB.hasAuthorSignedLogin = authors[0]

		err := HandlePullRequest(logger, mockDB, webhook.PullRequestPayload{}, 0, "")
		assert.EqualError(t, err, forcedError.Error())
	})

	t.Run("TestHandlePullRequestIsCollaboratorError", func(t *testing.T) {
		authors := []string{"myAuthorLogin3"}
		forcedError := fmt.Errorf("forced IsCollaborator error")
		repositoriesMock := RepositoriesMock{
			isCollaboratorErr: forcedError,
		}
		GHImpl = getGHMock(
			getMockRepositoryCommits(authors, true),
			nil,
			&repositoriesMock,
		)

		mockDB, logger := setupMockDB(t, true)
		mockDB.hasAuthorSignedLogin = authors[0]

		err := HandlePullRequest(logger, mockDB, webhook.PullRequestPayload{}, 0, "")
		assert.EqualError(t, err, forcedError.Error())
	})

	t.Run("TestHandlePullRequestIsCollaboratorTrueCollaborator", func(t *testing.T) {
		authors := []string{"anAuthor4"}
		issuesMock := IssuesMock{
			MockGetLabelResponse: &github.Response{
				Response: &http.Response{},
			},
			MockRemoveLabelResponse: &github.Response{
				Response: &http.Response{},
			},
		}
		repositoriesMock := RepositoriesMock{
			isCollaboratorResult: true,
		}
		GHImpl = getGHMock(getMockRepositoryCommits(authors, true), &issuesMock, &repositoriesMock)

		mockDB, logger := setupMockDB(t, true)
		mockDB.hasAuthorSignedLogin = authors[0]
		mockDB.removePRsEvalInfo = &types.EvaluationInfo{}

		err := HandlePullRequest(logger, mockDB, webhook.PullRequestPayload{}, 0, "")
		assert.NoError(t, err)
	})

	t.Run("TestHandlePullRequestListCommitsError", func(t *testing.T) {
		forcedError := fmt.Errorf("forced ListCommits error")
		GHImpl = &GHInterfaceMock{
			RepositoriesMock: *setupMockRepositoriesService(t, []bool{false}, nil),
			PullRequestsMock: PullRequestsMock{
				mockListCommitsError: forcedError,
			},
		}

		// GHImpl = getGHMock(nil, nil, setupMockRepositoriesService(t, false))

		mockDB, logger := setupMockDB(t, true)
		err := HandlePullRequest(logger, mockDB, webhook.PullRequestPayload{}, 0, "")
		assert.EqualError(t, err, forcedError.Error())
	})

	t.Run("TestHandlePullRequestListCommits", func(t *testing.T) {
		mockExternalUrl := "fakeExternalURL"
		GHJWTImpl = &GHJWTMock{
			AppsMock: AppsMock{
				mockInstallation: &github.Installation{
					AppSlug: &MockAppSlug,
				},
				mockAppResp: &github.Response{Response: &http.Response{StatusCode: http.StatusOK}},
				mockApp:     &github.App{ExternalURL: &mockExternalUrl},
			},
		}

		origGithubImpl := GHImpl
		defer func() {
			GHImpl = origGithubImpl
		}()
		authors := []string{"john", "doe"}
		GHImpl = getGHMock(getMockRepositoryCommits(authors, true), nil, nil)
		mockDB, logger := setupMockDB(t, false)
		err := HandlePullRequest(logger, mockDB, webhook.PullRequestPayload{}, 0, "")
		assert.NoError(t, err)
	})

	t.Run("TestHandlePullRequestListCommitsUnsignedCommit", func(t *testing.T) {
		authors := []string{"john", "doe"}

		repositoriesMock := *setupMockRepositoriesService(t,
			[]bool{true, true},
			[]any{
				[]context.Context{context.Background(), context.Background()}, // ctx
				[]string{"", ""}, // owner
				[]string{"", ""}, // repo
				[]string{"", ""}, // sha
				[]*github.RepoStatus{
					{
						State:       github.String("pending"),
						Description: github.String("Paul Botsco, the CLA verifier is running"),
						Context:     &MockAppSlug,
					},
					{
						State:       github.String("failure"),
						Description: github.String("One or more commits haven't met our Quality requirements."),
						Context:     &MockAppSlug,
					},
				},
			})

		GHImpl = getGHMock(getMockRepositoryCommits(authors, false), nil, &repositoriesMock)
		mockDB, logger := setupMockDB(t, false)
		err := HandlePullRequest(logger, mockDB, webhook.PullRequestPayload{}, 0, "")
		assert.NoError(t, err)
	})
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
				AppSlug: &MockAppSlug,
			},
			//mockAppErr: forcedError,
			mockAppResp: &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}},
		},
	}

	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	authors := []string{"myAuthorLogin"}
	mockRepositoryCommits := getMockRepositoryCommits(authors, true)
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
	mockDB.hasAuthorSignedLogin = authors[0]
	mockDB.storeUsersNeedingToSignEvalInfo = &types.EvaluationInfo{
		UserSignatures: []types.UserSignature{
			{
				User: types.User{
					Login: authors[0],
					Email: "myAuthorLogin@somewhere.tld",
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

func TestHandlePullRequestListCommitsNoAuthor(t *testing.T) {
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
				AppSlug: &MockAppSlug,
			},
			mockAppResp: &github.Response{Response: &http.Response{StatusCode: http.StatusOK}},
			mockApp:     &github.App{ExternalURL: &mockExternalUrl},
		},
	}

	origGithubImpl := GHImpl
	defer func() {
		GHImpl = origGithubImpl
	}()
	mockRepositoryCommits := []*github.RepositoryCommit{
		{
			Commit: &github.Commit{
				Author: &github.CommitAuthor{
					Name:  github.String("someuser"),
					Email: github.String("someuser@some.where.tld"),
					// Date:  github.Timestamp.Local(),
				},
			},
			SHA:     github.String("johnSHA"),
			HTMLURL: github.String("https://github.com"),
		},
	}
	GHImpl = &GHInterfaceMock{
		PullRequestsMock: PullRequestsMock{
			mockRepositoryCommits: mockRepositoryCommits,
		},
		IssuesMock: IssuesMock{
			t: t,
			assertParamsCreateComment: assertParams{
				assertParameters: []bool{true},
				expectedParameters: []any{
					[]context.Context{context.Background()}, // ctx
					[]string{""},                            // owner
					[]string{""},                            // repo
					[]int{0},                                // number
					[]*github.IssueComment{
						{Body: github.String(
							`Thanks for the contribution. Unfortunately some of your commits don't meet our standards. All commits must be signed and have author information set.
		
The commits to review are:
		
- <a href="https://github.com">johnSHA</a> - missing author :cop:
- <a href="https://github.com">johnSHA</a> - unsigned commit :key:
`,
						)},
					}, // comment
				},
			},
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
			createStatusError: []error{forcedError},
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

func getSignedSignatureVerification() *github.SignatureVerification {
	return &github.SignatureVerification{
		Verified:  github.Bool(true),
		Reason:    github.String("valid"),
		Signature: github.String("some-signature"),
		Payload:   github.String("some-payload"),
	}
}

func getUnsignedSignatureVerification() *github.SignatureVerification {
	return &github.SignatureVerification{
		Verified:  github.Bool(false),
		Reason:    github.String("unsigned"),
		Signature: nil,
		Payload:   nil,
	}
}

func getMockRepositoryCommits(mockAuthorLogins []string, signed bool) []*github.RepositoryCommit {
	mockRepositoryCommits := make([]*github.RepositoryCommit, 0)

	for _, author := range mockAuthorLogins {
		email := fmt.Sprintf("%s@somewhere.tld", author)
		var signatureVerification = getSignedSignatureVerification()
		if signed == false {
			signatureVerification = getUnsignedSignatureVerification()
		}

		commit := github.RepositoryCommit{
			Author: &github.User{
				Login: &author,
				Email: &email,
			},
			Commit: &github.Commit{
				Verification: signatureVerification,
			},
			HTMLURL: github.String("https://github.com"),
			SHA:     github.String(author + "SHA"),
		}
		mockRepositoryCommits = append(mockRepositoryCommits, &commit)
	}
	return mockRepositoryCommits
}

func getGHMock(repoCommits []*github.RepositoryCommit, issuesMock *IssuesMock, repositoriesMock *RepositoriesMock) *GHInterfaceMock {
	mock := &GHInterfaceMock{
		PullRequestsMock: PullRequestsMock{
			mockRepositoryCommits: repoCommits,
		},
	}

	if issuesMock != nil {
		mock.IssuesMock = *issuesMock
	} else {
		mock.IssuesMock = IssuesMock{
			mockGetLabel: &github.Label{},
			MockGetLabelResponse: &github.Response{
				Response: &http.Response{},
			},
			MockRemoveLabelResponse: &github.Response{
				Response: &http.Response{},
			},
		}
	}

	if repositoriesMock != nil {
		mock.RepositoriesMock = *repositoriesMock
	}

	return mock
}
