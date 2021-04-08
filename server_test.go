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

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

const mockClaText = `mock Cla text.`

func setupMockContextCLA() echo.Context {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, pathClaText, strings.NewReader(mockClaText))
	req.Header.Set(echo.HeaderContentType, echo.MIMETextPlainCharsetUTF8)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c
}

func TestRetrieveCLAText_MissingClaURL(t *testing.T) {
	assert.EqualError(t, retrieveCLAText(setupMockContextCLA()), msgMissingClaUrl)
}

func TestRetrieveCLAText_BadResponseCode(t *testing.T) {
	origClaUrl := os.Getenv(envClsUrl)
	defer func() {
		if origClaUrl == "" {
			assert.NoError(t, os.Unsetenv(envClsUrl))
		} else {
			assert.NoError(t, os.Setenv(envClsUrl, origClaUrl))
		}
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, pathClaText, r.URL.EscapedPath())

		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	assert.NoError(t, os.Setenv(envClsUrl, ts.URL+pathClaText))
	assert.EqualError(t, retrieveCLAText(setupMockContextCLA()), "unexpected cla text response code: 403")
}

func TestRetrieveCLAText(t *testing.T) {
	callCount := 0

	origClaUrl := os.Getenv(envClsUrl)
	defer func() {
		if origClaUrl == "" {
			assert.NoError(t, os.Unsetenv(envClsUrl))
		} else {
			assert.NoError(t, os.Setenv(envClsUrl, origClaUrl))
		}
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, pathClaText, r.URL.EscapedPath())
		callCount += 1

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockClaText))
	}))

	defer ts.Close()

	assert.NoError(t, os.Setenv(envClsUrl, ts.URL+pathClaText))
	assert.NoError(t, retrieveCLAText(setupMockContextCLA()))
	assert.Equal(t, callCount, 1)

	// Ensure that subsequent calls use the cache

	assert.NoError(t, retrieveCLAText(setupMockContextCLA()))
	assert.Equal(t, callCount, 1)
}
