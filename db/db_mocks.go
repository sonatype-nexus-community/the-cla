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

package db

import (
	"database/sql/driver"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	"regexp"
	"testing"
	"time"
)

// SetupMockDB should always be followed by a call to the closeDbFunc, like so:
//
//	mock, db, closeDbFunc := SetupMockDB(t)
//	defer closeDbFunc()
func SetupMockDB(t *testing.T) (mock sqlmock.Sqlmock, mockDbIf *ClaDB, closeDbFunc func()) {
	db, mock, err := sqlmock.New()
	if err != nil {
		assert.NoError(t, err)
	}
	closeDbFunc = func() {
		_ = db.Close()
	}
	mockDbIf = New(db, zaptest.NewLogger(t))
	return
}

type AnyTime struct{}

// Match satisfies sqlmock.Argument interface
func (a AnyTime) Match(v driver.Value) bool {
	_, ok := v.(time.Time)
	return ok
}

// ConvertSqlToDbMockExpect takes a "real" sql string and adds escape characters as needed to produce a
// regex matching string for use with database mock expect calls.
func ConvertSqlToDbMockExpect(realSql string) string {
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
