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
	"encoding/base64"
	"os"
	"testing"

	"github.com/google/go-github/v72/github"
	"github.com/stretchr/testify/assert"
)

func setTestEnv(t *testing.T, key, value string) {
	t.Helper()
	orig, wasSet := os.LookupEnv(key)
	t.Cleanup(func() {
		if wasSet {
			assert.NoError(t, os.Setenv(key, orig))
		} else {
			assert.NoError(t, os.Unsetenv(key))
		}
	})
	assert.NoError(t, os.Setenv(key, value))
}

func TestGetPrivateKeyFromEnvPem(t *testing.T) {
	setTestEnv(t, envClaPem, testPrivatePem)
	key, err := getPrivateKey()
	assert.NoError(t, err)
	assert.Equal(t, testPrivatePem, string(key))
}

func TestGetPrivateKeyFromEnvPemBase64(t *testing.T) {
	setTestEnv(t, envClaPem, base64.StdEncoding.EncodeToString([]byte(testPrivatePem)))
	key, err := getPrivateKey()
	assert.NoError(t, err)
	assert.Equal(t, testPrivatePem, string(key))
}

func TestGetPrivateKeyFromEnvPemInvalidBase64(t *testing.T) {
	setTestEnv(t, envClaPem, "not base64 !!")
	key, err := getPrivateKey()
	assert.Nil(t, key)
	assert.ErrorContains(t, err, "could not decode private key from CLA_PEM")
}

func TestGetPrivateKeyEnvPemTakesPrecedenceOverFile(t *testing.T) {
	setTestEnv(t, envClaPem, testPrivatePem)
	orig := FilenameTheClaPem
	defer func() { FilenameTheClaPem = orig }()
	FilenameTheClaPem = "does-not-exist.pem"
	key, err := getPrivateKey()
	assert.NoError(t, err)
	assert.Equal(t, testPrivatePem, string(key))
}

func TestGetPrivateKeyFromFile(t *testing.T) {
	setTestEnv(t, envClaPem, "")
	orig := FilenameTheClaPem
	defer func() { FilenameTheClaPem = orig }()
	FilenameTheClaPem = t.TempDir() + "/key.pem"
	assert.NoError(t, os.WriteFile(FilenameTheClaPem, []byte(testPrivatePem), 0600))
	key, err := getPrivateKey()
	assert.NoError(t, err)
	assert.Equal(t, testPrivatePem, string(key))
}

func TestGetPrivateKeyFileMissing(t *testing.T) {
	setTestEnv(t, envClaPem, "")
	orig := FilenameTheClaPem
	defer func() { FilenameTheClaPem = orig }()
	FilenameTheClaPem = "does-not-exist.pem"
	key, err := getPrivateKey()
	assert.Nil(t, key)
	assert.EqualError(t, err, "could not read private key: open does-not-exist.pem: no such file or directory")
}

func TestStripTrailingLinkTarget(t *testing.T) {
	assert.Equal(t, stripTrailingLinkTarget("a [b](https://one.example/)"), stripTrailingLinkTarget("a [b](https://two.example/x?y=1)\n"))
	assert.NotEqual(t, stripTrailingLinkTarget("a [b](https://one.example/)"), stripTrailingLinkTarget("c [b](https://one.example/)"))
	assert.Equal(t, "no link here", stripTrailingLinkTarget("no link here"))
}

func commentsWith(bodies ...string) []*github.IssueComment {
	var comments []*github.IssueComment
	for i := range bodies {
		comments = append(comments, &github.IssueComment{Body: &bodies[i]})
	}
	return comments
}

func TestAddCommentMatchingSkipsCommentThatOnlyDiffersByLinkTarget(t *testing.T) {
	existing := "Thanks for the contribution. Before we can merge this, we need someone to [sign the Contributor License Agreement](https://old.example/)"
	issues := &IssuesMock{mockListComments: commentsWith(existing), mockComment: &github.IssueComment{}}
	message := "Thanks for the contribution. Before we can merge this, we need someone to [sign the Contributor License Agreement](https://new.example/)"

	created, err := addCommentToIssueIfNotExistsMatching(issues, "o", "r", 1, message, stripTrailingLinkTarget)
	assert.NoError(t, err)
	assert.Nil(t, created)
}

func TestAddCommentMatchingPostsWhenTextDiffers(t *testing.T) {
	existing := "Thanks for the contribution. Before we can merge this, we need someone else to [sign the Contributor License Agreement](https://old.example/)"
	issues := &IssuesMock{mockListComments: commentsWith(existing), mockComment: &github.IssueComment{}}
	message := "Thanks for the contribution. Before we can merge this, we need someone to [sign the Contributor License Agreement](https://new.example/)"

	created, err := addCommentToIssueIfNotExistsMatching(issues, "o", "r", 1, message, stripTrailingLinkTarget)
	assert.NoError(t, err)
	assert.NotNil(t, created)
}

func TestAddCommentIfNotExistsStillExactMatch(t *testing.T) {
	existing := "Thanks for the contribution. Before we can merge this, we need someone to [sign the Contributor License Agreement](https://old.example/)"
	issues := &IssuesMock{mockListComments: commentsWith(existing), mockComment: &github.IssueComment{}}

	created, err := addCommentToIssueIfNotExists(issues, "o", "r", 1, existing)
	assert.NoError(t, err)
	assert.Nil(t, created)

	created, err = addCommentToIssueIfNotExists(issues, "o", "r", 1, existing+" changed")
	assert.NoError(t, err)
	assert.NotNil(t, created)
}
