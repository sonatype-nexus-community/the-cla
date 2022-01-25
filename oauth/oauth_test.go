package oauth

import (
	"context"
	"github.com/google/go-github/v42/github"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/oauth2"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func resetEnvVariable(t *testing.T, variableName, originalValue string) {
	if originalValue == "" {
		assert.NoError(t, os.Unsetenv(variableName))
	} else {
		assert.NoError(t, os.Setenv(variableName, originalValue))
	}
}

func TestCreateOAuth(t *testing.T) {
	origGHClientId := os.Getenv(envReactAppGithubClientId)
	defer func() {
		resetEnvVariable(t, envReactAppGithubClientId, origGHClientId)
	}()
	forcedClientId := "myGHClientId"
	assert.NoError(t, os.Setenv(envReactAppGithubClientId, forcedClientId))

	origGHClientSecret := os.Getenv(envGithubClientSecret)
	defer func() {
		resetEnvVariable(t, envGithubClientSecret, origGHClientSecret)
	}()
	forcedGHClientSecret := "myGHClientSecret"
	assert.NoError(t, os.Setenv(envGithubClientSecret, forcedGHClientSecret))

	oauth := CreateOAuth(forcedClientId, forcedGHClientSecret)

	assert.Equal(t, forcedClientId, oauth.getConf().ClientID)
	assert.Equal(t, forcedGHClientSecret, oauth.getConf().ClientSecret)
}

type OAuthMock struct {
	t                *testing.T
	assertParameters bool
	exchangeToken    *oauth2.Token
	exchangeError    error
	getUserLogger    *zap.Logger
	getUserCode      string
	getUserUser      *github.User
	getUserErr       error
}

var _ OAuthInterface = (*OAuthMock)(nil)

// Exchange takes the code and returns a real token.
//goland:noinspection GoUnusedParameter
func (o *OAuthMock) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return o.exchangeToken, o.exchangeError
}

// Client returns a new http.Client.
//goland:noinspection GoUnusedParameter
func (o *OAuthMock) Client(ctx context.Context, t *oauth2.Token) *http.Client {
	return &http.Client{}
}

func (o *OAuthMock) getConf() *oauth2.Config {
	return nil
}

func (o *OAuthMock) GetOAuthUser(logger *zap.Logger, code string) (user *github.User, err error) {
	if o.assertParameters {
		assert.Equal(o.t, o.getUserLogger, logger)
		assert.Equal(o.t, o.getUserCode, code)
	}
	return o.getUserUser, o.getUserErr
}

func setupMockOAuth(t *testing.T, assertParameters bool) (mockOAuth OAuthMock, logger *zap.Logger) {
	logger = zaptest.NewLogger(t)
	mockOAuth = OAuthMock{
		t:                t,
		assertParameters: assertParameters,
	}
	return
}

func TestGetOAuthUser(t *testing.T) {
	oauth, logger := setupMockOAuth(t, true)
	oauth.getUserLogger = logger

	user, err := oauth.GetOAuthUser(logger, "")
	assert.Equal(t, "myUser", user)
	assert.Equal(t, "myErr", err)
}
