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
	StorePRAuthorsMissingSignature(evalInfo *types.EvaluationInfo, checkedAt time.Time) error
	GetPRsForUser(*types.UserSignature) ([]types.EvaluationInfo, error)
	RemovePRsForUser(*types.EvaluationInfo) error
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

const sqlInsertPRMissing = `INSERT INTO unsigned_pr
		(RepoOwner, RepoName, sha, PRNumber, AppID, InstallID)
		VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING RETURNING id`
const msgTemplateErrInsertPRMissing = "insert error tracking missing PR CLA. repo: %s, PR: %d, error: %+v"

const errMsgInsertedRowExists = "sql: no rows in result set"
const sqlSelectPR = `SELECT Id from unsigned_pr WHERE RepoName = $1 AND PRNumber = $2`

const sqlInsertUserMissing = `INSERT INTO unsigned_user
		(UnsignedPRID, LoginName, Email, GivenName, ClaVersion, CheckedAt)
		VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING RETURNING id`

const msgTemplateErrInsertAuthorMissing = "insert error tracking missing author CLA. user: %+v, error: %+v"

func (p *ClaDB) StorePRAuthorsMissingSignature(evalInfo *types.EvaluationInfo, checkedAt time.Time) (err error) {
	var parentUUID string
	err = p.db.QueryRow(sqlInsertPRMissing, evalInfo.RepoOwner, evalInfo.RepoName, evalInfo.Sha, evalInfo.PRNumber, evalInfo.AppId, evalInfo.InstallId).
		Scan(&parentUUID)
	if err != nil {
		if errMsgInsertedRowExists == err.Error() {
			p.logger.Info("special case, try to read the UUID of the existing parent",
				zap.String("repoName", evalInfo.RepoName),
				zap.Int64("PRNumber", evalInfo.PRNumber),
			)
			err = p.db.QueryRow(sqlSelectPR, evalInfo.RepoName, evalInfo.PRNumber).Scan(&parentUUID)
			if err != nil {
				return fmt.Errorf(msgTemplateErrInsertPRMissing, evalInfo.RepoName, evalInfo.PRNumber, err)
			}
		} else {
			return fmt.Errorf(msgTemplateErrInsertPRMissing, evalInfo.RepoName, evalInfo.PRNumber, err)
		}
	}
	if parentUUID == "" {
		// we can not ignore an empty parentId, to fail loudly
		return fmt.Errorf(msgTemplateErrInsertPRMissing, evalInfo.RepoName, evalInfo.PRNumber, fmt.Errorf("empty parentUUID"))
	}

	for _, missingAuthor := range evalInfo.UserSignatures {
		var authorUUID string
		err = p.db.QueryRow(sqlInsertUserMissing, parentUUID, missingAuthor.User.Login, missingAuthor.User.Email,
			missingAuthor.User.GivenName, missingAuthor.CLAVersion, checkedAt).Scan(&authorUUID)
		if err != nil {
			if errMsgInsertedRowExists == err.Error() {
				// We ignore lack of insert for cases where a PR is closed and reopened - ON CONFLICT DO NOTHING
				p.logger.Info("special case author not inserted",
					zap.Any("parentUUID", parentUUID),
					zap.Any("user", missingAuthor.User.Login),
					zap.Any("CLAVersion", missingAuthor.CLAVersion),
				)
				// clear the error we are ignoring, so return can be nil
				err = nil
			} else {
				return fmt.Errorf(msgTemplateErrInsertAuthorMissing, missingAuthor.User.Login, err)
			}
		}
	}
	return
}

const sqlSelectPRsForUser = `SELECT DISTINCT * from unsigned_pr, unsigned_user 
WHERE LoginName = $1 AND ClaVersion = $2`

func (p *ClaDB) GetPRsForUser(user *types.UserSignature) (evalInfos []types.EvaluationInfo, err error) {
	var rows *sql.Rows
	if rows, err = p.db.Query(sqlSelectPRsForUser, user.User.Login, user.CLAVersion); err != nil {
		return
	}

	var evalInfo *types.EvaluationInfo
	var foundUser *types.UserSignature
	var foundUserId string
	var duplicatePRId string
	var ignoreTimeChecked time.Time
	for rows.Next() {
		evalInfo = &types.EvaluationInfo{}
		foundUser = &types.UserSignature{}
		err = rows.Scan(
			&evalInfo.UnsignedPRID,
			&evalInfo.RepoOwner,
			&evalInfo.RepoName,
			&evalInfo.Sha,
			&evalInfo.PRNumber,
			&evalInfo.AppId,
			&evalInfo.InstallId,
			&foundUserId,
			&duplicatePRId,
			&foundUser.User.Login,
			&foundUser.User.Email,
			&foundUser.User.GivenName,
			&foundUser.CLAVersion,
			&ignoreTimeChecked, // don't populate TimeSigned, as it is really the timeChecked in the db
		)
		if err != nil {
			return
		}
		evalInfo.UserSignatures = []types.UserSignature{*foundUser}

		p.logger.Debug("found missing author signature",
			zap.String("owner", evalInfo.RepoOwner),
			zap.String("repo", evalInfo.RepoName),
			zap.Int64("pr", evalInfo.PRNumber),
			zap.String("login", evalInfo.UserSignatures[0].User.Login),
			//zap.Time("timeSigned", evalInfo.UserSignatures[0].TimeSigned),
			zap.String("claVersion", evalInfo.UserSignatures[0].CLAVersion),
		)

		evalInfos = append(evalInfos, *evalInfo)
	}
	return
}

const sqlDeleteUnsignedUser = `DELETE FROM unsigned_user 
WHERE UnsignedPRID = $1 AND LoginName = $2 AND ClaVersion = $3`

const SqlSelectUnsignedUsersForPR = `SELECT count(*) from unsigned_pr, unsigned_user
WHERE unsigned_pr.Id = unsigned_user.Id AND unsigned_pr.Id = $1`

const sqlDeleteUnsignedPR = `DELETE FROM unsigned_pr 
WHERE Id = $1`

func (p *ClaDB) RemovePRsForUser(evalInfo *types.EvaluationInfo) (err error) {
	for _, user := range evalInfo.UserSignatures {
		_, err = p.db.Exec(sqlDeleteUnsignedUser, evalInfo.UnsignedPRID, user.User.Login, user.CLAVersion)
		if err != nil {
			return
		}
	}

	//  Verify all unsigned_user rows are deleted, and if so, delete the parent unsigned_pr row
	var countUnsigned int64
	err = p.db.QueryRow(SqlSelectUnsignedUsersForPR, evalInfo.UnsignedPRID).Scan(&countUnsigned)
	if err != nil {
		return
	}

	if countUnsigned == 0 {
		_, err = p.db.Exec(sqlDeleteUnsignedPR, evalInfo.UnsignedPRID)
		if err != nil {
			return
		}
	}
	return
}
