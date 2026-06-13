package xsession

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func Test_init(t *testing.T) {

	t.Run("default ID is defined with expected len", func(t *testing.T) {
		s := Sid().String()
		if len(s) != 36 {
			t.Fatalf("Unexpected id string len: %q, len %v", s, len(s))
		}
	})

	t.Run("ID value is value of env var OSVC_SESSION_ID when defined", func(t *testing.T) {
		origSid := Sid()
		defer func() { ResetSid(*origSid) }()
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
		assert.Equal(t, expectedValue, Sid().String())
	})

	t.Run("invalid env var OSVC_SESSION_ID value are ignored and a valid ID is created", func(t *testing.T) {
		origSid := Sid()
		defer func() { ResetSid(*origSid) }()
		varName := "OSVC_SESSION_ID"
		envVarValue, ok := os.LookupEnv(varName)
		if ok {
			defer func() { _ = os.Setenv(varName, envVarValue) }()
		} else {
			defer func() { _ = os.Unsetenv(varName) }()
		}
		_ = os.Setenv(varName, "bad-uuid")

		initID()
		s := Sid().String()
		if len(s) != 36 {
			t.Fatalf("Unexpected ID value: %q, len %v", s, len(s))
		}
	})
}

func TestSessionID_MarshalJSON(t *testing.T) {
	t.Run("marshal SessionID to JSON", func(t *testing.T) {
		sid := NewSid()
		b, err := json.Marshal(sid)
		require.NoError(t, err)
		// Should be a quoted string
		assert.JSONEq(t, `"`+sid.String()+`"`, string(b))
	})
}

func TestSessionID_UnmarshalJSON(t *testing.T) {
	t.Run("unmarshal JSON string to SessionID", func(t *testing.T) {
		expectedUUID := "def79ece-b952-4e48-9ec7-23e2ffb47aa7"
		jsonStr := `"` + expectedUUID + `"`
		var sid Id
		err := json.Unmarshal([]byte(jsonStr), &sid)
		require.NoError(t, err)
		assert.Equal(t, expectedUUID, sid.String())
	})

	t.Run("unmarshal invalid UUID returns error", func(t *testing.T) {
		jsonStr := `"invalid-uuid"`
		var sid Id
		err := json.Unmarshal([]byte(jsonStr), &sid)
		require.Error(t, err)
	})

	t.Run("unmarshal invalid JSON returns error", func(t *testing.T) {
		jsonStr := `{not valid json}`
		var sid Id
		err := json.Unmarshal([]byte(jsonStr), &sid)
		require.Error(t, err)
	})
}

func TestSessionID_MarshalUnmarshalJSON_Roundtrip(t *testing.T) {
	t.Run("marshal and unmarshal preserves value", func(t *testing.T) {
		original := NewSid()
		b, err := json.Marshal(original)
		require.NoError(t, err)
		var unmarshaled Id
		err = json.Unmarshal(b, &unmarshaled)
		require.NoError(t, err)
		assert.Equal(t, original.String(), unmarshaled.String())
	})
}

func TestSessionID_MarshalYAML(t *testing.T) {
	t.Run("marshal SessionID to YAML", func(t *testing.T) {
		sid := NewSid()
		b, err := yaml.Marshal(sid)
		require.NoError(t, err)
		// YAML should marshal as a plain string
		assert.Equal(t, sid.String()+"\n", string(b))
	})
}

func TestSessionID_UnmarshalYAML(t *testing.T) {
	t.Run("unmarshal YAML string to SessionID", func(t *testing.T) {
		expectedUUID := "def79ece-b952-4e48-9ec7-23e2ffb47aa7"
		yamlStr := expectedUUID
		var sid Id
		err := yaml.Unmarshal([]byte(yamlStr), &sid)
		require.NoError(t, err)
		assert.Equal(t, expectedUUID, sid.String())
	})

	t.Run("unmarshal invalid UUID returns error", func(t *testing.T) {
		yamlStr := "invalid-uuid"
		var sid Id
		err := yaml.Unmarshal([]byte(yamlStr), &sid)
		require.Error(t, err)
	})
}

func TestSessionID_MarshalUnmarshalYAML_Roundtrip(t *testing.T) {
	t.Run("marshal and unmarshal preserves value", func(t *testing.T) {
		original := NewSid()
		b, err := yaml.Marshal(original)
		require.NoError(t, err)
		var unmarshaled Id
		err = yaml.Unmarshal(b, &unmarshaled)
		require.NoError(t, err)
		assert.Equal(t, original.String(), unmarshaled.String())
	})
}
