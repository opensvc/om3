package xsession

import (
	"github.com/google/uuid"
	"os"
	"testing"
)

func TestValue(t *testing.T) {

	t.Run("have expected len", func(t *testing.T) {
		id := Id()
		if len(id) != 36 {
			t.Fatalf("Unexpected string len returned by Id(): %q, len %v", id, len(id))
		}
	})

	t.Run("pickup id from OSVC_SESSION_ID if set", func(t *testing.T) {
		varName := "OSVC_SESSION_ID"
		envVarValue, ok := os.LookupEnv(varName)
		if ok {
			defer func() { _ = os.Setenv(varName, envVarValue) }()
		} else {
			defer func() { _ = os.Unsetenv(varName) }()
		}
		expectedValue := "def79ece-b952-4e48-9ec7-23e2ffb47aa7"
		_ = os.Setenv(varName, expectedValue)

		id = ""
		id := Id()
		if id != expectedValue {
			t.Fatalf("Unexpected string len returned by Id(): %q, len %v", id, len(id))
		}
	})

	t.Run("generate valid Id if OSVC_SESSION_ID env var is corrupted", func(t *testing.T) {
		varName := "OSVC_SESSION_ID"
		envVarValue, ok := os.LookupEnv(varName)
		if ok {
			defer func() { _ = os.Setenv(varName, envVarValue) }()
		} else {
			defer func() { _ = os.Unsetenv(varName) }()
		}
		_ = os.Setenv(varName, "bad-uuid")

		id = ""
		retrievedId := Id()
		if _, err := uuid.Parse(retrievedId); err != nil {
			t.Fatalf("Unexpected uuid returned returned by Id(): %q, len %v", retrievedId, len(retrievedId))
		}
	})
}
