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
// +build go1.16

package db

import (
	"database/sql"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/labstack/echo/v4"
	"github.com/sonatype-nexus-community/the-cla/types"
)

const sqlInsertSignature = `INSERT INTO signatures
		(LoginName, Email, GivenName, SignedAt, ClaVersion)
		VALUES ($1, $2, $3, $4, $5)`

const msgTemplateErrInsertSignatureDuplicate = "insert error. did user previously sign the cla? user: %+v, error: %+v"

type IClaDB interface {
	InsertSignature(u *types.UserSignature) error
	HasAuthorSignedTheCla(l, c string) (bool, error)
	MigrateDB() error
}

type ClaDB struct {
	db     *sql.DB
	logger echo.Logger
}

// Roll that beautiful bean footage
var _ IClaDB = (*ClaDB)(nil)

func New(db *sql.DB, logger echo.Logger) *ClaDB {
	return &ClaDB{db: db, logger: logger}
}

func (p *ClaDB) InsertSignature(user *types.UserSignature) error {
	_, err := p.db.Exec(sqlInsertSignature, user.User.Login, user.User.Email, user.User.GivenName, user.TimeSigned, user.CLAVersion)
	if err != nil {
		errWithDetails := fmt.Errorf(msgTemplateErrInsertSignatureDuplicate, user.User, err)

		return errWithDetails
	}
	return nil
}

const sqlSelectUserSignature = `SELECT 
		LoginName, Email, GivenName, SignedAt, ClaVersion 
		FROM signatures		
		WHERE LoginName = $1
		AND ClaVersion = $2`

func (p *ClaDB) HasAuthorSignedTheCla(login, claVersion string) (bool, error) {
	p.logger.Debug("Checking to see if author signed the CLA")
	p.logger.Debug(login)

	rows, err := p.db.Query(sqlSelectUserSignature, login, claVersion)
	if err != nil {
		return false, err
	}

	var foundUserSignature types.UserSignature
	isSigned := false
	for rows.Next() {
		isSigned = true
		foundUserSignature = types.UserSignature{}
		err = rows.Scan(
			&foundUserSignature.User.Login,
			&foundUserSignature.User.Email,
			&foundUserSignature.User.GivenName,
			&foundUserSignature.TimeSigned,
			&foundUserSignature.CLAVersion,
		)
		if err != nil {
			return isSigned, err
		}
		p.logger.Debugf("Found user signature for author: %s, TimeSigned: %s, CLAVersion: %s",
			foundUserSignature.User.Login, foundUserSignature.TimeSigned, foundUserSignature.CLAVersion)
	}

	return isSigned, nil
}

func (p *ClaDB) MigrateDB() (err error) {
	driver, err := postgres.WithInstance(p.db, &postgres.Config{})
	if err != nil {
		return
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://db/migrations",
		"postgres", driver)

	if err != nil {
		return
	}

	if err = m.Up(); err != nil {
		return
	}

	return
}
