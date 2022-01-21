package oauth

import (
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
