package db

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"go.uber.org/zap/zaptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/go-github/v42/github"
	"github.com/sonatype-nexus-community/the-cla/types"
	"github.com/stretchr/testify/assert"
)

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

// convertSqlToDbMockExpect takes a "real" sql string and adds escape characters as needed to produce a
// regex matching string for use with database mock expect calls.
func convertSqlToDbMockExpect(realSql string) string {
	reDollarSign := regexp.MustCompile(`(\$)`)
	sqlMatch := reDollarSign.ReplaceAll([]byte(realSql), []byte(`\$`))

	reLeftParen := regexp.MustCompile(`(\()`)
	sqlMatch = reLeftParen.ReplaceAll(sqlMatch, []byte(`\(`))

	reRightParen := regexp.MustCompile(`(\))`)
	sqlMatch = reRightParen.ReplaceAll(sqlMatch, []byte(`\)`))

	reStar := regexp.MustCompile(`(\*)`)
	sqlMatch = reStar.ReplaceAll(sqlMatch, []byte(`\*`))
	return string(sqlMatch)
}

func TestConvertSqlToDbMockExpect(t *testing.T) {
	// sanity check all the cases we've found so far
	assert.Equal(t, `\$\(\)\*`, convertSqlToDbMockExpect(`$()*`))
}

func setupMockDB(t *testing.T) (mock sqlmock.Sqlmock, mockDbIf *ClaDB, closeDbFunc func()) {
	db, mock := newMockDb(t)
	closeDbFunc = func() {
		_ = db.Close()
	}
	mockDbIf = New(db, zaptest.NewLogger(t))
	return
}

func TestInsertSignatureError(t *testing.T) {
	mock, db, closeDbFunc := setupMockDB(t)
	defer closeDbFunc()

	user := types.UserSignature{}
	forcedError := fmt.Errorf("forced SQL insert error")
	mock.ExpectExec(convertSqlToDbMockExpect(sqlInsertSignature)).
		WithArgs(user.User.Login, user.User.Email, user.User.GivenName, AnyTime{}, user.CLAVersion).
		WillReturnError(forcedError).
		WillReturnResult(sqlmock.NewErrorResult(forcedError))

	assert.Error(t, db.InsertSignature(&user), forcedError.Error())
}

func TestProcessSignClaSigned(t *testing.T) {
	mock, db, closeDbFunc := setupMockDB(t)
	defer closeDbFunc()

	user := types.UserSignature{
		User:       types.User{Login: "myUserId", Email: "myEmail", GivenName: "myGivenName"},
		CLAVersion: mockCLAVersion,
	}

	forcedError := fmt.Errorf("forced SQL insert error")
	mock.ExpectExec(convertSqlToDbMockExpect(sqlInsertSignature)).
		WithArgs(user.User.Login, user.User.Email, user.User.GivenName, AnyTime{}, user.CLAVersion).
		WillReturnResult(sqlmock.NewErrorResult(forcedError))

	assert.NoError(t, db.InsertSignature(&user))
}

// exclude parent 'db' directory for tests
const testMigrateSourceURL = "file://migrations"

func TestMigrateDBErrorPostgresWithInstance(t *testing.T) {
	_, db, closeDbFunc := setupMockDB(t)
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
	mock.ExpectExec(convertSqlToDbMockExpect(`SELECT pg_advisory_lock($1)`)).
		WithArgs(args...).
		//WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery(convertSqlToDbMockExpect(`SELECT COUNT(1) FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2 LIMIT 1`)).
		WithArgs("theDatabaseSchema", "schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"theCount"}).AddRow(0))

	mock.ExpectExec(convertSqlToDbMockExpect(`CREATE TABLE IF NOT EXISTS "theDatabaseSchema"."schema_migrations" (version bigint not null primary key, dirty boolean not null)`)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec(convertSqlToDbMockExpect(`SELECT pg_advisory_unlock($1)`)).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(0, 0))
	return
}

func TestMigrateDBErrorMigrateUp(t *testing.T) {
	mock, db, closeDbFunc := setupMockDB(t)
	defer closeDbFunc()

	args := setupMockPostgresWithInstance(mock)

	assert.EqualError(t, db.MigrateDB(testMigrateSourceURL), fmt.Sprintf("try lock failed in line 0: SELECT pg_advisory_lock($1) (details: all expectations were already fulfilled, call to ExecQuery 'SELECT pg_advisory_lock($1)' with args [{Name: Ordinal:1 Value:%s}] was not expected)", args[0]))
}

func TestMigrateDB(t *testing.T) {
	mock, db, closeDbFunc := setupMockDB(t)
	defer closeDbFunc()

	args := setupMockPostgresWithInstance(mock)

	// mocks for migrate.Up()
	mock.ExpectExec(convertSqlToDbMockExpect(`SELECT pg_advisory_lock($1)`)).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery(`SELECT version, dirty FROM "theDatabaseSchema"."schema_migrations" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"version", "dirty"}).FromCSVString("-1,false"))

	mock.ExpectBegin()
	mock.ExpectExec(`TRUNCATE "theDatabaseSchema"."schema_migrations"`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(convertSqlToDbMockExpect(`INSERT INTO "theDatabaseSchema"."schema_migrations" (version, dirty) VALUES ($1, $2)`)).
		WithArgs(1, true).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	mock.ExpectExec(`BEGIN; CREATE EXTENSION pgcrypto; CREATE TABLE signatures*`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectExec(`TRUNCATE "theDatabaseSchema"."schema_migrations"`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(convertSqlToDbMockExpect(`INSERT INTO "theDatabaseSchema"."schema_migrations" (version, dirty) VALUES ($1, $2)`)).
		WithArgs(1, false).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	mock.ExpectExec(convertSqlToDbMockExpect(`SELECT pg_advisory_unlock($1)`)).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(0, 0))

	assert.NoError(t, db.MigrateDB(testMigrateSourceURL))
}

func TestHasAuthorSignedTheClaQueryError(t *testing.T) {
	mock, db, closeDbFunc := setupMockDB(t)
	defer closeDbFunc()

	forcedError := fmt.Errorf("forced SQL query error")
	mock.ExpectQuery(convertSqlToDbMockExpect(sqlSelectUserSignature)).
		WillReturnError(forcedError)

	hasSigned, err := db.HasAuthorSignedTheCla("", "")
	assert.EqualError(t, err, forcedError.Error())
	assert.False(t, hasSigned)
}

const mockCLAVersion = "myClaVersion"

func TestHasAuthorSignedTheClaReadRowError(t *testing.T) {
	mock, db, closeDbFunc := setupMockDB(t)
	defer closeDbFunc()

	loginName := "myLoginName"
	mock.ExpectQuery(convertSqlToDbMockExpect(sqlSelectUserSignature)).
		WithArgs(loginName, mockCLAVersion).
		WillReturnRows(sqlmock.NewRows([]string{"LoginName", "Email", "GivenName", "SignedAt", "ClaVersion"}).
			FromCSVString(`myLoginName,myEmail,myGivenName,INVALID_TIME_VALUE_TO_CAUSE_ROW_READ_ERROR,` + mockCLAVersion))

	hasSigned, err := db.HasAuthorSignedTheCla(loginName, mockCLAVersion)
	assert.EqualError(t, err, "sql: Scan error on column index 3, name \"SignedAt\": unsupported Scan, storing driver.Value type []uint8 into type *time.Time")
	assert.True(t, hasSigned)
}

func TestHasAuthorSignedTheClaTrue(t *testing.T) {
	mock, db, closeDbFunc := setupMockDB(t)
	defer closeDbFunc()

	loginName := "myLoginName"
	rs := sqlmock.NewRows([]string{"LoginName", "Email", "GivenName", "SignedAt", "ClaVersion"})
	now := time.Now()
	rs.AddRow(loginName, "myEmail", "myGivenName", now, "myClaVersion")
	mock.ExpectQuery(convertSqlToDbMockExpect(sqlSelectUserSignature)).
		WithArgs(loginName, mockCLAVersion).
		WillReturnRows(rs)

	committer := github.User{}
	committer.Login = &loginName
	hasSigned, err := db.HasAuthorSignedTheCla(loginName, mockCLAVersion)
	assert.NoError(t, err)
	assert.True(t, hasSigned)
}
