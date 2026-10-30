package object

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ai.agent_url takes an https origin: another value is refused when written,
// not when om3-webapp calls it.
func TestValidAIAgentURL(t *testing.T) {
	for _, s := range []string{"https://ai.example.com:8090", "https://[2001:db8::1]:8090/"} {
		assert.NoError(t, validAIAgentURL(s), s)
	}
	for _, s := range []string{"http://ai.example.com:8090", "http://127.0.0.1:8090", "https://ai.example.com:8090/v1", "https://user@ai.example.com", "ai.example.com:8090"} {
		assert.Error(t, validAIAgentURL(s), s)
	}
}

// A refused value is an error alert of the validation, which refuses the
// write of the configuration.
func TestClusterConfigValidateAIAgentURL(t *testing.T) {
	for value, refused := range map[string]bool{
		"https://ai.example.com:8090": false,
		"http://ai.example.com:8090":  true,
	} {
		o, err := NewCluster(WithConfigData([]byte("[ai]\nagent_url = "+value+"\n")), WithVolatile(true))
		require.NoError(t, err, value)
		alerts, err := o.Config().Validate()
		require.NoError(t, err, value)
		assert.Equal(t, refused, alerts.HasError(), "%s: %s", value, alerts)
		if refused {
			assert.Contains(t, alerts.String(), "agent_url", value)
		}
	}
}
