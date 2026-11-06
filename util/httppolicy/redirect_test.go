package httppolicy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The target of a redirect is held to the cidr lists, not to the url lists,
// which judge the url a configuration names.
func TestCheckRedirect(t *testing.T) {
	p := New([]string{"https://example.com/*"}, []string{"*"}, nil, []string{"127.0.0.0/8", "169.254.0.0/16"})

	_, _, err := p.Check("http://8.8.8.8/x")
	assert.Error(t, err, "a url not allowed")

	_, port, err := p.CheckRedirect("http://8.8.8.8/x")
	require.NoError(t, err, "a redirect target is not held to the url lists")
	assert.Equal(t, "80", port)

	_, _, err = p.CheckRedirect("http://169.254.169.254/latest/meta-data/")
	assert.Error(t, err, "a blocked address, whatever redirects to it")

	_, _, err = p.CheckRedirect("http://127.0.0.1:8080/")
	assert.Error(t, err, "a blocked address, whatever redirects to it")
}
