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
	githuboauth "golang.org/x/oauth2/github"
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

func TestRetrieveCLATextWithBadURL(t *testing.T) {
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

		// nobody home, be we should not even be knocking on this door - call should not occur
		w.WriteHeader(http.StatusNotFound)
	}))

	defer ts.Close()

	assert.NoError(t, os.Setenv(envClsUrl, "badURLProtocol"+ts.URL+pathClaText))
	assert.Error(t, retrieveCLAText(setupMockContextCLA()), "unsupported protocol scheme \"badurlprotocolhttp\"")
	assert.Equal(t, callCount, 0)
}

func setupMockContextOAuth(queryParams map[string]string) echo.Context {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, pathOAuthCallback, strings.NewReader("mock OAuth stuff"))

	q := req.URL.Query()
	for k, v := range queryParams {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c
}

func TestProcessGitHubOAuthMissingQueryParamState(t *testing.T) {
	assert.NoError(t, processGitHubOAuth(setupMockContextOAuth(map[string]string{})))
}

func TestProcessGitHubOAuthMissingQueryParamCode(t *testing.T) {
	assert.Error(t, processGitHubOAuth(setupMockContextOAuth(map[string]string{
		"state": "testState",
	})))
}

func WIP_TestProcessGitHubOAuthMissingQueryParamClientID(t *testing.T) {
	origOAuthGHTokenURL := githuboauth.Endpoint.TokenURL
	defer func() {
		githuboauth.Endpoint.TokenURL = origOAuthGHTokenURL
	}()
	pathTestOAuthToken := "/login/oauth/access_token"

	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, pathTestOAuthToken, r.URL.EscapedPath())
		callCount += 1

		w.WriteHeader(http.StatusOK)

		//respKeys := map[string]string{
		//	"access_token": "test_access_token",
		//	"token_type": "test_token_type",
		//	"refresh_token": "test_refresh_token",
		//}
		//respKeysBytes, err := json.Marshal(respKeys)
		//assert.NoError(t, err)
		//_, _ = w.Write(respKeysBytes)

		_, _ = w.Write([]byte("access_token=test_access_token&token_type=test_token_type&refresh_token=test_refresh_token"))
	}))
	defer ts.Close()

	githuboauth.Endpoint.TokenURL = ts.URL + pathTestOAuthToken

	assert.Error(t, processGitHubOAuth(setupMockContextOAuth(map[string]string{
		"state": "testState",
		"code":  "testCode",
	})))
}
