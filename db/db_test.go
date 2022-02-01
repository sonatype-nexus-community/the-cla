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
//go:build go1.16
// +build go1.16

package db

import (
	"database/sql/driver"
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
	forcedError := fmt.Errorf("forced SQL insert error")
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

	forcedError := fmt.Errorf("forced SQL insert error")
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

	forcedError := fmt.Errorf("forced SQL query error")
	mock.ExpectQuery(ConvertSqlToDbMockExpect(SqlSelectUserSignature)).
		WillReturnError(forcedError)

	hasSigned, _, err := db.HasAuthorSignedTheCla("", "")
	assert.EqualError(t, err, forcedError.Error())
	assert.False(t, hasSigned)
}

const mockCLAVersion = "myClaVersion"

func TestHasAuthorSignedTheClaReadRowError(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	loginName := "myLoginName"
	mock.ExpectQuery(ConvertSqlToDbMockExpect(SqlSelectUserSignature)).
		WithArgs(loginName, mockCLAVersion).
		WillReturnRows(sqlmock.NewRows([]string{"LoginName", "Email", "GivenName", "SignedAt", "ClaVersion"}).
			FromCSVString(`myLoginName,myEmail,myGivenName,INVALID_TIME_VALUE_TO_CAUSE_ROW_READ_ERROR,` + mockCLAVersion))

	hasSigned, foundSignature, err := db.HasAuthorSignedTheCla(loginName, mockCLAVersion)
	assert.EqualError(t, err, "sql: Scan error on column index 3, name \"SignedAt\": unsupported Scan, storing driver.Value type []uint8 into type *time.Time")
	assert.True(t, hasSigned)
	assert.NotNil(t, foundSignature)
}

func TestHasAuthorSignedTheClaTrue(t *testing.T) {
	mock, db, closeDbFunc := SetupMockDB(t)
	defer closeDbFunc()

	rs := sqlmock.NewRows([]string{"LoginName", "Email", "GivenName", "SignedAt", "ClaVersion"})
	loginName := "myLoginName"
	email := "myEmail"
	givenName := "myGivenName"
	now := time.Now()
	claVersion := "myCLAVersion"
	rs.AddRow(loginName, email, givenName, now, claVersion)
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
}
