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

package db

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/go-github/v42/github"
	"github.com/sonatype-nexus-community/the-cla/types"
	"github.com/stretchr/testify/assert"
)

func TestConvertSqlToDbMockExpect(t *testing.T) {
	// sanity check all the cases we've found so far
	assert.Equal(t, `\$\(\)\*`, ConvertSqlToDbMockExpect(`$()*`))
}

func TestInsertSignatureError(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	user := types.UserSignature{}
	forcedError := errors.New("forced SQL insert error")
	mock.ExpectExec(ConvertSqlToDbMockExpect(sqlInsertSignature)).
		WithArgs(user.User.Login, user.User.Email, user.User.GivenName, AnyTime{}, user.CLAVersion).
		WillReturnError(forcedError).
		WillReturnResult(sqlmock.NewErrorResult(forcedError))

	assert.Error(t, db.InsertSignature(&user), forcedError.Error())
}

func TestInsertSignatureErrorDuplicateSignature(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	user := types.UserSignature{
		User:       types.User{Login: "myUserId", Email: "myEmail", GivenName: "myGivenName"},
		CLAVersion: mockCLAVersion,
	}

	forcedError := errors.New("forced SQL insert error")
	mock.ExpectExec(ConvertSqlToDbMockExpect(sqlInsertSignature)).
		WithArgs(user.User.Login, user.User.Email, user.User.GivenName, AnyTime{}, user.CLAVersion).
		WillReturnResult(sqlmock.NewErrorResult(forcedError))

	assert.Error(t, db.InsertSignature(&user), forcedError.Error())
}

// exclude parent 'db' directory for tests
const testMigrateSourceURL = "file://migrations"

func TestMigrateDBErrorPostgresWithInstance(t *testing.T) {
	_, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	assert.EqualError(t, db.MigrateDB(testMigrateSourceURL), "all expectations were already fulfilled, call to Query 'SELECT CURRENT_DATABASE()' with args [] was not expected in line 0: SELECT CURRENT_DATABASE()")
}

func setupMockPostgresWithInstance(mock sqlmock.Sqlmock) (args []driver.Value) {
	// mocks for 'postgres.WithInstance()'
	mock.ExpectQuery(`SELECT CURRENT_DATABASE()`).
		WillReturnRows(sqlmock.NewRows([]string{"col1"}).FromCSVString("theDatabaseName"))
	mock.ExpectQuery(`SELECT CURRENT_SCHEMA()`).
		WillReturnRows(sqlmock.NewRows([]string{"col1"}).FromCSVString("theDatabaseSchema"))

	//args = []driver.Value{"1014225327"}
	args = []driver.Value{"1560208929"}
	mock.ExpectExec(ConvertSqlToDbMockExpect(`SELECT pg_advisory_lock($1)`)).
		WithArgs(args...).
		//WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery(ConvertSqlToDbMockExpect(`SELECT COUNT(1) FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2 LIMIT 1`)).
		WithArgs("theDatabaseSchema", "schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"theCount"}).AddRow(0))

	mock.ExpectExec(ConvertSqlToDbMockExpect(`CREATE TABLE IF NOT EXISTS "theDatabaseSchema"."schema_migrations" (version bigint not null primary key, dirty boolean not null)`)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec(ConvertSqlToDbMockExpect(`SELECT pg_advisory_unlock($1)`)).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(0, 0))
	return
}

func TestMigrateDBErrorMigrateUp(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	args := setupMockPostgresWithInstance(mock)

	assert.EqualError(t, db.MigrateDB(testMigrateSourceURL), fmt.Sprintf("try lock failed in line 0: SELECT pg_advisory_lock($1) (details: all expectations were already fulfilled, call to ExecQuery 'SELECT pg_advisory_lock($1)' with args [{Name: Ordinal:1 Value:%s}] was not expected)", args[0]))
}

func TestHasAuthorSignedTheClaQueryError(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	forcedError := errors.New("forced SQL query error")
	mock.ExpectQuery(ConvertSqlToDbMockExpect(SqlSelectUserSignature)).
		WillReturnError(forcedError)

	hasSigned, _, err := db.HasAuthorSignedTheCla("", "")
	assert.EqualError(t, err, forcedError.Error())
	assert.False(t, hasSigned)
}

const mockCLAVersion = "myClaVersion"
const mockCLATextUrl = "https://my.url/cla.text"
const mockCLAText = "This is a CLA"

func TestHasAuthorSignedTheClaReadRowError(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	loginName := "myLoginName"
	mock.ExpectQuery(ConvertSqlToDbMockExpect(SqlSelectUserSignature)).
		WithArgs(loginName, mockCLAVersion).
		WillReturnRows(sqlmock.NewRows([]string{"LoginName", "Email", "GivenName", "SignedAt", "ClaVersion", "ClaTextUrl", "ClaText"}).
			FromCSVString(`myLoginName,myEmail,myGivenName,INVALID_TIME_VALUE_TO_CAUSE_ROW_READ_ERROR,` + mockCLAVersion + `,` + mockCLATextUrl + `,` + mockCLAText))

	hasSigned, foundSignature, err := db.HasAuthorSignedTheCla(loginName, mockCLAVersion)
	assert.EqualError(t, err, "sql: Scan error on column index 3, name \"SignedAt\": unsupported Scan, storing driver.Value type []uint8 into type *time.Time")
	assert.True(t, hasSigned)
	assert.NotNil(t, foundSignature)
}

func TestHasAuthorSignedTheClaTrue(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	rs := sqlmock.NewRows([]string{"LoginName", "Email", "GivenName", "SignedAt", "ClaVersion", "ClaTextUrl", "ClaText"})
	loginName := "myLoginName"
	email := "myEmail"
	givenName := "myGivenName"
	now := time.Now()
	claVersion := "myCLAVersion"
	rs.AddRow(loginName, email, givenName, now, claVersion, mockCLATextUrl, mockCLAText)
	mock.ExpectQuery(ConvertSqlToDbMockExpect(SqlSelectUserSignature)).
		WithArgs(loginName, mockCLAVersion).
		WillReturnRows(rs)

	committer := github.User{}
	committer.Login = &loginName
	hasSigned, foundSignature, err := db.HasAuthorSignedTheCla(loginName, mockCLAVersion)
	assert.NoError(t, err)
	assert.True(t, hasSigned)
	assert.Equal(t, loginName, foundSignature.User.Login)
	assert.Equal(t, email, foundSignature.User.Email)
	assert.Equal(t, givenName, foundSignature.User.GivenName)
	assert.Equal(t, now, foundSignature.TimeSigned)
	assert.Equal(t, claVersion, foundSignature.CLAVersion)
	assert.Equal(t, mockCLATextUrl, foundSignature.CLATextUrl)
	assert.Equal(t, mockCLAText, foundSignature.CLAText)
}

func TestStorePRAuthorsMissingSignatureInsertError(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	repoOwner := "myRepoOwner"
	repoName := "myRepoName"
	sha := "mySha"
	pullRequestID := int64(-1)
	appId := int64(-2)
	installId := int64(-3)
	loginName := "myLoginName"
	email := "myEmail"
	givenName := "myGivenName"
	users := []types.UserSignature{
		{
			User: types.User{
				Login:     loginName,
				Email:     email,
				GivenName: givenName,
			},
			CLAVersion: mockCLAVersion,
		},
	}
	evalInfo := types.EvaluationInfo{
		RepoOwner:      repoOwner,
		RepoName:       repoName,
		Sha:            sha,
		PRNumber:       pullRequestID,
		AppId:          appId,
		InstallId:      installId,
		UserSignatures: users,
	}

	forcedError := errors.New("forced insert error")
	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlInsertPRMissing)).
		WithArgs(evalInfo.RepoOwner, evalInfo.RepoName, evalInfo.Sha, evalInfo.PRNumber, evalInfo.AppId, evalInfo.InstallId).
		WillReturnError(forcedError)

	assert.EqualError(t, db.StorePRAuthorsMissingSignature(&evalInfo, time.Now()),
		fmt.Sprintf(msgTemplateErrInsertPRMissing, repoName, pullRequestID, forcedError))
}

func TestStorePRAuthorsMissingSignatureInsertErrorRowExists(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	repoOwner := "myRepoOwner"
	repoName := "myRepoName"
	sha := "mySha"
	pullRequestID := int64(-1)
	appId := int64(-2)
	installId := int64(-3)
	loginName := "myLoginName"
	email := "myEmail"
	givenName := "myGivenName"
	users := []types.UserSignature{
		{
			User: types.User{
				Login:     loginName,
				Email:     email,
				GivenName: givenName,
			},
			CLAVersion: mockCLAVersion,
		},
	}
	evalInfo := types.EvaluationInfo{
		RepoOwner:      repoOwner,
		RepoName:       repoName,
		Sha:            sha,
		PRNumber:       pullRequestID,
		AppId:          appId,
		InstallId:      installId,
		UserSignatures: users,
	}

	forcedRowExistsError := errors.New(errMsgInsertedRowExists)
	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlInsertPRMissing)).
		WithArgs(evalInfo.RepoOwner, evalInfo.RepoName, evalInfo.Sha, evalInfo.PRNumber, evalInfo.AppId, evalInfo.InstallId).
		WillReturnError(forcedRowExistsError)

	forcedError := errors.New("forced insert error")
	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlSelectPR)).
		WithArgs(evalInfo.RepoName, evalInfo.PRNumber).
		WillReturnError(forcedError)

	assert.EqualError(t, db.StorePRAuthorsMissingSignature(&evalInfo, time.Now()),
		fmt.Sprintf(msgTemplateErrInsertPRMissing, repoName, pullRequestID, forcedError))
}

func TestStorePRAuthorsMissingSignatureQueryParentPRError(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	repoOwner := "myRepoOwner"
	repoName := "myRepoName"
	sha := "mySha"
	pullRequestID := int64(-1)
	appId := int64(-2)
	installId := int64(-3)
	loginName := "myLoginName"
	email := "myEmail"
	givenName := "myGivenName"
	users := []types.UserSignature{
		{
			User: types.User{
				Login:     loginName,
				Email:     email,
				GivenName: givenName,
			},
			CLAVersion: mockCLAVersion,
		},
	}
	evalInfo := types.EvaluationInfo{
		RepoOwner:      repoOwner,
		RepoName:       repoName,
		Sha:            sha,
		PRNumber:       pullRequestID,
		AppId:          appId,
		InstallId:      installId,
		UserSignatures: users,
	}

	forcedRowExistsError := errors.New(errMsgInsertedRowExists)
	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlInsertPRMissing)).
		WithArgs(evalInfo.RepoOwner, evalInfo.RepoName, evalInfo.Sha, evalInfo.PRNumber, evalInfo.AppId, evalInfo.InstallId).
		WillReturnError(forcedRowExistsError)

	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlSelectPR)).
		WithArgs(evalInfo.RepoName, evalInfo.PRNumber).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	assert.EqualError(t, db.StorePRAuthorsMissingSignature(&evalInfo, time.Now()),
		fmt.Sprintf(msgTemplateErrInsertPRMissing, repoName, pullRequestID, errors.New(errMsgInsertedRowExists)))
}

func TestStorePRAuthorsMissingSignatureInsertEmptyParentUUID(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	repoOwner := "myRepoOwner"
	repoName := "myRepoName"
	sha := "mySha"
	pullRequestID := int64(-1)
	appId := int64(-2)
	installId := int64(-3)
	loginName := "myLoginName"
	email := "myEmail"
	givenName := "myGivenName"
	users := []types.UserSignature{
		{
			User: types.User{
				Login:     loginName,
				Email:     email,
				GivenName: givenName,
			},
			CLAVersion: mockCLAVersion,
		},
	}
	evalInfo := types.EvaluationInfo{
		RepoOwner:      repoOwner,
		RepoName:       repoName,
		Sha:            sha,
		PRNumber:       pullRequestID,
		AppId:          appId,
		InstallId:      installId,
		UserSignatures: users,
	}

	forcedRowExistsError := errors.New(errMsgInsertedRowExists)
	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlInsertPRMissing)).
		WithArgs(evalInfo.RepoOwner, evalInfo.RepoName, evalInfo.Sha, evalInfo.PRNumber, evalInfo.AppId, evalInfo.InstallId).
		WillReturnError(forcedRowExistsError)

	parentUUID := ""
	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlSelectPR)).
		WithArgs(evalInfo.RepoName, evalInfo.PRNumber).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(parentUUID))

	assert.EqualError(t, db.StorePRAuthorsMissingSignature(&evalInfo, time.Now()),
		fmt.Sprintf(msgTemplateErrInsertPRMissing, repoName, pullRequestID, errors.New("empty parentUUID")))
}

func TestStorePRAuthorsMissingSignatureParentInsertError(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	repoOwner := "myRepoOwner"
	repoName := "myRepoName"
	sha := "mySha"
	pullRequestID := int64(-1)
	appId := int64(-2)
	installId := int64(-3)
	loginName := "myLoginName"
	email := "myEmail"
	givenName := "myGivenName"
	users := []types.UserSignature{
		{
			User: types.User{
				Login:     loginName,
				Email:     email,
				GivenName: givenName,
			},
			CLAVersion: mockCLAVersion,
		},
	}
	evalInfo := types.EvaluationInfo{
		RepoOwner:      repoOwner,
		RepoName:       repoName,
		Sha:            sha,
		PRNumber:       pullRequestID,
		AppId:          appId,
		InstallId:      installId,
		UserSignatures: users,
	}

	forcedError := errors.New("forced insert error")
	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlInsertPRMissing)).
		WithArgs(evalInfo.RepoOwner, evalInfo.RepoName, evalInfo.Sha, evalInfo.PRNumber, evalInfo.AppId, evalInfo.InstallId).
		WillReturnError(forcedError)

	assert.EqualError(t, db.StorePRAuthorsMissingSignature(&evalInfo, time.Now()),
		fmt.Sprintf(msgTemplateErrInsertPRMissing, repoName, pullRequestID, forcedError))
}

func TestStorePRAuthorsMissingSignatureUserInsertError(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	repoOwner := "myRepoOwner"
	repoName := "myRepoName"
	sha := "mySha"
	pullRequestID := int64(-1)
	appId := int64(-2)
	installId := int64(-3)
	loginName := "myLoginName"
	email := "myEmail"
	givenName := "myGivenName"
	users := []types.UserSignature{
		{
			User: types.User{
				Login:     loginName,
				Email:     email,
				GivenName: givenName,
			},
			CLAVersion: mockCLAVersion,
		},
	}
	evalInfo := types.EvaluationInfo{
		RepoOwner:      repoOwner,
		RepoName:       repoName,
		Sha:            sha,
		PRNumber:       pullRequestID,
		AppId:          appId,
		InstallId:      installId,
		UserSignatures: users,
	}

	parentUUID := "myParentUUID"
	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlInsertPRMissing)).
		WithArgs(evalInfo.RepoOwner, evalInfo.RepoName, evalInfo.Sha, evalInfo.PRNumber, evalInfo.AppId, evalInfo.InstallId).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(parentUUID))

	forcedError := errors.New("forced insert error")
	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlInsertUserMissing)).
		WithArgs(parentUUID, users[0].User.Login, users[0].User.Email, users[0].User.GivenName, users[0].CLAVersion, AnyTime{}).
		WillReturnError(forcedError)

	assert.EqualError(t, db.StorePRAuthorsMissingSignature(&evalInfo, time.Now()),
		fmt.Sprintf(msgTemplateErrInsertAuthorMissing, loginName, forcedError))
}

func TestStorePRAuthorsMissingSignatureUserInsertZero(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	repoOwner := "myRepoOwner"
	repoName := "myRepoName"
	sha := "mySha"
	pullRequestID := int64(-1)
	appId := int64(-2)
	installId := int64(-3)
	loginName := "myLoginName"
	email := "myEmail"
	givenName := "myGivenName"
	users := []types.UserSignature{
		{
			User: types.User{
				Login:     loginName,
				Email:     email,
				GivenName: givenName,
			},
			CLAVersion: mockCLAVersion,
		},
	}
	evalInfo := types.EvaluationInfo{
		RepoOwner:      repoOwner,
		RepoName:       repoName,
		Sha:            sha,
		PRNumber:       pullRequestID,
		AppId:          appId,
		InstallId:      installId,
		UserSignatures: users,
	}

	parentUUID := "myParentUUID"
	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlInsertPRMissing)).
		WithArgs(evalInfo.RepoOwner, evalInfo.RepoName, evalInfo.Sha, evalInfo.PRNumber, evalInfo.AppId, evalInfo.InstallId).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(parentUUID))

	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlInsertUserMissing)).
		WithArgs(parentUUID, users[0].User.Login, users[0].User.Email, users[0].User.GivenName, users[0].CLAVersion, AnyTime{}).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	assert.NoError(t, db.StorePRAuthorsMissingSignature(&evalInfo, time.Now()))
}

func TestStorePRAuthorsMissingSignatureUserInsert(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	repoOwner := "myRepoOwner"
	repoName := "myRepoName"
	sha := "mySha"
	pullRequestID := int64(-1)
	appId := int64(-2)
	installId := int64(-3)
	loginName := "myLoginName"
	email := "myEmail"
	givenName := "myGivenName"
	users := []types.UserSignature{
		{
			User: types.User{
				Login:     loginName,
				Email:     email,
				GivenName: givenName,
			},
			CLAVersion: mockCLAVersion,
		},
	}
	evalInfo := types.EvaluationInfo{
		RepoOwner:      repoOwner,
		RepoName:       repoName,
		Sha:            sha,
		PRNumber:       pullRequestID,
		AppId:          appId,
		InstallId:      installId,
		UserSignatures: users,
	}

	parentUUID := "myParentUUID"
	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlInsertPRMissing)).
		WithArgs(evalInfo.RepoOwner, evalInfo.RepoName, evalInfo.Sha, evalInfo.PRNumber, evalInfo.AppId, evalInfo.InstallId).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(parentUUID))

	authorUUID := "myAuthorUUID"
	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlInsertUserMissing)).
		WithArgs(parentUUID, users[0].User.Login, users[0].User.Email, users[0].User.GivenName, users[0].CLAVersion, AnyTime{}).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(authorUUID))

	assert.NoError(t, db.StorePRAuthorsMissingSignature(&evalInfo, time.Now()))
}

func TestGetPRsForUserSelectPRsError(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	user := types.UserSignature{
		User: types.User{
			Login: "myLogin",
		},
		CLAVersion: "myCLAVersion",
	}

	forcedError := errors.New("forced select PRs error")
	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlSelectPRsForUser)).
		WithArgs(user.User.Login, user.CLAVersion).
		WillReturnError(forcedError)

	evalInfos, err := db.GetPRsForUser(&user)
	assert.EqualError(t, err, forcedError.Error())
	assert.Equal(t, []types.EvaluationInfo(nil), evalInfos)
}

func TestGetPRsForUserZeroRows(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	user := types.UserSignature{
		User: types.User{
			Login: "myLogin",
		},
		CLAVersion: "myCLAVersion",
	}

	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlSelectPRsForUser)).
		WithArgs(user.User.Login, user.CLAVersion).
		WillReturnRows(sqlmock.NewRows(nil))

	evalInfos, err := db.GetPRsForUser(&user)
	assert.NoError(t, err)
	assert.Equal(t, []types.EvaluationInfo(nil), evalInfos)
}

func TestGetPRsForUserScanError(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	user := types.UserSignature{
		User: types.User{
			Login: "myLogin",
		},
		CLAVersion: "myCLAVersion",
	}

	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlSelectPRsForUser)).
		WithArgs(user.User.Login, user.CLAVersion).
		WillReturnRows(sqlmock.NewRows([]string{"tooFewCollumns"}).AddRow("oneValue"))

	evalInfos, err := db.GetPRsForUser(&user)
	assert.EqualError(t, err, "sql: expected 1 destination arguments in Scan, not 7")
	assert.Equal(t, []types.EvaluationInfo(nil), evalInfos)
}

func TestGetPRsForUserTwoRows(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	login := "myLogin"
	claVersion := "myCLAVersion"
	user := types.UserSignature{
		User: types.User{
			Login: login,
		},
		CLAVersion: claVersion,
	}

	mock.ExpectQuery(ConvertSqlToDbMockExpect(sqlSelectPRsForUser)).
		WithArgs(user.User.Login, user.CLAVersion).
		WillReturnRows(sqlmock.NewRows([]string{"1", "2", "3", "4", "5", "6", "7"}).
			AddRow("UnsignedPRID", "RepoOwner", "RepoName", "Sha", -1, -2, -3).
			AddRow("1", "2", "3", "4", "5", "6", "7"),
		)

	evalInfos, err := db.GetPRsForUser(&user)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(evalInfos))
	assert.Equal(t,
		types.EvaluationInfo{
			UnsignedPRID: "UnsignedPRID",
			RepoOwner:    "RepoOwner",
			RepoName:     "RepoName",
			Sha:          "Sha",
			PRNumber:     -1,
			AppId:        -2,
			InstallId:    -3,
		},
		evalInfos[0],
	)
}

func TestRemovePRsForUsersRemoveUserError(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	prUUID := "myPRUUID"
	login := "myLogin"
	claVersion := "myCLAVersion"
	forcedError := errors.New("forced delete unsigned user db error")
	mock.ExpectExec(ConvertSqlToDbMockExpect(sqlDeleteUnsignedUser)).
		WithArgs(prUUID, login, claVersion).
		WillReturnError(forcedError)

	usersSigned := []types.UserSignature{
		{
			User:       types.User{Login: login},
			CLAVersion: claVersion,
		},
	}
	assert.EqualError(t, db.RemovePRsForUsers(usersSigned, &types.EvaluationInfo{UnsignedPRID: prUUID}), forcedError.Error())
}

func TestRemovePRsForUsersCountUsersError(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	prUUID := "myPRUUID"
	login := "myLogin"
	claVersion := "myCLAVersion"
	mock.ExpectExec(ConvertSqlToDbMockExpect(sqlDeleteUnsignedUser)).
		WithArgs(prUUID, login, claVersion).
		WillReturnResult(sqlmock.NewResult(0, 0))

	forcedError := errors.New("forced count unsigned user db error")
	mock.ExpectQuery(ConvertSqlToDbMockExpect(SqlSelectUnsignedUsersForPR)).
		WithArgs(prUUID).
		WillReturnError(forcedError)

	usersSigned := []types.UserSignature{
		{
			User:       types.User{Login: login},
			CLAVersion: claVersion,
		},
	}
	assert.EqualError(t, db.RemovePRsForUsers(usersSigned, &types.EvaluationInfo{UnsignedPRID: prUUID}), forcedError.Error())
}

func TestRemovePRsForUsersRemovePRError(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	prUUID := "myPRUUID"
	login := "myLogin"
	claVersion := "myCLAVersion"
	mock.ExpectExec(ConvertSqlToDbMockExpect(sqlDeleteUnsignedUser)).
		WithArgs(prUUID, login, claVersion).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery(ConvertSqlToDbMockExpect(SqlSelectUnsignedUsersForPR)).
		WithArgs(prUUID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	forcedError := errors.New("forced delete unsigned PR db error")
	mock.ExpectExec(ConvertSqlToDbMockExpect(sqlDeleteUnsignedPR)).
		WithArgs(prUUID).
		WillReturnError(forcedError)

	usersSigned := []types.UserSignature{
		{
			User:       types.User{Login: login},
			CLAVersion: claVersion,
		},
	}
	assert.EqualError(t, db.RemovePRsForUsers(usersSigned, &types.EvaluationInfo{UnsignedPRID: prUUID}), forcedError.Error())
}

func TestRemovePRsForUsersNilUsers(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	prUUID := "myPRUUID"
	mock.ExpectQuery(ConvertSqlToDbMockExpect(SqlSelectUnsignedUsersForPR)).
		WithArgs(prUUID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectExec(ConvertSqlToDbMockExpect(sqlDeleteUnsignedPR)).
		WithArgs(prUUID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	assert.NoError(t, db.RemovePRsForUsers(nil, &types.EvaluationInfo{UnsignedPRID: prUUID}))
}

func TestRemovePRsForUsersZeroUsers(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	prUUID := "myPRUUID"
	mock.ExpectQuery(ConvertSqlToDbMockExpect(SqlSelectUnsignedUsersForPR)).
		WithArgs(prUUID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectExec(ConvertSqlToDbMockExpect(sqlDeleteUnsignedPR)).
		WithArgs(prUUID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	var usersSigned []types.UserSignature

	assert.NoError(t, db.RemovePRsForUsers(usersSigned, &types.EvaluationInfo{UnsignedPRID: prUUID}))
}

func TestRemovePRsForUsersEmptyPRUUID(t *testing.T) {
	_, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	assert.NoError(t, db.RemovePRsForUsers(nil, &types.EvaluationInfo{}))
}
