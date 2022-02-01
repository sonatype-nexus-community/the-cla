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
	"database/sql"
	"fmt"
	"go.uber.org/zap"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/sonatype-nexus-community/the-cla/types"
)

const sqlInsertSignature = `INSERT INTO signatures
		(LoginName, Email, GivenName, SignedAt, ClaVersion)
		VALUES ($1, $2, $3, $4, $5)`

const msgTemplateErrInsertSignatureDuplicate = "insert error. did user previously sign the cla? user: %+v, error: %+v"

type IClaDB interface {
	InsertSignature(u *types.UserSignature) error
	HasAuthorSignedTheCla(login, claVersion string) (bool, *types.UserSignature, error)
	StorePRAuthorsMissingSignature(owner, repo string, pullRequestID int, usersNeedingToSignCLA []types.UserSignature, checkedAt time.Time) error
	MigrateDB(migrateSourceURL string) error
}

type ClaDB struct {
	db     *sql.DB
	logger *zap.Logger
}

// Roll that beautiful bean footage
var _ IClaDB = (*ClaDB)(nil)

func New(db *sql.DB, logger *zap.Logger) *ClaDB {
	return &ClaDB{db: db, logger: logger}
}

func (p *ClaDB) InsertSignature(user *types.UserSignature) error {
	result, err := p.db.Exec(sqlInsertSignature, user.User.Login, user.User.Email, user.User.GivenName, user.TimeSigned, user.CLAVersion)
	if err != nil {
		return fmt.Errorf(msgTemplateErrInsertSignatureDuplicate, user.User, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		return fmt.Errorf(msgTemplateErrInsertSignatureDuplicate, user.User, err)
	}
	return nil
}

const SqlSelectUserSignature = `SELECT 
		LoginName, Email, GivenName, SignedAt, ClaVersion 
		FROM signatures		
		WHERE LoginName = $1
		AND ClaVersion = $2`

func (p *ClaDB) HasAuthorSignedTheCla(login, claVersion string) (isSigned bool, foundUserSignature *types.UserSignature, err error) {
	p.logger.Debug("did author sign the CLA",
		zap.String("login", login),
		zap.String("claVersion", claVersion),
	)

	rows, err := p.db.Query(SqlSelectUserSignature, login, claVersion)
	if err != nil {
		return
	}

	for rows.Next() {
		isSigned = true
		foundUserSignature = &types.UserSignature{}
		err = rows.Scan(
			&foundUserSignature.User.Login,
			&foundUserSignature.User.Email,
			&foundUserSignature.User.GivenName,
			&foundUserSignature.TimeSigned,
			&foundUserSignature.CLAVersion,
		)
		if err != nil {
			return
		}
		p.logger.Debug("found author signature",
			zap.String("login", foundUserSignature.User.Login),
			zap.Time("timeSigned", foundUserSignature.TimeSigned),
			zap.String("claVersion", foundUserSignature.CLAVersion),
		)
	}

	return
}

func (p *ClaDB) MigrateDB(migrateSourceURL string) (err error) {
	driver, err := postgres.WithInstance(p.db, &postgres.Config{})
	if err != nil {
		return
	}

	m, err := migrate.NewWithDatabaseInstance(
		migrateSourceURL,
		"postgres", driver)
	if err != nil {
		return
	}

	if err = m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			// we can ignore (and clear) the "no change" error
			err = nil
		}
	}
	return
}

const sqlInsertAuthorMissing = `INSERT INTO unsigned_pr
		(repo_owner, repo_name, pr_number, login_name, ClaVersion, CheckedAt)
		VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING`

const msgTemplateErrInsertAuthorMissing = "insert error tracking missing author CLA. user: %+v, error: %+v"

func (p *ClaDB) StorePRAuthorsMissingSignature(owner, repo string, pullRequestID int, usersNeedingToSignCLA []types.UserSignature, checkedAt time.Time) (err error) {
	for _, missingAuthor := range usersNeedingToSignCLA {
		_, err = p.db.Exec(sqlInsertAuthorMissing, owner, repo, pullRequestID, missingAuthor.User.Login, missingAuthor.CLAVersion, checkedAt)
		if err != nil {
			return fmt.Errorf(msgTemplateErrInsertAuthorMissing, missingAuthor.User.Login, err)
		}
		// We ignore lack of insert (rowsAffected) for cases where a PR is closed and reopened - ON CONFLICT DO NOTHING
	}
	return
}
