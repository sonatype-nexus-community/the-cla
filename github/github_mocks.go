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

package github

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/google/go-github/v64/github"
	"github.com/stretchr/testify/assert"
)

// RepositoriesMock mocks RepositoriesService
type RepositoriesMock struct {
	t *testing.T
	/* callCount is the number of times the CreateStatus function has been called by production code. */
	callCount int
	/* assertParameters is a slice of booleans that determine whether to assert the parameters passed to the function for
	each call to the CreateStatus function. If the value at the index of the callCount is true, the parameters will be
	asserted.
	*/
	assertParameters                                       []bool
	expectedCtx                                            []context.Context
	expectedOwner, expectedRepo, expectedRef, expectedUser []string
	// expectedOpts                                           *github.ListOptions
	expectedCreateStatusRepoStatus []*github.RepoStatus
	createStatusRepoStatus         []*github.RepoStatus
	createStatusResponse           []*github.Response
	createStatusError              []error
	isCollaboratorResult           bool
	isCollaboratorResp             *github.Response
	isCollaboratorErr              error
}

var _ RepositoriesService = (*RepositoriesMock)(nil)

func setupMockRepositoriesService(t *testing.T, assertParameters []bool) (mock *RepositoriesMock) {
	mock = &RepositoriesMock{
		t:                t,
		assertParameters: assertParameters,
	}
	return mock
}

//goland:noinspection GoUnusedParameter
func (r *RepositoriesMock) ListStatuses(ctx context.Context, owner, repo, ref string, opts *github.ListOptions) ([]*github.RepoStatus, *github.Response, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RepositoriesMock) CreateStatus(ctx context.Context, owner, repo, ref string, status *github.RepoStatus) (retRepoStatus *github.RepoStatus, createStatusResponse *github.Response, createStatusError error) {
	defer func() { r.callCount++ }()
	if r.assertParameters != nil && r.assertParameters[r.callCount] {
		if r.expectedCtx != nil {
			assert.Equal(r.t, r.expectedCtx[r.callCount], ctx)
		}
		if r.expectedOwner != nil {
			assert.Equal(r.t, r.expectedOwner[r.callCount], owner)
		}
		if r.expectedRepo != nil {
			assert.Equal(r.t, r.expectedRepo[r.callCount], repo)
		}
		if r.expectedRef != nil {
			assert.Equal(r.t, r.expectedRef[r.callCount], ref)
		}
		if r.expectedCreateStatusRepoStatus != nil {
			assert.Equal(r.t, r.expectedCreateStatusRepoStatus[r.callCount], status)
		}
	}

	if r.createStatusRepoStatus != nil {
		retRepoStatus = r.createStatusRepoStatus[r.callCount]
	}

	if r.createStatusRepoStatus != nil {
		createStatusResponse = r.createStatusResponse[r.callCount]
	}

	if r.createStatusError != nil {
		createStatusError = r.createStatusError[r.callCount]
	}
	return
}

// Get returns a repository.
func (r *RepositoriesMock) Get(context.Context, string, string) (*github.Repository, *github.Response, error) {
	return &github.Repository{
		ID:              github.Int64(185409993),
		Name:            github.String("wayne"),
		Description:     github.String("some description"),
		Language:        github.String("JavaScript"),
		StargazersCount: github.Int(3141),
		HTMLURL:         github.String("https://www.foo.com"),
		FullName:        github.String("john/wayne"),
	}, nil, nil
}

func (r *RepositoriesMock) IsCollaborator(ctx context.Context, owner, repo, user string) (bool, *github.Response, error) {
	if r.assertParameters != nil && r.assertParameters[r.callCount] {
		assert.Equal(r.t, r.expectedCtx[r.callCount], ctx)
		assert.Equal(r.t, r.expectedOwner[r.callCount], owner)
		assert.Equal(r.t, r.expectedRepo[r.callCount], repo)
		assert.Equal(r.t, r.expectedUser[r.callCount], user)
	}
	return r.isCollaboratorResult, r.isCollaboratorResp, r.isCollaboratorErr
}

// UsersMock mocks UsersService
type UsersMock struct {
	mockUser     *github.User
	mockResponse *github.Response
	mockGetError error
}

var _ UsersService = (*UsersMock)(nil)

// Get returns a user.
func (u *UsersMock) Get(context.Context, string) (*github.User, *github.Response, error) {
	return u.mockUser, u.mockResponse, u.mockGetError
}

// PullRequestsMock mocks PullRequestsService
type PullRequestsMock struct {
	mockRepositoryCommits []*github.RepositoryCommit
	mockResponse          *github.Response
	mockListCommitsError  error
}

var _ PullRequestsService = (*PullRequestsMock)(nil)

//goland:noinspection GoUnusedParameter
func (p *PullRequestsMock) ListCommits(ctx context.Context, owner string, repo string, number int, opts *github.ListOptions) ([]*github.RepositoryCommit, *github.Response, error) {
	return p.mockRepositoryCommits, p.mockResponse, p.mockListCommitsError
}

type IssuesMock struct {
	mockGetLabel                  *github.Label
	MockGetLabelResponse          *github.Response
	mockGetLabelError             error
	mockListLabelsByIssue         []*github.Label
	mockListLabelsByIssueResponse *github.Response
	mockListLabelsByIssueError    error
	mockCreateLabel               *github.Label
	mockCreateLabelResponse       *github.Response
	mockCreateLabelError          error
	mockAddLabels                 []*github.Label
	mockAddLabelsResponse         *github.Response
	mockAddLabelsError            error
	MockRemoveLabelResponse       *github.Response
	mockRemoveLabelError          error
	mockComment                   *github.IssueComment
	mockCreateCommentResponse     *github.Response
	mockCreateCommentError        error
	mockListComments              []*github.IssueComment
	mockListCommentsResponse      *github.Response
	mockListCommentsError         error
}

var _ IssuesService = (*IssuesMock)(nil)

//goland:noinspection GoUnusedParameter
func (i *IssuesMock) GetLabel(ctx context.Context, owner string, repo string, labelName string) (*github.Label, *github.Response, error) {
	return i.mockGetLabel, i.MockGetLabelResponse, i.mockGetLabelError
}

//goland:noinspection GoUnusedParameter
func (i *IssuesMock) CreateLabel(ctx context.Context, owner string, repo string, label *github.Label) (*github.Label, *github.Response, error) {
	return i.mockCreateLabel, i.mockCreateLabelResponse, i.mockCreateLabelError
}

//goland:noinspection GoUnusedParameter
func (i *IssuesMock) ListLabelsByIssue(ctx context.Context, owner string, repo string, issueNumber int, opts *github.ListOptions) ([]*github.Label, *github.Response, error) {
	return i.mockListLabelsByIssue, i.mockListLabelsByIssueResponse, i.mockListLabelsByIssueError
}

//goland:noinspection GoUnusedParameter
func (i *IssuesMock) AddLabelsToIssue(ctx context.Context, owner string, repo string, number int, labels []string) ([]*github.Label, *github.Response, error) {
	return i.mockAddLabels, i.mockAddLabelsResponse, i.mockAddLabelsError
}

//goland:noinspection GoUnusedParameter
func (i *IssuesMock) RemoveLabelForIssue(ctx context.Context, owner string, repo string, number int, label string) (*github.Response, error) {
	return i.MockRemoveLabelResponse, i.mockRemoveLabelError
}

//goland:noinspection GoUnusedParameter
func (i *IssuesMock) CreateComment(ctx context.Context, owner string, repo string, number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error) {
	return i.mockComment, i.mockCreateCommentResponse, i.mockCreateCommentError
}

//goland:noinspection GoUnusedParameter
func (i *IssuesMock) ListComments(ctx context.Context, owner string, repo string, number int, opts *github.IssueListCommentsOptions) ([]*github.IssueComment, *github.Response, error) {
	return i.mockListComments, i.mockListCommentsResponse, i.mockListCommentsError
}

type AppsMock struct {
	mockApp               *github.App
	mockAppResp           *github.Response
	mockAppErr            error
	mockInstallation      *github.Installation
	mockInstallationResp  *github.Response
	mockInstallationError error
}

var _ AppsService = (*AppsMock)(nil)

//goland:noinspection GoUnusedParameter
func (a *AppsMock) Get(ctx context.Context, appSlug string) (*github.App, *github.Response, error) {
	return a.mockApp, a.mockAppResp, a.mockAppErr
}

//goland:noinspection GoUnusedParameter
func (a *AppsMock) GetInstallation(ctx context.Context, id int64) (*github.Installation, *github.Response, error) {
	return a.mockInstallation, a.mockInstallationResp, a.mockInstallationError
}

var MockAppSlug = "myAppSlug"

func SetupMockGHJWT() (resetImpl func()) {
	origGHJWT := GHJWTImpl
	resetImpl = func() {
		GHJWTImpl = origGHJWT
	}
	GHJWTImpl = &GHJWTMock{
		AppsMock: AppsMock{
			mockInstallation: &github.Installation{
				AppSlug: &MockAppSlug,
			},
		},
	}
	return
}

type GHJWTMock struct {
	AppsMock AppsMock
}

var _ GHJWTInterface = (*GHJWTMock)(nil)

//goland:noinspection GoUnusedParameter
func (gj *GHJWTMock) NewJWTClient(httpClient *http.Client, installID int64) IGitHubJWTClient {
	return &GHJWTClient{
		installID: installID,
		apps:      &gj.AppsMock,
	}
}

// GHInterfaceMock implements GHInterface.
type GHInterfaceMock struct {
	RepositoriesMock RepositoriesMock
	UsersMock        UsersMock
	PullRequestsMock PullRequestsMock
	IssuesMock       IssuesMock
}

var _ GHInterface = (*GHInterfaceMock)(nil)

// NewClient something
//
//goland:noinspection GoUnusedParameter
func (g *GHInterfaceMock) NewClient(httpClient *http.Client) GHClient {
	return GHClient{
		Repositories: &g.RepositoriesMock,
		Users: &UsersMock{
			mockGetError: g.UsersMock.mockGetError,
			mockUser:     g.UsersMock.mockUser,
			mockResponse: g.UsersMock.mockResponse,
		},
		PullRequests: &PullRequestsMock{
			mockListCommitsError:  g.PullRequestsMock.mockListCommitsError,
			mockRepositoryCommits: g.PullRequestsMock.mockRepositoryCommits,
			mockResponse:          g.PullRequestsMock.mockResponse,
		},
		Issues: &IssuesMock{
			mockGetLabel:                  g.IssuesMock.mockGetLabel,
			MockGetLabelResponse:          g.IssuesMock.MockGetLabelResponse,
			mockGetLabelError:             g.IssuesMock.mockGetLabelError,
			mockListLabelsByIssue:         g.IssuesMock.mockListLabelsByIssue,
			mockListLabelsByIssueResponse: g.IssuesMock.mockListLabelsByIssueResponse,
			mockListLabelsByIssueError:    g.IssuesMock.mockListLabelsByIssueError,
			mockCreateLabel:               g.IssuesMock.mockCreateLabel,
			mockCreateLabelResponse:       g.IssuesMock.mockCreateLabelResponse,
			mockCreateLabelError:          g.IssuesMock.mockCreateLabelError,
			mockAddLabels:                 g.IssuesMock.mockAddLabels,
			mockAddLabelsResponse:         g.IssuesMock.mockAddLabelsResponse,
			mockAddLabelsError:            g.IssuesMock.mockAddLabelsError,
			MockRemoveLabelResponse:       g.IssuesMock.MockRemoveLabelResponse,
			mockRemoveLabelError:          g.IssuesMock.mockRemoveLabelError,
		},
	}
}

// generated via: openssl genpkey -algorithm RSA  -outform PEM -out private_key.pem -pkeyopt rsa_keygen_bits:2048
const testPrivatePem = `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDtQtWKdnW9OKJk
XuSx45oixrJqWqpaly23iXvAAcTqg+pFD7Yw1bL9viAYoc7ATcd6Uonz7/d6RugO
JuozsC4X1xYotEWYlB7tKrp+InQ2H0fRC6afGiCdDUgLINfmqShPWgGft4cA7mwH
JSHB6XAGwVsZsxqYIi4wXVPYYJaI3OX5nA/BiRvZMrsaF2PT8dt/5rptMIXxXlwK
tuQVvICxh5CXn5/FaeQcnkXoDESoZcG9nhqSmRdeUJxoiGZ7epVljj7Ef5XKJYoz
uv8vJVTVXwxb7MbcjQ6Zna4iJj4FscwkQyaoFQOzBf+1H5ypZ8CFn/E236tLpwh0
7Xspu5CrAgMBAAECggEBAOd51CKBjj8s+OpZ1l9jgea52il/CULWyciNvolGcJqo
VrBIMuUUKMv8aQ3/F1pwx9QkoOi4TsciVJYyCz6gfWfO9ZSCxH+my0Fx9X7IGH8R
J5zg9A+3iugOpCIPSfSFRomcc4cio/kZo5WY+YVZPW2pyTqajbCtcEjJVNr+6P7e
PAWKI6RXbwGa4Fp8dLHMRq+/i2zuznEzdrTJPBSoW5HUMDvPixhjd+WeYT9pNfZP
P8V2HhSt1qvuVM/epZ8llnmyPaw7ojwAOurG19fDGUvEfjAORYJopOvxeJ1mCY++
HVxcumbx4N2D8IQ/dwbtarMBLpw89GQztxCxokJ7a5ECgYEA/QFTsgQKFQbdlv1z
ooBq3EZPfzebx4mkyCcLmQAliSArJezRewCyelP2A102p5125SMEA1vcsSkZOes8
h4z4HaptHZob1OxG2EBNdOzY41TaG1nzbOAJEkF71ksT30dpaLRCECUfcEWc0waB
cwia1v1xUvfcvwhPJIdzye5V7hkCgYEA8BHMYRfvIMtRgHNPoFNoRxr6BU/gjfV/
FRJLNdMSk3KYve459XGPFvLSAh0eucOVjmkZY8y0BJJdeFVdTjPa2nvk70i9yhGk
MhjVHs1Y7VIRYB6SSoA7hPK3zMELTbMudZS1/Dxe8fCc1/oDhamLAcT1474hXIR2
AYe8T97qBWMCgYA77yWJhSVyR7cUfqP2+d7WoZ1RcLXpdfTgKUe5DezWaBVwnYIe
VlLxYZRkxZ8d49J3g2z+8rL8ENVWACDNp5pbRLUmjwxKy1IZBlqS+UyDxeUJF6zv
vL7JYVPZtt1VRlB1KkaAFps0+HinEOJ3grFTfqRq2Cal5m0BJUlLq7cVeQKBgHLB
Hz/+L9kuNxw+gn5xwDPVClRFtWJGSmPpJbhp18RRj/+iA2R2zt46XfaSsuA7RJ8Z
UACrlhVlXXaq33oFQYUUmf9jdw1DV4h25FDf+bUfeJzIoEcqesj3OLKQSHXww7GC
z2bt+LiPunlm0g4vV/oVizA87zeJPdtHZdWMCbNfAoGBALEVP1RXKsI9M7R01ML5
cocpE9qF81DkPzYsQxDRnheFNE9GOK2snADOiXa/ObvzQ5g57FJ7sJVkm2YECI9N
pNEMHXmW70G0upWmOnjZL6WxXcJjbpZ94SOFiD7GFFLgWs9bI4BdxMDX/EyXQafy
Scy7y5rzNperE0E7Xy1N10NX
-----END PRIVATE KEY-----`

func SetupTestPemFile(t *testing.T) (resetImpl func()) {
	// move pem file if it exists
	pemBackupFile := FilenameTheClaPem + "_orig"
	errRename := os.Rename(FilenameTheClaPem, pemBackupFile)
	resetImpl = func() {
		assert.NoError(t, os.Remove(FilenameTheClaPem))
		if errRename == nil {
			assert.NoError(t, os.Rename(pemBackupFile, FilenameTheClaPem), "error renaming pem file in test")
		}
	}

	assert.NoError(t, os.WriteFile(FilenameTheClaPem, []byte(testPrivatePem), 0644))

	return resetImpl
}

func resetEnvVariable(t *testing.T, variableName, originalValue string) {
	if originalValue == "" {
		assert.NoError(t, os.Unsetenv(variableName))
	} else {
		assert.NoError(t, os.Setenv(variableName, originalValue))
	}
}
