package xsession

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_init(t *testing.T) {

	t.Run("default ID is defined with expected len", func(t *testing.T) {
		if len(ID.String()) != 36 {
			t.Fatalf("Unexpected ID value: %q, len %v", ID, len(ID))
		}
	})

	t.Run("ID value is value of env var OSVC_SESSION_ID when defined", func(t *testing.T) {
		origId := ID
		defer func() { ID = origId }()
		varName := "OSVC_SESSION_ID"
		envVarValue, ok := os.LookupEnv(varName)
		if ok {
			defer func() { _ = os.Setenv(varName, envVarValue) }()
		} else {
			defer func() { _ = os.Unsetenv(varName) }()
		}
		expectedValue := "def79ece-b952-4e48-9ec7-23e2ffb47aa7"
		_ = os.Setenv(varName, expectedValue)
		initID()
		assert.Equal(t, expectedValue, ID.String())
	})

	t.Run("invalid env var OSVC_SESSION_ID value are ignored and a valid ID is created", func(t *testing.T) {
		origId := ID
		defer func() { ID = origId }()
		varName := "OSVC_SESSION_ID"
		envVarValue, ok := os.LookupEnv(varName)
		if ok {
			defer func() { _ = os.Setenv(varName, envVarValue) }()
		} else {
			defer func() { _ = os.Unsetenv(varName) }()
		}
		_ = os.Setenv(varName, "bad-uuid")

		initID()
		if len(ID.String()) != 36 {
			t.Fatalf("Unexpected ID value: %q, len %v", ID, len(ID))
		}
	})
}
