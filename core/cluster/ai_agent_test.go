package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAIAgentURL(t *testing.T) {
	for value, want := range map[string]string{
		"https://ai.opensvc.com:8090":  "https://ai.opensvc.com:8090",
		"https://ai.opensvc.com:8090/": "https://ai.opensvc.com:8090",
		"https://[2001:db8::1]:8090":   "https://[2001:db8::1]:8090",
		" https://ai.opensvc.com ":     "https://ai.opensvc.com",
	} {
		u, err := ParseAIAgentURL(value)
		if assert.NoError(t, err, value) {
			assert.Equal(t, want, u.String(), value)
		}
	}
	for _, value := range []string{
		"http://ai.opensvc.com:8090",
		"http://127.0.0.1:8090",
		"https://u:p@ai.opensvc.com",
		"https://ai.opensvc.com/v1",
		"https://ai.opensvc.com?x=1",
		"https://ai.opensvc.com#x",
		"https://",
		"ai.opensvc.com:8090",
		"ftp://ai.opensvc.com",
	} {
		_, err := ParseAIAgentURL(value)
		assert.Error(t, err, value)
	}
}
